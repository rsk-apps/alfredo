package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
	"github.com/rafaelsoares/alfredo/internal/petcare/port"
)

type CreateObservationInput struct {
	PetID       string
	ObservedAt  time.Time
	Description string
}

type ObservationService struct {
	repo port.ObservationRepository
}

func NewObservationService(repo port.ObservationRepository) *ObservationService {
	return &ObservationService{repo: repo}
}

func (s *ObservationService) Create(ctx context.Context, in CreateObservationInput) (*domain.Observation, error) {
	if in.PetID == "" {
		return nil, fmt.Errorf("%w: pet_id is required", domain.ErrValidation)
	}
	if in.ObservedAt.IsZero() {
		return nil, fmt.Errorf("%w: observed_at is required", domain.ErrValidation)
	}
	if in.Description == "" {
		return nil, fmt.Errorf("%w: description is required", domain.ErrValidation)
	}
	observation := domain.Observation{
		ID:          uuid.New().String(),
		PetID:       in.PetID,
		ObservedAt:  in.ObservedAt.UTC(),
		Description: in.Description,
		CreatedAt:   time.Now().UTC(),
	}
	created, err := s.repo.Create(ctx, observation)
	if err != nil {
		return nil, fmt.Errorf("create observation: %w", err)
	}
	return created, nil
}

func (s *ObservationService) ListByPet(ctx context.Context, petID string) ([]domain.Observation, error) {
	observations, err := s.repo.ListByPet(ctx, petID)
	if err != nil {
		return nil, fmt.Errorf("list observations for pet %q: %w", petID, err)
	}
	return observations, nil
}

func (s *ObservationService) GetByID(ctx context.Context, petID, observationID string) (*domain.Observation, error) {
	observation, err := s.repo.GetByID(ctx, petID, observationID)
	if err != nil {
		return nil, fmt.Errorf("get observation %q for pet %q: %w", observationID, petID, err)
	}
	return observation, nil
}
