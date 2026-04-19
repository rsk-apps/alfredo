package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
	"github.com/rafaelsoares/alfredo/internal/petcare/service"
)

var errVaccineRepoDown = errors.New("vaccine repo down")

// --- mock ---

type mockVaccineRepo struct {
	vaccines []domain.Vaccine
	err      error
	created  *domain.Vaccine
	deleted  bool
}

func (m *mockVaccineRepo) ListVaccines(_ context.Context, _ string) ([]domain.Vaccine, error) {
	return m.vaccines, m.err
}
func (m *mockVaccineRepo) CreateVaccine(_ context.Context, v domain.Vaccine) (*domain.Vaccine, error) {
	m.created = &v
	return &v, m.err
}
func (m *mockVaccineRepo) GetVaccine(_ context.Context, _, _ string) (*domain.Vaccine, error) {
	if len(m.vaccines) == 0 {
		return nil, domain.ErrNotFound
	}
	return &m.vaccines[0], m.err
}
func (m *mockVaccineRepo) DeleteVaccine(_ context.Context, _, _ string) error {
	m.deleted = true
	return m.err
}

// --- tests ---

func TestVaccineService_RecordVaccine_AssignsID(t *testing.T) {
	svc := service.NewVaccineService(&mockVaccineRepo{}, &mockPetRepo{})
	v, err := svc.RecordVaccine(context.Background(), service.RecordVaccineInput{
		PetID: "p1", Name: "Rabies", AdministeredAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.ID == "" {
		t.Error("expected ID to be set")
	}
}

func TestVaccineService_RecordVaccine_ValidationError(t *testing.T) {
	svc := service.NewVaccineService(&mockVaccineRepo{}, &mockPetRepo{})
	_, err := svc.RecordVaccine(context.Background(), service.RecordVaccineInput{PetID: "p1", Name: ""})
	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("got %v, want ErrValidation", err)
	}
}

func TestVaccineService_DeleteVaccine_NotFound(t *testing.T) {
	svc := service.NewVaccineService(&mockVaccineRepo{}, &mockPetRepo{})
	err := svc.DeleteVaccine(context.Background(), "p1", "v1")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestVaccineService_ListAndGet_ReturnRepositoryValues(t *testing.T) {
	want := domain.Vaccine{ID: "v1", PetID: "p1", Name: "Rabies"}
	svc := service.NewVaccineService(&mockVaccineRepo{vaccines: []domain.Vaccine{want}}, &mockPetRepo{})

	listed, err := svc.ListVaccines(context.Background(), "p1")
	if err != nil {
		t.Fatalf("ListVaccines error = %v", err)
	}
	if len(listed) != 1 || listed[0].ID != want.ID {
		t.Fatalf("ListVaccines = %#v, want %#v", listed, want)
	}

	got, err := svc.GetVaccine(context.Background(), "p1", "v1")
	if err != nil {
		t.Fatalf("GetVaccine error = %v", err)
	}
	if got == nil || got.ID != want.ID {
		t.Fatalf("GetVaccine = %#v, want %#v", got, want)
	}
}

func TestVaccineService_RecordVaccine_PropagatesRepositoryError(t *testing.T) {
	svc := service.NewVaccineService(&mockVaccineRepo{err: errVaccineRepoDown}, &mockPetRepo{})
	_, err := svc.RecordVaccine(context.Background(), service.RecordVaccineInput{
		PetID:          "p1",
		Name:           "Rabies",
		AdministeredAt: time.Now(),
	})
	if !errors.Is(err, errVaccineRepoDown) {
		t.Fatalf("RecordVaccine error = %v, want wrapped repository error", err)
	}
}

func TestVaccineService_DeleteVaccine_RemovesExistingRecord(t *testing.T) {
	repo := &mockVaccineRepo{vaccines: []domain.Vaccine{{ID: "v1", PetID: "p1", Name: "Rabies"}}}
	svc := service.NewVaccineService(repo, &mockPetRepo{})

	if err := svc.DeleteVaccine(context.Background(), "p1", "v1"); err != nil {
		t.Fatalf("DeleteVaccine error = %v", err)
	}
	if !repo.deleted {
		t.Fatal("expected DeleteVaccine to call repository")
	}
}
