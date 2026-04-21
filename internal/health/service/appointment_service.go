package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rafaelsoares/alfredo/internal/health/domain"
	"github.com/rafaelsoares/alfredo/internal/health/port"
)

// CreateHealthAppointmentInput is the input for creating a health appointment.
type CreateHealthAppointmentInput struct {
	Specialty             string
	ScheduledAt           time.Time
	Doctor                *string
	Notes                 *string
	GoogleCalendarEventID string
}

// HealthAppointmentService provides CRUD operations for health appointments.
type HealthAppointmentService struct {
	repo port.HealthAppointmentRepository
}

// NewHealthAppointmentService creates a new health appointment service.
func NewHealthAppointmentService(repo port.HealthAppointmentRepository) *HealthAppointmentService {
	return &HealthAppointmentService{repo: repo}
}

// Create creates a new health appointment.
func (s *HealthAppointmentService) Create(ctx context.Context, in CreateHealthAppointmentInput) (*domain.HealthAppointment, error) {
	appt := domain.HealthAppointment{
		ID:                    uuid.New().String(),
		Specialty:             in.Specialty,
		ScheduledAt:           in.ScheduledAt,
		Doctor:                in.Doctor,
		Notes:                 in.Notes,
		GoogleCalendarEventID: in.GoogleCalendarEventID,
		CreatedAt:             time.Now().UTC(),
	}
	if err := s.repo.Create(ctx, appt); err != nil {
		return nil, err
	}
	return &appt, nil
}

// GetByID retrieves a health appointment by ID.
func (s *HealthAppointmentService) GetByID(ctx context.Context, id string) (*domain.HealthAppointment, error) {
	return s.repo.GetByID(ctx, id)
}

// List retrieves all health appointments ordered by scheduled_at.
func (s *HealthAppointmentService) List(ctx context.Context) ([]domain.HealthAppointment, error) {
	return s.repo.List(ctx)
}

// Delete deletes a health appointment.
func (s *HealthAppointmentService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}
