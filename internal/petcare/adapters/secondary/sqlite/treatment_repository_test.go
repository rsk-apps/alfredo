package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
)

func TestTreatmentRepository_CreateListGetAndStop(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	insertTestPet(t, db, "pet-1")
	insertTestPet(t, db, "pet-2")
	repo := NewTreatmentRepository(db)
	startedAt := time.Date(2026, 4, 17, 8, 0, 0, 0, time.UTC)
	endedAt := startedAt.Add(24 * time.Hour)
	vet := "Dra Ana"
	notes := "com comida"

	created, err := repo.Create(ctx, domain.Treatment{
		ID:                    "treatment-1",
		PetID:                 "pet-1",
		Name:                  "Antibiotico",
		DosageAmount:          2.5,
		DosageUnit:            "ml",
		Route:                 "oral",
		IntervalHours:         12,
		StartedAt:             startedAt,
		EndedAt:               &endedAt,
		VetName:               &vet,
		Notes:                 &notes,
		GoogleCalendarEventID: "series-1",
		CreatedAt:             startedAt,
	})
	if err != nil {
		t.Fatalf("create treatment: %v", err)
	}
	if created.ID != "treatment-1" {
		t.Fatalf("created id = %q, want treatment-1", created.ID)
	}

	got, err := repo.GetByID(ctx, "pet-1", "treatment-1")
	if err != nil {
		t.Fatalf("get treatment: %v", err)
	}
	if got.EndedAt == nil || !got.EndedAt.Equal(endedAt) || got.VetName == nil || *got.VetName != vet {
		t.Fatalf("treatment details = %#v", got)
	}
	_, err = repo.GetByID(ctx, "pet-2", "treatment-1")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("wrong pet get error = %v, want ErrNotFound", err)
	}
	listed, err := repo.List(ctx, "pet-1")
	if err != nil {
		t.Fatalf("list treatments: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != "treatment-1" {
		t.Fatalf("listed treatments = %#v, want treatment-1", listed)
	}

	stoppedAt := startedAt.Add(time.Hour)
	if err := repo.Stop(ctx, "treatment-1", stoppedAt); err != nil {
		t.Fatalf("stop treatment: %v", err)
	}
	stopped, err := repo.GetByID(ctx, "pet-1", "treatment-1")
	if err != nil {
		t.Fatalf("get stopped treatment: %v", err)
	}
	if stopped.StoppedAt == nil || !stopped.StoppedAt.Equal(stoppedAt) {
		t.Fatalf("stopped_at = %#v, want %s", stopped.StoppedAt, stoppedAt)
	}
	if err := repo.Stop(ctx, "treatment-1", stoppedAt); !errors.Is(err, domain.ErrAlreadyStopped) {
		t.Fatalf("second stop error = %v, want ErrAlreadyStopped", err)
	}
	if err := repo.Stop(ctx, "missing", stoppedAt); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing stop error = %v, want ErrNotFound", err)
	}
}

func insertTestTreatment(t *testing.T, db dbtx, id, petID string) {
	t.Helper()
	startedAt := time.Date(2026, 4, 17, 8, 0, 0, 0, time.UTC)
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO treatments (id, pet_id, name, dosage_amount, dosage_unit, route, interval_hours, started_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, petID, "Antibiotico", 2.5, "ml", "oral", 12, startedAt.Format(time.RFC3339), startedAt.Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("insert treatment %q: %v", id, err)
	}
}
