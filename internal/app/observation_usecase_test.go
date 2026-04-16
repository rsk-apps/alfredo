package app_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/rafaelsoares/alfredo/internal/app"
	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
	"github.com/rafaelsoares/alfredo/internal/petcare/service"
	"github.com/rafaelsoares/alfredo/internal/telegram"
)

type observationServiceFake struct {
	created       *domain.Observation
	createCalls   int
	listCalls     int
	getCalls      int
	createErr     error
	observedInput service.CreateObservationInput
}

func (s *observationServiceFake) Create(_ context.Context, in service.CreateObservationInput) (*domain.Observation, error) {
	s.createCalls++
	s.observedInput = in
	if s.createErr != nil {
		return nil, s.createErr
	}
	observation := &domain.Observation{
		ID:          "obs-1",
		PetID:       in.PetID,
		ObservedAt:  in.ObservedAt,
		Description: in.Description,
		CreatedAt:   time.Now().UTC(),
	}
	s.created = observation
	return observation, nil
}

func (s *observationServiceFake) ListByPet(context.Context, string) ([]domain.Observation, error) {
	s.listCalls++
	return nil, nil
}

func (s *observationServiceFake) GetByID(context.Context, string, string) (*domain.Observation, error) {
	s.getCalls++
	return nil, nil
}

type petGetterErr struct{ err error }

func (g petGetterErr) GetByID(context.Context, string) (*domain.Pet, error) {
	return nil, g.err
}

func TestObservationUseCaseCreateSendsTelegramAfterPersistence(t *testing.T) {
	pet := &domain.Pet{ID: "p1", Name: `Luna & Bob`}
	observations := &observationServiceFake{}
	telegramRecorder := &telegramFake{}
	uc := app.NewObservationUseCase(observations, &fakePetGetterWithCalendar{pet: pet}, telegramRecorder, "America/Sao_Paulo", zap.NewNop())
	observedAt := time.Date(2026, 4, 15, 9, 30, 0, 0, time.FixedZone("BRT", -3*60*60))

	observation, err := uc.Create(context.Background(), service.CreateObservationInput{
		PetID:       pet.ID,
		ObservedAt:  observedAt,
		Description: `Vomited <again>`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if observation.ID != "obs-1" {
		t.Fatalf("observation id = %q, want obs-1", observation.ID)
	}
	if observations.createCalls != 1 {
		t.Fatalf("create calls = %d, want 1", observations.createCalls)
	}
	if len(telegramRecorder.messages) != 1 {
		t.Fatalf("telegram messages = %d, want 1", len(telegramRecorder.messages))
	}
	msg := telegramRecorder.messages[0]
	if msg.ParseMode != telegram.ParseModeHTML {
		t.Fatalf("parse mode = %q, want %q", msg.ParseMode, telegram.ParseModeHTML)
	}
	for _, want := range []string{
		"<b>Observação registrada</b>",
		"<b>Pet:</b> Luna &amp; Bob",
		"<b>Observado em:</b> 15/04/2026 09:30",
		"Vomited &lt;again&gt;",
	} {
		if !strings.Contains(msg.Text, want) {
			t.Fatalf("telegram message = %q, want substring %q", msg.Text, want)
		}
	}
}

func TestObservationUseCaseTelegramFailureDoesNotRollback(t *testing.T) {
	pet := &domain.Pet{ID: "p1", Name: "Luna"}
	observations := &observationServiceFake{}
	telegramRecorder := &telegramFake{err: errors.New("telegram unavailable")}
	uc := app.NewObservationUseCase(observations, &fakePetGetterWithCalendar{pet: pet}, telegramRecorder, "America/Sao_Paulo", zap.NewNop())

	observation, err := uc.Create(context.Background(), service.CreateObservationInput{
		PetID:       pet.ID,
		ObservedAt:  time.Now(),
		Description: "Vomited",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if observation == nil || observations.created == nil {
		t.Fatal("expected persisted observation despite telegram failure")
	}
	if len(telegramRecorder.messages) != 1 {
		t.Fatalf("telegram send attempts = %d, want 1", len(telegramRecorder.messages))
	}
}

func TestObservationUseCaseMissingPetDoesNotCreateOrNotify(t *testing.T) {
	observations := &observationServiceFake{}
	telegramRecorder := &telegramFake{}
	uc := app.NewObservationUseCase(observations, petGetterErr{err: domain.ErrNotFound}, telegramRecorder, "America/Sao_Paulo", zap.NewNop())

	_, err := uc.Create(context.Background(), service.CreateObservationInput{
		PetID:       "missing",
		ObservedAt:  time.Now(),
		Description: "Vomited",
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
	if observations.createCalls != 0 {
		t.Fatalf("create calls = %d, want 0", observations.createCalls)
	}
	if len(telegramRecorder.messages) != 0 {
		t.Fatalf("telegram messages = %d, want 0", len(telegramRecorder.messages))
	}
}
