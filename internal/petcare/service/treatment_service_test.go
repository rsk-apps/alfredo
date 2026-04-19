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
	treatment  *domain.Treatment
	treatments []domain.Treatment
	createErr  error
	listErr    error
	getErr     error
	stopErr    error

	calls      []string
	stopCalled bool
	stopAt     time.Time
}

func (m *mockTreatmentRepo) Create(_ context.Context, t domain.Treatment) (*domain.Treatment, error) {
	return &t, m.createErr
}
func (m *mockTreatmentRepo) GetByID(_ context.Context, _, _ string) (*domain.Treatment, error) {
	m.calls = append(m.calls, "get")
	return m.treatment, m.getErr
}
func (m *mockTreatmentRepo) List(_ context.Context, _ string) ([]domain.Treatment, error) {
	if m.treatments != nil {
		return m.treatments, m.listErr
	}
	if m.treatment != nil {
		return []domain.Treatment{*m.treatment}, m.listErr
	}
	return nil, m.listErr
}
func (m *mockTreatmentRepo) Stop(_ context.Context, _ string, at time.Time) error {
	m.calls = append(m.calls, "stop")
	m.stopCalled = true
	m.stopAt = at
	return m.stopErr
}

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
	repo := &mockTreatmentRepo{getErr: domain.ErrNotFound}
	svc := service.NewTreatmentService(repo)
	err := svc.Stop(context.Background(), "p1", "t1")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
	if len(repo.calls) != 1 || repo.calls[0] != "get" {
		t.Fatalf("expected lookup before stop, got calls %#v", repo.calls)
	}
	if repo.stopCalled {
		t.Fatal("expected Stop not to reach repository when lookup fails")
	}
}

func TestTreatmentService_GetByIDAndList_ReturnRepositoryValues(t *testing.T) {
	want := &domain.Treatment{ID: "t1", PetID: "p1", Name: "Amoxicillin"}
	svc := service.NewTreatmentService(&mockTreatmentRepo{treatment: want, treatments: []domain.Treatment{*want}})

	got, err := svc.GetByID(context.Background(), "p1", "t1")
	if err != nil {
		t.Fatalf("GetByID error = %v", err)
	}
	if got == nil || got.ID != want.ID {
		t.Fatalf("GetByID = %#v, want %#v", got, want)
	}

	listed, err := svc.List(context.Background(), "p1")
	if err != nil {
		t.Fatalf("List error = %v", err)
	}
	if len(listed) != 1 || listed[0].ID != want.ID {
		t.Fatalf("List = %#v, want %#v", listed, want)
	}
}

func TestTreatmentService_Stop_DelegatesToRepository(t *testing.T) {
	repo := &mockTreatmentRepo{treatment: &domain.Treatment{ID: "t1", PetID: "p1"}}
	svc := service.NewTreatmentService(repo)

	if err := svc.Stop(context.Background(), "p1", "t1"); err != nil {
		t.Fatalf("Stop error = %v", err)
	}
	if !repo.stopCalled {
		t.Fatal("expected Stop to call repository")
	}
	if repo.stopAt.IsZero() {
		t.Fatal("expected Stop timestamp to be set")
	}
	if len(repo.calls) != 2 || repo.calls[0] != "get" || repo.calls[1] != "stop" {
		t.Fatalf("expected GetByID before Stop, got calls %#v", repo.calls)
	}
}
