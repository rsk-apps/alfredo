// internal/app/fitness_ingestion_usecase.go
package app

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"

	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
	fitnesssvc "github.com/rafaelsoares/alfredo/internal/fitness/service"
	"github.com/rafaelsoares/alfredo/internal/webhook"
)

// FitnessIngestionUseCase saves workouts and emits fitness.workout.saved.
// It also satisfies fitness/port.WorkoutIngester so external adapters (future Apple Fitness
// spike) can call IngestWorkout directly without going through HTTP.
type FitnessIngestionUseCase struct {
	workouts FitnessWorkoutServicer
	emitter  webhook.EventEmitter
	logger   *zap.Logger
}

func NewFitnessIngestionUseCase(
	workouts FitnessWorkoutServicer,
	emitter webhook.EventEmitter,
	logger *zap.Logger,
) *FitnessIngestionUseCase {
	return &FitnessIngestionUseCase{workouts: workouts, emitter: emitter, logger: logger}
}

func (uc *FitnessIngestionUseCase) IngestWorkout(ctx context.Context, w domain.Workout) (*domain.Workout, error) {
	saved, err := uc.workouts.Create(ctx, fitnesssvc.CreateWorkoutInput{
		ExternalID:      w.ExternalID,
		Type:            w.Type,
		StartedAt:       w.StartedAt,
		DurationSeconds: w.DurationSeconds,
		ActiveCalories:  w.ActiveCalories,
		TotalCalories:   w.TotalCalories,
		DistanceMeters:  w.DistanceMeters,
		AvgPaceSecPerKm: w.AvgPaceSecPerKm,
		AvgHeartRate:    w.AvgHeartRate,
		MaxHeartRate:    w.MaxHeartRate,
		HRZone1Pct:      w.HRZone1Pct,
		HRZone2Pct:      w.HRZone2Pct,
		HRZone3Pct:      w.HRZone3Pct,
		HRZone4Pct:      w.HRZone4Pct,
		HRZone5Pct:      w.HRZone5Pct,
		Source:          w.Source,
	})
	if err != nil {
		return nil, err
	}
	uc.emitter.Emit(ctx, "fitness.workout.saved", fitnessWorkoutSavedPayload{
		WorkoutID:       saved.ID,
		Type:            saved.Type,
		StartedAt:       saved.StartedAt,
		DurationSeconds: saved.DurationSeconds,
		ActiveCalories:  saved.ActiveCalories,
		Source:          saved.Source,
	})
	return saved, nil
}

func (uc *FitnessIngestionUseCase) IngestWorkoutBatch(ctx context.Context, ws []domain.Workout) ([]domain.Workout, error) {
	var saved []domain.Workout
	for _, w := range ws {
		s, err := uc.IngestWorkout(ctx, w)
		if err != nil {
			if errors.Is(err, domain.ErrAlreadyExists) {
				// Workout already ingested — skip silently to make batches idempotent.
				continue
			}
			return nil, err
		}
		saved = append(saved, *s)
	}
	return saved, nil
}

func (uc *FitnessIngestionUseCase) GetByID(ctx context.Context, id string) (*domain.Workout, error) {
	return uc.workouts.GetByID(ctx, id)
}

func (uc *FitnessIngestionUseCase) List(ctx context.Context, from, to *time.Time) ([]domain.Workout, error) {
	return uc.workouts.List(ctx, from, to)
}

func (uc *FitnessIngestionUseCase) Delete(ctx context.Context, id string) error {
	return uc.workouts.Delete(ctx, id)
}

// --- payload types ---

type fitnessWorkoutSavedPayload struct {
	WorkoutID       string    `json:"workout_id"`
	Type            string    `json:"type"`
	StartedAt       time.Time `json:"started_at"`
	DurationSeconds int       `json:"duration_seconds"`
	ActiveCalories  float64   `json:"active_calories"`
	Source          string    `json:"source"`
}
