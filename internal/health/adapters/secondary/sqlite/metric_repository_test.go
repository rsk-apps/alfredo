package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/rafaelsoares/alfredo/internal/health/domain"
)

func TestMetricRepositoryBulkUpsertIdempotency(t *testing.T) {
	db := openHealthTestDB(t)
	repo := NewMetricRepository(db)
	ctx := context.Background()

	metrics := []domain.DailyMetric{
		{Date: "2026-04-18", MetricType: "weight", Value: 80.5, Unit: "kg"},
		{Date: "2026-04-18", MetricType: "restingHeartRate", Value: 65, Unit: "bpm"},
		{Date: "2026-04-19", MetricType: "weight", Value: 80.6, Unit: "kg"},
	}

	count1, err := repo.BulkUpsert(ctx, metrics)
	if err != nil {
		t.Fatalf("first BulkUpsert: %v", err)
	}
	if count1 != 3 {
		t.Fatalf("first BulkUpsert count = %d, want 3", count1)
	}

	// Re-upsert with same data — should replace without duplicating
	count2, err := repo.BulkUpsert(ctx, metrics)
	if err != nil {
		t.Fatalf("second BulkUpsert: %v", err)
	}
	if count2 != 3 {
		t.Fatalf("second BulkUpsert count = %d, want 3", count2)
	}

	// Verify only 3 rows exist
	var totalCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM health_daily_metrics").Scan(&totalCount); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if totalCount != 3 {
		t.Fatalf("total rows = %d, want 3 (idempotency failed)", totalCount)
	}
}

func TestMetricRepositoryListFiltersAndOrders(t *testing.T) {
	db := openHealthTestDB(t)
	repo := NewMetricRepository(db)
	ctx := context.Background()

	metrics := []domain.DailyMetric{
		{Date: "2026-04-16", MetricType: "weight", Value: 80.0, Unit: "kg"},
		{Date: "2026-04-17", MetricType: "weight", Value: 80.2, Unit: "kg"},
		{Date: "2026-04-18", MetricType: "weight", Value: 80.5, Unit: "kg"},
		{Date: "2026-04-19", MetricType: "weight", Value: 80.6, Unit: "kg"},
		{Date: "2026-04-18", MetricType: "restingHeartRate", Value: 65, Unit: "bpm"},
	}

	_, err := repo.BulkUpsert(ctx, metrics)
	if err != nil {
		t.Fatalf("BulkUpsert: %v", err)
	}

	// List only weight metrics from 2026-04-17 to 2026-04-19
	got, err := repo.List(ctx, "weight",
		time.Date(2026, 4, 17, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 19, 0, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("List count = %d, want 3", len(got))
	}

	// Verify descending order by date
	if got[0].Date != "2026-04-19" || got[1].Date != "2026-04-18" || got[2].Date != "2026-04-17" {
		t.Fatalf("List order = %v, want descending by date", []string{got[0].Date, got[1].Date, got[2].Date})
	}

	// Verify all are weight type
	for _, m := range got {
		if m.MetricType != "weight" {
			t.Fatalf("MetricType = %s, want weight", m.MetricType)
		}
	}
}

func TestMetricRepositorySleepStagesRoundTrip(t *testing.T) {
	db := openHealthTestDB(t)
	repo := NewMetricRepository(db)
	ctx := context.Background()

	sleepStages := &domain.SleepStages{
		Awake:       30.5,
		Core:        120.0,
		Deep:        60.5,
		REM:         45.0,
		Unspecified: 4.0,
	}

	metrics := []domain.DailyMetric{
		{
			Date:        "2026-04-18",
			MetricType:  "sleepTime",
			Value:       260.0,
			Unit:        "minutes",
			SleepStages: sleepStages,
		},
	}

	_, err := repo.BulkUpsert(ctx, metrics)
	if err != nil {
		t.Fatalf("BulkUpsert: %v", err)
	}

	// List and verify sleep stages
	got, err := repo.List(ctx, "sleepTime",
		time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("List count = %d, want 1", len(got))
	}

	m := got[0]
	if m.SleepStages == nil {
		t.Fatal("SleepStages = nil, want non-nil")
	}

	if m.SleepStages.Awake != 30.5 || m.SleepStages.Core != 120.0 || m.SleepStages.Deep != 60.5 ||
		m.SleepStages.REM != 45.0 || m.SleepStages.Unspecified != 4.0 {
		t.Fatalf("SleepStages round-trip failed: %#v, want %#v", m.SleepStages, sleepStages)
	}
}

func TestMetricRepositoryNullSleepStagesForOtherMetrics(t *testing.T) {
	db := openHealthTestDB(t)
	repo := NewMetricRepository(db)
	ctx := context.Background()

	metrics := []domain.DailyMetric{
		{Date: "2026-04-18", MetricType: "weight", Value: 80.5, Unit: "kg"},
	}

	_, err := repo.BulkUpsert(ctx, metrics)
	if err != nil {
		t.Fatalf("BulkUpsert: %v", err)
	}

	got, err := repo.List(ctx, "weight",
		time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("List count = %d, want 1", len(got))
	}

	if got[0].SleepStages != nil {
		t.Fatalf("SleepStages = %#v, want nil", got[0].SleepStages)
	}
}
