package sqlite

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/rafaelsoares/alfredo/internal/health/domain"
)

func TestHealthAppointmentRepositoryCRUD(t *testing.T) {
	db := openInMemoryDB(t)
	defer func() {
		_ = db.Close()
	}()

	repo := NewHealthAppointmentRepository(db)
	ctx := context.Background()

	// Create
	appt := domain.HealthAppointment{
		ID:                    "appt-1",
		Specialty:             "Cardiologia",
		ScheduledAt:           time.Date(2026, 5, 10, 9, 0, 0, 0, time.UTC),
		Doctor:                ptrString("Dr. Silva"),
		Notes:                 ptrString("Checkup anual"),
		GoogleCalendarEventID: "evt-123",
		CreatedAt:             time.Now().UTC(),
	}

	err := repo.Create(ctx, appt)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// GetByID
	got, err := repo.GetByID(ctx, "appt-1")
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if got.Specialty != "Cardiologia" {
		t.Fatalf("expected specialty Cardiologia, got %q", got.Specialty)
	}

	// GetByID not found
	_, err = repo.GetByID(ctx, "nonexistent")
	if err != domain.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	// List
	appts, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(appts) != 1 {
		t.Fatalf("expected 1 appointment, got %d", len(appts))
	}

	// Delete
	err = repo.Delete(ctx, "appt-1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deleted
	_, err = repo.GetByID(ctx, "appt-1")
	if err != domain.ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}

	// Delete nonexistent
	err = repo.Delete(ctx, "nonexistent")
	if err != domain.ErrNotFound {
		t.Fatalf("expected ErrNotFound on delete nonexistent, got %v", err)
	}
}

func TestHealthAppointmentRepositoryListOrdering(t *testing.T) {
	db := openInMemoryDB(t)
	defer func() {
		_ = db.Close()
	}()

	repo := NewHealthAppointmentRepository(db)
	ctx := context.Background()

	now := time.Now().UTC()
	appts := []domain.HealthAppointment{
		{
			ID:          "appt-3",
			Specialty:   "Neurologia",
			ScheduledAt: now.Add(2 * time.Hour),
			CreatedAt:   now,
		},
		{
			ID:          "appt-1",
			Specialty:   "Cardiologia",
			ScheduledAt: now,
			CreatedAt:   now,
		},
		{
			ID:          "appt-2",
			Specialty:   "Dentista",
			ScheduledAt: now.Add(1 * time.Hour),
			CreatedAt:   now,
		},
	}

	for _, a := range appts {
		if err := repo.Create(ctx, a); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(list) != 3 {
		t.Fatalf("expected 3 appointments, got %d", len(list))
	}

	if list[0].ID != "appt-1" || list[1].ID != "appt-2" || list[2].ID != "appt-3" {
		t.Fatalf("expected order appt-1, appt-2, appt-3; got %v, %v, %v", list[0].ID, list[1].ID, list[2].ID)
	}
}

func openInMemoryDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}

	// Create the health_appointments table
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS health_appointments (
			id                       TEXT PRIMARY KEY,
			specialty                TEXT NOT NULL,
			scheduled_at             TEXT NOT NULL,
			doctor                   TEXT,
			notes                    TEXT,
			google_calendar_event_id TEXT NOT NULL DEFAULT '',
			created_at               TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_health_appointments_scheduled_at ON health_appointments(scheduled_at);
	`); err != nil {
		_ = db.Close()
		t.Fatalf("failed to create table: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Close()
	})

	return db
}

func ptrString(s string) *string {
	return &s
}
