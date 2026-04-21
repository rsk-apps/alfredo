package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/rafaelsoares/alfredo/internal/gcalendar"
	healthdomain "github.com/rafaelsoares/alfredo/internal/health/domain"
	healthsvc "github.com/rafaelsoares/alfredo/internal/health/service"
	"github.com/rafaelsoares/alfredo/internal/telegram"
)

// HealthAppointmentUseCase orchestrates health appointment CRUD with calendar and Telegram side effects.
type HealthAppointmentUseCase struct {
	appointments HealthAppointmentServicer
	profiles     HealthCalendarIDStorer
	calendar     CalendarPort
	telegram     TelegramPort
	logger       *zap.Logger
	timezone     string
}

// NewHealthAppointmentUseCase creates a new health appointment use case.
func NewHealthAppointmentUseCase(
	appointments HealthAppointmentServicer,
	profiles HealthCalendarIDStorer,
	calendar CalendarPort,
	telegram TelegramPort,
	timezone string,
	logger *zap.Logger,
) *HealthAppointmentUseCase {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &HealthAppointmentUseCase{
		appointments: appointments,
		profiles:     profiles,
		calendar:     calendar,
		telegram:     telegram,
		logger:       logger,
		timezone:     timezone,
	}
}

// Create creates a new health appointment with calendar and Telegram notifications.
func (uc *HealthAppointmentUseCase) Create(ctx context.Context, specialty string, scheduledAt time.Time, doctor, notes *string) (*healthdomain.HealthAppointment, error) {
	// Validate specialty is non-empty
	if specialty == "" {
		return nil, fmt.Errorf("specialty must not be empty: %w", healthdomain.ErrValidation)
	}

	// Ensure calendar exists
	calendarID, err := uc.ensureCalendar(ctx)
	if err != nil {
		return nil, err
	}

	// Create calendar event
	eventID, err := uc.calendar.CreateEvent(ctx, calendarID, gcalendar.Event{
		Title:       "Consulta: " + specialty,
		Description: extractDescription(doctor),
		StartTime:   scheduledAt,
		EndTime:     scheduledAt.Add(time.Hour),
		ReminderMins: []int{1440, 120, 60}, // 1 day, 2 hours, 1 hour before
		TimeZone:    uc.timezone,
	})
	if err != nil {
		return nil, fmt.Errorf("create appointment calendar event: %w", err)
	}

	// Create appointment in database
	in := healthsvc.CreateHealthAppointmentInput{
		Specialty:             specialty,
		ScheduledAt:           scheduledAt,
		Doctor:                doctor,
		Notes:                 notes,
		GoogleCalendarEventID: eventID,
	}
	appt, err := uc.appointments.Create(ctx, in)
	if err != nil {
		if delErr := uc.calendar.DeleteEvent(ctx, calendarID, eventID); delErr != nil {
			uc.logger.Warn("calendar compensation failed after appointment create error",
				zap.String("calendar_id", calendarID),
				zap.String("event_id", eventID),
				zap.Error(delErr),
			)
		}
		return nil, fmt.Errorf("create appointment: %w", err)
	}

	// Send Telegram notification (best-effort)
	uc.sendTelegram(ctx, formatHealthAppointmentCreatedMessage(appt, uc.timezone))

	return appt, nil
}

// GetByID retrieves a health appointment by ID.
func (uc *HealthAppointmentUseCase) GetByID(ctx context.Context, id string) (*healthdomain.HealthAppointment, error) {
	return uc.appointments.GetByID(ctx, id)
}

// List retrieves all health appointments ordered by scheduled_at.
func (uc *HealthAppointmentUseCase) List(ctx context.Context) ([]healthdomain.HealthAppointment, error) {
	return uc.appointments.List(ctx)
}

// Delete deletes a health appointment and its calendar event.
func (uc *HealthAppointmentUseCase) Delete(ctx context.Context, id string) error {
	// Load appointment
	appt, err := uc.appointments.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Ensure calendar exists
	calendarID, err := uc.ensureCalendar(ctx)
	if err != nil {
		return err
	}

	// Delete calendar event if it exists
	if appt.GoogleCalendarEventID != "" {
		if err := uc.calendar.DeleteEvent(ctx, calendarID, appt.GoogleCalendarEventID); err != nil {
			return fmt.Errorf("delete appointment calendar event: %w", err)
		}
	}

	// Delete appointment from database
	if err := uc.appointments.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete appointment: %w", err)
	}

	// Send Telegram notification (best-effort)
	uc.sendTelegram(ctx, formatHealthAppointmentDeletedMessage(appt, uc.timezone))

	return nil
}

// ensureCalendar lazily provisions a health calendar if one doesn't exist.
func (uc *HealthAppointmentUseCase) ensureCalendar(ctx context.Context) (string, error) {
	id, err := uc.profiles.GetCalendarID(ctx)
	if err != nil {
		return "", fmt.Errorf("get health calendar id: %w", err)
	}
	if id != "" {
		return id, nil
	}

	// Create a new calendar
	id, err = uc.calendar.CreateCalendar(ctx, "Saúde")
	if err != nil {
		return "", fmt.Errorf("provision health calendar: %w", err)
	}

	// Persist the calendar ID to profile
	if err := uc.profiles.SetCalendarID(ctx, id); err != nil {
		_ = uc.calendar.DeleteCalendar(ctx, id) // best-effort rollback
		return "", fmt.Errorf("persist health calendar id: %w", err)
	}

	return id, nil
}

// sendTelegram sends a message via Telegram, swallowing errors.
func (uc *HealthAppointmentUseCase) sendTelegram(ctx context.Context, msg telegram.Message) {
	if err := uc.telegram.Send(ctx, msg); err != nil {
		uc.logger.Warn("telegram notification failed", zap.Error(err))
	}
}

func extractDescription(doctor *string) string {
	if doctor == nil {
		return ""
	}
	return *doctor
}

func formatHealthAppointmentCreatedMessage(appt *healthdomain.HealthAppointment, timezone string) telegram.Message {
	var b strings.Builder
	b.WriteString("🗓️ <b>Consulta agendada</b>\n\n")
	writeHTMLLine(&b, "Especialidade", appt.Specialty)
	writeHTMLLine(&b, "Data", formatUserTime(appt.ScheduledAt, timezone))
	if appt.Doctor != nil {
		writeHTMLLine(&b, "Médico", *appt.Doctor)
	}
	if appt.Notes != nil {
		writeHTMLLine(&b, "Notas", *appt.Notes)
	}
	return telegram.Message{Text: b.String(), ParseMode: telegram.ParseModeHTML}
}

func formatHealthAppointmentDeletedMessage(appt *healthdomain.HealthAppointment, timezone string) telegram.Message {
	var b strings.Builder
	b.WriteString("🗑️ <b>Consulta cancelada</b>\n\n")
	writeHTMLLine(&b, "Especialidade", appt.Specialty)
	writeHTMLLine(&b, "Data", formatUserTime(appt.ScheduledAt, timezone))
	return telegram.Message{Text: b.String(), ParseMode: telegram.ParseModeHTML}
}
