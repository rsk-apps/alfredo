package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rafaelsoares/alfredo/internal/app"
	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
	"github.com/rafaelsoares/alfredo/internal/petcare/service"
)

type supplyServiceFake struct {
	created     []service.CreateSupplyInput
	createErr   error
	listCalls   int
	getCalls    int
	updateCalls int
	deleteCalls int
}

func (s *supplyServiceFake) Create(_ context.Context, in service.CreateSupplyInput) (*domain.Supply, error) {
	s.created = append(s.created, in)
	if s.createErr != nil {
		return nil, s.createErr
	}
	return &domain.Supply{
		ID:                  "supply-1",
		PetID:               in.PetID,
		Name:                in.Name,
		LastPurchasedAt:     in.LastPurchasedAt,
		EstimatedDaysSupply: in.EstimatedDaysSupply,
		Notes:               in.Notes,
	}, nil
}

func (s *supplyServiceFake) GetByID(context.Context, string, string) (*domain.Supply, error) {
	s.getCalls++
	return &domain.Supply{ID: "supply-1"}, nil
}

func (s *supplyServiceFake) List(context.Context, string) ([]domain.Supply, error) {
	s.listCalls++
	return []domain.Supply{{ID: "supply-1"}}, nil
}

func (s *supplyServiceFake) Update(context.Context, string, string, service.UpdateSupplyInput) (*domain.Supply, error) {
	s.updateCalls++
	return &domain.Supply{ID: "supply-1"}, nil
}

func (s *supplyServiceFake) Delete(context.Context, string, string) error {
	s.deleteCalls++
	return nil
}

func TestSupplyUseCaseCreateRequiresExistingPet(t *testing.T) {
	supplies := &supplyServiceFake{}
	uc := app.NewSupplyUseCase(supplies, petGetterErr{err: domain.ErrNotFound})

	_, err := uc.Create(context.Background(), service.CreateSupplyInput{
		PetID:               "missing",
		Name:                "Racao",
		LastPurchasedAt:     time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		EstimatedDaysSupply: 30,
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Create error = %v, want ErrNotFound", err)
	}
	if len(supplies.created) != 0 {
		t.Fatalf("created supplies = %#v, want none for missing pet", supplies.created)
	}
}

func TestSupplyUseCaseCreatePersistsAfterPetLookup(t *testing.T) {
	pet := &domain.Pet{ID: "pet-1", Name: "Luna"}
	supplies := &supplyServiceFake{}
	uc := app.NewSupplyUseCase(supplies, &fakePetGetterWithCalendar{pet: pet})
	purchasedAt := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	created, err := uc.Create(context.Background(), service.CreateSupplyInput{
		PetID:               pet.ID,
		Name:                "Racao",
		LastPurchasedAt:     purchasedAt,
		EstimatedDaysSupply: 30,
	})
	if err != nil {
		t.Fatalf("Create = %v", err)
	}
	if created.ID != "supply-1" {
		t.Fatalf("created supply id = %q, want supply-1", created.ID)
	}
	if len(supplies.created) != 1 {
		t.Fatalf("create calls = %d, want 1", len(supplies.created))
	}
	if supplies.created[0].PetID != pet.ID {
		t.Fatalf("created pet id = %q, want %q", supplies.created[0].PetID, pet.ID)
	}
}

func TestSupplyUseCaseDelegatesReadUpdateAndDelete(t *testing.T) {
	supplies := &supplyServiceFake{}
	uc := app.NewSupplyUseCase(supplies, &fakePetGetterWithCalendar{pet: &domain.Pet{ID: "pet-1"}})

	listed, err := uc.List(context.Background(), "pet-1")
	if err != nil {
		t.Fatalf("List = %v", err)
	}
	if len(listed) != 1 || supplies.listCalls != 1 {
		t.Fatalf("listed = %#v listCalls = %d, want one supply and one call", listed, supplies.listCalls)
	}
	got, err := uc.GetByID(context.Background(), "pet-1", "supply-1")
	if err != nil {
		t.Fatalf("GetByID = %v", err)
	}
	if got.ID != "supply-1" || supplies.getCalls != 1 {
		t.Fatalf("got = %#v getCalls = %d, want supply-1 and one call", got, supplies.getCalls)
	}
	updated, err := uc.Update(context.Background(), "pet-1", "supply-1", service.UpdateSupplyInput{})
	if err != nil {
		t.Fatalf("Update = %v", err)
	}
	if updated.ID != "supply-1" || supplies.updateCalls != 1 {
		t.Fatalf("updated = %#v updateCalls = %d, want supply-1 and one call", updated, supplies.updateCalls)
	}
	if err := uc.Delete(context.Background(), "pet-1", "supply-1"); err != nil {
		t.Fatalf("Delete = %v", err)
	}
	if supplies.deleteCalls != 1 {
		t.Fatalf("deleteCalls = %d, want 1", supplies.deleteCalls)
	}
}
