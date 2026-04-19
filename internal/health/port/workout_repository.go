package port

import (
	"context"
	"time"

	"github.com/rafaelsoares/alfredo/internal/health/domain"
)

type WorkoutRepository interface {
	BulkUpsert(ctx context.Context, sessions []domain.WorkoutSession) (int, error)
	List(ctx context.Context, from, to time.Time) ([]domain.WorkoutSession, error)
}
