package sqlite

import (
	"context"
	"testing"
	"time"

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
