package port

import (
	"context"

	"github.com/rafaelsoares/alfredo/internal/health/domain"
)

// HealthAppointmentRepository defines the repository interface for health appointments.
type HealthAppointmentRepository interface {
	Create(ctx context.Context, a domain.HealthAppointment) error
	GetByID(ctx context.Context, id string) (*domain.HealthAppointment, error)
	List(ctx context.Context) ([]domain.HealthAppointment, error)
	Delete(ctx context.Context, id string) error
}
