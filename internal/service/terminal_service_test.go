package service

import (
	"context"
	"testing"
	"time"

	"patrol-platform/internal/config"
	"patrol-platform/internal/domain"
)

func TestOfflineSyncReplaysInOrder(t *testing.T) {
	st, taskSvc, _, clock := setupTaskTest(t)
	cfg := &config.Config{ClockInMaxGap: 6 * time.Hour}
	terminalSvc := NewTerminalService(st, cfg, taskSvc, clock)
	ctx := context.Background()

	task, _ := taskSvc.Dispatch(ctx, "d1", "g1", "Patrol", domain.PriorityRoutine, nil)
	taskSvc.Claim(ctx, task.ID, "p1")
	taskSvc.Start(ctx, task.ID, "p1")

	terminalSvc.Register(ctx, "term1", "p1")
	terminalSvc.SetOffline(ctx, "term1")

	t1 := clock.Now()
	t2 := t1.Add(1 * time.Hour)
	t3 := t1.Add(2 * time.Hour)

	terminalSvc.CacheClockIn(ctx, "term1", ClockInRequest{
		TaskID: task.ID, KeyPointID: "kp3", PatrollerID: "p1", RequestID: "ci3", Timestamp: t3,
	})
	terminalSvc.CacheClockIn(ctx, "term1", ClockInRequest{
		TaskID: task.ID, KeyPointID: "kp1", PatrollerID: "p1", RequestID: "ci1", Timestamp: t1,
	})
	terminalSvc.CacheClockIn(ctx, "term1", ClockInRequest{
		TaskID: task.ID, KeyPointID: "kp2", PatrollerID: "p1", RequestID: "ci2", Timestamp: t2,
	})

	result, err := terminalSvc.SyncOffline(ctx, "term1")
	if err != nil {
		t.Fatal(err)
	}
	if result.ClockInsSynced != 3 {
		t.Fatalf("expected 3 synced, got %d", result.ClockInsSynced)
	}
	if result.ClockInsFailed != 0 {
		t.Fatalf("expected 0 failures, got %d", result.ClockInsFailed)
	}
	if !result.Cleared {
		t.Fatal("expected cleared flag")
	}

	stored, _ := st.GetTask(task.ID)
	if len(stored.ClockIns) != 3 {
		t.Fatalf("expected 3 clock-ins, got %d", len(stored.ClockIns))
	}
	expected := []string{"ci1", "ci2", "ci3"}
	for i, reqID := range expected {
		if stored.ClockIns[i].RequestID != reqID {
			t.Errorf("position %d: expected %s, got %s", i, reqID, stored.ClockIns[i].RequestID)
		}
	}
	for _, ci := range stored.ClockIns {
		if !ci.Offline {
			t.Error("expected clock-in to be marked offline")
		}
	}

	term, _ := st.GetTerminal("term1")
	if term.HasPendingData() {
		t.Fatal("pending data should be cleared after sync")
	}
}

func TestOfflineSyncIdempotency(t *testing.T) {
	st, taskSvc, _, clock := setupTaskTest(t)
	cfg := &config.Config{ClockInMaxGap: 6 * time.Hour}
	terminalSvc := NewTerminalService(st, cfg, taskSvc, clock)
	ctx := context.Background()

	task, _ := taskSvc.Dispatch(ctx, "d1", "g1", "Patrol", domain.PriorityRoutine, nil)
	taskSvc.Claim(ctx, task.ID, "p1")
	taskSvc.Start(ctx, task.ID, "p1")

	terminalSvc.Register(ctx, "term1", "p1")
	terminalSvc.SetOffline(ctx, "term1")

	terminalSvc.CacheClockIn(ctx, "term1", ClockInRequest{
		TaskID: task.ID, KeyPointID: "kp1", PatrollerID: "p1", RequestID: "ci-dedup", Timestamp: clock.Now(),
	})
	terminalSvc.CacheTrack(ctx, "term1", TrackRequest{
		TaskID: task.ID, TerminalID: "term1", Lat: 38.5, Lng: 102.3, Sequence: 1, Timestamp: clock.Now(),
	})

	result, _ := terminalSvc.SyncOffline(ctx, "term1")
	if result.ClockInsSynced != 1 || result.TracksSynced != 1 {
		t.Fatalf("first sync: expected 1 ci + 1 track, got ci=%d track=%d", result.ClockInsSynced, result.TracksSynced)
	}

	terminalSvc.SetOffline(ctx, "term1")
	terminalSvc.CacheClockIn(ctx, "term1", ClockInRequest{
		TaskID: task.ID, KeyPointID: "kp1", PatrollerID: "p1", RequestID: "ci-dedup", Timestamp: clock.Now(),
	})
	terminalSvc.CacheTrack(ctx, "term1", TrackRequest{
		TaskID: task.ID, TerminalID: "term1", Lat: 38.5, Lng: 102.3, Sequence: 1, Timestamp: clock.Now(),
	})

	result, _ = terminalSvc.SyncOffline(ctx, "term1")
	if result.ClockInsSynced != 1 || result.TracksSynced != 1 {
		t.Fatalf("second sync: expected 1 ci + 1 track (idempotent), got ci=%d track=%d", result.ClockInsSynced, result.TracksSynced)
	}

	stored, _ := st.GetTask(task.ID)
	if len(stored.ClockIns) != 1 {
		t.Fatalf("expected 1 clock-in (no duplicates), got %d", len(stored.ClockIns))
	}
	if len(stored.TrackPoints) != 1 {
		t.Fatalf("expected 1 track point (no duplicates), got %d", len(stored.TrackPoints))
	}
}

func TestCacheRejectsWhenOnline(t *testing.T) {
	st, taskSvc, _, clock := setupTaskTest(t)
	cfg := &config.Config{ClockInMaxGap: 6 * time.Hour}
	terminalSvc := NewTerminalService(st, cfg, taskSvc, clock)
	ctx := context.Background()

	terminalSvc.Register(ctx, "term1", "p1")

	err := terminalSvc.CacheClockIn(ctx, "term1", ClockInRequest{
		TaskID: "t1", KeyPointID: "kp1", PatrollerID: "p1", RequestID: "ci1",
	})
	if err == nil {
		t.Fatal("expected error when caching on online terminal")
	}
}
