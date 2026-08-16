package store

import (
	"sync"
	"testing"

	"patrol-platform/internal/domain"
)

func TestStoreConcurrentTaskUpdates(t *testing.T) {
	s := New()
	s.SaveTask(&domain.Task{ID: "t1", Status: domain.TaskPending, Version: 0})

	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			s.UpdateTask("t1", func(t *domain.Task) error {
				t.Version++
				return nil
			})
		}()
	}
	wg.Wait()

	task, err := s.GetTask("t1")
	if err != nil {
		t.Fatal(err)
	}
	if task.Version != n {
		t.Fatalf("expected version %d after %d concurrent increments, got %d", n, n, task.Version)
	}
}

func TestStoreConcurrentReadsAndWrites(t *testing.T) {
	s := New()
	s.SaveTask(&domain.Task{ID: "t1", Status: domain.TaskPending, Version: 0})
	s.SaveAlarm(&domain.Alarm{ID: "a1", Status: domain.AlarmReported, Version: 0})

	const writers = 50
	const readers = 50
	var wg sync.WaitGroup

	wg.Add(writers)
	for i := 0; i < writers; i++ {
		go func() {
			defer wg.Done()
			s.UpdateTask("t1", func(t *domain.Task) error {
				t.Version++
				return nil
			})
			s.UpdateAlarm("a1", func(a *domain.Alarm) error {
				a.Version++
				return nil
			})
		}()
	}

	wg.Add(readers)
	for i := 0; i < readers; i++ {
		go func() {
			defer wg.Done()
			task, err := s.GetTask("t1")
			if err != nil {
				t.Errorf("task read error: %v", err)
				return
			}
			if task.ID != "t1" {
				t.Errorf("expected t1, got %s", task.ID)
			}
			_, err = s.GetAlarm("a1")
			if err != nil {
				t.Errorf("alarm read error: %v", err)
			}
		}()
	}

	wg.Wait()

	task, _ := s.GetTask("t1")
	if task.Version != writers {
		t.Fatalf("expected task version %d, got %d", writers, task.Version)
	}
	alarm, _ := s.GetAlarm("a1")
	if alarm.Version != writers {
		t.Fatalf("expected alarm version %d, got %d", writers, alarm.Version)
	}
}

func TestStoreCloneIsolation(t *testing.T) {
	s := New()
	original := &domain.Task{
		ID:     "t1",
		Status: domain.TaskPending,
		Audit:  []domain.AuditEntry{{Action: "dispatch"}},
	}
	s.SaveTask(original)

	got, _ := s.GetTask("t1")
	got.Audit[0].Action = "modified"

	got2, _ := s.GetTask("t1")
	if got2.Audit[0].Action != "dispatch" {
		t.Fatal("modifying the returned clone should not affect stored data")
	}
}
