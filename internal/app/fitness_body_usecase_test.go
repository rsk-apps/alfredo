// internal/app/fitness_body_usecase_test.go
package app_test

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/rafaelsoares/alfredo/internal/app"
	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
	fitnesssvc "github.com/rafaelsoares/alfredo/internal/fitness/service"
)

type stubFitnessBodySnapshotService struct {
	snapshot *domain.BodySnapshot
}

func (s *stubFitnessBodySnapshotService) Create(_ context.Context, in fitnesssvc.CreateBodySnapshotInput) (*domain.BodySnapshot, error) {
	return &domain.BodySnapshot{ID: "bs1", Date: in.Date, WeightKg: in.WeightKg}, nil
}
func (s *stubFitnessBodySnapshotService) GetByID(_ context.Context, _ string) (*domain.BodySnapshot, error) {
	return s.snapshot, nil
}
func (s *stubFitnessBodySnapshotService) List(_ context.Context, _, _ *time.Time) ([]domain.BodySnapshot, error) {
	return nil, nil
}
func (s *stubFitnessBodySnapshotService) Delete(_ context.Context, _ string) error { return nil }
func (s *stubFitnessBodySnapshotService) CurrentBodyState(_ context.Context) (*domain.BodySnapshot, error) {
	return s.snapshot, nil
}

func TestFitnessBodyUseCase_CreateSnapshot_EmitsEvent(t *testing.T) {
	spy := &spyEmitter{}
	uc := app.NewFitnessBodyUseCase(&stubFitnessBodySnapshotService{}, spy, zap.NewNop())

	w := 75.0
	_, err := uc.CreateSnapshot(context.Background(), fitnesssvc.CreateBodySnapshotInput{
		Date: time.Now(), WeightKg: &w,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(spy.events) != 1 || spy.events[0] != "fitness.body_snapshot.saved" {
		t.Errorf("events = %v, want [fitness.body_snapshot.saved]", spy.events)
	}
}
