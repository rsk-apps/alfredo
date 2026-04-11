// internal/fitness/port/ports.go
package port

import (
	"context"
	"time"

	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
)

// ProfileRepository persists and retrieves the single fitness profile record.
type ProfileRepository interface {
	Create(ctx context.Context, p domain.Profile) (*domain.Profile, error)
	Get(ctx context.Context) (*domain.Profile, error)
	Update(ctx context.Context, p domain.Profile) (*domain.Profile, error)
}

// WorkoutRepository persists and retrieves workout sessions.
type WorkoutRepository interface {
	Create(ctx context.Context, w domain.Workout) (*domain.Workout, error)
	GetByID(ctx context.Context, id string) (*domain.Workout, error)
	List(ctx context.Context, from, to *time.Time) ([]domain.Workout, error)
	Delete(ctx context.Context, id string) error
}

// BodySnapshotRepository persists and retrieves point-in-time body measurements.
type BodySnapshotRepository interface {
	Create(ctx context.Context, s domain.BodySnapshot) (*domain.BodySnapshot, error)
	GetByID(ctx context.Context, id string) (*domain.BodySnapshot, error)
	List(ctx context.Context, from, to *time.Time) ([]domain.BodySnapshot, error)
	// LatestBefore returns snapshots strictly before date, ordered by date DESC.
	// limit <= 0 means no limit.
	LatestBefore(ctx context.Context, date time.Time, limit int) ([]domain.BodySnapshot, error)
	Delete(ctx context.Context, id string) error
}

// GoalRepository persists and retrieves fitness goals.
type GoalRepository interface {
	Create(ctx context.Context, g domain.Goal) (*domain.Goal, error)
	GetByID(ctx context.Context, id string) (*domain.Goal, error)
	List(ctx context.Context) ([]domain.Goal, error)
	Update(ctx context.Context, g domain.Goal) (*domain.Goal, error)
	Delete(ctx context.Context, id string) error
}
