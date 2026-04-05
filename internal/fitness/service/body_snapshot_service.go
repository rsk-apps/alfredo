package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
	"github.com/rafaelsoares/alfredo/internal/fitness/port"
)

type CreateBodySnapshotInput struct {
	Date       time.Time
	WeightKg   *float64
	WaistCm    *float64
	HipCm      *float64
	NeckCm     *float64
	BodyFatPct *float64
	PhotoPath  *string
}

type BodySnapshotService struct {
	repo port.BodySnapshotRepository
}

func NewBodySnapshotService(repo port.BodySnapshotRepository) *BodySnapshotService {
	return &BodySnapshotService{repo: repo}
}

func (s *BodySnapshotService) Create(ctx context.Context, in CreateBodySnapshotInput) (*domain.BodySnapshot, error) {
	if in.Date.IsZero() {
		return nil, fmt.Errorf("%w: date is required", domain.ErrValidation)
	}
	// Normalise to date-only (midnight UTC) so unique index on date works correctly.
	date := time.Date(in.Date.Year(), in.Date.Month(), in.Date.Day(), 0, 0, 0, 0, time.UTC)
	return s.repo.Create(ctx, domain.BodySnapshot{
		ID:         uuid.New().String(),
		Date:       date,
		WeightKg:   in.WeightKg,
		WaistCm:    in.WaistCm,
		HipCm:      in.HipCm,
		NeckCm:     in.NeckCm,
		BodyFatPct: in.BodyFatPct,
		PhotoPath:  in.PhotoPath,
		CreatedAt:  time.Now().UTC(),
	})
}

func (s *BodySnapshotService) GetByID(ctx context.Context, id string) (*domain.BodySnapshot, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *BodySnapshotService) List(ctx context.Context, from, to *time.Time) ([]domain.BodySnapshot, error) {
	return s.repo.List(ctx, from, to)
}

func (s *BodySnapshotService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}
