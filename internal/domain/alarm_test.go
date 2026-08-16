package domain

import (
	"testing"
	"time"
)

func TestAlarmStateMachineValidTransitions(t *testing.T) {
	cases := []struct{ from, to AlarmStatus }{
		{AlarmReported, AlarmReviewed},
		{AlarmReviewed, AlarmAssigned},
		{AlarmAssigned, AlarmAccepted},
		{AlarmAccepted, AlarmInProgress},
		{AlarmInProgress, AlarmResolved},
		{AlarmReported, AlarmEscalated},
		{AlarmReviewed, AlarmEscalated},
		{AlarmAssigned, AlarmEscalated},
		{AlarmAssigned, AlarmReviewed},
		{AlarmEscalated, AlarmReviewed},
		{AlarmEscalated, AlarmAssigned},
	}
	for _, c := range cases {
		if !c.from.CanTransitionTo(c.to) {
			t.Errorf("expected transition %s -> %s to be valid", c.from, c.to)
		}
	}
}

func TestAlarmStateMachineInvalidTransition(t *testing.T) {
	cases := []struct{ from, to AlarmStatus }{
		{AlarmReported, AlarmAccepted},
		{AlarmReported, AlarmResolved},
		{AlarmReviewed, AlarmAccepted},
		{AlarmResolved, AlarmInProgress},
		{AlarmResolved, AlarmEscalated},
	}
	for _, c := range cases {
		if c.from.CanTransitionTo(c.to) {
			t.Errorf("expected transition %s -> %s to be invalid", c.from, c.to)
		}
	}
}

func TestAlarmOverdueCheck(t *testing.T) {
	base := time.Date(2026, 8, 16, 8, 0, 0, 0, time.UTC)
	a := &Alarm{
		Level:      LevelOne,
		Status:     AlarmReviewed,
		ReviewedAt: base,
	}
	if a.IsOverdue(base.Add(1*time.Hour), 2*time.Hour) {
		t.Error("should not be overdue within 2h")
	}
	if a.IsOverdue(base.Add(2*time.Hour), 2*time.Hour) {
		t.Error("should not be overdue at exactly 2h")
	}
	if !a.IsOverdue(base.Add(2*time.Hour+time.Minute), 2*time.Hour) {
		t.Error("should be overdue after 2h")
	}

	a.Status = AlarmResolved
	if a.IsOverdue(base.Add(3*time.Hour), 2*time.Hour) {
		t.Error("resolved alarm should not be overdue")
	}

	a2 := &Alarm{Level: LevelTwo, Status: AlarmReviewed, ReviewedAt: base}
	if a2.IsOverdue(base.Add(10*time.Hour), 2*time.Hour) {
		t.Error("level-2 alarm should never be overdue by level-1 rule")
	}
}

func TestAlarmRollbackToReview(t *testing.T) {
	now := time.Now()
	a := &Alarm{
		Status:    AlarmAssigned,
		HandlerID: "h1",
		Version:   2,
	}
	a.RollbackToReview("accept timeout", now)
	if a.Status != AlarmReviewed {
		t.Fatalf("expected reviewed, got %s", a.Status)
	}
	if a.HandlerID != "" {
		t.Fatal("handler should be cleared on rollback")
	}
	if a.Version != 3 {
		t.Fatalf("expected version 3, got %d", a.Version)
	}
}

func TestAlarmEscalateSetsFlag(t *testing.T) {
	now := time.Now()
	a := &Alarm{
		Level:  LevelOne,
		Status: AlarmAccepted,
	}
	a.Escalate("overdue by 2h5m", now)
	if a.Status != AlarmEscalated {
		t.Fatalf("expected escalated, got %s", a.Status)
	}
	if !a.Escalated {
		t.Error("escalated flag should be set")
	}
	if !a.EscalatedAt.Equal(now) {
		t.Error("escalatedAt should be set")
	}
}
