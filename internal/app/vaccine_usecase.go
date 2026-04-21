package app

import (
	"context"
	"errors"
	"fmt"
	"html"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/rafaelsoares/alfredo/internal/gcalendar"
	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
	"github.com/rafaelsoares/alfredo/internal/petcare/service"
	"github.com/rafaelsoares/alfredo/internal/telegram"
)

// VaccineUseCase wraps VaccineService and orchestrates calendar side effects.
type VaccineUseCase struct {
	vaccine  VaccineServicer
	pets     PetNameGetter
	txRunner PetCareTxRunner
	calendar CalendarPort
	telegram TelegramPort
	logger   *zap.Logger
	timezone string
}

func NewVaccineUseCase(vaccine VaccineServicer, pets PetNameGetter, txRunner PetCareTxRunner, calendar CalendarPort, telegramPort TelegramPort, timezone string, logger *zap.Logger) *VaccineUseCase {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &VaccineUseCase{vaccine: vaccine, pets: pets, txRunner: txRunner, calendar: calendar, telegram: telegramPort, timezone: timezone, logger: logger}
}

func (uc *VaccineUseCase) ListVaccines(ctx context.Context, petID string) ([]domain.Vaccine, error) {
	return uc.vaccine.ListVaccines(ctx, petID)
}

func (uc *VaccineUseCase) RecordVaccine(ctx context.Context, in service.RecordVaccineInput) (*domain.Vaccine, error) {
	pet, err := uc.pets.GetByID(ctx, in.PetID)
	if err != nil {
		return nil, fmt.Errorf("load pet %q: %w", in.PetID, err)
	}
	if pet.GoogleCalendarID == "" {
		return nil, fmt.Errorf("pet %q is missing google calendar id", in.PetID)
	}

	if in.RecurrenceDays != nil && *in.RecurrenceDays > 0 {
		nextDue := in.AdministeredAt.AddDate(0, 0, *in.RecurrenceDays)
		in.NextDueAt = &nextDue
	}

	eventID, err := uc.calendar.CreateEvent(ctx, pet.GoogleCalendarID, gcalendar.Event{
		Title:       in.Name,
		Description: fmt.Sprintf("Pet: %s", pet.Name),
		StartTime:   in.AdministeredAt,
		EndTime:     in.AdministeredAt,
		ReminderMins: nil,
		TimeZone:    uc.timezone,
	})
	if err != nil {
		return nil, fmt.Errorf("create vaccine calendar event: %w", err)
	}

	nextDueEventID := ""
	if in.NextDueAt != nil {
		nextDueEventID, err = uc.calendar.CreateEvent(ctx, pet.GoogleCalendarID, gcalendar.Event{
			Title:       fmt.Sprintf("Next due: %s", in.Name),
			Description: fmt.Sprintf("Pet: %s", pet.Name),
			StartTime:   *in.NextDueAt,
			EndTime:     *in.NextDueAt,
			ReminderMins: []int{7 * 24 * 60},
			TimeZone:    uc.timezone,
		})
		if err != nil {
			uc.compensateVaccineEvents(ctx, pet.GoogleCalendarID, []string{eventID}, in.PetID)
			return nil, fmt.Errorf("create vaccine next due calendar event: %w", err)
		}
	}

	in.GoogleCalendarEventID = eventID
	in.GoogleCalendarNextDueEventID = nextDueEventID
	var vaccine *domain.Vaccine
	err = uc.txRunner.WithinTx(ctx, func(_ *service.PetService, vaccines *service.VaccineService, _ *service.TreatmentService, _ *service.DoseService) error {
		recorded, err := vaccines.RecordVaccine(ctx, in)
		if err != nil {
			return fmt.Errorf("record vaccine: %w", err)
		}
		vaccine = recorded
		return nil
	})
	if err != nil {
		uc.compensateVaccineEvents(ctx, pet.GoogleCalendarID, []string{eventID, nextDueEventID}, in.PetID)
		return nil, err
	}
	uc.sendTelegram(ctx, formatVaccineCreatedMessage(pet, vaccine, uc.timezone), zap.String("pet_id", pet.ID), zap.String("vaccine_id", vaccine.ID))
	return vaccine, nil
}

func (uc *VaccineUseCase) DeleteVaccine(ctx context.Context, petID, vaccineID string) error {
	var (
		pet             *domain.Pet
		vaccine         *domain.Vaccine
		externalDeleted bool
	)

	err := uc.txRunner.WithinTx(ctx, func(pets *service.PetService, vaccines *service.VaccineService, _ *service.TreatmentService, _ *service.DoseService) error {
		loadedPet, err := pets.GetByID(ctx, petID)
		if err != nil {
			return fmt.Errorf("load pet %q: %w", petID, err)
		}
		pet = loadedPet
		loadedVaccine, err := vaccines.GetVaccine(ctx, petID, vaccineID)
		if err != nil {
			return fmt.Errorf("load vaccine %q: %w", vaccineID, err)
		}
		vaccine = loadedVaccine
		for _, eventID := range []string{vaccine.GoogleCalendarEventID, vaccine.GoogleCalendarNextDueEventID} {
			if eventID == "" {
				continue
			}
			if err := uc.calendar.DeleteEvent(ctx, pet.GoogleCalendarID, eventID); err != nil {
				return fmt.Errorf("delete vaccine calendar event %q: %w", eventID, err)
			}
			externalDeleted = true
		}
		if err := vaccines.DeleteVaccine(ctx, petID, vaccineID); err != nil {
			return fmt.Errorf("delete vaccine %q: %w", vaccineID, err)
		}
		return nil
	})
	if err != nil && externalDeleted && errors.Is(err, ErrTxCommit) {
		uc.logger.Error("vaccine delete committed external change before local commit failed",
			zap.String("pet_id", petID),
			zap.String("calendar_id", pet.GoogleCalendarID),
			zap.String("event_id", vaccine.GoogleCalendarEventID),
			zap.Error(err),
		)
	}
	if err == nil {
		uc.sendTelegram(ctx, formatVaccineDeletedMessage(pet, vaccine, uc.timezone), zap.String("pet_id", petID), zap.String("vaccine_id", vaccineID))
	}
	return err
}

func (uc *VaccineUseCase) compensateVaccineEvents(ctx context.Context, calendarID string, eventIDs []string, petID string) {
	for _, eventID := range eventIDs {
		if eventID == "" {
			continue
		}
		if delErr := uc.calendar.DeleteEvent(ctx, calendarID, eventID); delErr != nil {
			uc.logger.Error("calendar compensation failed after vaccine create error",
				zap.String("pet_id", petID),
				zap.String("calendar_id", calendarID),
				zap.String("event_id", eventID),
				zap.Error(delErr),
			)
		}
	}
}

func (uc *VaccineUseCase) sendTelegram(ctx context.Context, msg telegram.Message, fields ...zap.Field) {
	if uc.telegram == nil {
		return
	}
	if err := uc.telegram.Send(ctx, msg); err != nil {
		allFields := append([]zap.Field{zap.Error(err)}, fields...)
		uc.logger.Warn("telegram notification failed", allFields...)
	}
}

func formatVaccineCreatedMessage(pet *domain.Pet, vaccine *domain.Vaccine, timezone string) telegram.Message {
	var b strings.Builder
	b.WriteString("<b>💉 Vacina registrada</b>\n\n")
	writeHTMLLine(&b, "Pet", pet.Name)
	writeHTMLLine(&b, "Vacina", vaccine.Name)
	writeHTMLLine(&b, "Aplicada em", formatUserTime(vaccine.AdministeredAt, timezone))
	if vaccine.NextDueAt != nil {
		writeHTMLLine(&b, "Próxima dose", formatVaccineNextDue(*vaccine.NextDueAt, timezone))
	} else {
		writeRawHTMLLine(&b, "Próxima dose", "não configurada")
	}
	writeOptionalVaccineDetails(&b, vaccine)
	return telegram.Message{Text: b.String(), ParseMode: telegram.ParseModeHTML}
}

func formatVaccineDeletedMessage(pet *domain.Pet, vaccine *domain.Vaccine, timezone string) telegram.Message {
	var b strings.Builder
	b.WriteString("<b>🗑️ Vacina removida</b>\n\n")
	writeHTMLLine(&b, "Pet", pet.Name)
	writeHTMLLine(&b, "Vacina", vaccine.Name)
	writeHTMLLine(&b, "Aplicada em", formatUserTime(vaccine.AdministeredAt, timezone))
	if vaccine.NextDueAt != nil {
		writeHTMLLine(&b, "Próxima dose removida", formatVaccineNextDue(*vaccine.NextDueAt, timezone))
	}
	return telegram.Message{Text: b.String(), ParseMode: telegram.ParseModeHTML}
}

func writeOptionalVaccineDetails(b *strings.Builder, vaccine *domain.Vaccine) {
	hasDetails := vaccine.VetName != nil || vaccine.BatchNumber != nil || vaccine.Notes != nil
	if !hasDetails {
		return
	}
	b.WriteString("\n")
	if vaccine.VetName != nil {
		writeHTMLLine(b, "Veterinário", *vaccine.VetName)
	}
	if vaccine.BatchNumber != nil {
		writeHTMLLine(b, "Lote", *vaccine.BatchNumber)
	}
	if vaccine.Notes != nil {
		writeHTMLLine(b, "Observações", *vaccine.Notes)
	}
}

func writeHTMLLine(b *strings.Builder, label, value string) {
	writeRawHTMLLine(b, label, html.EscapeString(value))
}

func writeRawHTMLLine(b *strings.Builder, label, value string) {
	b.WriteString("<b>")
	b.WriteString(html.EscapeString(label))
	b.WriteString(":</b> ")
	b.WriteString(value)
	b.WriteString("\n")
}

func formatUserTime(t time.Time, timezone string) string {
	if timezone != "" {
		if loc, err := time.LoadLocation(timezone); err == nil {
			t = t.In(loc)
		}
	}
	return t.Format("02/01/2006 15:04")
}

func formatVaccineNextDue(t time.Time, timezone string) string {
	if t.Location() == time.UTC && t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 && t.Nanosecond() == 0 {
		return t.Format("02/01/2006")
	}
	return formatUserTime(t, timezone)
}
