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
	ChestCm    *float64
	BicepsCm   *float64
	TricepsCm  *float64
	BodyFatPct *float64
	// Pollock 7-site skinfold measurements (mm)
	ChestSkinfoldMm       *float64
	MidaxillarySkinfoldMm *float64
	TricepsSkinfoldMm     *float64
	SubscapularSkinfoldMm *float64
	AbdominalSkinfoldMm   *float64
	SuprailiacSkinfoldMm  *float64
	ThighSkinfoldMm       *float64
	PhotoPath             *string
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
		ID:                    uuid.New().String(),
		Date:                  date,
		WeightKg:              in.WeightKg,
		WaistCm:               in.WaistCm,
		HipCm:                 in.HipCm,
		NeckCm:                in.NeckCm,
		ChestCm:               in.ChestCm,
		BicepsCm:              in.BicepsCm,
		TricepsCm:             in.TricepsCm,
		BodyFatPct:            in.BodyFatPct,
		ChestSkinfoldMm:       in.ChestSkinfoldMm,
		MidaxillarySkinfoldMm: in.MidaxillarySkinfoldMm,
		TricepsSkinfoldMm:     in.TricepsSkinfoldMm,
		SubscapularSkinfoldMm: in.SubscapularSkinfoldMm,
		AbdominalSkinfoldMm:   in.AbdominalSkinfoldMm,
		SuprailiacSkinfoldMm:  in.SuprailiacSkinfoldMm,
		ThighSkinfoldMm:       in.ThighSkinfoldMm,
		PhotoPath:             in.PhotoPath,
		CreatedAt:             time.Now().UTC(),
	})
}

func (s *BodySnapshotService) GetByID(ctx context.Context, id string) (*domain.BodySnapshot, error) {
	snap, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	previous, err := s.repo.LatestBefore(ctx, snap.Date, 0)
	if err != nil {
		return nil, err
	}
	// Fold previous snapshots (oldest-first) into a baseline, then merge with the target.
	baseline := foldBaseline(previous)
	filled := merge(baseline, *snap)
	return &filled, nil
}

func (s *BodySnapshotService) List(ctx context.Context, from, to *time.Time) ([]domain.BodySnapshot, error) {
	snapshots, err := s.repo.List(ctx, from, to)
	if err != nil {
		return nil, err
	}
	if len(snapshots) == 0 {
		return snapshots, nil
	}
	// Seed the baseline from the most recent snapshot before the range.
	var baseline domain.BodySnapshot
	if from != nil {
		seed, err := s.repo.LatestBefore(ctx, *from, 1)
		if err != nil {
			return nil, err
		}
		if len(seed) > 0 {
			baseline = seed[0]
		}
	}
	// List returns snapshots ordered ASC; forward-fill in a single pass.
	filled := make([]domain.BodySnapshot, len(snapshots))
	for i, snap := range snapshots {
		filled[i] = merge(baseline, snap)
		baseline = filled[i]
	}
	return filled, nil
}

func (s *BodySnapshotService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *BodySnapshotService) CurrentBodyState(ctx context.Context) (*domain.BodySnapshot, error) {
	// Grab the single most recent snapshot (tomorrow as upper bound to include today).
	recent, err := s.repo.LatestBefore(ctx, time.Now().AddDate(0, 0, 1), 1)
	if err != nil {
		return nil, err
	}
	if len(recent) == 0 {
		return nil, domain.ErrNotFound
	}
	target := recent[0]

	// Load all snapshots strictly before the target for forward-fill context.
	history, err := s.repo.LatestBefore(ctx, target.Date, 0)
	if err != nil {
		return nil, err
	}

	baseline := foldBaseline(history)
	result := merge(baseline, target)
	return &result, nil
}

// merge returns a new BodySnapshot where each pointer field takes override's value
// if non-nil, falling back to base's value. Non-pointer identity fields (ID, Date,
// CreatedAt) always come from override.
func merge(base, override domain.BodySnapshot) domain.BodySnapshot {
	result := override
	pick := func(o, b *float64) *float64 {
		if o != nil {
			return o
		}
		return b
	}
	pickStr := func(o, b *string) *string {
		if o != nil {
			return o
		}
		return b
	}
	result.WeightKg = pick(override.WeightKg, base.WeightKg)
	result.WaistCm = pick(override.WaistCm, base.WaistCm)
	result.HipCm = pick(override.HipCm, base.HipCm)
	result.NeckCm = pick(override.NeckCm, base.NeckCm)
	result.ChestCm = pick(override.ChestCm, base.ChestCm)
	result.BicepsCm = pick(override.BicepsCm, base.BicepsCm)
	result.TricepsCm = pick(override.TricepsCm, base.TricepsCm)
	result.BodyFatPct = pick(override.BodyFatPct, base.BodyFatPct)
	result.ChestSkinfoldMm = pick(override.ChestSkinfoldMm, base.ChestSkinfoldMm)
	result.MidaxillarySkinfoldMm = pick(override.MidaxillarySkinfoldMm, base.MidaxillarySkinfoldMm)
	result.TricepsSkinfoldMm = pick(override.TricepsSkinfoldMm, base.TricepsSkinfoldMm)
	result.SubscapularSkinfoldMm = pick(override.SubscapularSkinfoldMm, base.SubscapularSkinfoldMm)
	result.AbdominalSkinfoldMm = pick(override.AbdominalSkinfoldMm, base.AbdominalSkinfoldMm)
	result.SuprailiacSkinfoldMm = pick(override.SuprailiacSkinfoldMm, base.SuprailiacSkinfoldMm)
	result.ThighSkinfoldMm = pick(override.ThighSkinfoldMm, base.ThighSkinfoldMm)
	result.PhotoPath = pickStr(override.PhotoPath, base.PhotoPath)
	return result
}

// foldBaseline folds a DESC-ordered slice of snapshots into a single baseline by
// iterating from oldest to newest so that more-recent values win.
func foldBaseline(snapshots []domain.BodySnapshot) domain.BodySnapshot {
	var baseline domain.BodySnapshot
	for i := len(snapshots) - 1; i >= 0; i-- {
		baseline = merge(baseline, snapshots[i])
	}
	return baseline
}
