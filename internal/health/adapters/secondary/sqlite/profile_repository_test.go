package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/rafaelsoares/alfredo/internal/database"
	"github.com/rafaelsoares/alfredo/internal/health/domain"
)

func TestProfileRepositoryGetUpsertAndSingletonSemantics(t *testing.T) {
	db := openHealthTestDB(t)
	repo := NewProfileRepository(db)
	ctx := context.Background()

	_, err := repo.Get(ctx)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("fresh Get error = %v, want ErrNotFound", err)
	}

	first, err := repo.Upsert(ctx, domain.HealthProfile{
		HeightCM:  178.0,
		BirthDate: "1993-06-15",
		Sex:       "male",
	})
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if first.ID != 1 {
		t.Fatalf("first ID = %d, want 1", first.ID)
	}

	got, err := repo.Get(ctx)
	if err != nil {
		t.Fatalf("get profile: %v", err)
	}
	assertProfile(t, got, 178.0, "1993-06-15", "male")

	second, err := repo.Upsert(ctx, domain.HealthProfile{
		HeightCM:  180.0,
		BirthDate: "1993-06-15",
		Sex:       "other",
	})
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if second.HeightCM != 180.0 || second.Sex != "other" {
		t.Fatalf("second upsert = %#v, want updated values", second)
	}
	if !second.CreatedAt.Equal(first.CreatedAt) {
		t.Fatalf("created_at changed: first=%v second=%v", first.CreatedAt, second.CreatedAt)
	}
	if !second.UpdatedAt.After(first.UpdatedAt) {
		t.Fatalf("updated_at did not advance: first=%v second=%v", first.UpdatedAt, second.UpdatedAt)
	}

	got, err = repo.Get(ctx)
	if err != nil {
		t.Fatalf("get updated profile: %v", err)
	}
	assertProfile(t, got, 180.0, "1993-06-15", "other")

	if n := countHealthRows(t, db); n != 1 {
		t.Fatalf("health_profiles rows = %d, want 1", n)
	}
}

func TestProfileRepositoryCalendarIDDoesNotRequireProfileRow(t *testing.T) {
	db := openHealthTestDB(t)
	repo := NewProfileRepository(db)
	ctx := context.Background()

	got, err := repo.GetCalendarID(ctx)
	if err != nil {
		t.Fatalf("get empty calendar id: %v", err)
	}
	if got != "" {
		t.Fatalf("empty calendar id = %q, want empty", got)
	}

	if err := repo.SetCalendarID(ctx, "cal-health"); err != nil {
		t.Fatalf("set calendar id: %v", err)
	}

	got, err = repo.GetCalendarID(ctx)
	if err != nil {
		t.Fatalf("get stored calendar id: %v", err)
	}
	if got != "cal-health" {
		t.Fatalf("stored calendar id = %q, want %q", got, "cal-health")
	}

	if n := countHealthRows(t, db); n != 0 {
		t.Fatalf("health_profiles rows = %d, want 0", n)
	}
}

func openHealthTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "alfredo.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close test db: %v", err)
		}
	})
	return db
}

func countHealthRows(t *testing.T, db *sql.DB) int {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM health_profiles`).Scan(&count); err != nil {
		t.Fatalf("count health rows: %v", err)
	}
	return count
}

func assertProfile(t *testing.T, got domain.HealthProfile, height float64, birthDate, sex string) {
	t.Helper()
	if got.ID != 1 {
		t.Fatalf("id = %d, want 1", got.ID)
	}
	if got.HeightCM != height {
		t.Fatalf("height_cm = %v, want %v", got.HeightCM, height)
	}
	if got.BirthDate != birthDate {
		t.Fatalf("birth_date = %q, want %q", got.BirthDate, birthDate)
	}
	if got.Sex != sex {
		t.Fatalf("sex = %q, want %q", got.Sex, sex)
	}
	if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Fatalf("timestamps are zero: %#v", got)
	}
}
