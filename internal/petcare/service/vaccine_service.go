package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
	"github.com/rafaelsoares/alfredo/internal/petcare/port"
)

type RecordVaccineInput struct {
	PetID          string
	Name           string
	AdministeredAt time.Time
	NextDueAt      *time.Time
	VetName        *string
	BatchNumber    *string
	Notes          *string
}

type VaccineService struct {
	repo    port.VaccineRepository
	petRepo port.PetRepository
}

func NewVaccineService(repo port.VaccineRepository, petRepo port.PetRepository) *VaccineService {
	return &VaccineService{repo: repo, petRepo: petRepo}
}

func (s *VaccineService) ListVaccines(ctx context.Context, petID string) ([]domain.Vaccine, error) {
	return s.repo.ListVaccines(ctx, petID)
}

func (s *VaccineService) RecordVaccine(ctx context.Context, in RecordVaccineInput) (*domain.Vaccine, error) {
	if in.Name == "" {
		return nil, fmt.Errorf("%w: name is required", domain.ErrValidation)
	}
	if in.AdministeredAt.IsZero() {
		return nil, fmt.Errorf("%w: administered_at is required", domain.ErrValidation)
	}
	v, err := s.repo.CreateVaccine(ctx, domain.Vaccine{
		ID:             uuid.New().String(),
		PetID:          in.PetID,
		Name:           in.Name,
		AdministeredAt: in.AdministeredAt,
		NextDueAt:      in.NextDueAt,
		VetName:        in.VetName,
		BatchNumber:    in.BatchNumber,
		Notes:          in.Notes,
	})
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (s *VaccineService) DeleteVaccine(ctx context.Context, petID, vaccineID string) error {
	if _, err := s.repo.GetVaccine(ctx, petID, vaccineID); err != nil {
		return err
	}
	return s.repo.DeleteVaccine(ctx, petID, vaccineID)
}
