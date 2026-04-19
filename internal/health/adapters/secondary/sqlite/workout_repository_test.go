package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/rafaelsoares/alfredo/internal/database"
	"github.com/rafaelsoares/alfredo/internal/health/domain"
)

func TestWorkoutRepositoryBulkUpsertIdempotency(t *testing.T) {
	db := openHealthTestDB(t)
	repo := NewWorkoutRepository(db)
	ctx := context.Background()

	startDate1 := time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)
	endDate1 := startDate1.Add(30 * time.Minute)

	sessions := []domain.WorkoutSession{
		{
			ActivityType:    "Running",
			StartDate:       startDate1,
			EndDate:         endDate1,
			DurationSeconds: 1800,
			Source:          "Apple Watch",
		},
	}

	count1, err := repo.BulkUpsert(ctx, sessions)
	if err != nil {
		t.Fatalf("first BulkUpsert: %v", err)
	}
	if count1 != 1 {
		t.Fatalf("first BulkUpsert count = %d, want 1", count1)
	}

	// Re-upsert with same start_date — should replace without duplicating
	count2, err := repo.BulkUpsert(ctx, sessions)
	if err != nil {
		t.Fatalf("second BulkUpsert: %v", err)
	}
	if count2 != 1 {
		t.Fatalf("second BulkUpsert count = %d, want 1", count2)
	}

	// Verify only 1 row exists
	var totalCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM health_workout_sessions").Scan(&totalCount); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if totalCount != 1 {
		t.Fatalf("total rows = %d, want 1 (idempotency failed)", totalCount)
	}
}

func TestWorkoutRepositoryListFiltersAndOrders(t *testing.T) {
	db := openHealthTestDB(t)
	repo := NewWorkoutRepository(db)
	ctx := context.Background()

	start1 := time.Date(2026, 4, 16, 10, 0, 0, 0, time.UTC)
	start2 := time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)
	start3 := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)

	sessions := []domain.WorkoutSession{
		{ActivityType: "Running", StartDate: start1, EndDate: start1.Add(30 * time.Minute), DurationSeconds: 1800, Source: "Apple Watch"},
		{ActivityType: "Cycling", StartDate: start2, EndDate: start2.Add(45 * time.Minute), DurationSeconds: 2700, Source: "Apple Watch"},
		{ActivityType: "Swimming", StartDate: start3, EndDate: start3.Add(60 * time.Minute), DurationSeconds: 3600, Source: "Apple Watch"},
	}

	_, err := repo.BulkUpsert(ctx, sessions)
	if err != nil {
		t.Fatalf("BulkUpsert: %v", err)
	}

	// List workouts from 2026-04-17 to 2026-04-19
	got, err := repo.List(ctx,
		time.Date(2026, 4, 17, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 19, 23, 59, 59, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("List count = %d, want 1 (only 2026-04-18 session)", len(got))
	}

	if got[0].ActivityType != "Cycling" {
		t.Fatalf("ActivityType = %s, want Cycling", got[0].ActivityType)
	}
}

func TestWorkoutRepositoryBulkUpsertPreservesCreatedAt(t *testing.T) {
	db := openHealthTestDB(t)
	repo := NewWorkoutRepository(db)
	ctx := context.Background()

	startDate := time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)
	sessions := []domain.WorkoutSession{
		{ActivityType: "Running", StartDate: startDate, EndDate: startDate.Add(30 * time.Minute), DurationSeconds: 1800, Source: "Apple Watch"},
	}

	_, err := repo.BulkUpsert(ctx, sessions)
	if err != nil {
		t.Fatalf("first BulkUpsert: %v", err)
	}

	var originalCreatedAt string
	if err := db.QueryRow("SELECT created_at FROM health_workout_sessions WHERE activity_type='Running'").Scan(&originalCreatedAt); err != nil {
		t.Fatalf("read created_at: %v", err)
	}

	time.Sleep(2 * time.Millisecond)

	// Re-upsert same start_date with different activity type
	sessions[0].ActivityType = "Trail Running"
	_, err = repo.BulkUpsert(ctx, sessions)
	if err != nil {
		t.Fatalf("second BulkUpsert: %v", err)
	}

	var newCreatedAt, newUpdatedAt string
	if err := db.QueryRow("SELECT created_at, updated_at FROM health_workout_sessions").Scan(&newCreatedAt, &newUpdatedAt); err != nil {
		t.Fatalf("read timestamps after re-upsert: %v", err)
	}

	if newCreatedAt != originalCreatedAt {
		t.Fatalf("created_at changed from %q to %q on re-upsert (should be preserved)", originalCreatedAt, newCreatedAt)
	}
	if newUpdatedAt <= originalCreatedAt {
		t.Fatalf("updated_at %q should be after original created_at %q", newUpdatedAt, originalCreatedAt)
	}
}

func TestWorkoutRepositoryBulkUpsertEmptySlice(t *testing.T) {
	db := openHealthTestDB(t)
	repo := NewWorkoutRepository(db)

	count, err := repo.BulkUpsert(context.Background(), []domain.WorkoutSession{})
	if err != nil {
		t.Fatalf("BulkUpsert empty: %v", err)
	}
	if count != 0 {
		t.Fatalf("count = %d, want 0 for empty slice", count)
	}
}

func TestWorkoutRepositoryBulkUpsertClosedDBReturnsError(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "closed.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	repo := NewWorkoutRepository(db)
	_, err = repo.BulkUpsert(context.Background(), []domain.WorkoutSession{
		{ActivityType: "Running", StartDate: time.Now(), EndDate: time.Now().Add(30 * time.Minute), DurationSeconds: 1800, Source: "Apple Watch"},
	})
	if err == nil {
		t.Fatal("want error from closed db, got nil")
	}
}

func TestWorkoutRepositoryListClosedDBReturnsError(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "closed2.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	repo := NewWorkoutRepository(db)
	_, err = repo.List(context.Background(),
		time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 30, 23, 59, 59, 0, time.UTC),
	)
	if err == nil {
		t.Fatal("want error from closed db, got nil")
	}
}

func TestWorkoutRepositoryListBadStartDateReturnsError(t *testing.T) {
	db := openHealthTestDB(t)
	repo := NewWorkoutRepository(db)
	ctx := context.Background()

	// Insert a row with an invalid start_date that will be returned by the range query
	// but fail time.Parse(RFC3339Nano, ...) — use a value that sorts between the query bounds
	_, err := db.ExecContext(ctx, `
		INSERT INTO health_workout_sessions
		(activity_type, start_date, end_date, duration_seconds, source, created_at, updated_at)
		VALUES ('Test', '2026-04-18T15:00:00.bad', '2026-04-18T15:30:00Z', 1800, 'test', '2026-04-18T10:00:00Z', '2026-04-18T10:00:00Z')
	`)
	if err != nil {
		t.Fatalf("raw insert: %v", err)
	}

	_, err = repo.List(ctx,
		time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 18, 23, 59, 59, 0, time.UTC),
	)
	if err == nil {
		t.Fatal("want parse error for invalid start_date, got nil")
	}
}

func TestWorkoutRepositoryNullableStatistics(t *testing.T) {
	db := openHealthTestDB(t)
	repo := NewWorkoutRepository(db)
	ctx := context.Background()

	startDate := time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)
	endDate := startDate.Add(30 * time.Minute)

	activeCalories := 250.5
	sessions := []domain.WorkoutSession{
		{
			ActivityType:       "CoreTraining",
			StartDate:          startDate,
			EndDate:            endDate,
			DurationSeconds:    1800,
			ActiveCaloriesKcal: &activeCalories,
			// All other nullable fields are nil
			Source: "Apple Watch",
		},
	}

	_, err := repo.BulkUpsert(ctx, sessions)
	if err != nil {
		t.Fatalf("BulkUpsert: %v", err)
	}

	got, err := repo.List(ctx,
		time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 18, 23, 59, 59, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("List count = %d, want 1", len(got))
	}

	w := got[0]
	if w.ActiveCaloriesKcal == nil || *w.ActiveCaloriesKcal != activeCalories {
		t.Fatalf("ActiveCaloriesKcal = %#v, want %#v", w.ActiveCaloriesKcal, &activeCalories)
	}

	if w.BasalCaloriesKcal != nil || w.HRAvgBPM != nil || w.HRMinBPM != nil ||
	   w.HRMaxBPM != nil || w.DistanceM != nil {
		t.Fatalf("nullable fields should be nil, got: basal=%v, hrAvg=%v, hrMin=%v, hrMax=%v, distance=%v",
			w.BasalCaloriesKcal, w.HRAvgBPM, w.HRMinBPM, w.HRMaxBPM, w.DistanceM)
	}
}
