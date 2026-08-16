package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"patrol-platform/internal/domain"
)

func TestAlarmReportReviewAcceptResolve(t *testing.T) {
	_, _, alarmSvc, _ := setupTaskTest(t)
	ctx := context.Background()

	alarm, err := alarmSvc.Report(ctx, "p1", "task-1", domain.AlarmTypeFire, "Wildfire near north ridge", []string{"img1.jpg"})
	if err != nil {
		t.Fatal(err)
	}
	if alarm.Status != domain.AlarmReported {
		t.Fatalf("expected reported, got %s", alarm.Status)
	}
	if alarm.Type != domain.AlarmTypeFire {
		t.Fatalf("expected fire type, got %s", alarm.Type)
	}

	alarm, err = alarmSvc.Review(ctx, alarm.ID, "chief", domain.LevelOne)
	if err != nil {
		t.Fatal(err)
	}
	if alarm.Status != domain.AlarmReviewed {
		t.Fatalf("expected reviewed, got %s", alarm.Status)
	}
	if alarm.Level != domain.LevelOne {
		t.Fatalf("expected level 1, got %d", alarm.Level)
	}

	alarm, err = alarmSvc.Assign(ctx, alarm.ID, "chief", "h1")
	if err != nil {
		t.Fatal(err)
	}
	if alarm.HandlerID != "h1" {
		t.Fatalf("expected handler h1, got %s", alarm.HandlerID)
	}

	alarm, err = alarmSvc.Accept(ctx, alarm.ID, "h1")
	if err != nil {
		t.Fatal(err)
	}
	if alarm.Status != domain.AlarmAccepted {
		t.Fatalf("expected accepted, got %s", alarm.Status)
	}

	alarm, err = alarmSvc.StartHandling(ctx, alarm.ID, "h1")
	if err != nil {
		t.Fatal(err)
	}
	if alarm.Status != domain.AlarmInProgress {
		t.Fatalf("expected in_progress, got %s", alarm.Status)
	}

	alarm, err = alarmSvc.Resolve(ctx, alarm.ID, "h1", "fire extinguished at 14:30")
	if err != nil {
		t.Fatal(err)
	}
	if alarm.Status != domain.AlarmResolved {
		t.Fatalf("expected resolved, got %s", alarm.Status)
	}
	if alarm.ResolvedAt.IsZero() {
		t.Fatal("expected resolvedAt to be set")
	}
}

func TestConcurrentAcceptEarliestWins(t *testing.T) {
	_, _, alarmSvc, _ := setupTaskTest(t)
	ctx := context.Background()

	alarm, _ := alarmSvc.Report(ctx, "p1", "task-1", domain.AlarmTypePoaching, "snare traps found", nil)
	alarmSvc.Review(ctx, alarm.ID, "chief", domain.LevelTwo)
	alarmSvc.Assign(ctx, alarm.ID, "chief", "h1")

	start := make(chan struct{})
	var wg sync.WaitGroup
	var err1, err2 error
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		_, err1 = alarmSvc.Accept(ctx, alarm.ID, "h1")
	}()
	go func() {
		defer wg.Done()
		<-start
		_, err2 = alarmSvc.Accept(ctx, alarm.ID, "h2")
	}()
	close(start)
	wg.Wait()

	if (err1 == nil) == (err2 == nil) {
		t.Fatalf("expected exactly one accept to succeed, got err1=%v err2=%v", err1, err2)
	}

	stored, _ := alarmSvc.GetAlarm(ctx, alarm.ID)
	if stored.Status != domain.AlarmAccepted {
		t.Fatalf("expected accepted, got %s", stored.Status)
	}

	loserErr := err1
	if err1 == nil {
		loserErr = err2
	}
	if !errors.Is(loserErr, domain.ErrAlreadyAccepted) {
		t.Fatalf("expected loser to get ErrAlreadyAccepted, got %v", loserErr)
	}
}

func TestAlarmEscalationOverdue(t *testing.T) {
	_, _, alarmSvc, clock := setupTaskTest(t)
	ctx := context.Background()

	alarm, _ := alarmSvc.Report(ctx, "p1", "task-1", domain.AlarmTypeFire, "wildfire", nil)
	alarmSvc.Review(ctx, alarm.ID, "chief", domain.LevelOne)

	n, _ := alarmSvc.EscalateOverdueAlarms(ctx)
	if n != 0 {
		t.Fatalf("expected 0 escalations before timeout, got %d", n)
	}

	clock.Advance(2*time.Hour + time.Minute)

	n, _ = alarmSvc.EscalateOverdueAlarms(ctx)
	if n != 1 {
		t.Fatalf("expected 1 escalation after timeout, got %d", n)
	}

	stored, _ := alarmSvc.GetAlarm(ctx, alarm.ID)
	if stored.Status != domain.AlarmEscalated {
		t.Fatalf("expected escalated, got %s", stored.Status)
	}
	if !stored.Escalated {
		t.Fatal("expected escalated flag set")
	}

	n, _ = alarmSvc.EscalateOverdueAlarms(ctx)
	if n != 0 {
		t.Fatalf("should not escalate twice, got %d", n)
	}
}

func TestAlarmRollbackUnaccepted(t *testing.T) {
	_, _, alarmSvc, clock := setupTaskTest(t)
	ctx := context.Background()

	alarm, _ := alarmSvc.Report(ctx, "p1", "task-1", domain.AlarmTypeGrazing, "cattle in buffer zone", nil)
	alarmSvc.Review(ctx, alarm.ID, "chief", domain.LevelThree)
	alarmSvc.Assign(ctx, alarm.ID, "chief", "h1")

	clock.Advance(31 * time.Minute)

	n, _ := alarmSvc.RollbackUnacceptedAlarms(ctx)
	if n != 1 {
		t.Fatalf("expected 1 rollback, got %d", n)
	}

	stored, _ := alarmSvc.GetAlarm(ctx, alarm.ID)
	if stored.Status != domain.AlarmReviewed {
		t.Fatalf("expected reviewed after rollback, got %s", stored.Status)
	}
	if stored.HandlerID != "" {
		t.Fatal("handler should be cleared on rollback")
	}
}
