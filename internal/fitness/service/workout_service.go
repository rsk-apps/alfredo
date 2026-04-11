package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
	"github.com/rafaelsoares/alfredo/internal/fitness/port"
)

type CreateHeartRateInput struct {
	Avg      *float64
	Max      *float64
	Zone1Pct *float64
	Zone2Pct *float64
	Zone3Pct *float64
	Zone4Pct *float64
	Zone5Pct *float64
}

type CreateCardioInput struct {
	DistanceMeters  *float64
	AvgPaceSecPerKm *float64
}

type CreateSetInput struct {
	SetNumber    int
	Reps         *int
	WeightKg     *float64
	DurationSecs *int
	Notes        *string
}

type CreateExerciseInput struct {
	Name      string
	Equipment *string
	OrderIdx  int
	Sets      []CreateSetInput
}

type CreateStrengthInput struct {
	Exercises []CreateExerciseInput
}

type CreateWorkoutInput struct {
	ExternalID      string
	Type            string
	StartedAt       time.Time
	DurationSeconds int
	ActiveCalories  float64
	TotalCalories   float64
	HeartRate       *CreateHeartRateInput
	Cardio          *CreateCardioInput
	Strength        *CreateStrengthInput
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

	w := domain.Workout{
		ID:              uuid.New().String(),
		ExternalID:      in.ExternalID,
		Type:            in.Type,
		StartedAt:       in.StartedAt.UTC(),
		DurationSeconds: in.DurationSeconds,
		ActiveCalories:  in.ActiveCalories,
		TotalCalories:   in.TotalCalories,
		Source:          in.Source,
		CreatedAt:       time.Now().UTC(),
	}

	if in.HeartRate != nil {
		w.HeartRate = &domain.WorkoutHeartRate{
			Avg:      in.HeartRate.Avg,
			Max:      in.HeartRate.Max,
			Zone1Pct: in.HeartRate.Zone1Pct,
			Zone2Pct: in.HeartRate.Zone2Pct,
			Zone3Pct: in.HeartRate.Zone3Pct,
			Zone4Pct: in.HeartRate.Zone4Pct,
			Zone5Pct: in.HeartRate.Zone5Pct,
		}
	}

	if in.Cardio != nil {
		w.Cardio = &domain.CardioData{
			DistanceMeters:  in.Cardio.DistanceMeters,
			AvgPaceSecPerKm: in.Cardio.AvgPaceSecPerKm,
		}
	}

	if in.Strength != nil {
		exercises, err := mapExercises(w.ID, in.Strength.Exercises)
		if err != nil {
			return nil, err
		}
		w.Strength = &domain.StrengthData{Exercises: exercises}
	}

	return s.repo.Create(ctx, w)
}

func mapExercises(workoutID string, inputs []CreateExerciseInput) ([]domain.WorkoutExercise, error) {
	exercises := make([]domain.WorkoutExercise, 0, len(inputs))
	for _, ei := range inputs {
		if ei.Name == "" {
			return nil, fmt.Errorf("%w: exercise name is required", domain.ErrValidation)
		}
		if len(ei.Sets) == 0 {
			return nil, fmt.Errorf("%w: exercise %q must have at least one set", domain.ErrValidation, ei.Name)
		}
		exerciseID := uuid.New().String()
		sets := make([]domain.WorkoutSet, 0, len(ei.Sets))
		for _, si := range ei.Sets {
			if si.SetNumber <= 0 {
				return nil, fmt.Errorf("%w: set_number must be greater than zero", domain.ErrValidation)
			}
			sets = append(sets, domain.WorkoutSet{
				ID:           uuid.New().String(),
				ExerciseID:   exerciseID,
				SetNumber:    si.SetNumber,
				Reps:         si.Reps,
				WeightKg:     si.WeightKg,
				DurationSecs: si.DurationSecs,
				Notes:        si.Notes,
			})
		}
		exercises = append(exercises, domain.WorkoutExercise{
			ID:        exerciseID,
			WorkoutID: workoutID,
			Name:      ei.Name,
			Equipment: ei.Equipment,
			OrderIdx:  ei.OrderIdx,
			Sets:      sets,
		})
	}
	return exercises, nil
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
