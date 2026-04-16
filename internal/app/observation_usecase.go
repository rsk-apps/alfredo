package app

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
	"github.com/rafaelsoares/alfredo/internal/petcare/service"
	"github.com/rafaelsoares/alfredo/internal/telegram"
)

// ObservationUseCase wraps ObservationService and orchestrates best-effort notifications.
type ObservationUseCase struct {
	observations ObservationServicer
	pets         PetNameGetter
	telegram     TelegramPort
	timezone     string
	logger       *zap.Logger
}

func NewObservationUseCase(observations ObservationServicer, pets PetNameGetter, telegramPort TelegramPort, timezone string, logger *zap.Logger) *ObservationUseCase {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &ObservationUseCase{observations: observations, pets: pets, telegram: telegramPort, timezone: timezone, logger: logger}
}

func (uc *ObservationUseCase) Create(ctx context.Context, in service.CreateObservationInput) (*domain.Observation, error) {
	pet, err := uc.pets.GetByID(ctx, in.PetID)
	if err != nil {
		return nil, fmt.Errorf("load pet %q: %w", in.PetID, err)
	}
	observation, err := uc.observations.Create(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("create observation for pet %q: %w", in.PetID, err)
	}
	uc.sendTelegram(ctx, formatObservationCreatedMessage(pet, observation, uc.timezone), zap.String("pet_id", pet.ID), zap.String("observation_id", observation.ID))
	return observation, nil
}

func (uc *ObservationUseCase) ListByPet(ctx context.Context, petID string) ([]domain.Observation, error) {
	return uc.observations.ListByPet(ctx, petID)
}

func (uc *ObservationUseCase) GetByID(ctx context.Context, petID, observationID string) (*domain.Observation, error) {
	return uc.observations.GetByID(ctx, petID, observationID)
}

func (uc *ObservationUseCase) sendTelegram(ctx context.Context, msg telegram.Message, fields ...zap.Field) {
	if uc.telegram == nil {
		return
	}
	if err := uc.telegram.Send(ctx, msg); err != nil {
		allFields := append([]zap.Field{zap.Error(err)}, fields...)
		uc.logger.Warn("telegram notification failed", allFields...)
	}
}

func formatObservationCreatedMessage(pet *domain.Pet, observation *domain.Observation, timezone string) telegram.Message {
	var b strings.Builder
	b.WriteString("<b>Observação registrada</b>\n\n")
	writeHTMLLine(&b, "Pet", pet.Name)
	writeHTMLLine(&b, "Observado em", formatUserTime(observation.ObservedAt, timezone))
	writeHTMLLine(&b, "Descrição", observation.Description)
	return telegram.Message{Text: b.String(), ParseMode: telegram.ParseModeHTML}
}
