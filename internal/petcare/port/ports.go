package port

import (
	"context"
	"time"

	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
)

// PetRepository persists and retrieves Pet records.
type PetRepository interface {
	List(ctx context.Context) ([]domain.Pet, error)
	Create(ctx context.Context, pet domain.Pet) (*domain.Pet, error)
	GetByID(ctx context.Context, id string) (*domain.Pet, error)
	Update(ctx context.Context, pet domain.Pet) (*domain.Pet, error)
	Delete(ctx context.Context, id string) error
}

// VaccineRepository persists vaccine records.
type VaccineRepository interface {
	ListVaccines(ctx context.Context, petID string) ([]domain.Vaccine, error)
	CreateVaccine(ctx context.Context, v domain.Vaccine) (*domain.Vaccine, error)
	GetVaccine(ctx context.Context, petID, vaccineID string) (*domain.Vaccine, error)
	DeleteVaccine(ctx context.Context, petID, vaccineID string) error
}

// TreatmentRepository persists treatment records.
type TreatmentRepository interface {
	Create(ctx context.Context, t domain.Treatment) (*domain.Treatment, error)
	GetByID(ctx context.Context, petID, treatmentID string) (*domain.Treatment, error)
	List(ctx context.Context, petID string) ([]domain.Treatment, error)
	Stop(ctx context.Context, treatmentID string, stoppedAt time.Time) error
}

// DoseRepository persists dose records and supports the rolling-window extension job.
type DoseRepository interface {
	CreateBatch(ctx context.Context, doses []domain.Dose) error
	ListByTreatment(ctx context.Context, treatmentID string) ([]domain.Dose, error)
	// DeleteFutureDoses deletes doses scheduled after `after` and returns their IDs.
	DeleteFutureDoses(ctx context.Context, treatmentID string, after time.Time) ([]string, error)
	// ListOpenEndedActiveTreatments returns treatments with ended_at IS NULL AND stopped_at IS NULL.
	ListOpenEndedActiveTreatments(ctx context.Context) ([]domain.Treatment, error)
	// LatestDoseFor returns the latest scheduled dose for a treatment, or nil if none exist.
	LatestDoseFor(ctx context.Context, treatmentID string) (*domain.Dose, error)
}
