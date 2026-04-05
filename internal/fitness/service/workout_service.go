package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
	"github.com/rafaelsoares/alfredo/internal/fitness/port"
)

type CreateWorkoutInput struct {
	ExternalID      string
	Type            string
	StartedAt       time.Time
	DurationSeconds int
	ActiveCalories  float64
	TotalCalories   float64
	DistanceMeters  *float64
	AvgPaceSecPerKm *float64
	AvgHeartRate    *float64
	MaxHeartRate    *float64
	HRZone1Pct      *float64
	HRZone2Pct      *float64
	HRZone3Pct      *float64
	HRZone4Pct      *float64
	HRZone5Pct      *float64
	Source          string
}

type WorkoutService struct {
	repo port.WorkoutRepository
}

func NewWorkoutService(repo port.WorkoutRepository) *WorkoutService {
	return &WorkoutService{repo: repo}
}

func (s *WorkoutService) Create(ctx context.Context, in CreateWorkoutInput) (*domain.Workout, error) {
	if in.ExternalID == "" {
		return nil, fmt.Errorf("%w: external_id is required", domain.ErrValidation)
	}
	if in.Type == "" {
		return nil, fmt.Errorf("%w: type is required", domain.ErrValidation)
	}
	if in.Source == "" {
		return nil, fmt.Errorf("%w: source is required", domain.ErrValidation)
	}
	if in.DurationSeconds <= 0 {
		return nil, fmt.Errorf("%w: duration_seconds must be greater than zero", domain.ErrValidation)
	}
	return s.repo.Create(ctx, domain.Workout{
		ID:              uuid.New().String(),
		ExternalID:      in.ExternalID,
		Type:            in.Type,
		StartedAt:       in.StartedAt.UTC(),
		DurationSeconds: in.DurationSeconds,
		ActiveCalories:  in.ActiveCalories,
		TotalCalories:   in.TotalCalories,
		DistanceMeters:  in.DistanceMeters,
		AvgPaceSecPerKm: in.AvgPaceSecPerKm,
		AvgHeartRate:    in.AvgHeartRate,
		MaxHeartRate:    in.MaxHeartRate,
		HRZone1Pct:      in.HRZone1Pct,
		HRZone2Pct:      in.HRZone2Pct,
		HRZone3Pct:      in.HRZone3Pct,
		HRZone4Pct:      in.HRZone4Pct,
		HRZone5Pct:      in.HRZone5Pct,
		Source:          in.Source,
		CreatedAt:       time.Now().UTC(),
	})
}

func (s *WorkoutService) GetByID(ctx context.Context, id string) (*domain.Workout, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *WorkoutService) List(ctx context.Context, from, to *time.Time) ([]domain.Workout, error) {
	return s.repo.List(ctx, from, to)
}

func (s *WorkoutService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}
