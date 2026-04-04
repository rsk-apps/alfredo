// internal/app/fitness_ports.go
package app

import (
	"context"
	"time"

	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
	fitnesssvc "github.com/rafaelsoares/alfredo/internal/fitness/service"
)

// FitnessProfileServicer is the narrow interface consumed by FitnessProfileUseCase.
type FitnessProfileServicer interface {
	Create(ctx context.Context, in fitnesssvc.CreateProfileInput) (*domain.Profile, error)
	Get(ctx context.Context) (*domain.Profile, error)
	Update(ctx context.Context, in fitnesssvc.UpdateProfileInput) (*domain.Profile, error)
}

// FitnessWorkoutServicer is the narrow interface consumed by FitnessIngestionUseCase.
type FitnessWorkoutServicer interface {
	Create(ctx context.Context, in fitnesssvc.CreateWorkoutInput) (*domain.Workout, error)
	GetByID(ctx context.Context, id string) (*domain.Workout, error)
	List(ctx context.Context, from, to *time.Time) ([]domain.Workout, error)
	Delete(ctx context.Context, id string) error
}

// FitnessBodySnapshotServicer is the narrow interface consumed by FitnessBodyUseCase.
type FitnessBodySnapshotServicer interface {
	Create(ctx context.Context, in fitnesssvc.CreateBodySnapshotInput) (*domain.BodySnapshot, error)
	GetByID(ctx context.Context, id string) (*domain.BodySnapshot, error)
	List(ctx context.Context, from, to *time.Time) ([]domain.BodySnapshot, error)
	Delete(ctx context.Context, id string) error
}

// FitnessGoalServicer is the narrow interface consumed by FitnessGoalUseCase.
type FitnessGoalServicer interface {
	Create(ctx context.Context, in fitnesssvc.CreateGoalInput) (*domain.Goal, error)
	GetByID(ctx context.Context, id string) (*domain.Goal, error)
	List(ctx context.Context) ([]domain.Goal, error)
	Update(ctx context.Context, id string, in fitnesssvc.UpdateGoalInput) (*domain.Goal, error)
	Delete(ctx context.Context, id string) error
	Achieve(ctx context.Context, id string) (*domain.Goal, error)
}
