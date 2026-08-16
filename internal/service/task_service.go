package service

import (
	"context"
	"fmt"
	"time"

	"patrol-platform/internal/config"
	"patrol-platform/internal/domain"
	"patrol-platform/internal/store"
)

// ClockInRequest carries the data needed to record a key-point clock-in.
type ClockInRequest struct {
	TaskID      string
	KeyPointID  string
	PatrollerID string
	Lat         float64
	Lng         float64
	PhotoRef    string
	RequestID   string
	Timestamp   time.Time
	Offline     bool
}

// TrackRequest carries a single GPS track reading from a terminal.
type TrackRequest struct {
	TaskID     string
	TerminalID string
	Lat        float64
	Lng        float64
	Sequence   int
	Timestamp  time.Time
}

// TaskService orchestrates the patrol task lifecycle: dispatch, claim,
// start, clock-in, track recording, completion, and timeout rollback.
type TaskService struct {
	store *store.Store
	cfg   *config.Config
	clock Clock
}

// NewTaskService creates a task service. If clock is nil, RealClock is used.
func NewTaskService(s *store.Store, cfg *config.Config, clock Clock) *TaskService {
	if clock == nil {
		clock = RealClock{}
	}
	return &TaskService{store: s, cfg: cfg, clock: clock}
}

// Dispatch creates a task and places it in the assigned state for a grid.
// Core-zone grids automatically require dual patrol.
func (s *TaskService) Dispatch(ctx context.Context, dispatcherID, gridID, title string, priority domain.Priority, keyPoints []domain.KeyPoint) (*domain.Task, error) {
	grid, err := s.store.GetGrid(gridID)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	task := &domain.Task{
		ID:           domain.NewID("task"),
		GridID:       gridID,
		Title:        title,
		DispatcherID: dispatcherID,
		Status:       domain.TaskAssigned,
		Priority:     priority,
		RequireDual:  grid.Zone == domain.ZoneCore,
		KeyPoints:    keyPoints,
		CreatedAt:    now,
		DispatchedAt: now,
		Version:      1,
	}
	task.AddAudit("dispatch", dispatcherID, fmt.Sprintf("dispatched to grid %s (%s)", gridID, grid.Zone), now)
	s.store.SaveTask(task)
	return task, nil
}

// Claim lets a patroller claim a task. When two patrollers race, the first
// to acquire the store write lock wins; the loser receives ErrAlreadyClaimed.
func (s *TaskService) Claim(ctx context.Context, taskID, patrollerID string) (*domain.Task, error) {
	task, err := s.store.GetTask(taskID)
	if err != nil {
		return nil, err
	}
	grid, err := s.store.GetGrid(task.GridID)
	if err != nil {
		return nil, err
	}
	if !grid.HasPatroller(patrollerID) {
		return nil, fmt.Errorf("%w: patroller %s not in grid %s", domain.ErrNotAuthorized, patrollerID, grid.ID)
	}
	return s.store.UpdateTask(taskID, func(t *domain.Task) error {
		if err := t.CanClaim(patrollerID, nil); err != nil {
			return err
		}
		t.Claim(patrollerID, s.clock.Now())
		return nil
	})
}

// PreReport files the advance report required for dual-patrol core-zone tasks.
func (s *TaskService) PreReport(ctx context.Context, taskID, reporterID, partnerID string) (*domain.Task, error) {
	return s.store.UpdateTask(taskID, func(t *domain.Task) error {
		return t.PreReport(reporterID, partnerID, s.clock.Now())
	})
}

// Start transitions a claimed task to in_progress, enforcing dual-patrol rules.
func (s *TaskService) Start(ctx context.Context, taskID, patrollerID string) (*domain.Task, error) {
	return s.store.UpdateTask(taskID, func(t *domain.Task) error {
		if err := t.CanStart(patrollerID); err != nil {
			return err
		}
		t.Start(s.clock.Now())
		return nil
	})
}

// RecordClockIn records a key-point clock-in. Duplicate request IDs are
// idempotent — the existing entry is returned without error. The six-hour
// gap rule is enforced; violations are flagged but still recorded.
func (s *TaskService) RecordClockIn(ctx context.Context, req ClockInRequest) (*domain.ClockIn, error) {
	now := s.clock.Now()
	ts := req.Timestamp
	if ts.IsZero() {
		ts = now
	}
	if req.RequestID == "" {
		req.RequestID = domain.NewID("clockin")
	}
	ci := domain.ClockIn{
		RequestID:   req.RequestID,
		TaskID:      req.TaskID,
		KeyPointID:  req.KeyPointID,
		PatrollerID: req.PatrollerID,
		Lat:         req.Lat,
		Lng:         req.Lng,
		PhotoRef:    req.PhotoRef,
		Timestamp:   ts,
		Offline:     req.Offline,
	}
	updated, err := s.store.UpdateTask(req.TaskID, func(t *domain.Task) error {
		if t.Status != domain.TaskInProgress {
			return fmt.Errorf("%w: task is %s, not in progress", domain.ErrInvalidTransition, t.Status)
		}
		if t.HasClockIn(ci.RequestID) {
			return nil
		}
		t.AddClockIn(ci, s.cfg.ClockInMaxGap, now)
		return nil
	})
	if err != nil {
		return nil, err
	}
	for i := range updated.ClockIns {
		if updated.ClockIns[i].RequestID == ci.RequestID {
			return &updated.ClockIns[i], nil
		}
	}
	return &ci, nil
}

// RecordTrack records a GPS track point. Duplicate (terminal, sequence) pairs
// are silently ignored (idempotent).
func (s *TaskService) RecordTrack(ctx context.Context, req TrackRequest) error {
	now := s.clock.Now()
	ts := req.Timestamp
	if ts.IsZero() {
		ts = now
	}
	tp := domain.TrackPoint{
		TaskID:     req.TaskID,
		TerminalID: req.TerminalID,
		Lat:        req.Lat,
		Lng:        req.Lng,
		Timestamp:  ts,
		Sequence:   req.Sequence,
	}
	_, err := s.store.UpdateTask(req.TaskID, func(t *domain.Task) error {
		if t.Status != domain.TaskInProgress {
			return fmt.Errorf("%w: task is %s, not in progress", domain.ErrInvalidTransition, t.Status)
		}
		t.AddTrackPoint(tp, now)
		return nil
	})
	return err
}

// Complete transitions an in_progress task to completed.
func (s *TaskService) Complete(ctx context.Context, taskID, patrollerID string) (*domain.Task, error) {
	return s.store.UpdateTask(taskID, func(t *domain.Task) error {
		if t.Status != domain.TaskInProgress {
			return fmt.Errorf("%w: expected in_progress, got %s", domain.ErrInvalidTransition, t.Status)
		}
		if t.ClaimerID != patrollerID {
			return fmt.Errorf("%w: only the claimer may complete", domain.ErrNotAuthorized)
		}
		t.Complete(s.clock.Now())
		return nil
	})
}

// RollbackTask manually rolls a task back to the pending dispatch pool.
func (s *TaskService) RollbackTask(ctx context.Context, taskID, actor, reason string) (*domain.Task, error) {
	return s.store.UpdateTask(taskID, func(t *domain.Task) error {
		if t.Status == domain.TaskCompleted || t.Status == domain.TaskCancelled {
			return fmt.Errorf("%w: cannot rollback from %s", domain.ErrInvalidTransition, t.Status)
		}
		t.Rollback(actor, reason, s.clock.Now())
		return nil
	})
}

// Cancel transitions a task to cancelled.
func (s *TaskService) Cancel(ctx context.Context, taskID, actor string) (*domain.Task, error) {
	return s.store.UpdateTask(taskID, func(t *domain.Task) error {
		if t.Status == domain.TaskCompleted {
			return fmt.Errorf("%w: cannot cancel completed task", domain.ErrInvalidTransition)
		}
		t.Cancel(actor, s.clock.Now())
		return nil
	})
}

// RollbackTimedOutTasks scans for tasks whose claim window has expired and
// rolls them back to the pending pool. Returns the number rolled back.
func (s *TaskService) RollbackTimedOutTasks(ctx context.Context) (int, error) {
	now := s.clock.Now()
	tasks := s.store.ListTasks()
	count := 0
	for _, t := range tasks {
		if !s.isTaskTimedOut(t, now) {
			continue
		}
		_, err := s.store.UpdateTask(t.ID, func(task *domain.Task) error {
			if !s.isTaskTimedOut(task, now) {
				return nil
			}
			task.Rollback("system", "claim timeout exceeded", now)
			return nil
		})
		if err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func (s *TaskService) isTaskTimedOut(t *domain.Task, now time.Time) bool {
	switch t.Status {
	case domain.TaskAssigned:
		return now.Sub(t.DispatchedAt) > s.cfg.ClaimTimeout
	case domain.TaskClaimed:
		return now.Sub(t.ClaimedAt) > s.cfg.ClaimTimeout
	default:
		return false
	}
}
