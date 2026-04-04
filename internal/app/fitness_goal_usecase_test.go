// internal/app/fitness_goal_usecase_test.go
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

type stubFitnessGoalService struct {
	goal *domain.Goal
}

func (s *stubFitnessGoalService) Create(_ context.Context, in fitnesssvc.CreateGoalInput) (*domain.Goal, error) {
	return &domain.Goal{ID: "g1", Name: in.Name}, nil
}
func (s *stubFitnessGoalService) GetByID(_ context.Context, _ string) (*domain.Goal, error) {
	return s.goal, nil
}
func (s *stubFitnessGoalService) List(_ context.Context) ([]domain.Goal, error) { return nil, nil }
func (s *stubFitnessGoalService) Update(_ context.Context, _ string, _ fitnesssvc.UpdateGoalInput) (*domain.Goal, error) {
	return s.goal, nil
}
func (s *stubFitnessGoalService) Delete(_ context.Context, _ string) error { return nil }
func (s *stubFitnessGoalService) Achieve(_ context.Context, _ string) (*domain.Goal, error) {
	now := time.Now()
	g := *s.goal
	g.AchievedAt = &now
	return &g, nil
}

func TestFitnessGoalUseCase_Achieve_EmitsEvent(t *testing.T) {
	spy := &spyEmitter{}
	uc := app.NewFitnessGoalUseCase(&stubFitnessGoalService{
		goal: &domain.Goal{ID: "g1", Name: "Run a 5k"},
	}, spy, zap.NewNop())

	_, err := uc.AchieveGoal(context.Background(), "g1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(spy.events) != 1 || spy.events[0] != "fitness.goal.achieved" {
		t.Errorf("events = %v, want [fitness.goal.achieved]", spy.events)
	}
}

func TestFitnessGoalUseCase_Create_DoesNotEmit(t *testing.T) {
	spy := &spyEmitter{}
	uc := app.NewFitnessGoalUseCase(&stubFitnessGoalService{}, spy, zap.NewNop())

	_, err := uc.CreateGoal(context.Background(), fitnesssvc.CreateGoalInput{Name: "Run a 5k"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(spy.events) != 0 {
		t.Errorf("expected no events, got %v", spy.events)
	}
}
