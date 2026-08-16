package store

import (
	"sync"

	"patrol-platform/internal/domain"
)

// Store provides thread-safe in-memory persistence for all domain aggregates.
// Each entity type uses its own RWMutex so reads of one type do not block
// writes of another, while avoiding reentrant-lock deadlocks in service code.
type Store struct {
	gridMu sync.RWMutex
	grids  map[string]*domain.Grid

	taskMu sync.RWMutex
	tasks  map[string]*domain.Task

	alarmMu sync.RWMutex
	alarms  map[string]*domain.Alarm

	termMu    sync.RWMutex
	terminals map[string]*domain.Terminal
}

// New creates an empty store.
func New() *Store {
	return &Store{
		grids:     make(map[string]*domain.Grid),
		tasks:     make(map[string]*domain.Task),
		alarms:    make(map[string]*domain.Alarm),
		terminals: make(map[string]*domain.Terminal),
	}
}

// --- Grid ---

func (s *Store) SaveGrid(g *domain.Grid) {
	s.gridMu.Lock()
	defer s.gridMu.Unlock()
	s.grids[g.ID] = g.Clone()
}

func (s *Store) GetGrid(id string) (*domain.Grid, error) {
	s.gridMu.RLock()
	defer s.gridMu.RUnlock()
	g, ok := s.grids[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return g.Clone(), nil
}

func (s *Store) ListGrids() []*domain.Grid {
	s.gridMu.RLock()
	defer s.gridMu.RUnlock()
	out := make([]*domain.Grid, 0, len(s.grids))
	for _, g := range s.grids {
		out = append(out, g.Clone())
	}
	return out
}

// --- Task ---

func (s *Store) SaveTask(t *domain.Task) {
	s.taskMu.Lock()
	defer s.taskMu.Unlock()
	s.tasks[t.ID] = t.Clone()
}

func (s *Store) GetTask(id string) (*domain.Task, error) {
	s.taskMu.RLock()
	defer s.taskMu.RUnlock()
	t, ok := s.tasks[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return t.Clone(), nil
}

func (s *Store) ListTasks() []*domain.Task {
	s.taskMu.RLock()
	defer s.taskMu.RUnlock()
	out := make([]*domain.Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		out = append(out, t.Clone())
	}
	return out
}

// UpdateTask atomically reads, mutates, and persists a task under a write lock.
// The callback receives a clone; if it returns an error the change is discarded.
func (s *Store) UpdateTask(id string, fn func(t *domain.Task) error) (*domain.Task, error) {
	s.taskMu.Lock()
	defer s.taskMu.Unlock()
	t, ok := s.tasks[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	working := t.Clone()
	if err := fn(working); err != nil {
		return nil, err
	}
	s.tasks[id] = working
	return working.Clone(), nil
}

// --- Alarm ---

func (s *Store) SaveAlarm(a *domain.Alarm) {
	s.alarmMu.Lock()
	defer s.alarmMu.Unlock()
	s.alarms[a.ID] = a.Clone()
}

func (s *Store) GetAlarm(id string) (*domain.Alarm, error) {
	s.alarmMu.RLock()
	defer s.alarmMu.RUnlock()
	a, ok := s.alarms[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return a.Clone(), nil
}

func (s *Store) ListAlarms() []*domain.Alarm {
	s.alarmMu.RLock()
	defer s.alarmMu.RUnlock()
	out := make([]*domain.Alarm, 0, len(s.alarms))
	for _, a := range s.alarms {
		out = append(out, a.Clone())
	}
	return out
}

// UpdateAlarm atomically reads, mutates, and persists an alarm under a write lock.
func (s *Store) UpdateAlarm(id string, fn func(a *domain.Alarm) error) (*domain.Alarm, error) {
	s.alarmMu.Lock()
	defer s.alarmMu.Unlock()
	a, ok := s.alarms[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	working := a.Clone()
	if err := fn(working); err != nil {
		return nil, err
	}
	s.alarms[id] = working
	return working.Clone(), nil
}

// --- Terminal ---

func (s *Store) SaveTerminal(term *domain.Terminal) {
	s.termMu.Lock()
	defer s.termMu.Unlock()
	s.terminals[term.ID] = term.Clone()
}

func (s *Store) GetTerminal(id string) (*domain.Terminal, error) {
	s.termMu.RLock()
	defer s.termMu.RUnlock()
	term, ok := s.terminals[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return term.Clone(), nil
}

func (s *Store) ListTerminals() []*domain.Terminal {
	s.termMu.RLock()
	defer s.termMu.RUnlock()
	out := make([]*domain.Terminal, 0, len(s.terminals))
	for _, term := range s.terminals {
		out = append(out, term.Clone())
	}
	return out
}

// UpdateTerminal atomically reads, mutates, and persists a terminal under a write lock.
func (s *Store) UpdateTerminal(id string, fn func(term *domain.Terminal) error) (*domain.Terminal, error) {
	s.termMu.Lock()
	defer s.termMu.Unlock()
	term, ok := s.terminals[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	working := term.Clone()
	if err := fn(working); err != nil {
		return nil, err
	}
	s.terminals[id] = working
	return working.Clone(), nil
}
