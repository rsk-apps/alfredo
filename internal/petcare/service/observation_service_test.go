package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
	"github.com/rafaelsoares/alfredo/internal/petcare/service"
)

type mockObservationRepo struct {
	observations []domain.Observation
	stored       domain.Observation
	err          error
}

func (m *mockObservationRepo) Create(_ context.Context, observation domain.Observation) (*domain.Observation, error) {
	m.stored = observation
	return &observation, m.err
}

func (m *mockObservationRepo) ListByPet(_ context.Context, _ string) ([]domain.Observation, error) {
	return m.observations, m.err
}

func (m *mockObservationRepo) GetByID(_ context.Context, _, _ string) (*domain.Observation, error) {
	if len(m.observations) == 0 {
		return nil, domain.ErrNotFound
	}
	return &m.observations[0], m.err
}

func TestObservationService_Create_AssignsIDAndCreatedAt(t *testing.T) {
	repo := &mockObservationRepo{}
	svc := service.NewObservationService(repo)
	observedAt := time.Date(2026, 4, 15, 9, 30, 0, 0, time.FixedZone("BRT", -3*60*60))

	observation, err := svc.Create(context.Background(), service.CreateObservationInput{
		PetID:       "p1",
		ObservedAt:  observedAt,
		Description: "Vomited after breakfast",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if observation.ID == "" {
		t.Fatal("expected ID to be set")
	}
	if observation.CreatedAt.IsZero() {
		t.Fatal("expected CreatedAt to be set")
	}
	if got, want := observation.ObservedAt.Format(time.RFC3339), "2026-04-15T09:30:00-03:00"; got != want {
		t.Fatalf("observed_at = %s, want %s", got, want)
	}
	if got, want := repo.stored.ObservedAt.Format(time.RFC3339), "2026-04-15T09:30:00-03:00"; got != want {
		t.Fatalf("stored observed_at = %s, want %s", got, want)
	}
	if !observation.ObservedAt.Equal(observedAt) {
		t.Fatalf("observed_at = %s, want %s", observation.ObservedAt, observedAt)
	}
}

func TestObservationService_Create_ValidationErrors(t *testing.T) {
	svc := service.NewObservationService(&mockObservationRepo{})
	observedAt := time.Now()
	cases := []struct {
		name  string
		input service.CreateObservationInput
	}{
		{"missing pet", service.CreateObservationInput{ObservedAt: observedAt, Description: "Vomited"}},
		{"missing observed_at", service.CreateObservationInput{PetID: "p1", Description: "Vomited"}},
		{"missing description", service.CreateObservationInput{PetID: "p1", ObservedAt: observedAt}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Create(context.Background(), tc.input)
			if !errors.Is(err, domain.ErrValidation) {
				t.Fatalf("got %v, want ErrValidation", err)
			}
		})
	}
}

func TestObservationService_ListAndGetWrapRepositoryErrors(t *testing.T) {
	repoErr := errors.New("db unavailable")
	svc := service.NewObservationService(&mockObservationRepo{err: repoErr})

	_, err := svc.ListByPet(context.Background(), "p1")
	if !errors.Is(err, repoErr) {
		t.Fatalf("list error = %v, want repo error", err)
	}
	svc = service.NewObservationService(&mockObservationRepo{
		observations: []domain.Observation{{ID: "o1"}},
		err:          repoErr,
	})
	_, err = svc.GetByID(context.Background(), "p1", "o1")
	if !errors.Is(err, repoErr) {
		t.Fatalf("get error = %v, want repo error", err)
	}
}
