package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
	"github.com/rafaelsoares/alfredo/internal/fitness/service"
)

type mockWorkoutRepo struct {
	workout *domain.Workout
	err     error
}

func (m *mockWorkoutRepo) Create(_ context.Context, w domain.Workout) (*domain.Workout, error) {
	return &w, m.err
}
func (m *mockWorkoutRepo) GetByID(_ context.Context, _ string) (*domain.Workout, error) {
	return m.workout, m.err
}
func (m *mockWorkoutRepo) List(_ context.Context, _, _ *time.Time) ([]domain.Workout, error) {
	if m.workout != nil {
		return []domain.Workout{*m.workout}, m.err
	}
	return nil, m.err
}
func (m *mockWorkoutRepo) Delete(_ context.Context, _ string) error { return m.err }

func TestWorkoutService_Create_AssignsID(t *testing.T) {
	svc := service.NewWorkoutService(&mockWorkoutRepo{})
	w, err := svc.Create(context.Background(), service.CreateWorkoutInput{
		ExternalID: "ext-1", Type: "run", Source: "apple_fitness",
		StartedAt: time.Now(), DurationSeconds: 3600, ActiveCalories: 400, TotalCalories: 500,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.ID == "" {
		t.Error("expected ID to be assigned")
	}
}

func TestWorkoutService_Create_ValidationErrors(t *testing.T) {
	svc := service.NewWorkoutService(&mockWorkoutRepo{})
	cases := []struct {
		name  string
		input service.CreateWorkoutInput
	}{
		{"missing external_id", service.CreateWorkoutInput{Type: "run", Source: "apple_fitness", StartedAt: time.Now(), DurationSeconds: 1, ActiveCalories: 1, TotalCalories: 1}},
		{"missing type", service.CreateWorkoutInput{ExternalID: "e1", Source: "apple_fitness", StartedAt: time.Now(), DurationSeconds: 1, ActiveCalories: 1, TotalCalories: 1}},
		{"missing source", service.CreateWorkoutInput{ExternalID: "e1", Type: "run", StartedAt: time.Now(), DurationSeconds: 1, ActiveCalories: 1, TotalCalories: 1}},
		{"zero duration", service.CreateWorkoutInput{ExternalID: "e1", Type: "run", Source: "apple_fitness", StartedAt: time.Now(), ActiveCalories: 1, TotalCalories: 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Create(context.Background(), tc.input)
			if !errors.Is(err, domain.ErrValidation) {
				t.Errorf("got %v, want ErrValidation", err)
			}
		})
	}
}

func TestWorkoutService_Create_PropagatesDuplicateError(t *testing.T) {
	svc := service.NewWorkoutService(&mockWorkoutRepo{err: domain.ErrAlreadyExists})
	_, err := svc.Create(context.Background(), service.CreateWorkoutInput{
		ExternalID: "ext-1", Type: "run", Source: "apple_fitness",
		StartedAt: time.Now(), DurationSeconds: 60, ActiveCalories: 100, TotalCalories: 120,
	})
	if !errors.Is(err, domain.ErrAlreadyExists) {
		t.Errorf("got %v, want ErrAlreadyExists", err)
	}
}
