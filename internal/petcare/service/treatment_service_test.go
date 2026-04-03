package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
	"github.com/rafaelsoares/alfredo/internal/petcare/service"
)

// --- mock ---

type mockTreatmentRepo struct {
	treatment *domain.Treatment
	err       error
}

func (m *mockTreatmentRepo) Create(_ context.Context, t domain.Treatment) (*domain.Treatment, error) {
	return &t, m.err
}
func (m *mockTreatmentRepo) GetByID(_ context.Context, _, _ string) (*domain.Treatment, error) {
	return m.treatment, m.err
}
func (m *mockTreatmentRepo) List(_ context.Context, _ string) ([]domain.Treatment, error) {
	if m.treatment != nil {
		return []domain.Treatment{*m.treatment}, m.err
	}
	return nil, m.err
}
func (m *mockTreatmentRepo) Stop(_ context.Context, _ string, _ time.Time) error { return m.err }

// --- tests ---

func TestTreatmentService_Create_AssignsID(t *testing.T) {
	svc := service.NewTreatmentService(&mockTreatmentRepo{})
	tr, err := svc.Create(context.Background(), service.CreateTreatmentInput{
		PetID: "p1", Name: "Amoxicillin", DosageAmount: 250, DosageUnit: "mg",
		Route: "oral", IntervalHours: 12, StartedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tr.ID == "" {
		t.Error("expected ID to be assigned")
	}
}

func TestTreatmentService_Create_ValidationErrors(t *testing.T) {
	svc := service.NewTreatmentService(&mockTreatmentRepo{})
	cases := []struct {
		name  string
		input service.CreateTreatmentInput
	}{
		{"missing name", service.CreateTreatmentInput{PetID: "p1", DosageAmount: 1, DosageUnit: "mg", Route: "oral", IntervalHours: 24, StartedAt: time.Now()}},
		{"zero dosage", service.CreateTreatmentInput{PetID: "p1", Name: "Drug", DosageUnit: "mg", Route: "oral", IntervalHours: 24, StartedAt: time.Now()}},
		{"missing unit", service.CreateTreatmentInput{PetID: "p1", Name: "Drug", DosageAmount: 1, Route: "oral", IntervalHours: 24, StartedAt: time.Now()}},
		{"missing route", service.CreateTreatmentInput{PetID: "p1", Name: "Drug", DosageAmount: 1, DosageUnit: "mg", IntervalHours: 24, StartedAt: time.Now()}},
		{"zero interval", service.CreateTreatmentInput{PetID: "p1", Name: "Drug", DosageAmount: 1, DosageUnit: "mg", Route: "oral", StartedAt: time.Now()}},
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

func TestTreatmentService_Stop_NotFound(t *testing.T) {
	svc := service.NewTreatmentService(&mockTreatmentRepo{err: domain.ErrNotFound})
	err := svc.Stop(context.Background(), "p1", "t1")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}
