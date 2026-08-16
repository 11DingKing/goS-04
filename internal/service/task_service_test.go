package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"patrol-platform/internal/config"
	"patrol-platform/internal/domain"
	"patrol-platform/internal/store"
)

func setupTaskTest(t *testing.T) (*store.Store, *TaskService, *AlarmService, *FakeClock) {
	t.Helper()
	cfg := &config.Config{
		ClockInMaxGap: 6 * time.Hour,
		Level1Timeout: 2 * time.Hour,
		ClaimTimeout:  30 * time.Minute,
		AcceptTimeout: 30 * time.Minute,
	}
	clock := NewFakeClock(time.Date(2026, 8, 16, 8, 0, 0, 0, time.UTC))
	st := store.New()
	st.SaveGrid(&domain.Grid{ID: "g1", Name: "Buffer Grid", Zone: domain.ZoneBuffer, PatrollerIDs: []string{"p1", "p2"}})
	return st, NewTaskService(st, cfg, clock), NewAlarmService(st, cfg, clock), clock
}

func TestDispatchAndClaimTask(t *testing.T) {
	st, taskSvc, _, _ := setupTaskTest(t)
	ctx := context.Background()

	task, err := taskSvc.Dispatch(ctx, "d1", "g1", "Morning Patrol", domain.PriorityRoutine,
		[]domain.KeyPoint{{ID: "kp1", Name: "North Ridge", Lat: 38.5, Lng: 102.3}})
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != domain.TaskAssigned {
		t.Fatalf("expected assigned, got %s", task.Status)
	}
	if task.Version != 1 {
		t.Fatalf("expected version 1, got %d", task.Version)
	}

	task, err = taskSvc.Claim(ctx, task.ID, "p1")
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != domain.TaskClaimed {
		t.Fatalf("expected claimed, got %s", task.Status)
	}
	if task.ClaimerID != "p1" {
		t.Fatalf("expected claimer p1, got %s", task.ClaimerID)
	}

	task, _ = st.GetTask(task.ID)
	if len(task.Audit) < 2 {
		t.Fatalf("expected at least 2 audit entries, got %d", len(task.Audit))
	}
}

func TestClaimRejectsNonGridPatroller(t *testing.T) {
	_, taskSvc, _, _ := setupTaskTest(t)
	ctx := context.Background()

	task, _ := taskSvc.Dispatch(ctx, "d1", "g1", "Patrol", domain.PriorityRoutine, nil)
	_, err := taskSvc.Claim(ctx, task.ID, "stranger")
	if !errors.Is(err, domain.ErrNotAuthorized) {
		t.Fatalf("expected ErrNotAuthorized, got %v", err)
	}
}

func TestConcurrentClaimFirstWins(t *testing.T) {
	_, taskSvc, _, _ := setupTaskTest(t)
	ctx := context.Background()

	task, _ := taskSvc.Dispatch(ctx, "d1", "g1", "Contested Patrol", domain.PriorityRoutine, nil)

	start := make(chan struct{})
	var wg sync.WaitGroup
	var err1, err2 error
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		_, err1 = taskSvc.Claim(ctx, task.ID, "p1")
	}()
	go func() {
		defer wg.Done()
		<-start
		_, err2 = taskSvc.Claim(ctx, task.ID, "p2")
	}()
	close(start)
	wg.Wait()

	if (err1 == nil) == (err2 == nil) {
		t.Fatalf("expected exactly one claim to succeed, got err1=%v err2=%v", err1, err2)
	}

	stored, _ := taskSvc.GetTask(ctx, task.ID)
	if stored.Status != domain.TaskClaimed {
		t.Fatalf("expected claimed, got %s", stored.Status)
	}
	if stored.ClaimerID != "p1" && stored.ClaimerID != "p2" {
		t.Fatalf("expected claimer p1 or p2, got %s", stored.ClaimerID)
	}

	loserErr := err1
	if err1 == nil {
		loserErr = err2
	}
	if !errors.Is(loserErr, domain.ErrAlreadyClaimed) {
		t.Fatalf("expected loser to get ErrAlreadyClaimed, got %v", loserErr)
	}
}

func TestClockInIdempotency(t *testing.T) {
	_, taskSvc, _, clock := setupTaskTest(t)
	ctx := context.Background()

	task, _ := taskSvc.Dispatch(ctx, "d1", "g1", "Patrol", domain.PriorityRoutine, nil)
	taskSvc.Claim(ctx, task.ID, "p1")
	taskSvc.Start(ctx, task.ID, "p1")

	req := ClockInRequest{
		TaskID: task.ID, KeyPointID: "kp1", PatrollerID: "p1",
		Lat: 38.5, Lng: 102.3, PhotoRef: "photo.jpg",
		RequestID: "ci-dedup-001", Timestamp: clock.Now(),
	}
	ci1, err := taskSvc.RecordClockIn(ctx, req)
	if err != nil {
		t.Fatal(err)
	}

	ci2, err := taskSvc.RecordClockIn(ctx, req)
	if err != nil {
		t.Fatalf("duplicate clock-in should be idempotent, got %v", err)
	}

	stored, _ := taskSvc.GetTask(ctx, task.ID)
	if len(stored.ClockIns) != 1 {
		t.Fatalf("expected 1 clock-in after duplicate, got %d", len(stored.ClockIns))
	}
	if ci1.RequestID != ci2.RequestID {
		t.Fatal("idempotent response should match original")
	}
}

func TestClockInGapViolationFlagged(t *testing.T) {
	_, taskSvc, _, clock := setupTaskTest(t)
	ctx := context.Background()

	task, _ := taskSvc.Dispatch(ctx, "d1", "g1", "Patrol", domain.PriorityRoutine, nil)
	taskSvc.Claim(ctx, task.ID, "p1")
	taskSvc.Start(ctx, task.ID, "p1")

	taskSvc.RecordClockIn(ctx, ClockInRequest{
		TaskID: task.ID, KeyPointID: "kp1", PatrollerID: "p1",
		RequestID: "ci1", Timestamp: clock.Now(),
	})

	clock.Advance(7 * time.Hour)

	ci, err := taskSvc.RecordClockIn(ctx, ClockInRequest{
		TaskID: task.ID, KeyPointID: "kp2", PatrollerID: "p1",
		RequestID: "ci2", Timestamp: clock.Now(),
	})
	if err != nil {
		t.Fatalf("clock-in should still be recorded despite violation, got %v", err)
	}
	if !ci.Violation {
		t.Fatal("expected violation flag to be set")
	}

	stored, _ := taskSvc.GetTask(ctx, task.ID)
	if len(stored.ClockIns) != 2 {
		t.Fatalf("expected 2 clock-ins, got %d", len(stored.ClockIns))
	}
}

func TestTaskTimeoutRollback(t *testing.T) {
	st, taskSvc, _, clock := setupTaskTest(t)
	ctx := context.Background()

	task, _ := taskSvc.Dispatch(ctx, "d1", "g1", "Stale Patrol", domain.PriorityRoutine, nil)

	n, err := taskSvc.RollbackTimedOutTasks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("expected 0 rollbacks before timeout, got %d", n)
	}

	clock.Advance(31 * time.Minute)

	n, err = taskSvc.RollbackTimedOutTasks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 rollback after timeout, got %d", n)
	}

	stored, _ := st.GetTask(task.ID)
	if stored.Status != domain.TaskPending {
		t.Fatalf("expected pending after rollback, got %s", stored.Status)
	}

	foundRollback := false
	for _, a := range stored.Audit {
		if a.Action == "rollback" {
			foundRollback = true
		}
	}
	if !foundRollback {
		t.Fatal("expected rollback audit entry")
	}

	n, _ = taskSvc.RollbackTimedOutTasks(ctx)
	if n != 0 {
		t.Fatalf("should not rollback same task twice, got %d", n)
	}
}

func TestCompletePatrolFlow(t *testing.T) {
	_, taskSvc, _, clock := setupTaskTest(t)
	ctx := context.Background()

	task, _ := taskSvc.Dispatch(ctx, "d1", "g1", "Full Patrol", domain.PriorityRoutine,
		[]domain.KeyPoint{{ID: "kp1", Name: "Ridge", Lat: 38.5, Lng: 102.3}})

	taskSvc.Claim(ctx, task.ID, "p1")
	taskSvc.Start(ctx, task.ID, "p1")

	taskSvc.RecordClockIn(ctx, ClockInRequest{
		TaskID: task.ID, KeyPointID: "kp1", PatrollerID: "p1",
		Lat: 38.5, Lng: 102.3, RequestID: "ci1", Timestamp: clock.Now(),
	})

	err := taskSvc.RecordTrack(ctx, TrackRequest{
		TaskID: task.ID, TerminalID: "term1", Lat: 38.51, Lng: 102.31, Sequence: 1, Timestamp: clock.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}

	err = taskSvc.RecordTrack(ctx, TrackRequest{
		TaskID: task.ID, TerminalID: "term1", Lat: 38.52, Lng: 102.32, Sequence: 1, Timestamp: clock.Now(),
	})
	if err != nil {
		t.Fatal("duplicate track should be idempotent")
	}

	task, err = taskSvc.Complete(ctx, task.ID, "p1")
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != domain.TaskCompleted {
		t.Fatalf("expected completed, got %s", task.Status)
	}

	stored, _ := taskSvc.GetTask(ctx, task.ID)
	if len(stored.TrackPoints) != 1 {
		t.Fatalf("expected 1 track point (deduped), got %d", len(stored.TrackPoints))
	}
}

func TestCoreZoneDualPatrolEnforced(t *testing.T) {
	st, taskSvc, _, _ := setupTaskTest(t)
	ctx := context.Background()

	st.SaveGrid(&domain.Grid{ID: "g-core", Name: "Core Zone", Zone: domain.ZoneCore, PatrollerIDs: []string{"p1", "p2"}})

	task, _ := taskSvc.Dispatch(ctx, "d1", "g-core", "Core Patrol", domain.PriorityUrgent, nil)
	if !task.RequireDual {
		t.Fatal("core zone task should require dual patrol")
	}

	taskSvc.Claim(ctx, task.ID, "p1")

	_, err := taskSvc.Start(ctx, task.ID, "p1")
	if !errors.Is(err, domain.ErrDualPatrolRequired) {
		t.Fatalf("expected ErrDualPatrolRequired without pre-report, got %v", err)
	}

	taskSvc.PreReport(ctx, task.ID, "p1", "p2")

	task, err = taskSvc.Start(ctx, task.ID, "p1")
	if err != nil {
		t.Fatalf("expected success after pre-report, got %v", err)
	}
	if task.PartnerID != "p2" {
		t.Fatalf("expected partner p2, got %s", task.PartnerID)
	}
	if !task.PreReported {
		t.Fatal("expected pre-reported flag")
	}
}
