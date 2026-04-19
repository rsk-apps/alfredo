package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
)

func TestPetRepository_CRUDAndListOrdering(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := NewPetRepository(db)
	birthDate := time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)
	breed := "SRD"
	weight := 12.5
	food := 180.0
	photo := "/pets/luna.jpg"

	created, err := repo.Create(ctx, domain.Pet{
		ID:               "pet-1",
		Name:             "Luna",
		Species:          "dog",
		Breed:            &breed,
		BirthDate:        &birthDate,
		WeightKg:         &weight,
		DailyFoodGrams:   &food,
		PhotoPath:        &photo,
		GoogleCalendarID: "cal-1",
		CreatedAt:        time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("create pet: %v", err)
	}
	if created.ID != "pet-1" {
		t.Fatalf("created id = %q, want pet-1", created.ID)
	}

	got, err := repo.GetByID(ctx, "pet-1")
	if err != nil {
		t.Fatalf("get pet: %v", err)
	}
	if got.BirthDate == nil || got.BirthDate.Format("2006-01-02") != "2020-01-02" {
		t.Fatalf("birth_date = %#v, want 2020-01-02", got.BirthDate)
	}
	if got.Breed == nil || *got.Breed != breed || got.GoogleCalendarID != "cal-1" {
		t.Fatalf("pet details = %#v", got)
	}

	listed, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("list pets: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != "pet-1" {
		t.Fatalf("listed pets = %#v, want pet-1", listed)
	}

	got.Name = "Lua"
	got.GoogleCalendarID = "cal-2"
	updated, err := repo.Update(ctx, *got)
	if err != nil {
		t.Fatalf("update pet: %v", err)
	}
	if updated.Name != "Lua" || updated.GoogleCalendarID != "cal-2" {
		t.Fatalf("updated pet = %#v", updated)
	}

	if err := repo.Delete(ctx, "pet-1"); err != nil {
		t.Fatalf("delete pet: %v", err)
	}
	_, err = repo.GetByID(ctx, "pet-1")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("deleted get error = %v, want ErrNotFound", err)
	}
}

func TestPetRepository_NotFoundPaths(t *testing.T) {
	repo := NewPetRepository(openTestDB(t))

	if _, err := repo.GetByID(context.Background(), "missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("GetByID error = %v, want ErrNotFound", err)
	}
	if _, err := repo.Update(context.Background(), domain.Pet{ID: "missing"}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Update error = %v, want ErrNotFound", err)
	}
	if err := repo.Delete(context.Background(), "missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Delete error = %v, want ErrNotFound", err)
	}
}
