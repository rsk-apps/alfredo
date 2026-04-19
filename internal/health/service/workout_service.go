package service

import (
	"context"
	"fmt"
	"time"

	"github.com/rafaelsoares/alfredo/internal/health/domain"
	"github.com/rafaelsoares/alfredo/internal/health/port"
)

type WorkoutService struct {
	repo          port.WorkoutRepository
	rawImportRepo port.RawImportRepository
}

func NewWorkoutService(repo port.WorkoutRepository, rawImportRepo port.RawImportRepository) *WorkoutService {
	return &WorkoutService{
		repo:          repo,
		rawImportRepo: rawImportRepo,
	}
}

func (s *WorkoutService) Import(ctx context.Context, sessions []domain.WorkoutSession, payload string, importedAt time.Time) (int, error) {
	count, err := s.repo.BulkUpsert(ctx, sessions)
	if err != nil {
		return 0, fmt.Errorf("import workouts: %w", err)
	}

	// Store raw payload for audit trail
	if err := s.rawImportRepo.Store(ctx, "workouts", payload, importedAt); err != nil {
		return 0, fmt.Errorf("store raw workouts import: %w", err)
	}

	return count, nil
}

func (s *WorkoutService) List(ctx context.Context, from, to time.Time) ([]domain.WorkoutSession, error) {
	sessions, err := s.repo.List(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("list workouts: %w", err)
	}
	return sessions, nil
}
