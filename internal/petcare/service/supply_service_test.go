package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
	"github.com/rafaelsoares/alfredo/internal/petcare/service"
)

var errSupplyRepoDown = errors.New("supply repo down")

type mockSupplyRepo struct {
	supplies []domain.Supply
	err      error
	updated  *domain.Supply
	deleted  bool
}

func (m *mockSupplyRepo) Create(_ context.Context, supply domain.Supply) (*domain.Supply, error) {
	return &supply, m.err
}

func (m *mockSupplyRepo) GetByID(_ context.Context, _, _ string) (*domain.Supply, error) {
	if len(m.supplies) == 0 {
		return nil, domain.ErrNotFound
	}
	return &m.supplies[0], m.err
}

func (m *mockSupplyRepo) List(_ context.Context, _ string) ([]domain.Supply, error) {
	return m.supplies, m.err
}

func (m *mockSupplyRepo) Update(_ context.Context, supply domain.Supply) (*domain.Supply, error) {
	m.updated = &supply
	return &supply, m.err
}

func (m *mockSupplyRepo) Delete(_ context.Context, _, _ string) error {
	if len(m.supplies) == 0 {
		return domain.ErrNotFound
	}
	m.deleted = true
	return m.err
}

func TestSupplyService_Create_AssignsIDTimestampsAndComputesReorderDate(t *testing.T) {
	svc := service.NewSupplyService(&mockSupplyRepo{})
	lastPurchasedAt := time.Date(2026, 4, 16, 15, 30, 0, 0, time.FixedZone("BRT", -3*60*60))

	supply, err := svc.Create(context.Background(), service.CreateSupplyInput{
		PetID:               "pet-1",
		Name:                "  Royal Canin  ",
		LastPurchasedAt:     lastPurchasedAt,
		EstimatedDaysSupply: 30,
	})
	if err != nil {
		t.Fatalf("create supply: %v", err)
	}
	if supply.ID == "" {
		t.Fatal("expected ID to be set")
	}
	if supply.Name != "Royal Canin" {
		t.Fatalf("name = %q, want trimmed Royal Canin", supply.Name)
	}
	if got := supply.LastPurchasedAt.Format("2006-01-02T15:04:05Z07:00"); got != "2026-04-16T00:00:00Z" {
		t.Fatalf("last_purchased_at = %q, want UTC date-only midnight", got)
	}
	if got := supply.NextReorderAt().Format("2006-01-02"); got != "2026-05-16" {
		t.Fatalf("next reorder = %q, want 2026-05-16", got)
	}
	if supply.CreatedAt.IsZero() || supply.UpdatedAt.IsZero() {
		t.Fatalf("expected timestamps to be set: %#v", supply)
	}
}

func TestSupplyService_Create_Validation(t *testing.T) {
	validDate := time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		in   service.CreateSupplyInput
	}{
		{
			name: "missing pet id",
			in: service.CreateSupplyInput{
				Name:                "Food",
				LastPurchasedAt:     validDate,
				EstimatedDaysSupply: 30,
			},
		},
		{
			name: "blank name",
			in: service.CreateSupplyInput{
				PetID:               "pet-1",
				Name:                "   ",
				LastPurchasedAt:     validDate,
				EstimatedDaysSupply: 30,
			},
		},
		{
			name: "missing purchase date",
			in: service.CreateSupplyInput{
				PetID:               "pet-1",
				Name:                "Food",
				EstimatedDaysSupply: 30,
			},
		},
		{
			name: "non-positive supply days",
			in: service.CreateSupplyInput{
				PetID:               "pet-1",
				Name:                "Food",
				LastPurchasedAt:     validDate,
				EstimatedDaysSupply: 0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := service.NewSupplyService(&mockSupplyRepo{})
			_, err := svc.Create(context.Background(), tt.in)
			if !errors.Is(err, domain.ErrValidation) {
				t.Fatalf("error = %v, want ErrValidation", err)
			}
		})
	}
}

func TestSupplyService_GetByID_NotFound(t *testing.T) {
	svc := service.NewSupplyService(&mockSupplyRepo{})
	_, err := svc.GetByID(context.Background(), "pet-1", "supply-1")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestSupplyService_GetByIDAndList_ReturnRepositoryValues(t *testing.T) {
	expected := domain.Supply{
		ID:                  "supply-1",
		PetID:               "pet-1",
		Name:                "Food",
		LastPurchasedAt:     time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC),
		EstimatedDaysSupply: 30,
	}
	svc := service.NewSupplyService(&mockSupplyRepo{supplies: []domain.Supply{expected}})

	got, err := svc.GetByID(context.Background(), "pet-1", "supply-1")
	if err != nil {
		t.Fatalf("get supply: %v", err)
	}
	if got.ID != expected.ID {
		t.Fatalf("supply id = %q, want %q", got.ID, expected.ID)
	}

	listed, err := svc.List(context.Background(), "pet-1")
	if err != nil {
		t.Fatalf("list supplies: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != expected.ID {
		t.Fatalf("listed supplies = %#v, want %q", listed, expected.ID)
	}
}

func TestSupplyService_List_WrapsRepositoryErrors(t *testing.T) {
	svc := service.NewSupplyService(&mockSupplyRepo{err: errSupplyRepoDown})

	_, err := svc.List(context.Background(), "pet-1")
	if !errors.Is(err, errSupplyRepoDown) {
		t.Fatalf("List error = %v, want wrapped repository error", err)
	}
}

func TestSupplyService_Update_AppliesFieldsAndValidation(t *testing.T) {
	existing := domain.Supply{
		ID:                  "supply-1",
		PetID:               "pet-1",
		Name:                "Food",
		LastPurchasedAt:     time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC),
		EstimatedDaysSupply: 30,
		CreatedAt:           time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC),
		UpdatedAt:           time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC),
	}
	repo := &mockSupplyRepo{supplies: []domain.Supply{existing}}
	svc := service.NewSupplyService(repo)
	name := "Updated Food"
	lastPurchasedAt := time.Date(2026, 5, 16, 9, 0, 0, 0, time.Local)
	days := 45
	notes := "Novo pacote aberto"

	updated, err := svc.Update(context.Background(), "pet-1", "supply-1", service.UpdateSupplyInput{
		Name:                &name,
		LastPurchasedAt:     &lastPurchasedAt,
		EstimatedDaysSupply: &days,
		Notes:               &notes,
	})
	if err != nil {
		t.Fatalf("update supply: %v", err)
	}
	if repo.updated == nil {
		t.Fatal("expected repository update")
	}
	if updated.Name != name || updated.EstimatedDaysSupply != days {
		t.Fatalf("updated supply = %#v", updated)
	}
	if got := updated.LastPurchasedAt.Format("2006-01-02"); got != "2026-05-16" {
		t.Fatalf("last_purchased_at = %q, want 2026-05-16", got)
	}
	if updated.Notes == nil || *updated.Notes != notes {
		t.Fatalf("notes = %#v, want %q", updated.Notes, notes)
	}
	if !updated.UpdatedAt.After(existing.UpdatedAt) {
		t.Fatalf("updated_at = %v, want after %v", updated.UpdatedAt, existing.UpdatedAt)
	}

	blank := " "
	_, err = svc.Update(context.Background(), "pet-1", "supply-1", service.UpdateSupplyInput{Name: &blank})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("blank name error = %v, want ErrValidation", err)
	}

	zero := 0
	_, err = svc.Update(context.Background(), "pet-1", "supply-1", service.UpdateSupplyInput{EstimatedDaysSupply: &zero})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("zero days error = %v, want ErrValidation", err)
	}
}

func TestSupplyService_Delete_NotFound(t *testing.T) {
	svc := service.NewSupplyService(&mockSupplyRepo{})
	err := svc.Delete(context.Background(), "pet-1", "supply-1")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestSupplyService_Delete_RemovesExistingSupply(t *testing.T) {
	repo := &mockSupplyRepo{supplies: []domain.Supply{{ID: "supply-1", PetID: "pet-1"}}}
	svc := service.NewSupplyService(repo)
	if err := svc.Delete(context.Background(), "pet-1", "supply-1"); err != nil {
		t.Fatalf("delete supply: %v", err)
	}
	if !repo.deleted {
		t.Fatal("expected repository delete to be called")
	}
}
