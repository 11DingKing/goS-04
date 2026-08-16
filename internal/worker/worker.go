package worker

import (
	"context"
	"log"
	"time"

	"patrol-platform/internal/config"
	"patrol-platform/internal/service"
	"patrol-platform/internal/store"
)

// Worker runs periodic background checks for alarm escalation, alarm
// accept-timeout rollback, and task claim-timeout rollback.
type Worker struct {
	store    *store.Store
	taskSvc  *service.TaskService
	alarmSvc *service.AlarmService
	cfg      *config.Config
}

func New(s *store.Store, taskSvc *service.TaskService, alarmSvc *service.AlarmService, cfg *config.Config) *Worker {
	return &Worker{store: s, taskSvc: taskSvc, alarmSvc: alarmSvc, cfg: cfg}
}

// Run starts the periodic check loop until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) {
	interval := w.cfg.EscalationCheckInterval
	if w.cfg.TimeoutCheckInterval < interval {
		interval = w.cfg.TimeoutCheckInterval
	}
	if interval < time.Second {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Println("worker shutting down")
			return
		case <-ticker.C:
			w.Tick(ctx)
		}
	}
}

// Tick performs one round of escalation and timeout checks.
func (w *Worker) Tick(ctx context.Context) {
	if n, err := w.alarmSvc.EscalateOverdueAlarms(ctx); err != nil {
		log.Printf("escalation check error: %v", err)
	} else if n > 0 {
		log.Printf("escalated %d overdue level-1 alarms", n)
	}

	if n, err := w.alarmSvc.RollbackUnacceptedAlarms(ctx); err != nil {
		log.Printf("alarm rollback check error: %v", err)
	} else if n > 0 {
		log.Printf("rolled back %d unaccepted alarms for reassignment", n)
	}

	if n, err := w.taskSvc.RollbackTimedOutTasks(ctx); err != nil {
		log.Printf("task timeout check error: %v", err)
	} else if n > 0 {
		log.Printf("rolled back %d timed-out tasks to dispatch pool", n)
	}
}
