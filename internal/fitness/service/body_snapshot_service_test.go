package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
	"github.com/rafaelsoares/alfredo/internal/fitness/service"
)

type mockBodySnapshotRepo struct {
	snapshot *domain.BodySnapshot
	err      error
}

func (m *mockBodySnapshotRepo) Create(_ context.Context, s domain.BodySnapshot) (*domain.BodySnapshot, error) {
	return &s, m.err
}
func (m *mockBodySnapshotRepo) GetByID(_ context.Context, _ string) (*domain.BodySnapshot, error) {
	return m.snapshot, m.err
}
func (m *mockBodySnapshotRepo) List(_ context.Context, _, _ *time.Time) ([]domain.BodySnapshot, error) {
	if m.snapshot != nil {
		return []domain.BodySnapshot{*m.snapshot}, m.err
	}
	return nil, m.err
}
func (m *mockBodySnapshotRepo) Delete(_ context.Context, _ string) error { return m.err }

func TestBodySnapshotService_Create_AssignsID(t *testing.T) {
	svc := service.NewBodySnapshotService(&mockBodySnapshotRepo{})
	w := 75.0
	s, err := svc.Create(context.Background(), service.CreateBodySnapshotInput{
		Date: time.Now(), WeightKg: &w,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.ID == "" {
		t.Error("expected ID to be assigned")
	}
}

func TestBodySnapshotService_Create_RequiresDate(t *testing.T) {
	svc := service.NewBodySnapshotService(&mockBodySnapshotRepo{})
	_, err := svc.Create(context.Background(), service.CreateBodySnapshotInput{})
	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("got %v, want ErrValidation", err)
	}
}

func TestBodySnapshotService_Create_PropagatesDuplicateError(t *testing.T) {
	svc := service.NewBodySnapshotService(&mockBodySnapshotRepo{err: domain.ErrAlreadyExists})
	d := time.Now()
	_, err := svc.Create(context.Background(), service.CreateBodySnapshotInput{Date: d})
	if !errors.Is(err, domain.ErrAlreadyExists) {
		t.Errorf("got %v, want ErrAlreadyExists", err)
	}
}
