package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rafaelsoares/alfredo/internal/health/domain"
)

var errWorkoutRepoFail = errors.New("workout repo error")

type workoutRepoStub struct {
	bulkUpsertFn func(context.Context, []domain.WorkoutSession) (int, error)
	listFn       func(context.Context, time.Time, time.Time) ([]domain.WorkoutSession, error)
}

func (r *workoutRepoStub) BulkUpsert(ctx context.Context, sessions []domain.WorkoutSession) (int, error) {
	if r.bulkUpsertFn != nil {
		return r.bulkUpsertFn(ctx, sessions)
	}
	return 0, nil
}

func (r *workoutRepoStub) List(ctx context.Context, from, to time.Time) ([]domain.WorkoutSession, error) {
	if r.listFn != nil {
		return r.listFn(ctx, from, to)
	}
	return nil, nil
}

func TestWorkoutServiceImportDelegatesAndWrapsRepositoryErrors(t *testing.T) {
	svc := NewWorkoutService(
		&workoutRepoStub{
			bulkUpsertFn: func(context.Context, []domain.WorkoutSession) (int, error) {
				return 0, errWorkoutRepoFail
			},
		},
		&rawImportRepoStub{},
	)

	_, err := svc.Import(context.Background(), nil, "", time.Now())
	if !errors.Is(err, errWorkoutRepoFail) {
		t.Fatalf("Import error = %v, want wrapped workout repo error", err)
	}
}

func TestWorkoutServiceImportWritesToBothRepositories(t *testing.T) {
	startDate := time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)
	sessions := []domain.WorkoutSession{{
		ActivityType: "Running",
		StartDate:    startDate,
		EndDate:      startDate.Add(30 * time.Minute),
		DurationSeconds: 1800,
	}}
	payload := `{"workouts": []}`
	importedAt := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)

	workoutCalled := false
	rawCalled := false

	svc := NewWorkoutService(
		&workoutRepoStub{
			bulkUpsertFn: func(_ context.Context, s []domain.WorkoutSession) (int, error) {
				workoutCalled = true
				if len(s) != 1 || s[0].ActivityType != "Running" {
					t.Fatalf("BulkUpsert got %#v", s)
				}
				return 1, nil
			},
		},
		&rawImportRepoStub{
			storeFn: func(_ context.Context, importType, p string, ia time.Time) error {
				rawCalled = true
				if importType != "workouts" || p != payload || ia != importedAt {
					t.Fatalf("Store got importType=%s, payload=%s, importedAt=%v", importType, p, ia)
				}
				return nil
			},
		},
	)

	count, err := svc.Import(context.Background(), sessions, payload, importedAt)
	if err != nil {
		t.Fatalf("Import error = %v", err)
	}
	if !workoutCalled || !rawCalled {
		t.Fatalf("workoutCalled=%v, rawCalled=%v", workoutCalled, rawCalled)
	}
	if count != 1 {
		t.Fatalf("Import count = %d, want 1", count)
	}
}

func TestWorkoutServiceImportWrapsRawImportErrors(t *testing.T) {
	svc := NewWorkoutService(
		&workoutRepoStub{
			bulkUpsertFn: func(context.Context, []domain.WorkoutSession) (int, error) {
				return 1, nil
			},
		},
		&rawImportRepoStub{
			storeFn: func(context.Context, string, string, time.Time) error {
				return errRawImportFail
			},
		},
	)

	_, err := svc.Import(context.Background(), nil, "", time.Now())
	if !errors.Is(err, errRawImportFail) {
		t.Fatalf("Import error = %v, want wrapped raw import error", err)
	}
}

func TestWorkoutServiceListDelegatesAndWrapsErrors(t *testing.T) {
	svc := NewWorkoutService(
		&workoutRepoStub{
			listFn: func(context.Context, time.Time, time.Time) ([]domain.WorkoutSession, error) {
				return nil, errWorkoutRepoFail
			},
		},
		&rawImportRepoStub{},
	)

	_, err := svc.List(context.Background(), time.Now(), time.Now())
	if !errors.Is(err, errWorkoutRepoFail) {
		t.Fatalf("List error = %v, want wrapped error", err)
	}
}
