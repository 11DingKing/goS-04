package service

import (
	"context"
	"testing"
	"time"

	"patrol-platform/internal/domain"
)

// TestUnresolvedLevelOneAlarmsEscalateAfterTimeout drives three level-1 alarms
// past the two-hour response window: one that a handler accepted, one that a
// handler is still working on, and one that was already resolved in time. Both
// unresolved alarms must be escalated to the station chief; the resolved one
// must be left untouched.
func TestUnresolvedLevelOneAlarmsEscalateAfterTimeout(t *testing.T) {
	_, _, alarmSvc, clock := setupTaskTest(t)
	ctx := context.Background()

	accepted := reviewedLevelOneAlarm(t, alarmSvc, "ridge fire", "h1")

	handling := reviewedLevelOneAlarm(t, alarmSvc, "poaching snares", "h2")
	if _, err := alarmSvc.StartHandling(ctx, handling, "h2"); err != nil {
		t.Fatal(err)
	}

	done := reviewedLevelOneAlarm(t, alarmSvc, "smoke near station", "h3")
	if _, err := alarmSvc.Resolve(ctx, done, "h3", "handled on site"); err != nil {
		t.Fatal(err)
	}

	clock.Advance(2*time.Hour + time.Minute)

	n, err := alarmSvc.EscalateOverdueAlarms(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("expected the 2 unresolved level-1 alarms to escalate, got %d", n)
	}

	for _, id := range []string{accepted, handling} {
		stored, err := alarmSvc.GetAlarm(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if stored.Status != domain.AlarmEscalated {
			t.Errorf("alarm %s: status = %s, want escalated", id, stored.Status)
		}
		if !stored.Escalated {
			t.Errorf("alarm %s: escalated flag not set", id)
		}
		if stored.EscalatedAt.IsZero() {
			t.Errorf("alarm %s: escalatedAt not recorded", id)
		}
	}

	resolved, err := alarmSvc.GetAlarm(ctx, done)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Status != domain.AlarmResolved {
		t.Errorf("resolved alarm must stay resolved, got %s", resolved.Status)
	}

	if n, err := alarmSvc.EscalateOverdueAlarms(ctx); err != nil || n != 0 {
		t.Fatalf("escalation must be idempotent, got n=%d err=%v", n, err)
	}
}

func reviewedLevelOneAlarm(t *testing.T, alarmSvc *AlarmService, desc, handler string) string {
	t.Helper()
	ctx := context.Background()
	a, err := alarmSvc.Report(ctx, "p1", "task-1", domain.AlarmTypeFire, desc, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := alarmSvc.Review(ctx, a.ID, "chief", domain.LevelOne); err != nil {
		t.Fatal(err)
	}
	if _, err := alarmSvc.Assign(ctx, a.ID, "chief", handler); err != nil {
		t.Fatal(err)
	}
	if _, err := alarmSvc.Accept(ctx, a.ID, handler); err != nil {
		t.Fatal(err)
	}
	return a.ID
}
