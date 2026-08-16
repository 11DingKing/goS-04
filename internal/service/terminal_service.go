package service

import (
	"context"
	"fmt"
	"sort"

	"patrol-platform/internal/config"
	"patrol-platform/internal/domain"
	"patrol-platform/internal/store"
)

// SyncResult summarises the outcome of an offline data synchronisation.
type SyncResult struct {
	ClockInsSynced int  `json:"clock_ins_synced"`
	ClockInsFailed int  `json:"clock_ins_failed"`
	TracksSynced   int  `json:"tracks_synced"`
	TracksFailed   int  `json:"tracks_failed"`
	Cleared        bool `json:"cleared"`
}

// TerminalService manages handheld terminal registration, online/offline
// transitions, local caching of records, and time-ordered offline replay.
type TerminalService struct {
	store   *store.Store
	cfg     *config.Config
	taskSvc *TaskService
	clock   Clock
}

// NewTerminalService creates a terminal service.
func NewTerminalService(s *store.Store, cfg *config.Config, taskSvc *TaskService, clock Clock) *TerminalService {
	if clock == nil {
		clock = RealClock{}
	}
	return &TerminalService{store: s, cfg: cfg, taskSvc: taskSvc, clock: clock}
}

// Register creates a new terminal in the online state.
func (s *TerminalService) Register(ctx context.Context, terminalID, patrollerID string) (*domain.Terminal, error) {
	term := &domain.Terminal{
		ID:          terminalID,
		PatrollerID: patrollerID,
		Online:      true,
		LastSeen:    s.clock.Now(),
	}
	s.store.SaveTerminal(term)
	return term, nil
}

// SetOffline marks a terminal as offline.
func (s *TerminalService) SetOffline(ctx context.Context, terminalID string) (*domain.Terminal, error) {
	return s.store.UpdateTerminal(terminalID, func(term *domain.Terminal) error {
		term.GoOffline(s.clock.Now())
		return nil
	})
}

// SetOnline marks a terminal as online without syncing pending data.
func (s *TerminalService) SetOnline(ctx context.Context, terminalID string) (*domain.Terminal, error) {
	return s.store.UpdateTerminal(terminalID, func(term *domain.Terminal) error {
		term.GoOnline(s.clock.Now())
		return nil
	})
}

// CacheClockIn stores a clock-in record on the terminal for later replay.
func (s *TerminalService) CacheClockIn(ctx context.Context, terminalID string, req ClockInRequest) error {
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
	}
	_, err := s.store.UpdateTerminal(terminalID, func(term *domain.Terminal) error {
		if term.Online {
			return fmt.Errorf("%w: terminal is online, use direct clock-in", domain.ErrTerminalOffline)
		}
		term.CacheClockIn(ci)
		return nil
	})
	return err
}

// CacheTrack stores a track point on the terminal for later replay.
func (s *TerminalService) CacheTrack(ctx context.Context, terminalID string, req TrackRequest) error {
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
	_, err := s.store.UpdateTerminal(terminalID, func(term *domain.Terminal) error {
		if term.Online {
			return fmt.Errorf("%w: terminal is online, use direct track upload", domain.ErrTerminalOffline)
		}
		term.CacheTrack(tp)
		return nil
	})
	return err
}

// SyncOffline brings a terminal online and replays all cached records in
// timestamp order. Each record is applied idempotently; duplicates from
// retried syncs are silently deduplicated.
func (s *TerminalService) SyncOffline(ctx context.Context, terminalID string) (*SyncResult, error) {
	result := &SyncResult{}

	_, err := s.store.UpdateTerminal(terminalID, func(term *domain.Terminal) error {
		term.GoOnline(s.clock.Now())
		return nil
	})
	if err != nil {
		return nil, err
	}

	term, err := s.store.GetTerminal(terminalID)
	if err != nil {
		return nil, err
	}

	clockIns := make([]domain.ClockIn, len(term.PendingClockIns))
	copy(clockIns, term.PendingClockIns)
	sort.Slice(clockIns, func(i, j int) bool {
		return clockIns[i].Timestamp.Before(clockIns[j].Timestamp)
	})
	for _, ci := range clockIns {
		_, err := s.taskSvc.RecordClockIn(ctx, ClockInRequest{
			TaskID:      ci.TaskID,
			KeyPointID:  ci.KeyPointID,
			PatrollerID: ci.PatrollerID,
			Lat:         ci.Lat,
			Lng:         ci.Lng,
			PhotoRef:    ci.PhotoRef,
			RequestID:   ci.RequestID,
			Timestamp:   ci.Timestamp,
			Offline:     ci.Offline,
		})
		if err != nil {
			result.ClockInsFailed++
		} else {
			result.ClockInsSynced++
		}
	}

	tracks := make([]domain.TrackPoint, len(term.PendingTracks))
	copy(tracks, term.PendingTracks)
	sort.Slice(tracks, func(i, j int) bool {
		return tracks[i].Timestamp.Before(tracks[j].Timestamp)
	})
	for _, tp := range tracks {
		err := s.taskSvc.RecordTrack(ctx, TrackRequest{
			TaskID:     tp.TaskID,
			TerminalID: tp.TerminalID,
			Lat:        tp.Lat,
			Lng:        tp.Lng,
			Sequence:   tp.Sequence,
			Timestamp:  tp.Timestamp,
		})
		if err != nil {
			result.TracksFailed++
		} else {
			result.TracksSynced++
		}
	}

	_, err = s.store.UpdateTerminal(terminalID, func(term *domain.Terminal) error {
		term.ClearPending()
		return nil
	})
	if err != nil {
		return nil, err
	}
	result.Cleared = true
	return result, nil
}

// GetTerminal retrieves a terminal by ID.
func (s *TerminalService) GetTerminal(ctx context.Context, terminalID string) (*domain.Terminal, error) {
	return s.store.GetTerminal(terminalID)
}
