// internal/app/fitness_profile_usecase.go
package app

import (
	"context"

	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
	fitnesssvc "github.com/rafaelsoares/alfredo/internal/fitness/service"
)

// FitnessProfileUseCase is a pass-through to ProfileService.
// No webhook events are emitted for profile changes.
type FitnessProfileUseCase struct {
	profiles FitnessProfileServicer
}

func NewFitnessProfileUseCase(profiles FitnessProfileServicer) *FitnessProfileUseCase {
	return &FitnessProfileUseCase{profiles: profiles}
}

func (uc *FitnessProfileUseCase) Create(ctx context.Context, in fitnesssvc.CreateProfileInput) (*domain.Profile, error) {
	return uc.profiles.Create(ctx, in)
}

func (uc *FitnessProfileUseCase) Get(ctx context.Context) (*domain.Profile, error) {
	return uc.profiles.Get(ctx)
}

func (uc *FitnessProfileUseCase) Update(ctx context.Context, in fitnesssvc.UpdateProfileInput) (*domain.Profile, error) {
	return uc.profiles.Update(ctx, in)
}
