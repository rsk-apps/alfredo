package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
	"github.com/rafaelsoares/alfredo/internal/fitness/port"
)

type CreateGoalInput struct {
	Name        string
	Description *string
	TargetValue *float64
	TargetUnit  *string
	Deadline    *time.Time
}

type UpdateGoalInput struct {
	Name        *string
	Description *string
	TargetValue *float64
	TargetUnit  *string
	Deadline    *time.Time
}

type GoalService struct {
	repo port.GoalRepository
}

func NewGoalService(repo port.GoalRepository) *GoalService {
	return &GoalService{repo: repo}
}

func (s *GoalService) Create(ctx context.Context, in CreateGoalInput) (*domain.Goal, error) {
	if in.Name == "" {
		return nil, fmt.Errorf("%w: name is required", domain.ErrValidation)
	}
	now := time.Now().UTC()
	return s.repo.Create(ctx, domain.Goal{
		ID:          uuid.New().String(),
		Name:        in.Name,
		Description: in.Description,
		TargetValue: in.TargetValue,
		TargetUnit:  in.TargetUnit,
		Deadline:    in.Deadline,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
}

func (s *GoalService) GetByID(ctx context.Context, id string) (*domain.Goal, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *GoalService) List(ctx context.Context) ([]domain.Goal, error) {
	return s.repo.List(ctx)
}

func (s *GoalService) Update(ctx context.Context, id string, in UpdateGoalInput) (*domain.Goal, error) {
	g, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if in.Name != nil {
		g.Name = *in.Name
	}
	if in.Description != nil {
		g.Description = in.Description
	}
	if in.TargetValue != nil {
		g.TargetValue = in.TargetValue
	}
	if in.TargetUnit != nil {
		g.TargetUnit = in.TargetUnit
	}
	if in.Deadline != nil {
		g.Deadline = in.Deadline
	}
	g.UpdatedAt = time.Now().UTC()
	return s.repo.Update(ctx, *g)
}

func (s *GoalService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *GoalService) Achieve(ctx context.Context, id string) (*domain.Goal, error) {
	g, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if g.AchievedAt != nil {
		return nil, fmt.Errorf("%w: goal %s has already been achieved", domain.ErrAlreadyAchieved, id)
	}
	now := time.Now().UTC()
	g.AchievedAt = &now
	g.UpdatedAt = now
	return s.repo.Update(ctx, *g)
}
