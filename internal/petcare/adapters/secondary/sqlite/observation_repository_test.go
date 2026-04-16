package sqlite_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/rafaelsoares/alfredo/internal/database"
	"github.com/rafaelsoares/alfredo/internal/petcare/adapters/secondary/sqlite"
	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
)

func TestObservationRepository_RoundTripOrdersByObservedAtAndCascades(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "alfredo.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close db: %v", err)
		}
	})

	ctx := context.Background()
	petRepo := sqlite.NewPetRepository(db)
	pet, err := petRepo.Create(ctx, domain.Pet{
		ID:               "pet-1",
		Name:             "Luna",
		Species:          "dog",
		GoogleCalendarID: "cal-1",
		CreatedAt:        time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("create pet: %v", err)
	}

	repo := sqlite.NewObservationRepository(db)
	older := domain.Observation{
		ID:          "obs-old",
		PetID:       pet.ID,
		ObservedAt:  time.Date(2026, 4, 14, 9, 0, 0, 0, time.FixedZone("BRT", -3*60*60)),
		Description: "Tired",
		CreatedAt:   time.Date(2026, 4, 14, 9, 1, 0, 0, time.UTC),
	}
	newer := domain.Observation{
		ID:          "obs-new",
		PetID:       pet.ID,
		ObservedAt:  time.Date(2026, 4, 15, 9, 0, 0, 0, time.UTC),
		Description: "Vomited",
		CreatedAt:   time.Date(2026, 4, 15, 9, 1, 0, 0, time.UTC),
	}
	if _, err := repo.Create(ctx, older); err != nil {
		t.Fatalf("create older observation: %v", err)
	}
	if _, err := repo.Create(ctx, newer); err != nil {
		t.Fatalf("create newer observation: %v", err)
	}

	listed, err := repo.ListByPet(ctx, pet.ID)
	if err != nil {
		t.Fatalf("list observations: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("listed count = %d, want 2", len(listed))
	}
	if listed[0].ID != newer.ID || listed[1].ID != older.ID {
		t.Fatalf("unexpected order: %#v", listed)
	}

	fetched, err := repo.GetByID(ctx, pet.ID, older.ID)
	if err != nil {
		t.Fatalf("get observation: %v", err)
	}
	if fetched.Description != older.Description {
		t.Fatalf("description = %q, want %q", fetched.Description, older.Description)
	}
	if got, want := fetched.ObservedAt.Format(time.RFC3339), "2026-04-14T09:00:00-03:00"; got != want {
		t.Fatalf("observed_at = %s, want %s", got, want)
	}

	_, err = repo.GetByID(ctx, pet.ID, "missing")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing error = %v, want ErrNotFound", err)
	}

	if err := petRepo.Delete(ctx, pet.ID); err != nil {
		t.Fatalf("delete pet: %v", err)
	}
	listed, err = repo.ListByPet(ctx, pet.ID)
	if err != nil {
		t.Fatalf("list after cascade: %v", err)
	}
	if len(listed) != 0 {
		t.Fatalf("observations after pet delete = %d, want 0", len(listed))
	}
}
