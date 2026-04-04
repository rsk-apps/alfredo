// internal/app/fitness_ingestion_usecase_test.go
package app_test

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/rafaelsoares/alfredo/internal/app"
	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
	fitnesssvc "github.com/rafaelsoares/alfredo/internal/fitness/service"
)

type stubFitnessWorkoutService struct {
	workout *domain.Workout
}

func (s *stubFitnessWorkoutService) Create(_ context.Context, in fitnesssvc.CreateWorkoutInput) (*domain.Workout, error) {
	if s.workout != nil {
		return s.workout, nil
	}
	return &domain.Workout{ID: "w1", ExternalID: in.ExternalID, Type: in.Type, Source: in.Source}, nil
}
func (s *stubFitnessWorkoutService) GetByID(_ context.Context, _ string) (*domain.Workout, error) {
	return s.workout, nil
}
func (s *stubFitnessWorkoutService) List(_ context.Context, _, _ *time.Time) ([]domain.Workout, error) {
	return nil, nil
}
func (s *stubFitnessWorkoutService) Delete(_ context.Context, _ string) error { return nil }

func TestFitnessIngestionUseCase_IngestWorkout_EmitsEvent(t *testing.T) {
	spy := &spyEmitter{}
	uc := app.NewFitnessIngestionUseCase(&stubFitnessWorkoutService{}, spy, zap.NewNop())

	_, err := uc.IngestWorkout(context.Background(), domain.Workout{
		ExternalID: "ext-1", Type: "run", Source: "apple_fitness",
		StartedAt: time.Now(), DurationSeconds: 3600, ActiveCalories: 400, TotalCalories: 500,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(spy.events) != 1 || spy.events[0] != "fitness.workout.saved" {
		t.Errorf("events = %v, want [fitness.workout.saved]", spy.events)
	}
}

func TestFitnessIngestionUseCase_IngestWorkoutBatch_EmitsOneEventPerWorkout(t *testing.T) {
	spy := &spyEmitter{}
	uc := app.NewFitnessIngestionUseCase(&stubFitnessWorkoutService{}, spy, zap.NewNop())

	workouts := []domain.Workout{
		{ExternalID: "ext-1", Type: "run", Source: "apple_fitness", StartedAt: time.Now(), DurationSeconds: 1, ActiveCalories: 1, TotalCalories: 1},
		{ExternalID: "ext-2", Type: "cycle", Source: "apple_fitness", StartedAt: time.Now(), DurationSeconds: 1, ActiveCalories: 1, TotalCalories: 1},
	}
	_, err := uc.IngestWorkoutBatch(context.Background(), workouts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(spy.events) != 2 {
		t.Errorf("got %d events, want 2", len(spy.events))
	}
	for _, e := range spy.events {
		if e != "fitness.workout.saved" {
			t.Errorf("unexpected event %q", e)
		}
	}
}
