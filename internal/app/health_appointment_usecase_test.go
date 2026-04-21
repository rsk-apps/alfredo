package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rafaelsoares/alfredo/internal/gcalendar"
	healthdomain "github.com/rafaelsoares/alfredo/internal/health/domain"
	healthsvc "github.com/rafaelsoares/alfredo/internal/health/service"
	"github.com/rafaelsoares/alfredo/internal/telegram"
	"go.uber.org/zap"
)

type mockHealthAppointmentServicer struct {
	appts map[string]*healthdomain.HealthAppointment
	err   error
}

func (m *mockHealthAppointmentServicer) Create(ctx context.Context, in healthsvc.CreateHealthAppointmentInput) (*healthdomain.HealthAppointment, error) {
	if m.err != nil {
		return nil, m.err
	}
	appt := &healthdomain.HealthAppointment{
		ID:                    "appt-1",
		Specialty:             in.Specialty,
		ScheduledAt:           in.ScheduledAt,
		Doctor:                in.Doctor,
		Notes:                 in.Notes,
		GoogleCalendarEventID: in.GoogleCalendarEventID,
		CreatedAt:             time.Now().UTC(),
	}
	if m.appts == nil {
		m.appts = make(map[string]*healthdomain.HealthAppointment)
	}
	m.appts[appt.ID] = appt
	return appt, nil
}

func (m *mockHealthAppointmentServicer) GetByID(ctx context.Context, id string) (*healthdomain.HealthAppointment, error) {
	if m.err != nil {
		return nil, m.err
	}
	a, ok := m.appts[id]
	if !ok {
		return nil, healthdomain.ErrNotFound
	}
	return a, nil
}

func (m *mockHealthAppointmentServicer) List(ctx context.Context) ([]healthdomain.HealthAppointment, error) {
	if m.err != nil {
		return nil, m.err
	}
	var out []healthdomain.HealthAppointment
	for _, a := range m.appts {
		out = append(out, *a)
	}
	return out, nil
}

func (m *mockHealthAppointmentServicer) Delete(ctx context.Context, id string) error {
	if m.err != nil {
		return m.err
	}
	if _, ok := m.appts[id]; !ok {
		return healthdomain.ErrNotFound
	}
	delete(m.appts, id)
	return nil
}

type mockHealthCalendarIDStorer struct {
	calendarID string
	err        error
}

func (m *mockHealthCalendarIDStorer) GetCalendarID(ctx context.Context) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.calendarID, nil
}

func (m *mockHealthCalendarIDStorer) SetCalendarID(ctx context.Context, calendarID string) error {
	if m.err != nil {
		return m.err
	}
	m.calendarID = calendarID
	return nil
}

type mockCalendarPort struct {
	createdCalendars map[string]bool
	createdEvents    map[string]gcalendar.Event
	deletedEvents    []string
	createCalErr     error
	createEventErr   error
	deleteEventErr   error
}

func (m *mockCalendarPort) CreateCalendar(ctx context.Context, name string) (string, error) {
	if m.createCalErr != nil {
		return "", m.createCalErr
	}
	calID := "cal-" + name
	if m.createdCalendars == nil {
		m.createdCalendars = make(map[string]bool)
	}
	m.createdCalendars[calID] = true
	return calID, nil
}

func (m *mockCalendarPort) DeleteCalendar(ctx context.Context, calendarID string) error {
	if m.createdCalendars != nil {
		delete(m.createdCalendars, calendarID)
	}
	return nil
}

func (m *mockCalendarPort) CreateEvent(ctx context.Context, calendarID string, event gcalendar.Event) (string, error) {
	if m.createEventErr != nil {
		return "", m.createEventErr
	}
	eventID := "evt-" + event.Title
	if m.createdEvents == nil {
		m.createdEvents = make(map[string]gcalendar.Event)
	}
	m.createdEvents[eventID] = event
	return eventID, nil
}

func (m *mockCalendarPort) UpdateEvent(ctx context.Context, calendarID string, eventID string, event gcalendar.Event) error {
	return nil
}

func (m *mockCalendarPort) CreateRecurringEvent(ctx context.Context, calendarID string, event gcalendar.Event, intervalHours int) (string, error) {
	return "", nil
}

func (m *mockCalendarPort) StopRecurringEvent(ctx context.Context, calendarID string, eventID string, until time.Time) error {
	return nil
}

func (m *mockCalendarPort) DeleteEvent(ctx context.Context, calendarID string, eventID string) error {
	if m.deleteEventErr != nil {
		return m.deleteEventErr
	}
	m.deletedEvents = append(m.deletedEvents, eventID)
	if m.createdEvents != nil {
		delete(m.createdEvents, eventID)
	}
	return nil
}

type mockTelegramPort struct {
	messages []telegram.Message
	err      error
}

func (m *mockTelegramPort) Send(ctx context.Context, msg telegram.Message) error {
	if m.err != nil {
		return m.err
	}
	if m.messages == nil {
		m.messages = make([]telegram.Message, 0)
	}
	m.messages = append(m.messages, msg)
	return nil
}

func TestHealthAppointmentUseCaseCreate(t *testing.T) {
	apptSvc := &mockHealthAppointmentServicer{appts: make(map[string]*healthdomain.HealthAppointment)}
	profileSvc := &mockHealthCalendarIDStorer{}
	calendar := &mockCalendarPort{}
	telegram := &mockTelegramPort{}

	uc := NewHealthAppointmentUseCase(apptSvc, profileSvc, calendar, telegram, "America/Sao_Paulo", zap.NewNop())
	ctx := context.Background()

	scheduledAt := time.Date(2026, 5, 10, 9, 0, 0, 0, time.UTC)
	appt, err := uc.Create(ctx, "Cardiologia", scheduledAt, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if appt.Specialty != "Cardiologia" {
		t.Fatalf("expected specialty Cardiologia, got %q", appt.Specialty)
	}
	if appt.GoogleCalendarEventID == "" {
		t.Fatal("expected calendar event ID to be set")
	}

	// Verify calendar was created
	if profileSvc.calendarID != "cal-Saúde" {
		t.Fatalf("expected calendar to be saved, got %q", profileSvc.calendarID)
	}
}

func TestHealthAppointmentUseCaseCreateWithExistingCalendar(t *testing.T) {
	apptSvc := &mockHealthAppointmentServicer{appts: make(map[string]*healthdomain.HealthAppointment)}
	profileSvc := &mockHealthCalendarIDStorer{calendarID: "cal-existing"}
	calendar := &mockCalendarPort{}
	telegram := &mockTelegramPort{}

	uc := NewHealthAppointmentUseCase(apptSvc, profileSvc, calendar, telegram, "America/Sao_Paulo", zap.NewNop())
	ctx := context.Background()

	appt, err := uc.Create(ctx, "Dentista", time.Now(), nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify only one calendar exists (the existing one was reused)
	if len(calendar.createdCalendars) != 0 {
		t.Fatalf("expected no new calendars to be created, got %d", len(calendar.createdCalendars))
	}
	if appt.GoogleCalendarEventID == "" {
		t.Fatal("expected event ID to be set")
	}
}

func TestHealthAppointmentUseCaseDelete(t *testing.T) {
	appt := &healthdomain.HealthAppointment{
		ID:                    "appt-1",
		Specialty:             "Oftalmologia",
		GoogleCalendarEventID: "evt-123",
		CreatedAt:             time.Now(),
	}
	apptSvc := &mockHealthAppointmentServicer{appts: map[string]*healthdomain.HealthAppointment{"appt-1": appt}}
	profileSvc := &mockHealthCalendarIDStorer{calendarID: "cal-1"}
	calendar := &mockCalendarPort{}
	telegram := &mockTelegramPort{}

	uc := NewHealthAppointmentUseCase(apptSvc, profileSvc, calendar, telegram, "America/Sao_Paulo", zap.NewNop())
	ctx := context.Background()

	err := uc.Delete(ctx, "appt-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify calendar event was deleted
	if _, ok := calendar.createdEvents["evt-123"]; ok {
		t.Fatal("expected event to be deleted from calendar")
	}

	// Verify appointment was deleted from service
	_, err = apptSvc.GetByID(ctx, "appt-1")
	if err != healthdomain.ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestHealthAppointmentUseCaseDeleteNotFound(t *testing.T) {
	apptSvc := &mockHealthAppointmentServicer{appts: make(map[string]*healthdomain.HealthAppointment)}
	profileSvc := &mockHealthCalendarIDStorer{calendarID: "cal-1"}
	calendar := &mockCalendarPort{}
	telegram := &mockTelegramPort{}

	uc := NewHealthAppointmentUseCase(apptSvc, profileSvc, calendar, telegram, "America/Sao_Paulo", zap.NewNop())
	ctx := context.Background()

	err := uc.Delete(ctx, "nonexistent")
	if err != healthdomain.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestHealthAppointmentUseCaseCreateValidationAndCalendarFailures(t *testing.T) {
	uc := NewHealthAppointmentUseCase(
		&mockHealthAppointmentServicer{appts: make(map[string]*healthdomain.HealthAppointment)},
		&mockHealthCalendarIDStorer{},
		&mockCalendarPort{},
		&mockTelegramPort{},
		"America/Sao_Paulo",
		zap.NewNop(),
	)

	if _, err := uc.Create(context.Background(), "", time.Now(), nil, nil); !errors.Is(err, healthdomain.ErrValidation) {
		t.Fatalf("empty specialty err = %v, want validation error", err)
	}

	calendar := &mockCalendarPort{createCalErr: errors.New("calendar unavailable")}
	uc = NewHealthAppointmentUseCase(
		&mockHealthAppointmentServicer{appts: make(map[string]*healthdomain.HealthAppointment)},
		&mockHealthCalendarIDStorer{},
		calendar,
		&mockTelegramPort{},
		"America/Sao_Paulo",
		zap.NewNop(),
	)
	if _, err := uc.Create(context.Background(), "Cardiologia", time.Now(), nil, nil); err == nil {
		t.Fatal("expected calendar provisioning error")
	}

	calendar = &mockCalendarPort{createEventErr: errors.New("event unavailable")}
	uc = NewHealthAppointmentUseCase(
		&mockHealthAppointmentServicer{appts: make(map[string]*healthdomain.HealthAppointment)},
		&mockHealthCalendarIDStorer{calendarID: "cal-existing"},
		calendar,
		&mockTelegramPort{},
		"America/Sao_Paulo",
		zap.NewNop(),
	)
	if _, err := uc.Create(context.Background(), "Cardiologia", time.Now(), nil, nil); err == nil {
		t.Fatal("expected calendar create event error")
	}
}

func TestHealthAppointmentUseCaseCreateCompensatesAndSwallowsTelegramErrors(t *testing.T) {
	apptSvc := &mockHealthAppointmentServicer{err: errors.New("db unavailable")}
	calendar := &mockCalendarPort{}
	telegram := &mockTelegramPort{err: errors.New("telegram unavailable")}
	uc := NewHealthAppointmentUseCase(
		apptSvc,
		&mockHealthCalendarIDStorer{calendarID: "cal-existing"},
		calendar,
		telegram,
		"America/Sao_Paulo",
		zap.NewNop(),
	)

	if _, err := uc.Create(context.Background(), "Cardiologia", time.Now(), nil, nil); err == nil {
		t.Fatal("expected service create error")
	}
	if len(calendar.deletedEvents) != 1 {
		t.Fatalf("deleted events = %d, want 1 compensation", len(calendar.deletedEvents))
	}

	apptSvc = &mockHealthAppointmentServicer{appts: make(map[string]*healthdomain.HealthAppointment)}
	calendar = &mockCalendarPort{}
	uc = NewHealthAppointmentUseCase(
		apptSvc,
		&mockHealthCalendarIDStorer{calendarID: "cal-existing"},
		calendar,
		telegram,
		"America/Sao_Paulo",
		zap.NewNop(),
	)
	if _, err := uc.Create(context.Background(), "Cardiologia", time.Now(), nil, nil); err != nil {
		t.Fatalf("unexpected telegram-swallow create err: %v", err)
	}
}

func TestHealthAppointmentUseCaseDeleteBranches(t *testing.T) {
	apptSvc := &mockHealthAppointmentServicer{appts: map[string]*healthdomain.HealthAppointment{
		"appt-1": {
			ID:                    "appt-1",
			Specialty:             "Oftalmologia",
			GoogleCalendarEventID: "",
			CreatedAt:             time.Now(),
		},
	}}
	calendar := &mockCalendarPort{}
	uc := NewHealthAppointmentUseCase(
		apptSvc,
		&mockHealthCalendarIDStorer{calendarID: "cal-existing"},
		calendar,
		&mockTelegramPort{err: errors.New("telegram unavailable")},
		"America/Sao_Paulo",
		zap.NewNop(),
	)

	if err := uc.Delete(context.Background(), "appt-1"); err != nil {
		t.Fatalf("delete without event id: %v", err)
	}
	if len(calendar.deletedEvents) != 0 {
		t.Fatalf("deleted events = %d, want 0 when event id is empty", len(calendar.deletedEvents))
	}

	apptSvc = &mockHealthAppointmentServicer{appts: map[string]*healthdomain.HealthAppointment{
		"appt-2": {
			ID:                    "appt-2",
			Specialty:             "Dermatologia",
			GoogleCalendarEventID: "evt-2",
			CreatedAt:             time.Now(),
		},
	}}
	calendar = &mockCalendarPort{deleteEventErr: errors.New("delete failed")}
	uc = NewHealthAppointmentUseCase(
		apptSvc,
		&mockHealthCalendarIDStorer{calendarID: "cal-existing"},
		calendar,
		&mockTelegramPort{},
		"America/Sao_Paulo",
		zap.NewNop(),
	)
	if err := uc.Delete(context.Background(), "appt-2"); err == nil {
		t.Fatal("expected delete event error")
	}
}
