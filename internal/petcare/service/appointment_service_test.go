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

type mockAppointmentRepo struct {
	appointments []domain.Appointment
	err          error
	updated      *domain.Appointment
}

func (m *mockAppointmentRepo) Create(_ context.Context, a domain.Appointment) (*domain.Appointment, error) {
	return &a, m.err
}

func (m *mockAppointmentRepo) GetByID(_ context.Context, _, _ string) (*domain.Appointment, error) {
	if len(m.appointments) == 0 {
		return nil, domain.ErrNotFound
	}
	return &m.appointments[0], m.err
}

func (m *mockAppointmentRepo) List(_ context.Context, _ string) ([]domain.Appointment, error) {
	return m.appointments, m.err
}

func (m *mockAppointmentRepo) Update(_ context.Context, a domain.Appointment) (*domain.Appointment, error) {
	m.updated = &a
	return &a, m.err
}

func (m *mockAppointmentRepo) Delete(_ context.Context, _, _ string) error {
	if len(m.appointments) == 0 {
		return domain.ErrNotFound
	}
	return m.err
}

// --- tests ---

func TestAppointmentService_Create_AssignsID(t *testing.T) {
	svc := service.NewAppointmentService(&mockAppointmentRepo{})
	a, err := svc.Create(context.Background(), service.CreateAppointmentInput{
		PetID:       "p1",
		Type:        domain.AppointmentTypeVet,
		ScheduledAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.ID == "" {
		t.Error("expected ID to be set")
	}
}

func TestAppointmentService_Create_SetsCreatedAt(t *testing.T) {
	before := time.Now().Add(-time.Second)
	svc := service.NewAppointmentService(&mockAppointmentRepo{})
	a, err := svc.Create(context.Background(), service.CreateAppointmentInput{
		PetID:       "p1",
		Type:        domain.AppointmentTypeVet,
		ScheduledAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be non-zero")
	}
	if a.CreatedAt.Before(before) {
		t.Errorf("CreatedAt %v is before test start %v", a.CreatedAt, before)
	}
}

func TestAppointmentService_Create_ValidationError(t *testing.T) {
	svc := service.NewAppointmentService(&mockAppointmentRepo{})
	_, err := svc.Create(context.Background(), service.CreateAppointmentInput{
		PetID:       "p1",
		Type:        "",
		ScheduledAt: time.Now(),
	})
	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("got %v, want ErrValidation", err)
	}
}

func TestAppointmentService_Create_ValidationError_ZeroScheduledAt(t *testing.T) {
	svc := service.NewAppointmentService(&mockAppointmentRepo{})
	_, err := svc.Create(context.Background(), service.CreateAppointmentInput{
		PetID: "p1", Type: domain.AppointmentTypeVet, ScheduledAt: time.Time{},
	})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

func TestAppointmentService_GetByID_NotFound(t *testing.T) {
	svc := service.NewAppointmentService(&mockAppointmentRepo{})
	_, err := svc.GetByID(context.Background(), "p1", "a1")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestAppointmentService_List_Empty(t *testing.T) {
	svc := service.NewAppointmentService(&mockAppointmentRepo{})
	as, err := svc.List(context.Background(), "p1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(as) != 0 {
		t.Errorf("expected empty slice, got %d items", len(as))
	}
}

func TestAppointmentService_Update_AppliesNonNilFields(t *testing.T) {
	provider := "Dr. Smith"
	existing := domain.Appointment{
		ID:          "a1",
		PetID:       "p1",
		Type:        domain.AppointmentTypeVet,
		ScheduledAt: time.Now(),
		Provider:    nil,
		Location:    nil,
	}
	repo := &mockAppointmentRepo{appointments: []domain.Appointment{existing}}
	svc := service.NewAppointmentService(repo)

	newTime := time.Now().Add(24 * time.Hour)
	updated, err := svc.Update(context.Background(), "p1", "a1", service.UpdateAppointmentInput{
		ScheduledAt: &newTime,
		Provider:    &provider,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !updated.ScheduledAt.Equal(newTime) {
		t.Errorf("ScheduledAt not updated: got %v, want %v", updated.ScheduledAt, newTime)
	}
	if updated.Provider == nil || *updated.Provider != provider {
		t.Errorf("Provider not updated: got %v, want %q", updated.Provider, provider)
	}
	// Location was nil in input, should remain nil
	if updated.Location != nil {
		t.Errorf("Location should remain nil, got %v", *updated.Location)
	}
}

func TestAppointmentService_Delete_NotFound(t *testing.T) {
	svc := service.NewAppointmentService(&mockAppointmentRepo{})
	err := svc.Delete(context.Background(), "p1", "a1")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}
