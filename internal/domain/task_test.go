package domain

import (
	"errors"
	"testing"
	"time"
)

func TestTaskStateMachineValidTransitions(t *testing.T) {
	cases := []struct{ from, to TaskStatus }{
		{TaskPending, TaskAssigned},
		{TaskAssigned, TaskClaimed},
		{TaskClaimed, TaskInProgress},
		{TaskInProgress, TaskCompleted},
		{TaskAssigned, TaskPending},
		{TaskClaimed, TaskPending},
		{TaskInProgress, TaskPending},
		{TaskPending, TaskCancelled},
		{TaskAssigned, TaskCancelled},
		{TaskClaimed, TaskCancelled},
	}
	for _, c := range cases {
		if !c.from.CanTransitionTo(c.to) {
			t.Errorf("expected transition %s -> %s to be valid", c.from, c.to)
		}
	}
}

func TestTaskStateMachineInvalidTransition(t *testing.T) {
	cases := []struct{ from, to TaskStatus }{
		{TaskPending, TaskClaimed},
		{TaskPending, TaskInProgress},
		{TaskAssigned, TaskCompleted},
		{TaskClaimed, TaskCompleted},
		{TaskCompleted, TaskInProgress},
		{TaskCompleted, TaskPending},
		{TaskCancelled, TaskPending},
	}
	for _, c := range cases {
		if c.from.CanTransitionTo(c.to) {
			t.Errorf("expected transition %s -> %s to be invalid", c.from, c.to)
		}
	}
}

func TestTaskClockInGapViolation(t *testing.T) {
	base := time.Date(2026, 8, 16, 8, 0, 0, 0, time.UTC)
	task := &Task{
		Status:   TaskInProgress,
		ClockIns: []ClockIn{{RequestID: "ci1", Timestamp: base}},
	}

	if err := task.CheckClockInGap(base.Add(5*time.Hour), 6*time.Hour); err != nil {
		t.Errorf("expected no violation within 6h, got %v", err)
	}
	if err := task.CheckClockInGap(base.Add(6*time.Hour), 6*time.Hour); err != nil {
		t.Errorf("expected no violation at exactly 6h, got %v", err)
	}
	err := task.CheckClockInGap(base.Add(6*time.Hour+time.Minute), 6*time.Hour)
	if !errors.Is(err, ErrClockInGapExceeded) {
		t.Fatalf("expected ErrClockInGapExceeded, got %v", err)
	}

	task2 := &Task{Status: TaskInProgress}
	if err := task2.CheckClockInGap(base, 6*time.Hour); err != nil {
		t.Errorf("first clock-in should never violate, got %v", err)
	}
}

func TestTaskClockInFlagsViolation(t *testing.T) {
	base := time.Date(2026, 8, 16, 8, 0, 0, 0, time.UTC)
	task := &Task{
		Status:   TaskInProgress,
		ClockIns: []ClockIn{{RequestID: "ci1", Timestamp: base}},
	}

	task.AddClockIn(ClockIn{RequestID: "ci2", Timestamp: base.Add(5 * time.Hour)}, 6*time.Hour, base.Add(5*time.Hour))
	if task.ClockIns[len(task.ClockIns)-1].Violation {
		t.Error("clock-in within gap should not be flagged")
	}

	task.AddClockIn(ClockIn{RequestID: "ci3", Timestamp: base.Add(12 * time.Hour)}, 6*time.Hour, base.Add(12*time.Hour))
	if !task.ClockIns[len(task.ClockIns)-1].Violation {
		t.Error("clock-in exceeding gap should be flagged as violation")
	}
}

func TestTaskDualPatrolRequirement(t *testing.T) {
	task := &Task{Status: TaskClaimed, ClaimerID: "p1", RequireDual: true}

	if err := task.CanStart("p1"); !errors.Is(err, ErrDualPatrolRequired) {
		t.Fatalf("expected ErrDualPatrolRequired without pre-report, got %v", err)
	}

	task.PreReported = true
	if err := task.CanStart("p1"); !errors.Is(err, ErrDualPatrolRequired) {
		t.Fatalf("expected ErrDualPatrolRequired without partner, got %v", err)
	}

	task.PartnerID = "p2"
	if err := task.CanStart("p1"); err != nil {
		t.Fatalf("expected success with pre-report and partner, got %v", err)
	}

	if err := task.CanStart("p2"); !errors.Is(err, ErrNotAuthorized) {
		t.Fatalf("expected ErrNotAuthorized for non-claimer, got %v", err)
	}
}

func TestTaskRollbackClearsState(t *testing.T) {
	now := time.Now()
	task := &Task{
		Status:      TaskClaimed,
		ClaimerID:   "p1",
		PartnerID:   "p2",
		PreReported: true,
		Version:     3,
	}
	task.Rollback("system", "timeout", now)
	if task.Status != TaskPending {
		t.Fatalf("expected pending, got %s", task.Status)
	}
	if task.ClaimerID != "" || task.PartnerID != "" || task.PreReported {
		t.Fatal("rollback should clear claimer, partner, and pre-report")
	}
	if task.Version != 4 {
		t.Fatalf("expected version 4, got %d", task.Version)
	}
	found := false
	for _, a := range task.Audit {
		if a.Action == "rollback" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected rollback audit entry")
	}
}
