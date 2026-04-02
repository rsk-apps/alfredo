package app_test

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/rafaelsoares/alfredo/internal/app"
	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
	"github.com/rafaelsoares/alfredo/internal/petcare/service"
)

// spyEmitter captures Emit calls for assertions.
type spyEmitter struct {
	events []string
}

func (s *spyEmitter) Emit(_ context.Context, event string, _ any) {
	s.events = append(s.events, event)
}

// fakePetGetter always returns a pet named "Luna".
type fakePetGetter struct{}

func (f *fakePetGetter) GetByID(_ context.Context, id string) (*domain.Pet, error) {
	return &domain.Pet{ID: id, Name: "Luna"}, nil
}

// stubVaccineService returns preset values.
type stubVaccineService struct {
	vaccine *domain.Vaccine
}

func (s *stubVaccineService) RecordVaccine(_ context.Context, _ service.RecordVaccineInput) (*domain.Vaccine, error) {
	return s.vaccine, nil
}
func (s *stubVaccineService) DeleteVaccine(_ context.Context, _, _ string) error { return nil }
func (s *stubVaccineService) ListVaccines(_ context.Context, _ string) ([]domain.Vaccine, error) {
	return nil, nil
}

func TestVaccineUseCase_RecordVaccine_emitsWithRecurrenceDays(t *testing.T) {
	spy := &spyEmitter{}
	due := time.Now().Add(365 * 24 * time.Hour)
	recDays := 365
	svc := &stubVaccineService{
		vaccine: &domain.Vaccine{ID: "v1", PetID: "p1", Name: "Rabies", NextDueAt: &due},
	}
	uc := app.NewVaccineUseCase(svc, &fakePetGetter{}, spy, zap.NewNop())

	if _, err := uc.RecordVaccine(context.Background(), service.RecordVaccineInput{
		PetID:          "p1",
		RecurrenceDays: &recDays,
		AdministeredAt: time.Now(),
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(spy.events) != 1 || spy.events[0] != "vaccine.taken" {
		t.Errorf("events = %v, want [vaccine.taken]", spy.events)
	}
}

func TestVaccineUseCase_RecordVaccine_emitsTakenWithoutExpire(t *testing.T) {
	spy := &spyEmitter{}
	svc := &stubVaccineService{
		vaccine: &domain.Vaccine{ID: "v1", PetID: "p1", Name: "Rabies", NextDueAt: nil},
	}
	uc := app.NewVaccineUseCase(svc, &fakePetGetter{}, spy, zap.NewNop())

	if _, err := uc.RecordVaccine(context.Background(), service.RecordVaccineInput{PetID: "p1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(spy.events) != 1 || spy.events[0] != "vaccine.taken" {
		t.Errorf("events = %v, want [vaccine.taken]", spy.events)
	}
}
