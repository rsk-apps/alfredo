package service

import (
	"context"
	"testing"
	"time"

	"github.com/rafaelsoares/alfredo/internal/health/domain"
)

type mockAppointmentRepository struct {
	appointments map[string]*domain.HealthAppointment
	lastErr      error
}

func (m *mockAppointmentRepository) Create(ctx context.Context, a domain.HealthAppointment) error {
	if m.lastErr != nil {
		return m.lastErr
	}
	if m.appointments == nil {
		m.appointments = make(map[string]*domain.HealthAppointment)
	}
	m.appointments[a.ID] = &a
	return nil
}

func (m *mockAppointmentRepository) GetByID(ctx context.Context, id string) (*domain.HealthAppointment, error) {
	if m.lastErr != nil {
		return nil, m.lastErr
	}
	a, ok := m.appointments[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return a, nil
}

func (m *mockAppointmentRepository) List(ctx context.Context) ([]domain.HealthAppointment, error) {
	if m.lastErr != nil {
		return nil, m.lastErr
	}
	var out []domain.HealthAppointment
	for _, a := range m.appointments {
		out = append(out, *a)
	}
	return out, nil
}

func (m *mockAppointmentRepository) Delete(ctx context.Context, id string) error {
	if m.lastErr != nil {
		return m.lastErr
	}
	if _, ok := m.appointments[id]; !ok {
		return domain.ErrNotFound
	}
	delete(m.appointments, id)
	return nil
}

func TestHealthAppointmentServiceCreate(t *testing.T) {
	repo := &mockAppointmentRepository{appointments: make(map[string]*domain.HealthAppointment)}
	svc := NewHealthAppointmentService(repo)
	ctx := context.Background()

	scheduledAt := time.Date(2026, 5, 10, 9, 0, 0, 0, time.UTC)
	in := CreateHealthAppointmentInput{
		Specialty:             "Cardiologia",
		ScheduledAt:           scheduledAt,
		Doctor:                nil,
		Notes:                 nil,
		GoogleCalendarEventID: "evt-123",
	}

	appt, err := svc.Create(ctx, in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if appt.ID == "" {
		t.Fatal("expected appointment ID to be set")
	}
	if appt.Specialty != "Cardiologia" {
		t.Fatalf("expected specialty Cardiologia, got %q", appt.Specialty)
	}
	if !appt.ScheduledAt.Equal(scheduledAt) {
		t.Fatalf("expected scheduled_at %s, got %s", scheduledAt, appt.ScheduledAt)
	}
	if appt.GoogleCalendarEventID != "evt-123" {
		t.Fatalf("expected event ID evt-123, got %q", appt.GoogleCalendarEventID)
	}
}

func TestHealthAppointmentServiceGetByID(t *testing.T) {
	repo := &mockAppointmentRepository{appointments: make(map[string]*domain.HealthAppointment)}
	svc := NewHealthAppointmentService(repo)
	ctx := context.Background()

	appt := domain.HealthAppointment{
		ID:                    "appt-1",
		Specialty:             "Dentista",
		ScheduledAt:           time.Now(),
		GoogleCalendarEventID: "evt-456",
		CreatedAt:             time.Now(),
	}
	repo.appointments["appt-1"] = &appt

	got, err := svc.GetByID(ctx, "appt-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "appt-1" {
		t.Fatalf("expected ID appt-1, got %q", got.ID)
	}

	_, err = svc.GetByID(ctx, "nonexistent")
	if err != domain.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestHealthAppointmentServiceDelete(t *testing.T) {
	repo := &mockAppointmentRepository{appointments: make(map[string]*domain.HealthAppointment)}
	svc := NewHealthAppointmentService(repo)
	ctx := context.Background()

	appt := domain.HealthAppointment{
		ID:        "appt-1",
		Specialty: "Oftalmologia",
		CreatedAt: time.Now(),
	}
	repo.appointments["appt-1"] = &appt

	err := svc.Delete(ctx, "appt-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = svc.GetByID(ctx, "appt-1")
	if err != domain.ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}

	err = svc.Delete(ctx, "nonexistent")
	if err != domain.ErrNotFound {
		t.Fatalf("expected ErrNotFound on delete of nonexistent, got %v", err)
	}
}

func TestHealthAppointmentServiceList(t *testing.T) {
	repo := &mockAppointmentRepository{
		appointments: map[string]*domain.HealthAppointment{
			"appt-1": {ID: "appt-1", Specialty: "Cardiologia"},
			"appt-2": {ID: "appt-2", Specialty: "Dermatologia"},
		},
	}
	svc := NewHealthAppointmentService(repo)

	appts, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("list appointments: %v", err)
	}
	if len(appts) != 2 {
		t.Fatalf("appointments len = %d, want 2", len(appts))
	}
}

func TestHealthAppointmentServicePropagatesRepositoryErrors(t *testing.T) {
	repo := &mockAppointmentRepository{lastErr: domain.ErrValidation}
	svc := NewHealthAppointmentService(repo)
	ctx := context.Background()

	if _, err := svc.Create(ctx, CreateHealthAppointmentInput{}); err != domain.ErrValidation {
		t.Fatalf("create err = %v, want %v", err, domain.ErrValidation)
	}
	if _, err := svc.GetByID(ctx, "appt-1"); err != domain.ErrValidation {
		t.Fatalf("get err = %v, want %v", err, domain.ErrValidation)
	}
	if _, err := svc.List(ctx); err != domain.ErrValidation {
		t.Fatalf("list err = %v, want %v", err, domain.ErrValidation)
	}
	if err := svc.Delete(ctx, "appt-1"); err != domain.ErrValidation {
		t.Fatalf("delete err = %v, want %v", err, domain.ErrValidation)
	}
}
