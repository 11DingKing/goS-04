package service

import (
	"context"
	"fmt"

	"patrol-platform/internal/config"
	"patrol-platform/internal/domain"
	"patrol-platform/internal/store"
)

// AlarmService orchestrates the alarm lifecycle: report, review, assign,
// accept, handle, resolve, and automatic escalation of overdue events.
type AlarmService struct {
	store *store.Store
	cfg   *config.Config
	clock Clock
}

// NewAlarmService creates an alarm service. If clock is nil, RealClock is used.
func NewAlarmService(s *store.Store, cfg *config.Config, clock Clock) *AlarmService {
	if clock == nil {
		clock = RealClock{}
	}
	return &AlarmService{store: s, cfg: cfg, clock: clock}
}

// Report creates a new alarm in the reported state.
func (s *AlarmService) Report(ctx context.Context, reporterID, taskID string, typ domain.AlarmType, description string, media []string) (*domain.Alarm, error) {
	now := s.clock.Now()
	a := &domain.Alarm{
		ID:          domain.NewID("alarm"),
		TaskID:      taskID,
		ReporterID:  reporterID,
		Type:        typ,
		Description: description,
		MediaRefs:   media,
		Status:      domain.AlarmReported,
		CreatedAt:   now,
		Version:     1,
	}
	a.AddAudit("report", reporterID, fmt.Sprintf("%s alarm reported", typ), now)
	s.store.SaveAlarm(a)
	return a, nil
}

// Review classifies and levels an alarm, transitioning it to reviewed.
func (s *AlarmService) Review(ctx context.Context, alarmID, reviewerID string, level domain.AlarmLevel) (*domain.Alarm, error) {
	return s.store.UpdateAlarm(alarmID, func(a *domain.Alarm) error {
		if a.Status != domain.AlarmReported && a.Status != domain.AlarmEscalated {
			return fmt.Errorf("%w: expected reported or escalated, got %s", domain.ErrInvalidTransition, a.Status)
		}
		a.Review(reviewerID, level, s.clock.Now())
		return nil
	})
}

// Assign designates a handler for an alarm, transitioning it to assigned.
func (s *AlarmService) Assign(ctx context.Context, alarmID, reviewerID, handlerID string) (*domain.Alarm, error) {
	return s.store.UpdateAlarm(alarmID, func(a *domain.Alarm) error {
		if a.Status != domain.AlarmReviewed && a.Status != domain.AlarmEscalated {
			return fmt.Errorf("%w: expected reviewed or escalated, got %s", domain.ErrInvalidTransition, a.Status)
		}
		a.Assign(reviewerID, handlerID, s.clock.Now())
		return nil
	})
}

// Accept lets a handler accept an alarm. When multiple handlers race, only
// the earliest (first to acquire the write lock) succeeds.
func (s *AlarmService) Accept(ctx context.Context, alarmID, handlerID string) (*domain.Alarm, error) {
	return s.store.UpdateAlarm(alarmID, func(a *domain.Alarm) error {
		if err := a.CanAccept(handlerID); err != nil {
			return err
		}
		a.Accept(handlerID, s.clock.Now())
		return nil
	})
}

// StartHandling transitions an accepted alarm to in_progress.
func (s *AlarmService) StartHandling(ctx context.Context, alarmID, handlerID string) (*domain.Alarm, error) {
	return s.store.UpdateAlarm(alarmID, func(a *domain.Alarm) error {
		if a.Status != domain.AlarmAccepted {
			return fmt.Errorf("%w: expected accepted, got %s", domain.ErrInvalidTransition, a.Status)
		}
		if a.HandlerID != handlerID {
			return fmt.Errorf("%w: only the assigned handler may start", domain.ErrNotAuthorized)
		}
		a.StartHandling(handlerID, s.clock.Now())
		return nil
	})
}

// Resolve closes an alarm with a resolution note.
func (s *AlarmService) Resolve(ctx context.Context, alarmID, handlerID, resolution string) (*domain.Alarm, error) {
	return s.store.UpdateAlarm(alarmID, func(a *domain.Alarm) error {
		if a.Status != domain.AlarmInProgress && a.Status != domain.AlarmAccepted {
			return fmt.Errorf("%w: expected in_progress or accepted, got %s", domain.ErrInvalidTransition, a.Status)
		}
		if a.HandlerID != handlerID {
			return fmt.Errorf("%w: only the handler may resolve", domain.ErrNotAuthorized)
		}
		a.Resolve(handlerID, resolution, s.clock.Now())
		return nil
	})
}

// EscalateOverdueAlarms scans for level-1 alarms that have exceeded the
// two-hour response window and escalates them. Returns the count escalated.
func (s *AlarmService) EscalateOverdueAlarms(ctx context.Context) (int, error) {
	now := s.clock.Now()
	alarms := s.store.ListAlarms()
	count := 0
	for _, a := range alarms {
		if !a.IsOverdue(now, s.cfg.Level1Timeout) {
			continue
		}
		_, err := s.store.UpdateAlarm(a.ID, func(alm *domain.Alarm) error {
			if !alm.IsOverdue(now, s.cfg.Level1Timeout) {
				return nil
			}
			base := alm.ReviewedAt
			if base.IsZero() {
				base = alm.CreatedAt
			}
			reason := fmt.Sprintf("level %d alarm overdue by %v", alm.Level, now.Sub(base))
			alm.Escalate(reason, now)
			return nil
		})
		if err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

// RollbackUnacceptedAlarms resets alarms whose handler did not accept within
// the accept timeout, returning them to the reviewed state for reassignment.
func (s *AlarmService) RollbackUnacceptedAlarms(ctx context.Context) (int, error) {
	now := s.clock.Now()
	alarms := s.store.ListAlarms()
	count := 0
	for _, a := range alarms {
		if a.Status != domain.AlarmAssigned {
			continue
		}
		if now.Sub(a.ReviewedAt) <= s.cfg.AcceptTimeout {
			continue
		}
		_, err := s.store.UpdateAlarm(a.ID, func(alm *domain.Alarm) error {
			if alm.Status != domain.AlarmAssigned {
				return nil
			}
			if now.Sub(alm.ReviewedAt) <= s.cfg.AcceptTimeout {
				return nil
			}
			alm.RollbackToReview("accept timeout, returned for reassignment", now)
			return nil
		})
		if err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}
