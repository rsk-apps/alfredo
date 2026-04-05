package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
	"github.com/rafaelsoares/alfredo/internal/fitness/service"
)

type mockGoalRepo struct {
	goal *domain.Goal
	err  error
}

func (m *mockGoalRepo) Create(_ context.Context, g domain.Goal) (*domain.Goal, error) {
	return &g, m.err
}
func (m *mockGoalRepo) GetByID(_ context.Context, _ string) (*domain.Goal, error) {
	return m.goal, m.err
}
func (m *mockGoalRepo) List(_ context.Context) ([]domain.Goal, error) {
	if m.goal != nil {
		return []domain.Goal{*m.goal}, m.err
	}
	return nil, m.err
}
func (m *mockGoalRepo) Update(_ context.Context, g domain.Goal) (*domain.Goal, error) {
	return &g, m.err
}
func (m *mockGoalRepo) Delete(_ context.Context, _ string) error { return m.err }

func TestGoalService_Create_AssignsID(t *testing.T) {
	svc := service.NewGoalService(&mockGoalRepo{})
	g, err := svc.Create(context.Background(), service.CreateGoalInput{Name: "Run a 5k"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.ID == "" {
		t.Error("expected ID to be assigned")
	}
}

func TestGoalService_Create_RequiresName(t *testing.T) {
	svc := service.NewGoalService(&mockGoalRepo{})
	_, err := svc.Create(context.Background(), service.CreateGoalInput{})
	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("got %v, want ErrValidation", err)
	}
}

func TestGoalService_Achieve_AlreadyAchieved(t *testing.T) {
	achieved := time.Now()
	svc := service.NewGoalService(&mockGoalRepo{
		goal: &domain.Goal{ID: "g1", Name: "Run a 5k", AchievedAt: &achieved},
	})
	_, err := svc.Achieve(context.Background(), "g1")
	if !errors.Is(err, domain.ErrAlreadyAchieved) {
		t.Errorf("got %v, want ErrAlreadyAchieved", err)
	}
}

func TestGoalService_Achieve_SetsAchievedAt(t *testing.T) {
	svc := service.NewGoalService(&mockGoalRepo{
		goal: &domain.Goal{ID: "g1", Name: "Run a 5k"},
	})
	g, err := svc.Achieve(context.Background(), "g1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.AchievedAt == nil {
		t.Error("expected AchievedAt to be set")
	}
}
