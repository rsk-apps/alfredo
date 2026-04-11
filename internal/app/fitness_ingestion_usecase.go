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
	in := fitnesssvc.CreateWorkoutInput{
		ExternalID:      w.ExternalID,
		Type:            w.Type,
		StartedAt:       w.StartedAt,
		DurationSeconds: w.DurationSeconds,
		ActiveCalories:  w.ActiveCalories,
		TotalCalories:   w.TotalCalories,
		Source:          w.Source,
	}

	if w.HeartRate != nil {
		in.HeartRate = &fitnesssvc.CreateHeartRateInput{
			Avg:      w.HeartRate.Avg,
			Max:      w.HeartRate.Max,
			Zone1Pct: w.HeartRate.Zone1Pct,
			Zone2Pct: w.HeartRate.Zone2Pct,
			Zone3Pct: w.HeartRate.Zone3Pct,
			Zone4Pct: w.HeartRate.Zone4Pct,
			Zone5Pct: w.HeartRate.Zone5Pct,
		}
	}

	if w.Cardio != nil {
		in.Cardio = &fitnesssvc.CreateCardioInput{
			DistanceMeters:  w.Cardio.DistanceMeters,
			AvgPaceSecPerKm: w.Cardio.AvgPaceSecPerKm,
		}
	}

	if w.Strength != nil {
		exercises := make([]fitnesssvc.CreateExerciseInput, 0, len(w.Strength.Exercises))
		for _, ex := range w.Strength.Exercises {
			sets := make([]fitnesssvc.CreateSetInput, 0, len(ex.Sets))
			for _, s := range ex.Sets {
				sets = append(sets, fitnesssvc.CreateSetInput{
					SetNumber:    s.SetNumber,
					Reps:         s.Reps,
					WeightKg:     s.WeightKg,
					DurationSecs: s.DurationSecs,
					Notes:        s.Notes,
				})
			}
			exercises = append(exercises, fitnesssvc.CreateExerciseInput{
				Name:      ex.Name,
				Equipment: ex.Equipment,
				OrderIdx:  ex.OrderIdx,
				Sets:      sets,
			})
		}
		in.Strength = &fitnesssvc.CreateStrengthInput{Exercises: exercises}
	}

	saved, err := uc.workouts.Create(ctx, in)
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
