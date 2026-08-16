package domain

import (
	"fmt"
	"time"
)

// TaskStatus represents the lifecycle state of a patrol task.
type TaskStatus string

const (
	TaskPending    TaskStatus = "pending"     // 待派池
	TaskAssigned   TaskStatus = "assigned"    // 已下发到网格
	TaskClaimed    TaskStatus = "claimed"     // 已认领
	TaskInProgress TaskStatus = "in_progress" // 巡护中
	TaskCompleted  TaskStatus = "completed"   // 已完成
	TaskCancelled  TaskStatus = "cancelled"   // 已取消
)

// Priority indicates the urgency of a task.
type Priority string

const (
	PriorityRoutine Priority = "routine"
	PriorityUrgent  Priority = "urgent"
)

var taskTransitions = map[TaskStatus][]TaskStatus{
	TaskPending:    {TaskAssigned, TaskCancelled},
	TaskAssigned:   {TaskClaimed, TaskPending, TaskCancelled},
	TaskClaimed:    {TaskInProgress, TaskPending, TaskCancelled},
	TaskInProgress: {TaskCompleted, TaskPending, TaskCancelled},
	TaskCompleted:  {},
	TaskCancelled:  {},
}

// CanTransitionTo reports whether a status transition is valid.
func (s TaskStatus) CanTransitionTo(target TaskStatus) bool {
	for _, t := range taskTransitions[s] {
		if t == target {
			return true
		}
	}
	return false
}

// KeyPoint is a GPS waypoint that patrollers must visit and clock in at.
type KeyPoint struct {
	ID   string  `json:"id"`
	Name string  `json:"name"`
	Lat  float64 `json:"lat"`
	Lng  float64 `json:"lng"`
}

// ClockIn records a patroller's arrival at a key point.
type ClockIn struct {
	RequestID   string    `json:"request_id"`
	TaskID      string    `json:"task_id"`
	KeyPointID  string    `json:"key_point_id"`
	PatrollerID string    `json:"patroller_id"`
	Lat         float64   `json:"lat"`
	Lng         float64   `json:"lng"`
	PhotoRef    string    `json:"photo_ref"`
	Timestamp   time.Time `json:"timestamp"`
	Violation   bool      `json:"violation"`
	Offline     bool      `json:"offline"`
}

// TrackPoint is a periodic GPS reading transmitted by a handheld terminal.
type TrackPoint struct {
	TaskID     string    `json:"task_id"`
	TerminalID string    `json:"terminal_id"`
	Lat        float64   `json:"lat"`
	Lng        float64   `json:"lng"`
	Timestamp  time.Time `json:"timestamp"`
	Sequence   int       `json:"sequence"`
}

// AuditEntry captures an immutable record of a state-changing operation.
type AuditEntry struct {
	Action    string    `json:"action"`
	ActorID   string    `json:"actor_id"`
	Detail    string    `json:"detail"`
	Timestamp time.Time `json:"timestamp"`
}

// Task is the aggregate root for a patrol assignment.
type Task struct {
	ID           string       `json:"id"`
	GridID       string       `json:"grid_id"`
	Title        string       `json:"title"`
	DispatcherID string       `json:"dispatcher_id"`
	ClaimerID    string       `json:"claimer_id"`
	PartnerID    string       `json:"partner_id"`
	Status       TaskStatus   `json:"status"`
	Priority     Priority     `json:"priority"`
	RequireDual  bool         `json:"require_dual"`
	PreReported  bool         `json:"pre_reported"`
	KeyPoints    []KeyPoint   `json:"key_points"`
	ClockIns     []ClockIn    `json:"clock_ins"`
	TrackPoints  []TrackPoint `json:"track_points"`
	CreatedAt    time.Time    `json:"created_at"`
	DispatchedAt time.Time    `json:"dispatched_at"`
	ClaimedAt    time.Time    `json:"claimed_at"`
	StartedAt    time.Time    `json:"started_at"`
	CompletedAt  time.Time    `json:"completed_at"`
	Version      int          `json:"version"`
	Audit        []AuditEntry `json:"audit"`
}

// AddAudit appends an audit entry to the task.
func (t *Task) AddAudit(action, actor, detail string, ts time.Time) {
	t.Audit = append(t.Audit, AuditEntry{
		Action: action, ActorID: actor, Detail: detail, Timestamp: ts,
	})
}

// CanClaim validates whether a patroller may claim this task.
func (t *Task) CanClaim(patrollerID string, grid *Grid) error {
	if t.Status != TaskAssigned && t.Status != TaskPending {
		return fmt.Errorf("%w: task is %s", ErrAlreadyClaimed, t.Status)
	}
	if grid != nil && !grid.HasPatroller(patrollerID) {
		return fmt.Errorf("%w: patroller %s not assigned to grid %s", ErrNotAuthorized, patrollerID, grid.ID)
	}
	return nil
}

// Claim transitions the task to the claimed state.
func (t *Task) Claim(patrollerID string, now time.Time) {
	t.Status = TaskClaimed
	t.ClaimerID = patrollerID
	t.ClaimedAt = now
	t.Version++
	t.AddAudit("claim", patrollerID, "task claimed", now)
}

// CanStart validates whether a patroller may begin patrol.
func (t *Task) CanStart(patrollerID string) error {
	if t.Status != TaskClaimed {
		return fmt.Errorf("%w: expected claimed, got %s", ErrInvalidTransition, t.Status)
	}
	if t.ClaimerID != patrollerID {
		return fmt.Errorf("%w: only the claimer may start", ErrNotAuthorized)
	}
	if t.RequireDual {
		if !t.PreReported {
			return ErrDualPatrolRequired
		}
		if t.PartnerID == "" {
			return fmt.Errorf("%w: dual patrol requires a partner", ErrDualPatrolRequired)
		}
	}
	return nil
}

// Start transitions the task to in_progress.
func (t *Task) Start(now time.Time) {
	t.Status = TaskInProgress
	t.StartedAt = now
	t.Version++
	t.AddAudit("start", t.ClaimerID, "patrol started", now)
}

// PreReport marks advance reporting complete for dual-patrol tasks.
func (t *Task) PreReport(reporterID, partnerID string, now time.Time) error {
	if t.Status != TaskAssigned && t.Status != TaskClaimed {
		return fmt.Errorf("%w: cannot pre-report from %s", ErrInvalidTransition, t.Status)
	}
	t.PreReported = true
	t.PartnerID = partnerID
	t.Version++
	t.AddAudit("pre_report", reporterID, "advance report filed for dual patrol", now)
	return nil
}

// CheckClockInGap validates the six-hour clock-in interval rule.
func (t *Task) CheckClockInGap(now time.Time, maxGap time.Duration) error {
	if len(t.ClockIns) == 0 {
		return nil
	}
	last := t.ClockIns[len(t.ClockIns)-1]
	if now.Sub(last.Timestamp) > maxGap {
		return fmt.Errorf("%w: gap since last clock-in is %v", ErrClockInGapExceeded, now.Sub(last.Timestamp))
	}
	return nil
}

// HasClockIn checks idempotency by request ID.
func (t *Task) HasClockIn(requestID string) bool {
	for i := range t.ClockIns {
		if t.ClockIns[i].RequestID == requestID {
			return true
		}
	}
	return false
}

// AddClockIn records a clock-in entry, flagging violations.
func (t *Task) AddClockIn(ci ClockIn, maxGap time.Duration, now time.Time) {
	if err := t.CheckClockInGap(ci.Timestamp, maxGap); err != nil {
		ci.Violation = true
	}
	t.ClockIns = append(t.ClockIns, ci)
	t.Version++
	actor := ci.PatrollerID
	detail := fmt.Sprintf("clock-in at key point %s", ci.KeyPointID)
	if ci.Violation {
		detail += " (interval violation)"
	}
	if ci.Offline {
		detail += " (offline replay)"
	}
	t.AddAudit("clock_in", actor, detail, now)
}

// HasTrackPoint checks track idempotency by terminal and sequence.
func (t *Task) HasTrackPoint(terminalID string, seq int) bool {
	for i := range t.TrackPoints {
		if t.TrackPoints[i].TerminalID == terminalID && t.TrackPoints[i].Sequence == seq {
			return true
		}
	}
	return false
}

// AddTrackPoint records a GPS track point if not already present.
func (t *Task) AddTrackPoint(tp TrackPoint, now time.Time) bool {
	if t.HasTrackPoint(tp.TerminalID, tp.Sequence) {
		return false
	}
	t.TrackPoints = append(t.TrackPoints, tp)
	t.Version++
	return true
}

// Complete transitions the task to completed.
func (t *Task) Complete(now time.Time) {
	t.Status = TaskCompleted
	t.CompletedAt = now
	t.Version++
	t.AddAudit("complete", t.ClaimerID, "patrol completed", now)
}

// Rollback resets the task to the pending dispatch pool.
func (t *Task) Rollback(actor, reason string, now time.Time) {
	prev := t.Status
	t.Status = TaskPending
	t.ClaimerID = ""
	t.PartnerID = ""
	t.PreReported = false
	t.Version++
	t.AddAudit("rollback", actor, fmt.Sprintf("rolled back from %s: %s", prev, reason), now)
}

// Cancel transitions the task to cancelled.
func (t *Task) Cancel(actor string, now time.Time) {
	t.Status = TaskCancelled
	t.Version++
	t.AddAudit("cancel", actor, "task cancelled", now)
}

// Clone returns a deep copy of the task.
func (t *Task) Clone() *Task {
	cp := *t
	if t.KeyPoints != nil {
		cp.KeyPoints = append([]KeyPoint(nil), t.KeyPoints...)
	}
	if t.ClockIns != nil {
		cp.ClockIns = append([]ClockIn(nil), t.ClockIns...)
	}
	if t.TrackPoints != nil {
		cp.TrackPoints = append([]TrackPoint(nil), t.TrackPoints...)
	}
	if t.Audit != nil {
		cp.Audit = append([]AuditEntry(nil), t.Audit...)
	}
	return &cp
}
