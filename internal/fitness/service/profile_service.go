package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
	"github.com/rafaelsoares/alfredo/internal/fitness/port"
)

type CreateProfileInput struct {
	FirstName string
	LastName  string
	BirthDate time.Time
	Gender    string
	HeightCm  float64
}

type UpdateProfileInput struct {
	FirstName *string
	LastName  *string
	BirthDate *time.Time
	Gender    *string
	HeightCm  *float64
}

type ProfileService struct {
	repo port.ProfileRepository
}

func NewProfileService(repo port.ProfileRepository) *ProfileService {
	return &ProfileService{repo: repo}
}

func (s *ProfileService) Create(ctx context.Context, in CreateProfileInput) (*domain.Profile, error) {
	if in.FirstName == "" {
		return nil, fmt.Errorf("%w: first_name is required", domain.ErrValidation)
	}
	if in.LastName == "" {
		return nil, fmt.Errorf("%w: last_name is required", domain.ErrValidation)
	}
	if in.Gender == "" {
		return nil, fmt.Errorf("%w: gender is required", domain.ErrValidation)
	}
	if in.HeightCm <= 0 {
		return nil, fmt.Errorf("%w: height_cm must be greater than zero", domain.ErrValidation)
	}
	now := time.Now().UTC()
	return s.repo.Create(ctx, domain.Profile{
		ID:        uuid.New().String(),
		FirstName: in.FirstName,
		LastName:  in.LastName,
		BirthDate: in.BirthDate.UTC(),
		Gender:    in.Gender,
		HeightCm:  in.HeightCm,
		CreatedAt: now,
		UpdatedAt: now,
	})
}

func (s *ProfileService) Get(ctx context.Context) (*domain.Profile, error) {
	return s.repo.Get(ctx)
}

func (s *ProfileService) Update(ctx context.Context, in UpdateProfileInput) (*domain.Profile, error) {
	p, err := s.repo.Get(ctx)
	if err != nil {
		return nil, err
	}
	if in.HeightCm != nil && *in.HeightCm <= 0 {
		return nil, fmt.Errorf("%w: height_cm must be greater than zero", domain.ErrValidation)
	}
	if in.FirstName != nil {
		p.FirstName = *in.FirstName
	}
	if in.LastName != nil {
		p.LastName = *in.LastName
	}
	if in.BirthDate != nil {
		t := in.BirthDate.UTC()
		p.BirthDate = t
	}
	if in.Gender != nil {
		p.Gender = *in.Gender
	}
	if in.HeightCm != nil {
		p.HeightCm = *in.HeightCm
	}
	p.UpdatedAt = time.Now().UTC()
	return s.repo.Update(ctx, *p)
}
