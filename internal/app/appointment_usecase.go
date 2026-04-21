package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/rafaelsoares/alfredo/internal/gcalendar"
	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
	"github.com/rafaelsoares/alfredo/internal/petcare/service"
	"github.com/rafaelsoares/alfredo/internal/telegram"
)

// AppointmentUseCase wraps AppointmentService and orchestrates calendar side effects.
type AppointmentUseCase struct {
	appointments AppointmentServicer
	pets         PetNameGetter
	calendar     CalendarPort
	telegram     TelegramPort
	logger       *zap.Logger
	timezone     string
}

func NewAppointmentUseCase(appointments AppointmentServicer, pets PetNameGetter, calendar CalendarPort, telegram TelegramPort, timezone string, logger *zap.Logger) *AppointmentUseCase {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &AppointmentUseCase{
		appointments: appointments,
		pets:         pets,
		calendar:     calendar,
		telegram:     telegram,
		logger:       logger,
		timezone:     timezone,
	}
}

func (uc *AppointmentUseCase) Create(ctx context.Context, in service.CreateAppointmentInput) (*domain.Appointment, error) {
	pet, err := uc.pets.GetByID(ctx, in.PetID)
	if err != nil {
		return nil, fmt.Errorf("load pet %q: %w", in.PetID, err)
	}
	if pet.GoogleCalendarID == "" {
		return nil, fmt.Errorf("pet %q is missing google calendar id", in.PetID)
	}

	location := ""
	if in.Location != nil {
		location = *in.Location
	}

	eventID, err := uc.calendar.CreateEvent(ctx, pet.GoogleCalendarID, gcalendar.Event{
		Title:       appointmentTypeLabel(in.Type),
		Location:    location,
		Description: fmt.Sprintf("Pet: %s", pet.Name),
		StartTime:   in.ScheduledAt,
		EndTime:     in.ScheduledAt.Add(time.Hour),
		ReminderMins: []int{24 * 60},
		TimeZone:    uc.timezone,
	})
	if err != nil {
		return nil, fmt.Errorf("create appointment calendar event: %w", err)
	}

	in.GoogleCalendarEventID = eventID
	appt, err := uc.appointments.Create(ctx, in)
	if err != nil {
		if delErr := uc.calendar.DeleteEvent(ctx, pet.GoogleCalendarID, eventID); delErr != nil {
			uc.logger.Error("calendar compensation failed after appointment create error",
				zap.String("pet_id", in.PetID),
				zap.String("calendar_id", pet.GoogleCalendarID),
				zap.String("event_id", eventID),
				zap.Error(delErr),
			)
		}
		return nil, err
	}

	uc.sendTelegram(ctx, formatAppointmentCreatedMessage(pet, appt, uc.timezone), zap.String("pet_id", pet.ID), zap.String("appointment_id", appt.ID))
	return appt, nil
}

func (uc *AppointmentUseCase) GetByID(ctx context.Context, petID, appointmentID string) (*domain.Appointment, error) {
	appt, err := uc.appointments.GetByID(ctx, petID, appointmentID)
	if err != nil {
		return nil, fmt.Errorf("get appointment %q: %w", appointmentID, err)
	}
	return appt, nil
}

func (uc *AppointmentUseCase) List(ctx context.Context, petID string) ([]domain.Appointment, error) {
	appts, err := uc.appointments.List(ctx, petID)
	if err != nil {
		return nil, fmt.Errorf("list appointments for pet %q: %w", petID, err)
	}
	return appts, nil
}

func (uc *AppointmentUseCase) Update(ctx context.Context, petID, appointmentID string, in service.UpdateAppointmentInput) (*domain.Appointment, error) {
	var (
		previous *domain.Appointment
		pet      *domain.Pet
	)
	if in.ScheduledAt != nil || in.Location != nil {
		var err error
		previous, err = uc.appointments.GetByID(ctx, petID, appointmentID)
		if err != nil {
			return nil, fmt.Errorf("load appointment %q: %w", appointmentID, err)
		}
		pet, err = uc.pets.GetByID(ctx, petID)
		if err != nil {
			return nil, fmt.Errorf("load pet %q: %w", petID, err)
		}
		updated := *previous
		if in.ScheduledAt != nil {
			updated.ScheduledAt = *in.ScheduledAt
		}
		if in.Location != nil {
			updated.Location = in.Location
		}
		if err := uc.calendar.UpdateEvent(ctx, pet.GoogleCalendarID, previous.GoogleCalendarEventID, uc.appointmentCalendarEvent(pet, &updated)); err != nil {
			return nil, fmt.Errorf("update appointment calendar event %q: %w", previous.GoogleCalendarEventID, err)
		}
	}

	appt, err := uc.appointments.Update(ctx, petID, appointmentID, in)
	if err != nil {
		if previous != nil {
			if delErr := uc.calendar.UpdateEvent(ctx, pet.GoogleCalendarID, previous.GoogleCalendarEventID, uc.appointmentCalendarEvent(pet, previous)); delErr != nil {
				uc.logger.Error("calendar compensation failed after appointment update error",
					zap.String("pet_id", petID),
					zap.String("calendar_id", pet.GoogleCalendarID),
					zap.String("event_id", previous.GoogleCalendarEventID),
					zap.Error(delErr),
				)
			}
		}
		return nil, fmt.Errorf("update appointment %q: %w", appointmentID, err)
	}
	return appt, nil
}

func (uc *AppointmentUseCase) Delete(ctx context.Context, petID, appointmentID string) error {
	appt, err := uc.appointments.GetByID(ctx, petID, appointmentID)
	if err != nil {
		return fmt.Errorf("load appointment %q: %w", appointmentID, err)
	}

	pet, err := uc.pets.GetByID(ctx, petID)
	if err != nil {
		return fmt.Errorf("load pet %q: %w", petID, err)
	}

	if appt.GoogleCalendarEventID != "" {
		if err := uc.calendar.DeleteEvent(ctx, pet.GoogleCalendarID, appt.GoogleCalendarEventID); err != nil {
			return fmt.Errorf("delete appointment calendar event %q: %w", appt.GoogleCalendarEventID, err)
		}
	}

	if err := uc.appointments.Delete(ctx, petID, appointmentID); err != nil {
		uc.logger.Error("appointment delete committed external change before local delete failed",
			zap.String("pet_id", petID),
			zap.String("calendar_id", pet.GoogleCalendarID),
			zap.String("event_id", appt.GoogleCalendarEventID),
			zap.Error(err),
		)
		return err
	}

	uc.sendTelegram(ctx, formatAppointmentDeletedMessage(pet, appt, uc.timezone), zap.String("pet_id", petID), zap.String("appointment_id", appointmentID))
	return nil
}

func (uc *AppointmentUseCase) sendTelegram(ctx context.Context, msg telegram.Message, fields ...zap.Field) {
	if uc.telegram == nil {
		return
	}
	if err := uc.telegram.Send(ctx, msg); err != nil {
		allFields := append([]zap.Field{zap.Error(err)}, fields...)
		uc.logger.Warn("telegram notification failed", allFields...)
	}
}

func (uc *AppointmentUseCase) appointmentCalendarEvent(pet *domain.Pet, appt *domain.Appointment) gcalendar.Event {
	location := ""
	if appt.Location != nil {
		location = *appt.Location
	}
	return gcalendar.Event{
		Title:       appointmentTypeLabel(appt.Type),
		Location:    location,
		Description: fmt.Sprintf("Pet: %s", pet.Name),
		StartTime:   appt.ScheduledAt,
		EndTime:     appt.ScheduledAt.Add(time.Hour),
		ReminderMins: []int{24 * 60},
		TimeZone:    uc.timezone,
	}
}

func appointmentTypeLabel(t domain.AppointmentType) string {
	switch t {
	case domain.AppointmentTypeVet:
		return "Consulta Veterinária"
	case domain.AppointmentTypeGrooming:
		return "Banho e Tosa"
	default:
		return "Agendamento"
	}
}

func formatAppointmentCreatedMessage(pet *domain.Pet, appt *domain.Appointment, timezone string) telegram.Message {
	var b strings.Builder
	b.WriteString("🗓️ <b>Consulta agendada</b>\n\n")
	writeHTMLLine(&b, "Pet", pet.Name)
	writeHTMLLine(&b, "Tipo", appointmentTypeLabel(appt.Type))
	writeHTMLLine(&b, "Data", formatUserTime(appt.ScheduledAt, timezone))
	if appt.Location != nil {
		writeHTMLLine(&b, "Local", *appt.Location)
	}
	if appt.Provider != nil {
		writeHTMLLine(&b, "Prestador", *appt.Provider)
	}
	return telegram.Message{Text: b.String(), ParseMode: telegram.ParseModeHTML}
}

func formatAppointmentDeletedMessage(pet *domain.Pet, appt *domain.Appointment, timezone string) telegram.Message {
	var b strings.Builder
	b.WriteString("🗑️ <b>Consulta cancelada</b>\n\n")
	writeHTMLLine(&b, "Pet", pet.Name)
	writeHTMLLine(&b, "Tipo", appointmentTypeLabel(appt.Type))
	writeHTMLLine(&b, "Data", formatUserTime(appt.ScheduledAt, timezone))
	return telegram.Message{Text: b.String(), ParseMode: telegram.ParseModeHTML}
}
