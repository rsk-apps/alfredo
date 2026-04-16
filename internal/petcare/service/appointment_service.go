package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
	"github.com/rafaelsoares/alfredo/internal/petcare/port"
)

// CreateAppointmentInput holds the fields required to schedule a new appointment.
type CreateAppointmentInput struct {
	PetID                 string
	Type                  domain.AppointmentType
	ScheduledAt           time.Time
	Provider              *string
	Location              *string
	Notes                 *string
	GoogleCalendarEventID string
}

// UpdateAppointmentInput holds the mutable fields for an appointment update.
// Only non-nil pointer fields are applied.
type UpdateAppointmentInput struct {
	ScheduledAt *time.Time
	Provider    *string
	Location    *string
	Notes       *string
}

// AppointmentService is a pure CRUD service for pet appointments.
type AppointmentService struct {
	repo port.AppointmentRepository
}

// NewAppointmentService constructs an AppointmentService backed by repo.
func NewAppointmentService(repo port.AppointmentRepository) *AppointmentService {
	return &AppointmentService{repo: repo}
}

// Create validates input, assigns a new UUID and CreatedAt, then persists the appointment.
func (s *AppointmentService) Create(ctx context.Context, in CreateAppointmentInput) (*domain.Appointment, error) {
	if in.Type == "" {
		return nil, fmt.Errorf("%w: type is required", domain.ErrValidation)
	}
	if in.ScheduledAt.IsZero() {
		return nil, fmt.Errorf("%w: scheduled_at is required", domain.ErrValidation)
	}
	a, err := s.repo.Create(ctx, domain.Appointment{
		ID:                    uuid.New().String(),
		PetID:                 in.PetID,
		Type:                  in.Type,
		ScheduledAt:           in.ScheduledAt,
		Provider:              in.Provider,
		Location:              in.Location,
		Notes:                 in.Notes,
		GoogleCalendarEventID: in.GoogleCalendarEventID,
		CreatedAt:             time.Now().UTC(),
	})
	if err != nil {
		return nil, fmt.Errorf("create appointment: %w", err)
	}
	return a, nil
}

// GetByID retrieves a single appointment by petID and appointmentID.
func (s *AppointmentService) GetByID(ctx context.Context, petID, appointmentID string) (*domain.Appointment, error) {
	a, err := s.repo.GetByID(ctx, petID, appointmentID)
	if err != nil {
		return nil, fmt.Errorf("get appointment: %w", err)
	}
	return a, nil
}

// List returns all appointments for a pet ordered by scheduled_at.
func (s *AppointmentService) List(ctx context.Context, petID string) ([]domain.Appointment, error) {
	as, err := s.repo.List(ctx, petID)
	if err != nil {
		return nil, fmt.Errorf("list appointments: %w", err)
	}
	return as, nil
}

// Update applies non-nil fields from in to the existing appointment identified by petID and appointmentID.
func (s *AppointmentService) Update(ctx context.Context, petID, appointmentID string, in UpdateAppointmentInput) (*domain.Appointment, error) {
	existing, err := s.GetByID(ctx, petID, appointmentID)
	if err != nil {
		return nil, fmt.Errorf("update appointment: %w", err)
	}
	if in.ScheduledAt != nil {
		existing.ScheduledAt = *in.ScheduledAt
	}
	if in.Provider != nil {
		existing.Provider = in.Provider
	}
	if in.Location != nil {
		existing.Location = in.Location
	}
	if in.Notes != nil {
		existing.Notes = in.Notes
	}
	updated, err := s.repo.Update(ctx, *existing)
	if err != nil {
		return nil, fmt.Errorf("update appointment: %w", err)
	}
	return updated, nil
}

// Delete removes an appointment by petID and appointmentID.
func (s *AppointmentService) Delete(ctx context.Context, petID, appointmentID string) error {
	if err := s.repo.Delete(ctx, petID, appointmentID); err != nil {
		return fmt.Errorf("delete appointment: %w", err)
	}
	return nil
}
