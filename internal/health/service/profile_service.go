package service

import (
	"context"
	"fmt"

	"github.com/rafaelsoares/alfredo/internal/health/domain"
	"github.com/rafaelsoares/alfredo/internal/health/port"
)

type ProfileService struct {
	repo port.ProfileRepository
}

func NewProfileService(repo port.ProfileRepository) *ProfileService {
	return &ProfileService{repo: repo}
}

func (s *ProfileService) Get(ctx context.Context) (domain.HealthProfile, error) {
	profile, err := s.repo.Get(ctx)
	if err != nil {
		return domain.HealthProfile{}, fmt.Errorf("get health profile: %w", err)
	}
	return profile, nil
}

func (s *ProfileService) Upsert(ctx context.Context, profile domain.HealthProfile) (domain.HealthProfile, error) {
	created, err := s.repo.Upsert(ctx, profile)
	if err != nil {
		return domain.HealthProfile{}, fmt.Errorf("upsert health profile: %w", err)
	}
	return created, nil
}

func (s *ProfileService) GetCalendarID(ctx context.Context) (string, error) {
	return s.repo.GetCalendarID(ctx)
}

func (s *ProfileService) SetCalendarID(ctx context.Context, calendarID string) error {
	return s.repo.SetCalendarID(ctx, calendarID)
}
