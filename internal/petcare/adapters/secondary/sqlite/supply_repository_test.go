package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
)

func TestSupplyRepository_CRUDPetScopeAndListOrdering(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	repo := NewSupplyRepository(db)
	insertTestPet(t, db, "pet-1")
	insertTestPet(t, db, "pet-2")

	createdAt := time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)
	supplies := createTestSupplies(createdAt)
	for _, supply := range supplies {
		if _, err := repo.Create(ctx, supply); err != nil {
			t.Fatalf("create supply %q: %v", supply.ID, err)
		}
	}

	testSupplyGet(t, repo, ctx)
	testSupplyWrongPet(t, repo, ctx)
	testSupplyList(t, repo, ctx)
	testSupplyUpdate(t, repo, ctx, createdAt)
	testSupplyDelete(t, repo, ctx)
}

func createTestSupplies(createdAt time.Time) []domain.Supply {
	notes := "Comprar no Petlove"
	return []domain.Supply{
		{
			ID:                  "supply-1",
			PetID:               "pet-1",
			Name:                "Z Food",
			LastPurchasedAt:     time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC),
			EstimatedDaysSupply: 30,
			Notes:               &notes,
			CreatedAt:           createdAt,
			UpdatedAt:           createdAt,
		},
		{
			ID:                  "supply-2",
			PetID:               "pet-1",
			Name:                "A Snack",
			LastPurchasedAt:     time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC),
			EstimatedDaysSupply: 10,
			CreatedAt:           createdAt,
			UpdatedAt:           createdAt,
		},
		{
			ID:                  "supply-3",
			PetID:               "pet-1",
			Name:                "A Food",
			LastPurchasedAt:     time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC),
			EstimatedDaysSupply: 30,
			CreatedAt:           createdAt,
			UpdatedAt:           createdAt,
		},
	}
}

func testSupplyGet(t *testing.T, repo *SupplyRepository, ctx context.Context) {
	t.Helper()
	notes := "Comprar no Petlove"
	got, err := repo.GetByID(ctx, "pet-1", "supply-1")
	if err != nil {
		t.Fatalf("get supply: %v", err)
	}
	if got.Notes == nil || *got.Notes != notes {
		t.Fatalf("notes = %#v, want %q", got.Notes, notes)
	}
	if got.LastPurchasedAt.Format("2006-01-02") != "2026-04-16" {
		t.Fatalf("last_purchased_at = %v", got.LastPurchasedAt)
	}
	if got.NextReorderAt().Format("2006-01-02") != "2026-05-16" {
		t.Fatalf("next reorder = %v", got.NextReorderAt())
	}
}

func testSupplyWrongPet(t *testing.T, repo *SupplyRepository, ctx context.Context) {
	t.Helper()
	_, err := repo.GetByID(ctx, "pet-2", "supply-1")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("wrong pet get error = %v, want ErrNotFound", err)
	}
}

func testSupplyList(t *testing.T, repo *SupplyRepository, ctx context.Context) {
	t.Helper()
	listed, err := repo.List(ctx, "pet-1")
	if err != nil {
		t.Fatalf("list supplies: %v", err)
	}
	if len(listed) != 3 {
		t.Fatalf("listed supply count = %d, want 3", len(listed))
	}
	gotOrder := []string{listed[0].ID, listed[1].ID, listed[2].ID}
	wantOrder := []string{"supply-2", "supply-3", "supply-1"}
	for i := range wantOrder {
		if gotOrder[i] != wantOrder[i] {
			t.Fatalf("list order = %#v, want %#v", gotOrder, wantOrder)
		}
	}
}

func testSupplyUpdate(t *testing.T, repo *SupplyRepository, ctx context.Context, createdAt time.Time) {
	t.Helper()
	got, err := repo.GetByID(ctx, "pet-1", "supply-1")
	if err != nil {
		t.Fatalf("get supply for update: %v", err)
	}
	updatedNotes := "Novo pacote aberto"
	got.Name = "Updated Food"
	got.Notes = &updatedNotes
	got.UpdatedAt = createdAt.Add(time.Hour)
	updated, err := repo.Update(ctx, *got)
	if err != nil {
		t.Fatalf("update supply: %v", err)
	}
	if updated.Name != "Updated Food" || updated.Notes == nil || *updated.Notes != updatedNotes {
		t.Fatalf("updated supply = %#v", updated)
	}
}

func testSupplyDelete(t *testing.T, repo *SupplyRepository, ctx context.Context) {
	t.Helper()
	err := repo.Delete(ctx, "pet-2", "supply-1")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("wrong pet delete error = %v, want ErrNotFound", err)
	}
	if err := repo.Delete(ctx, "pet-1", "supply-1"); err != nil {
		t.Fatalf("delete supply: %v", err)
	}
	_, err = repo.GetByID(ctx, "pet-1", "supply-1")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("deleted get error = %v, want ErrNotFound", err)
	}
}
