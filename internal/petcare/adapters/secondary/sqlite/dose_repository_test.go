package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
)

func TestDoseRepository_CreateListFutureAndDeleteFuture(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	insertTestPet(t, db, "pet-1")
	insertTestTreatment(t, db, "treatment-1", "pet-1")
	repo := NewDoseRepository(db)
	start := time.Date(2026, 4, 17, 8, 0, 0, 0, time.UTC)

	if err := repo.CreateBatch(ctx, nil); err != nil {
		t.Fatalf("empty CreateBatch: %v", err)
	}
	if err := repo.CreateBatch(ctx, []domain.Dose{
		{ID: "dose-1", TreatmentID: "treatment-1", PetID: "pet-1", ScheduledFor: start, GoogleCalendarEventID: "evt-1"},
		{ID: "dose-2", TreatmentID: "treatment-1", PetID: "pet-1", ScheduledFor: start.Add(12 * time.Hour), GoogleCalendarEventID: "evt-2"},
	}); err != nil {
		t.Fatalf("CreateBatch: %v", err)
	}

	all, err := repo.ListByTreatment(ctx, "treatment-1")
	if err != nil {
		t.Fatalf("ListByTreatment: %v", err)
	}
	if len(all) != 2 || all[0].ID != "dose-1" || all[1].ID != "dose-2" {
		t.Fatalf("all doses = %#v", all)
	}
	future, err := repo.ListFutureByTreatment(ctx, "treatment-1", start.Add(time.Hour))
	if err != nil {
		t.Fatalf("ListFutureByTreatment: %v", err)
	}
	if len(future) != 1 || future[0].ID != "dose-2" {
		t.Fatalf("future doses = %#v, want dose-2", future)
	}
	if err := repo.DeleteFutureByTreatment(ctx, "treatment-1", start.Add(time.Hour)); err != nil {
		t.Fatalf("DeleteFutureByTreatment: %v", err)
	}
	remaining, err := repo.ListByTreatment(ctx, "treatment-1")
	if err != nil {
		t.Fatalf("ListByTreatment after delete: %v", err)
	}
	if len(remaining) != 1 || remaining[0].ID != "dose-1" {
		t.Fatalf("remaining doses = %#v, want dose-1", remaining)
	}
}
