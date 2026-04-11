// internal/app/fitness_body_usecase.go
package app

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
	fitnesssvc "github.com/rafaelsoares/alfredo/internal/fitness/service"
	"github.com/rafaelsoares/alfredo/internal/webhook"
)

type FitnessBodyUseCase struct {
	snapshots FitnessBodySnapshotServicer
	emitter   webhook.EventEmitter
	logger    *zap.Logger
}

func NewFitnessBodyUseCase(
	snapshots FitnessBodySnapshotServicer,
	emitter webhook.EventEmitter,
	logger *zap.Logger,
) *FitnessBodyUseCase {
	return &FitnessBodyUseCase{snapshots: snapshots, emitter: emitter, logger: logger}
}

func (uc *FitnessBodyUseCase) CreateSnapshot(ctx context.Context, in fitnesssvc.CreateBodySnapshotInput) (*domain.BodySnapshot, error) {
	s, err := uc.snapshots.Create(ctx, in)
	if err != nil {
		return nil, err
	}
	uc.emitter.Emit(ctx, "fitness.body_snapshot.saved", fitnessBodySnapshotSavedPayload{
		SnapshotID: s.ID,
		Date:       s.Date.Format("2006-01-02"),
		WeightKg:   s.WeightKg,
		BodyFatPct: s.BodyFatPct,
	})
	return s, nil
}

func (uc *FitnessBodyUseCase) GetSnapshot(ctx context.Context, id string) (*domain.BodySnapshot, error) {
	return uc.snapshots.GetByID(ctx, id)
}

func (uc *FitnessBodyUseCase) ListSnapshots(ctx context.Context, from, to *time.Time) ([]domain.BodySnapshot, error) {
	return uc.snapshots.List(ctx, from, to)
}

func (uc *FitnessBodyUseCase) DeleteSnapshot(ctx context.Context, id string) error {
	return uc.snapshots.Delete(ctx, id)
}

func (uc *FitnessBodyUseCase) CurrentBodyState(ctx context.Context) (*domain.BodySnapshot, error) {
	return uc.snapshots.CurrentBodyState(ctx)
}

// --- payload types ---

type fitnessBodySnapshotSavedPayload struct {
	SnapshotID string   `json:"snapshot_id"`
	Date       string   `json:"date"`
	WeightKg   *float64 `json:"weight_kg,omitempty"`
	BodyFatPct *float64 `json:"body_fat_pct,omitempty"`
}
