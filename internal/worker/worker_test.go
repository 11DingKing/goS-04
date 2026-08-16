package worker

import (
	"context"
	"testing"
	"time"

	"patrol-platform/internal/config"
	"patrol-platform/internal/domain"
	"patrol-platform/internal/service"
	"patrol-platform/internal/store"
)

func setupWorkerTest(t *testing.T) (*store.Store, *service.TaskService, *service.AlarmService, *service.FakeClock, *Worker) {
	t.Helper()
	cfg := &config.Config{
		ClockInMaxGap: 6 * time.Hour,
		Level1Timeout: 2 * time.Hour,
		ClaimTimeout:  30 * time.Minute,
		AcceptTimeout: 30 * time.Minute,
	}
	clock := service.NewFakeClock(time.Date(2026, 8, 16, 8, 0, 0, 0, time.UTC))
	st := store.New()
	st.SaveGrid(&domain.Grid{ID: "g1", Name: "Grid", Zone: domain.ZoneBuffer, PatrollerIDs: []string{"p1"}})
	taskSvc := service.NewTaskService(st, cfg, clock)
	alarmSvc := service.NewAlarmService(st, cfg, clock)
	wk := New(st, taskSvc, alarmSvc, cfg)
	return st, taskSvc, alarmSvc, clock, wk
}

func TestWorkerEscalatesOverdueAlarms(t *testing.T) {
	st, _, alarmSvc, clock, wk := setupWorkerTest(t)
	ctx := context.Background()

	alarm, _ := alarmSvc.Report(ctx, "p1", "t1", domain.AlarmTypeFire, "wildfire", nil)
	alarmSvc.Review(ctx, alarm.ID, "chief", domain.LevelOne)
	alarmSvc.Assign(ctx, alarm.ID, "chief", "h1")

	clock.Advance(2*time.Hour + time.Minute)

	wk.Tick(ctx)

	stored, _ := st.GetAlarm(alarm.ID)
	if stored.Status != domain.AlarmEscalated {
		t.Fatalf("expected escalated after worker tick, got %s", stored.Status)
	}
	if !stored.Escalated {
		t.Fatal("expected escalated flag")
	}
}

func TestWorkerRollsBackTimedOutTasks(t *testing.T) {
	st, taskSvc, _, clock, wk := setupWorkerTest(t)
	ctx := context.Background()

	task, _ := taskSvc.Dispatch(ctx, "d1", "g1", "Patrol", domain.PriorityRoutine, nil)

	clock.Advance(31 * time.Minute)

	wk.Tick(ctx)

	stored, _ := st.GetTask(task.ID)
	if stored.Status != domain.TaskPending {
		t.Fatalf("expected pending after worker rollback, got %s", stored.Status)
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
}

func TestWorkerRollsBackUnacceptedAlarms(t *testing.T) {
	st, _, alarmSvc, clock, wk := setupWorkerTest(t)
	ctx := context.Background()

	alarm, _ := alarmSvc.Report(ctx, "p1", "t1", domain.AlarmTypeGrazing, "grazing", nil)
	alarmSvc.Review(ctx, alarm.ID, "chief", domain.LevelTwo)
	alarmSvc.Assign(ctx, alarm.ID, "chief", "h1")

	clock.Advance(31 * time.Minute)

	wk.Tick(ctx)

	stored, _ := st.GetAlarm(alarm.ID)
	if stored.Status != domain.AlarmReviewed {
		t.Fatalf("expected reviewed after worker rollback, got %s", stored.Status)
	}
	if stored.HandlerID != "" {
		t.Fatal("handler should be cleared")
	}
}
