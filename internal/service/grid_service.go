package service

import (
	"context"

	"patrol-platform/internal/domain"
	"patrol-platform/internal/store"
)

// GridService manages patrol grid definitions and their patroller assignments.
type GridService struct {
	store *store.Store
}

func NewGridService(s *store.Store) *GridService {
	return &GridService{store: s}
}

func (s *GridService) CreateGrid(ctx context.Context, grid *domain.Grid) (*domain.Grid, error) {
	s.store.SaveGrid(grid)
	return grid, nil
}

func (s *GridService) GetGrid(ctx context.Context, id string) (*domain.Grid, error) {
	return s.store.GetGrid(id)
}

func (s *GridService) ListGrids(ctx context.Context) ([]*domain.Grid, error) {
	return s.store.ListGrids(), nil
}
