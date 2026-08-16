package service

import (
	"context"

	"patrol-platform/internal/domain"
)

// GetTask retrieves a task by ID.
func (s *TaskService) GetTask(ctx context.Context, id string) (*domain.Task, error) {
	return s.store.GetTask(id)
}

// ListTasks returns all tasks.
func (s *TaskService) ListTasks(ctx context.Context) []*domain.Task {
	return s.store.ListTasks()
}

// GetAlarm retrieves an alarm by ID.
func (s *AlarmService) GetAlarm(ctx context.Context, id string) (*domain.Alarm, error) {
	return s.store.GetAlarm(id)
}

// ListAlarms returns all alarms.
func (s *AlarmService) ListAlarms(ctx context.Context) []*domain.Alarm {
	return s.store.ListAlarms()
}
