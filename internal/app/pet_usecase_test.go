package app_test

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"

	"github.com/rafaelsoares/alfredo/internal/app"
	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
	"github.com/rafaelsoares/alfredo/internal/petcare/service"
)

type petUseCaseRepo struct {
	pet       *domain.Pet
	created   []domain.Pet
	deleted   []string
	createErr error
	deleteErr error
}

func (r *petUseCaseRepo) List(context.Context) ([]domain.Pet, error) {
	if r.pet == nil {
		return nil, nil
	}
	return []domain.Pet{*r.pet}, nil
}

func (r *petUseCaseRepo) Create(_ context.Context, pet domain.Pet) (*domain.Pet, error) {
	if r.createErr != nil {
		return nil, r.createErr
	}
	copy := pet
	r.created = append(r.created, copy)
	r.pet = &copy
	return &copy, nil
}

func (r *petUseCaseRepo) GetByID(context.Context, string) (*domain.Pet, error) {
	if r.pet == nil {
		return nil, domain.ErrNotFound
	}
	copy := *r.pet
	return &copy, nil
}

func (r *petUseCaseRepo) Update(_ context.Context, pet domain.Pet) (*domain.Pet, error) {
	copy := pet
	r.pet = &copy
	return &copy, nil
}

func (r *petUseCaseRepo) Delete(_ context.Context, id string) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deleted = append(r.deleted, id)
	return nil
}

func TestPetUseCaseCreateStoresCalendarIDWithPet(t *testing.T) {
	repo := &petUseCaseRepo{}
	calendar := &calendarFake{createCalendarID: "cal-new"}
	uc := app.NewPetUseCase(
		service.NewPetService(repo),
		serviceTxRunner{petRepo: repo},
		calendar,
		zap.NewNop(),
	)

	created, err := uc.Create(context.Background(), service.CreatePetInput{Name: "Milo", Species: "cat"})
	if err != nil {
		t.Fatalf("Create = %v", err)
	}
	if created.GoogleCalendarID != "cal-new" {
		t.Fatalf("created pet calendar id = %q, want cal-new", created.GoogleCalendarID)
	}
	if len(repo.created) != 1 {
		t.Fatalf("repo creates = %d, want 1", len(repo.created))
	}
	if repo.created[0].GoogleCalendarID != "cal-new" {
		t.Fatalf("repo create calendar id = %q, want cal-new", repo.created[0].GoogleCalendarID)
	}
}

func TestPetUseCaseDelegatesReadAndUpdateOperations(t *testing.T) {
	repo := &petUseCaseRepo{pet: &domain.Pet{ID: "pet-1", Name: "Luna", Species: "dog"}}
	uc := app.NewPetUseCase(
		service.NewPetService(repo),
		serviceTxRunner{petRepo: repo},
		&calendarFake{},
		zap.NewNop(),
	)

	pets, err := uc.List(context.Background())
	if err != nil {
		t.Fatalf("List = %v", err)
	}
	if len(pets) != 1 || pets[0].ID != "pet-1" {
		t.Fatalf("pets = %#v, want pet-1", pets)
	}
	got, err := uc.GetByID(context.Background(), "pet-1")
	if err != nil {
		t.Fatalf("GetByID = %v", err)
	}
	if got.ID != "pet-1" {
		t.Fatalf("got pet id = %q, want pet-1", got.ID)
	}
	updated, err := uc.Update(context.Background(), "pet-1", service.UpdatePetInput{Name: "Lua", Species: "dog"})
	if err != nil {
		t.Fatalf("Update = %v", err)
	}
	if updated.Name != "Lua" {
		t.Fatalf("updated name = %q, want Lua", updated.Name)
	}
}

func TestPetUseCaseCreateDoesNotPersistWhenCalendarCreateFails(t *testing.T) {
	repo := &petUseCaseRepo{}
	calendar := &calendarFake{createCalendarErr: errors.New("calendar down")}
	uc := app.NewPetUseCase(
		service.NewPetService(repo),
		serviceTxRunner{petRepo: repo},
		calendar,
		zap.NewNop(),
	)

	_, err := uc.Create(context.Background(), service.CreatePetInput{Name: "Milo", Species: "cat"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if len(repo.created) != 0 {
		t.Fatalf("repo creates = %#v, want none when calendar create fails", repo.created)
	}
}

func TestPetUseCaseCreateCompensatesCalendarWhenPersistenceFails(t *testing.T) {
	repo := &petUseCaseRepo{createErr: errors.New("insert pet failed")}
	calendar := &calendarFake{createCalendarID: "cal-1"}
	uc := app.NewPetUseCase(
		service.NewPetService(repo),
		serviceTxRunner{petRepo: repo},
		calendar,
		zap.NewNop(),
	)

	_, err := uc.Create(context.Background(), service.CreatePetInput{Name: "Luna", Species: "dog"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if len(calendar.deletedCalendars) != 1 || calendar.deletedCalendars[0] != "cal-1" {
		t.Fatalf("calendar compensation = %#v, want [cal-1]", calendar.deletedCalendars)
	}
}

func TestPetUseCaseDeleteRemovesCalendarBeforePet(t *testing.T) {
	repo := &petUseCaseRepo{pet: &domain.Pet{ID: "pet-1", Name: "Luna", GoogleCalendarID: "cal-1"}}
	calendar := &calendarFake{}
	uc := app.NewPetUseCase(
		service.NewPetService(repo),
		serviceTxRunner{petRepo: repo},
		calendar,
		zap.NewNop(),
	)

	err := uc.Delete(context.Background(), "pet-1")
	if err != nil {
		t.Fatalf("Delete = %v", err)
	}
	if len(calendar.deletedCalendars) != 1 || calendar.deletedCalendars[0] != "cal-1" {
		t.Fatalf("calendar deletions = %#v, want [cal-1]", calendar.deletedCalendars)
	}
	if len(repo.deleted) != 1 || repo.deleted[0] != "pet-1" {
		t.Fatalf("pet deletions = %#v, want [pet-1]", repo.deleted)
	}
}

func TestPetUseCaseDeleteDoesNotRemovePetWhenCalendarDeleteFails(t *testing.T) {
	repo := &petUseCaseRepo{pet: &domain.Pet{ID: "pet-1", Name: "Luna", GoogleCalendarID: "cal-1"}}
	calendar := &calendarFake{deleteCalendarErr: errors.New("calendar down")}
	uc := app.NewPetUseCase(
		service.NewPetService(repo),
		serviceTxRunner{petRepo: repo},
		calendar,
		zap.NewNop(),
	)

	err := uc.Delete(context.Background(), "pet-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if len(repo.deleted) != 0 {
		t.Fatalf("pet deletions = %#v, want none while calendar delete failed", repo.deleted)
	}
}
