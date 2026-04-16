package app

import (
	"context"
	"errors"
	"time"

	"github.com/rafaelsoares/alfredo/internal/gcalendar"
	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
	"github.com/rafaelsoares/alfredo/internal/petcare/service"
	"github.com/rafaelsoares/alfredo/internal/shared/health"
	"github.com/rafaelsoares/alfredo/internal/telegram"
)

var ErrTxCommit = errors.New("transaction commit failed")

// PetNameGetter allows use cases to look up a pet's name for event payloads.
// Satisfied by petcare/service.PetService.GetByID.
type PetNameGetter interface {
	GetByID(ctx context.Context, id string) (*domain.Pet, error)
}

// HealthPinger is the narrow health check interface used by HealthAggregator.
type HealthPinger interface {
	Ping(ctx context.Context) error
}

// --- Pet-care service interfaces (used by Use Cases) ---

type PetCareServicer interface {
	List(ctx context.Context) ([]domain.Pet, error)
	Create(ctx context.Context, in service.CreatePetInput) (*domain.Pet, error)
	GetByID(ctx context.Context, id string) (*domain.Pet, error)
	Update(ctx context.Context, id string, in service.UpdatePetInput) (*domain.Pet, error)
	Delete(ctx context.Context, id string) error
}

type VaccineServicer interface {
	ListVaccines(ctx context.Context, petID string) ([]domain.Vaccine, error)
	RecordVaccine(ctx context.Context, in service.RecordVaccineInput) (*domain.Vaccine, error)
	GetVaccine(ctx context.Context, petID, vaccineID string) (*domain.Vaccine, error)
	DeleteVaccine(ctx context.Context, petID, vaccineID string) error
}

type ObservationServicer interface {
	Create(ctx context.Context, in service.CreateObservationInput) (*domain.Observation, error)
	ListByPet(ctx context.Context, petID string) ([]domain.Observation, error)
	GetByID(ctx context.Context, petID, observationID string) (*domain.Observation, error)
}

// HealthResult mirrors shared/health.HealthResult (re-exported for convenience).
type HealthResult = health.HealthResult

// TreatmentServicer is the narrow interface consumed by TreatmentUseCase.
type TreatmentServicer interface {
	Create(ctx context.Context, in service.CreateTreatmentInput) (*domain.Treatment, error)
	GetByID(ctx context.Context, petID, treatmentID string) (*domain.Treatment, error)
	List(ctx context.Context, petID string) ([]domain.Treatment, error)
	Stop(ctx context.Context, petID, treatmentID string) error
}

// DoseServicer is the narrow interface consumed by TreatmentUseCase.
type DoseServicer interface {
	GenerateDoses(t domain.Treatment, upTo time.Time) []domain.Dose
	CreateBatch(ctx context.Context, doses []domain.Dose) error
	ListByTreatment(ctx context.Context, treatmentID string) ([]domain.Dose, error)
	ListFutureByTreatment(ctx context.Context, treatmentID string, after time.Time) ([]domain.Dose, error)
	DeleteFutureByTreatment(ctx context.Context, treatmentID string, after time.Time) error
}

type CalendarPort = gcalendar.Port

type TelegramPort = telegram.Port

type PetCareTxRunner interface {
	WithinTx(ctx context.Context, fn func(pets *service.PetService, vaccines *service.VaccineService, treatments *service.TreatmentService, doses *service.DoseService) error) error
}

// AppointmentServicer is the narrow interface consumed by AppointmentUseCase.
type AppointmentServicer interface {
	Create(ctx context.Context, in service.CreateAppointmentInput) (*domain.Appointment, error)
	GetByID(ctx context.Context, petID, appointmentID string) (*domain.Appointment, error)
	List(ctx context.Context, petID string) ([]domain.Appointment, error)
	Update(ctx context.Context, petID, appointmentID string, in service.UpdateAppointmentInput) (*domain.Appointment, error)
	Delete(ctx context.Context, petID, appointmentID string) error
}
