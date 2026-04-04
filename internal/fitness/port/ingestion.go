// internal/fitness/port/ingestion.go
package port

import (
	"context"

	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
)

// WorkoutIngester is the seam for Apple Fitness (and future) workout sources.
// Both single and batch ingestion are supported so the interface does not
// constrain the spike to a particular delivery mechanism.
type WorkoutIngester interface {
	IngestWorkout(ctx context.Context, w domain.Workout) (*domain.Workout, error)
	IngestWorkoutBatch(ctx context.Context, ws []domain.Workout) ([]domain.Workout, error)
}
