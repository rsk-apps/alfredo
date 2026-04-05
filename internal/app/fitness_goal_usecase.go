// internal/app/fitness_goal_usecase.go
package app

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
	fitnesssvc "github.com/rafaelsoares/alfredo/internal/fitness/service"
	"github.com/rafaelsoares/alfredo/internal/webhook"
)

type FitnessGoalUseCase struct {
	goals   FitnessGoalServicer
	emitter webhook.EventEmitter
	logger  *zap.Logger
}

func NewFitnessGoalUseCase(
	goals FitnessGoalServicer,
	emitter webhook.EventEmitter,
	logger *zap.Logger,
) *FitnessGoalUseCase {
	return &FitnessGoalUseCase{goals: goals, emitter: emitter, logger: logger}
}

func (uc *FitnessGoalUseCase) CreateGoal(ctx context.Context, in fitnesssvc.CreateGoalInput) (*domain.Goal, error) {
	return uc.goals.Create(ctx, in)
}

func (uc *FitnessGoalUseCase) GetGoal(ctx context.Context, id string) (*domain.Goal, error) {
	return uc.goals.GetByID(ctx, id)
}

func (uc *FitnessGoalUseCase) ListGoals(ctx context.Context) ([]domain.Goal, error) {
	return uc.goals.List(ctx)
}

func (uc *FitnessGoalUseCase) UpdateGoal(ctx context.Context, id string, in fitnesssvc.UpdateGoalInput) (*domain.Goal, error) {
	return uc.goals.Update(ctx, id, in)
}

func (uc *FitnessGoalUseCase) DeleteGoal(ctx context.Context, id string) error {
	return uc.goals.Delete(ctx, id)
}

func (uc *FitnessGoalUseCase) AchieveGoal(ctx context.Context, id string) (*domain.Goal, error) {
	g, err := uc.goals.Achieve(ctx, id)
	if err != nil {
		return nil, err
	}
	uc.emitter.Emit(ctx, "fitness.goal.achieved", fitnessGoalAchievedPayload{
		GoalID:     g.ID,
		GoalName:   g.Name,
		AchievedAt: *g.AchievedAt,
	})
	return g, nil
}

// --- payload types ---

type fitnessGoalAchievedPayload struct {
	GoalID     string    `json:"goal_id"`
	GoalName   string    `json:"goal_name"`
	AchievedAt time.Time `json:"achieved_at"`
}
