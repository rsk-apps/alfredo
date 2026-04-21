package app

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/rafaelsoares/alfredo/internal/gcalendar"
	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
	"github.com/rafaelsoares/alfredo/internal/petcare/service"
	"github.com/rafaelsoares/alfredo/internal/telegram"
)

// TreatmentUseCase orchestrates treatment creation, dose generation, and calendar side effects.
type TreatmentUseCase struct {
	treatments TreatmentServicer
	doses      DoseServicer
	pets       PetNameGetter
	txRunner   PetCareTxRunner
	calendar   CalendarPort
	telegram   TelegramPort
	timezone   string
	logger     *zap.Logger
}

func NewTreatmentUseCase(
	treatments TreatmentServicer,
	doses DoseServicer,
	pets PetNameGetter,
	txRunner PetCareTxRunner,
	calendar CalendarPort,
	telegramPort TelegramPort,
	timezone string,
	logger *zap.Logger,
) *TreatmentUseCase {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &TreatmentUseCase{treatments: treatments, doses: doses, pets: pets, txRunner: txRunner, calendar: calendar, telegram: telegramPort, timezone: timezone, logger: logger}
}

// Create starts a treatment and creates the corresponding calendar state.
func (uc *TreatmentUseCase) Create(ctx context.Context, in service.CreateTreatmentInput) (*domain.Treatment, []domain.Dose, error) {
	pet, err := uc.pets.GetByID(ctx, in.PetID)
	if err != nil {
		return nil, nil, fmt.Errorf("load pet %q: %w", in.PetID, err)
	}
	if pet.GoogleCalendarID == "" {
		return nil, nil, fmt.Errorf("pet %q is missing google calendar id", in.PetID)
	}

	if in.EndedAt == nil {
		return uc.createRecurringTreatment(ctx, pet, in)
	}

	var (
		tr              *domain.Treatment
		doses           []domain.Dose
		createdEventIDs []string
	)
	err = uc.txRunner.WithinTx(ctx, func(_ *service.PetService, _ *service.VaccineService, treatments *service.TreatmentService, dosesSvc *service.DoseService) error {
		createdTreatment, err := treatments.Create(ctx, in)
		if err != nil {
			return fmt.Errorf("create treatment: %w", err)
		}
		tr = createdTreatment
		doses = dosesSvc.GenerateDoses(*tr, *tr.EndedAt)
		for i := range doses {
			eventID, err := uc.calendar.CreateEvent(ctx, pet.GoogleCalendarID, gcalendar.Event{
				Title:       fmt.Sprintf("%d/%d %s", i+1, len(doses), tr.Name),
				Description: fmt.Sprintf("Pet: %s", pet.Name),
				StartTime:   doses[i].ScheduledFor,
				EndTime:     doses[i].ScheduledFor,
				ReminderMins: nil,
				TimeZone:    uc.timezone,
			})
			if err != nil {
				return fmt.Errorf("create dose calendar event %d for treatment %q: %w", i+1, tr.ID, err)
			}
			doses[i].GoogleCalendarEventID = eventID
			createdEventIDs = append(createdEventIDs, eventID)
		}
		if err := dosesSvc.CreateBatch(ctx, doses); err != nil {
			return fmt.Errorf("create treatment doses: %w", err)
		}
		return nil
	})
	if err != nil {
		uc.compensateEvents(ctx, pet.GoogleCalendarID, createdEventIDs, tr)
		return nil, nil, err
	}
	uc.sendTelegram(ctx, formatTreatmentCreatedMessage(pet, tr, doses, uc.timezone), zap.String("pet_id", pet.ID), zap.String("treatment_id", tr.ID))
	return tr, doses, nil
}

// GetByID returns a treatment and its doses.
func (uc *TreatmentUseCase) GetByID(ctx context.Context, petID, treatmentID string) (*domain.Treatment, []domain.Dose, error) {
	tr, err := uc.treatments.GetByID(ctx, petID, treatmentID)
	if err != nil {
		return nil, nil, err
	}
	doses, err := uc.doses.ListByTreatment(ctx, treatmentID)
	if err != nil {
		return nil, nil, err
	}
	return tr, doses, nil
}

// List returns all treatments for a pet with their doses.
func (uc *TreatmentUseCase) List(ctx context.Context, petID string) ([]domain.Treatment, map[string][]domain.Dose, error) {
	ts, err := uc.treatments.List(ctx, petID)
	if err != nil {
		return nil, nil, err
	}
	doseMap := make(map[string][]domain.Dose, len(ts))
	for _, t := range ts {
		doses, err := uc.doses.ListByTreatment(ctx, t.ID)
		if err != nil {
			return nil, nil, err
		}
		doseMap[t.ID] = doses
	}
	return ts, doseMap, nil
}

func (uc *TreatmentUseCase) createRecurringTreatment(ctx context.Context, pet *domain.Pet, in service.CreateTreatmentInput) (*domain.Treatment, []domain.Dose, error) {
	eventID, err := uc.calendar.CreateRecurringEvent(ctx, pet.GoogleCalendarID, gcalendar.Event{
		Title:       in.Name,
		Description: fmt.Sprintf("Pet: %s", pet.Name),
		StartTime:   in.StartedAt,
		EndTime:     in.StartedAt,
		ReminderMins: nil,
		TimeZone:    uc.timezone,
	}, in.IntervalHours)
	if err != nil {
		return nil, nil, fmt.Errorf("create recurring treatment event: %w", err)
	}

	in.GoogleCalendarEventID = eventID
	var treatment *domain.Treatment
	err = uc.txRunner.WithinTx(ctx, func(_ *service.PetService, _ *service.VaccineService, treatments *service.TreatmentService, _ *service.DoseService) error {
		created, err := treatments.Create(ctx, in)
		if err != nil {
			return fmt.Errorf("create recurring treatment: %w", err)
		}
		treatment = created
		return nil
	})
	if err != nil {
		if delErr := uc.calendar.DeleteEvent(ctx, pet.GoogleCalendarID, eventID); delErr != nil {
			uc.logger.Error("calendar compensation failed after recurring treatment create error",
				zap.String("pet_id", pet.ID),
				zap.String("calendar_id", pet.GoogleCalendarID),
				zap.String("event_id", eventID),
				zap.Error(delErr),
			)
		}
		return nil, nil, err
	}
	uc.sendTelegram(ctx, formatTreatmentCreatedMessage(pet, treatment, nil, uc.timezone), zap.String("pet_id", pet.ID), zap.String("treatment_id", treatment.ID))
	return treatment, nil, nil
}

// Stop marks a treatment as stopped and cleans up future calendar state.
func (uc *TreatmentUseCase) Stop(ctx context.Context, petID, treatmentID string) error {
	var (
		pet             *domain.Pet
		tr              *domain.Treatment
		allDoses        []domain.Dose
		futureDoses     []domain.Dose
		externalChanged bool
	)
	now := time.Now().UTC()
	err := uc.txRunner.WithinTx(ctx, func(pets *service.PetService, _ *service.VaccineService, treatments *service.TreatmentService, doses *service.DoseService) error {
		loadedPet, err := pets.GetByID(ctx, petID)
		if err != nil {
			return fmt.Errorf("load pet %q: %w", petID, err)
		}
		pet = loadedPet
		loadedTreatment, err := treatments.GetByID(ctx, petID, treatmentID)
		if err != nil {
			return fmt.Errorf("load treatment %q: %w", treatmentID, err)
		}
		tr = loadedTreatment

		if tr.EndedAt == nil {
			if tr.GoogleCalendarEventID != "" {
				if err := uc.calendar.StopRecurringEvent(ctx, pet.GoogleCalendarID, tr.GoogleCalendarEventID, now); err != nil {
					return fmt.Errorf("stop recurring treatment event %q: %w", tr.GoogleCalendarEventID, err)
				}
				externalChanged = true
			}
			if err := treatments.Stop(ctx, petID, treatmentID); err != nil {
				return fmt.Errorf("stop treatment %q: %w", treatmentID, err)
			}
			return nil
		}

		allDoses, err = doses.ListByTreatment(ctx, treatmentID)
		if err != nil {
			return fmt.Errorf("list doses for treatment %q: %w", treatmentID, err)
		}
		futureDoses, err = doses.ListFutureByTreatment(ctx, treatmentID, now)
		if err != nil {
			return fmt.Errorf("list future doses for treatment %q: %w", treatmentID, err)
		}
		for _, dose := range futureDoses {
			if dose.GoogleCalendarEventID == "" {
				continue
			}
			if err := uc.calendar.DeleteEvent(ctx, pet.GoogleCalendarID, dose.GoogleCalendarEventID); err != nil {
				return fmt.Errorf("delete dose calendar event %q: %w", dose.GoogleCalendarEventID, err)
			}
			externalChanged = true
		}
		if err := treatments.Stop(ctx, petID, treatmentID); err != nil {
			return fmt.Errorf("stop treatment %q: %w", treatmentID, err)
		}
		if err := doses.DeleteFutureByTreatment(ctx, treatmentID, now); err != nil {
			return fmt.Errorf("delete future doses for treatment %q: %w", treatmentID, err)
		}
		return nil
	})
	if err != nil && externalChanged && errors.Is(err, ErrTxCommit) {
		uc.logger.Error("treatment stop committed external change before local commit failed",
			zap.String("pet_id", petID),
			zap.String("calendar_id", pet.GoogleCalendarID),
			zap.String("treatment_id", treatmentID),
			zap.String("event_id", tr.GoogleCalendarEventID),
			zap.Error(err),
		)
	}
	if err == nil {
		uc.sendTelegram(ctx, formatTreatmentStoppedMessage(pet, tr, allDoses, futureDoses, now, uc.timezone), zap.String("pet_id", petID), zap.String("treatment_id", treatmentID))
	}
	return err
}

func (uc *TreatmentUseCase) compensateEvents(ctx context.Context, calendarID string, eventIDs []string, tr *domain.Treatment) {
	for _, eventID := range eventIDs {
		if delErr := uc.calendar.DeleteEvent(ctx, calendarID, eventID); delErr != nil {
			fields := []zap.Field{
				zap.String("calendar_id", calendarID),
				zap.String("event_id", eventID),
				zap.Error(delErr),
			}
			if tr != nil {
				fields = append(fields, zap.String("treatment_id", tr.ID))
			}
			uc.logger.Error("calendar compensation failed after treatment create error", fields...)
		}
	}
}

func (uc *TreatmentUseCase) sendTelegram(ctx context.Context, msg telegram.Message, fields ...zap.Field) {
	if uc.telegram == nil {
		return
	}
	if err := uc.telegram.Send(ctx, msg); err != nil {
		allFields := append([]zap.Field{zap.Error(err)}, fields...)
		uc.logger.Warn("telegram notification failed", allFields...)
	}
}

func formatTreatmentCreatedMessage(pet *domain.Pet, treatment *domain.Treatment, doses []domain.Dose, timezone string) telegram.Message {
	var b strings.Builder
	if treatment.EndedAt == nil {
		b.WriteString("<b>💊 Tratamento contínuo registrado</b>\n\n")
	} else {
		b.WriteString("<b>💊 Tratamento registrado</b>\n\n")
	}
	writeTreatmentBase(&b, pet, treatment, timezone)
	if treatment.EndedAt == nil {
		writeRawHTMLLine(&b, "Fim previsto", "sem data definida")
	} else {
		writeHTMLLine(&b, "Fim previsto", formatUserTime(*treatment.EndedAt, timezone))
		writeRawHTMLLine(&b, "Doses agendadas", strconv.Itoa(len(doses)))
	}
	writeOptionalTreatmentDetails(&b, treatment)
	return telegram.Message{Text: b.String(), ParseMode: telegram.ParseModeHTML}
}

func formatTreatmentStoppedMessage(pet *domain.Pet, treatment *domain.Treatment, allDoses []domain.Dose, futureDoses []domain.Dose, stoppedAt time.Time, timezone string) telegram.Message {
	var b strings.Builder
	if treatment.EndedAt == nil {
		b.WriteString("<b>⛔ Tratamento contínuo interrompido</b>\n\n")
		writeHTMLLine(&b, "Pet", pet.Name)
		writeHTMLLine(&b, "Tratamento", treatment.Name)
		writeHTMLLine(&b, "Interrompido em", formatUserTime(stoppedAt, timezone))
		writeRawHTMLLine(&b, "Série recorrente", "encerrada no calendário")
		return telegram.Message{Text: b.String(), ParseMode: telegram.ParseModeHTML}
	}

	futureDeleted := len(futureDoses)
	alreadyOccurred := len(allDoses) - futureDeleted
	if alreadyOccurred < 0 {
		alreadyOccurred = 0
	}

	b.WriteString("<b>⛔ Tratamento interrompido</b>\n\n")
	writeHTMLLine(&b, "Pet", pet.Name)
	writeHTMLLine(&b, "Tratamento", treatment.Name)
	writeHTMLLine(&b, "Interrompido em", formatUserTime(stoppedAt, timezone))
	writeRawHTMLLine(&b, "Doses já ocorridas", strconv.Itoa(alreadyOccurred))
	writeRawHTMLLine(&b, "Doses futuras removidas", strconv.Itoa(futureDeleted))
	return telegram.Message{Text: b.String(), ParseMode: telegram.ParseModeHTML}
}

func writeTreatmentBase(b *strings.Builder, pet *domain.Pet, treatment *domain.Treatment, timezone string) {
	writeHTMLLine(b, "Pet", pet.Name)
	writeHTMLLine(b, "Tratamento", treatment.Name)
	writeHTMLLine(b, "Dose", formatDose(treatment))
	writeHTMLLine(b, "Via", treatment.Route)
	writeRawHTMLLine(b, "Intervalo", fmt.Sprintf("a cada %d horas", treatment.IntervalHours))
	writeHTMLLine(b, "Início", formatUserTime(treatment.StartedAt, timezone))
}

func writeOptionalTreatmentDetails(b *strings.Builder, treatment *domain.Treatment) {
	hasDetails := treatment.VetName != nil || treatment.Notes != nil
	if !hasDetails {
		return
	}
	b.WriteString("\n")
	if treatment.VetName != nil {
		writeHTMLLine(b, "Veterinário", *treatment.VetName)
	}
	if treatment.Notes != nil {
		writeHTMLLine(b, "Observações", *treatment.Notes)
	}
}

func formatDose(treatment *domain.Treatment) string {
	amount := strconv.FormatFloat(treatment.DosageAmount, 'f', -1, 64)
	if treatment.DosageUnit == "" {
		return amount
	}
	return amount + " " + treatment.DosageUnit
}
