package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
	"github.com/rafaelsoares/alfredo/internal/petcare/port"
)

type CreateTreatmentInput struct {
	PetID         string
	Name          string
	DosageAmount  float64
	DosageUnit    string
	Route         string
	IntervalHours int
	StartedAt     time.Time
	EndedAt       *time.Time
	VetName       *string
	Notes         *string
}

type TreatmentService struct {
	repo port.TreatmentRepository
}

func NewTreatmentService(repo port.TreatmentRepository) *TreatmentService {
	return &TreatmentService{repo: repo}
}

func (s *TreatmentService) Create(ctx context.Context, in CreateTreatmentInput) (*domain.Treatment, error) {
	if in.Name == "" {
		return nil, fmt.Errorf("%w: name is required", domain.ErrValidation)
	}
	if in.DosageAmount <= 0 {
		return nil, fmt.Errorf("%w: dosage_amount must be greater than zero", domain.ErrValidation)
	}
	if in.DosageUnit == "" {
		return nil, fmt.Errorf("%w: dosage_unit is required", domain.ErrValidation)
	}
	if in.Route == "" {
		return nil, fmt.Errorf("%w: route is required", domain.ErrValidation)
	}
	if in.IntervalHours <= 0 {
		return nil, fmt.Errorf("%w: interval_hours must be at least 1", domain.ErrValidation)
	}
	now := time.Now().UTC()
	return s.repo.Create(ctx, domain.Treatment{
		ID:            uuid.New().String(),
		PetID:         in.PetID,
		Name:          in.Name,
		DosageAmount:  in.DosageAmount,
		DosageUnit:    in.DosageUnit,
		Route:         in.Route,
		IntervalHours: in.IntervalHours,
		StartedAt:     in.StartedAt.UTC(),
		EndedAt:       in.EndedAt,
		VetName:       in.VetName,
		Notes:         in.Notes,
		CreatedAt:     now,
	})
}

func (s *TreatmentService) GetByID(ctx context.Context, petID, treatmentID string) (*domain.Treatment, error) {
	return s.repo.GetByID(ctx, petID, treatmentID)
}

func (s *TreatmentService) List(ctx context.Context, petID string) ([]domain.Treatment, error) {
	return s.repo.List(ctx, petID)
}

func (s *TreatmentService) Stop(ctx context.Context, petID, treatmentID string) error {
	if _, err := s.repo.GetByID(ctx, petID, treatmentID); err != nil {
		return err
	}
	return s.repo.Stop(ctx, treatmentID, time.Now().UTC())
}
