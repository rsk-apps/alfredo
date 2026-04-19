package sqlite

import (
	"context"
	"testing"
	"time"
)

func TestRawImportRepositoryStore(t *testing.T) {
	db := openHealthTestDB(t)
	repo := NewRawImportRepository(db)
	ctx := context.Background()

	payload := `{"test": "data"}`
	importedAt := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)

	err := repo.Store(ctx, "metrics", payload, importedAt)
	if err != nil {
		t.Fatalf("Store: %v", err)
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM health_raw_imports").Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}

	var storedType, storedPayload string
	if err := db.QueryRow("SELECT import_type, payload FROM health_raw_imports WHERE import_type = ?", "metrics").
		Scan(&storedType, &storedPayload); err != nil {
		t.Fatalf("query: %v", err)
	}
	if storedType != "metrics" || storedPayload != payload {
		t.Fatalf("stored = (%s, %s), want (metrics, %s)", storedType, storedPayload, payload)
	}
}

func TestRawImportRepositoryMultipleImports(t *testing.T) {
	db := openHealthTestDB(t)
	repo := NewRawImportRepository(db)
	ctx := context.Background()

	err := repo.Store(ctx, "metrics", `{"metrics":"1"}`, time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Store metrics: %v", err)
	}

	err = repo.Store(ctx, "workouts", `{"workouts":"1"}`, time.Date(2026, 4, 18, 11, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Store workouts: %v", err)
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM health_raw_imports").Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}
}
