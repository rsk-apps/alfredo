package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
)

func TestVaccineRepository_CRUDPetScopeAndOrdering(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := NewVaccineRepository(db)
	insertTestPet(t, db, "pet-1")
	insertTestPet(t, db, "pet-2")
	nextDue := time.Date(2026, 5, 17, 0, 0, 0, 0, time.UTC)
	vet := "Dra Ana"
	batch := "L123"
	notes := "sem reacao"

	created, err := repo.CreateVaccine(ctx, domain.Vaccine{
		ID:                           "vaccine-1",
		PetID:                        "pet-1",
		Name:                         "V10",
		AdministeredAt:               time.Date(2026, 4, 17, 9, 0, 0, 0, time.UTC),
		NextDueAt:                    &nextDue,
		VetName:                      &vet,
		BatchNumber:                  &batch,
		Notes:                        &notes,
		GoogleCalendarEventID:        "evt-admin",
		GoogleCalendarNextDueEventID: "evt-next",
	})
	if err != nil {
		t.Fatalf("create vaccine: %v", err)
	}
	if created.ID != "vaccine-1" {
		t.Fatalf("created id = %q, want vaccine-1", created.ID)
	}

	got, err := repo.GetVaccine(ctx, "pet-1", "vaccine-1")
	if err != nil {
		t.Fatalf("get vaccine: %v", err)
	}
	if got.NextDueAt == nil || got.NextDueAt.Format("2006-01-02") != "2026-05-17" {
		t.Fatalf("next_due_at = %#v", got.NextDueAt)
	}
	if got.VetName == nil || *got.VetName != vet || got.GoogleCalendarNextDueEventID != "evt-next" {
		t.Fatalf("vaccine details = %#v", got)
	}

	_, err = repo.GetVaccine(ctx, "pet-2", "vaccine-1")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("wrong pet get error = %v, want ErrNotFound", err)
	}
	listed, err := repo.ListVaccines(ctx, "pet-1")
	if err != nil {
		t.Fatalf("list vaccines: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != "vaccine-1" {
		t.Fatalf("listed vaccines = %#v, want vaccine-1", listed)
	}
	if err := repo.DeleteVaccine(ctx, "pet-2", "vaccine-1"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("wrong pet delete error = %v, want ErrNotFound", err)
	}
	if err := repo.DeleteVaccine(ctx, "pet-1", "vaccine-1"); err != nil {
		t.Fatalf("delete vaccine: %v", err)
	}
}
