package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/rafaelsoares/alfredo/internal/app"
	"github.com/rafaelsoares/alfredo/internal/gcalendar"
	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
	"github.com/rafaelsoares/alfredo/internal/petcare/service"
)

// appointmentServiceFake is a test fake implementing AppointmentServicer.
type appointmentServiceFake struct {
	created   *domain.Appointment
	deleted   bool
	updated   *domain.Appointment
	createErr error
	getErr    error
	updateErr error
	deleteErr error
	stored    *domain.Appointment // pre-loaded for GetByID/Delete
}

func (f *appointmentServiceFake) Create(_ context.Context, in service.CreateAppointmentInput) (*domain.Appointment, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	a := &domain.Appointment{
		ID:                    "appt-1",
		PetID:                 in.PetID,
		Type:                  in.Type,
		ScheduledAt:           in.ScheduledAt,
		Provider:              in.Provider,
		Location:              in.Location,
		Notes:                 in.Notes,
		GoogleCalendarEventID: in.GoogleCalendarEventID,
		CreatedAt:             time.Now().UTC(),
	}
	f.created = a
	return a, nil
}

func (f *appointmentServiceFake) GetByID(_ context.Context, _, _ string) (*domain.Appointment, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.stored, nil
}

func (f *appointmentServiceFake) List(_ context.Context, _ string) ([]domain.Appointment, error) {
	if f.stored == nil {
		return []domain.Appointment{}, nil
	}
	return []domain.Appointment{*f.stored}, nil
}

func (f *appointmentServiceFake) Update(_ context.Context, _, _ string, in service.UpdateAppointmentInput) (*domain.Appointment, error) {
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	if f.stored == nil {
		return nil, domain.ErrNotFound
	}
	updated := *f.stored
	if in.ScheduledAt != nil {
		updated.ScheduledAt = *in.ScheduledAt
	}
	if in.Provider != nil {
		updated.Provider = in.Provider
	}
	if in.Location != nil {
		updated.Location = in.Location
	}
	if in.Notes != nil {
		updated.Notes = in.Notes
	}
	f.updated = &updated
	return &updated, nil
}

func (f *appointmentServiceFake) Delete(_ context.Context, _, _ string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deleted = true
	return nil
}

func newTestAppointmentUC(apptSvc *appointmentServiceFake, cal *calendarFake, tg *telegramFake, pet *domain.Pet) *app.AppointmentUseCase {
	return app.NewAppointmentUseCase(
		apptSvc,
		&fakePetGetterWithCalendar{pet: pet},
		cal,
		tg,
		"America/Sao_Paulo",
		zap.NewNop(),
	)
}

func baseAppt() *domain.Appointment {
	return &domain.Appointment{
		ID:                    "appt-1",
		PetID:                 "p1",
		Type:                  domain.AppointmentTypeVet,
		ScheduledAt:           time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC),
		GoogleCalendarEventID: "evt-1",
		CreatedAt:             time.Now().UTC(),
	}
}

func basePet() *domain.Pet {
	return &domain.Pet{ID: "p1", Name: "Luna", GoogleCalendarID: "cal-1"}
}

func baseCreateInput() service.CreateAppointmentInput {
	return service.CreateAppointmentInput{
		PetID:       "p1",
		Type:        domain.AppointmentTypeVet,
		ScheduledAt: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC),
	}
}

// 1. Create success: calendar created, DB written, Telegram fired.
func TestAppointmentUseCase_Create_success(t *testing.T) {
	pet := basePet()
	cal := &calendarFake{createEventIDs: []string{"evt-1"}}
	tg := &telegramFake{}
	apptSvc := &appointmentServiceFake{}
	uc := newTestAppointmentUC(apptSvc, cal, tg, pet)

	appt, err := uc.Create(context.Background(), baseCreateInput())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if appt.GoogleCalendarEventID != "evt-1" {
		t.Fatalf("expected event id evt-1, got %q", appt.GoogleCalendarEventID)
	}
	if apptSvc.created == nil {
		t.Fatal("expected appointment to be created in DB")
	}
	if len(tg.messages) != 1 {
		t.Fatalf("expected 1 telegram message, got %d", len(tg.messages))
	}
}

// 2. Create calendar failure: CreateEvent fails → use case returns error, Create not called.
func TestAppointmentUseCase_Create_calendarFailure(t *testing.T) {
	pet := basePet()
	calErr := errors.New("calendar unavailable")
	// calendarFake with no createEventIDs will panic if called, so we wrap it
	cal := &failingCalendarFake{createEventErr: calErr}
	tg := &telegramFake{}
	apptSvc := &appointmentServiceFake{}
	uc := app.NewAppointmentUseCase(apptSvc, &fakePetGetterWithCalendar{pet: pet}, cal, tg, "America/Sao_Paulo", zap.NewNop())

	_, err := uc.Create(context.Background(), baseCreateInput())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if apptSvc.created != nil {
		t.Fatal("expected Create not to be called when calendar fails")
	}
}

// 3. Create DB failure: CreateEvent succeeds, appointments.Create fails → DeleteEvent compensation called.
func TestAppointmentUseCase_Create_dbFailure(t *testing.T) {
	pet := basePet()
	cal := &calendarFake{createEventIDs: []string{"evt-1"}}
	tg := &telegramFake{}
	apptSvc := &appointmentServiceFake{createErr: errors.New("db error")}
	uc := newTestAppointmentUC(apptSvc, cal, tg, pet)

	_, err := uc.Create(context.Background(), baseCreateInput())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if len(cal.deletedEvents) != 1 || cal.deletedEvents[0] != "evt-1" {
		t.Fatalf("expected compensation delete of evt-1, got %#v", cal.deletedEvents)
	}
	if len(tg.messages) != 0 {
		t.Fatalf("expected no telegram message on db error, got %d", len(tg.messages))
	}
}

// 4. Create compensation failure: DB fails AND DeleteEvent fails → original error returned, compensation failure logged.
func TestAppointmentUseCase_Create_compensationFailure(t *testing.T) {
	pet := basePet()
	cal := &failCompensationCalendarFake{createEventID: "evt-1", deleteEventErr: errors.New("delete failed")}
	tg := &telegramFake{}
	dbErr := errors.New("db error")
	apptSvc := &appointmentServiceFake{createErr: dbErr}
	uc := app.NewAppointmentUseCase(apptSvc, &fakePetGetterWithCalendar{pet: pet}, cal, tg, "America/Sao_Paulo", zap.NewNop())

	_, err := uc.Create(context.Background(), baseCreateInput())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, dbErr) {
		t.Fatalf("expected original db error, got %v", err)
	}
}

// 5. Delete success: calendar deleted, DB deleted, Telegram fired.
func TestAppointmentUseCase_Delete_success(t *testing.T) {
	pet := basePet()
	cal := &calendarFake{}
	tg := &telegramFake{}
	apptSvc := &appointmentServiceFake{stored: baseAppt()}
	uc := newTestAppointmentUC(apptSvc, cal, tg, pet)

	err := uc.Delete(context.Background(), "p1", "appt-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cal.deletedEvents) != 1 || cal.deletedEvents[0] != "evt-1" {
		t.Fatalf("expected evt-1 deleted from calendar, got %#v", cal.deletedEvents)
	}
	if !apptSvc.deleted {
		t.Fatal("expected appointment deleted from DB")
	}
	if len(tg.messages) != 1 {
		t.Fatalf("expected 1 telegram message, got %d", len(tg.messages))
	}
}

// 6. Delete calendar failure: DeleteEvent fails → error returned, Delete not called.
func TestAppointmentUseCase_Delete_calendarFailure(t *testing.T) {
	pet := basePet()
	cal := &failingCalendarFake{deleteEventErr: errors.New("calendar delete failed")}
	tg := &telegramFake{}
	apptSvc := &appointmentServiceFake{stored: baseAppt()}
	uc := app.NewAppointmentUseCase(apptSvc, &fakePetGetterWithCalendar{pet: pet}, cal, tg, "America/Sao_Paulo", zap.NewNop())

	err := uc.Delete(context.Background(), "p1", "appt-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if apptSvc.deleted {
		t.Fatal("expected Delete not called when calendar delete fails")
	}
}

// 7. Delete DB failure after calendar delete: calendar deleted, DB delete fails → divergence logged, error returned.
func TestAppointmentUseCase_Delete_dbFailureAfterCalendarDelete(t *testing.T) {
	pet := basePet()
	cal := &calendarFake{}
	tg := &telegramFake{}
	dbErr := errors.New("db delete failed")
	apptSvc := &appointmentServiceFake{stored: baseAppt(), deleteErr: dbErr}
	uc := newTestAppointmentUC(apptSvc, cal, tg, pet)

	err := uc.Delete(context.Background(), "p1", "appt-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, dbErr) {
		t.Fatalf("expected db error, got %v", err)
	}
	// calendar delete should still have happened
	if len(cal.deletedEvents) != 1 {
		t.Fatalf("expected 1 calendar delete, got %d", len(cal.deletedEvents))
	}
	if len(tg.messages) != 0 {
		t.Fatalf("expected no telegram message on error, got %d", len(tg.messages))
	}
}

// 8. Update success: DB updated, no calendar or Telegram call.
func TestAppointmentUseCase_Update_success(t *testing.T) {
	pet := basePet()
	cal := &calendarFake{}
	tg := &telegramFake{}
	apptSvc := &appointmentServiceFake{stored: baseAppt()}
	uc := newTestAppointmentUC(apptSvc, cal, tg, pet)

	newTime := time.Date(2026, 5, 2, 11, 0, 0, 0, time.UTC)
	appt, err := uc.Update(context.Background(), "p1", "appt-1", service.UpdateAppointmentInput{
		ScheduledAt: &newTime,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !appt.ScheduledAt.Equal(newTime) {
		t.Fatalf("expected updated ScheduledAt %v, got %v", newTime, appt.ScheduledAt)
	}
	if cal.createEventCalls != 0 {
		t.Fatal("expected no calendar calls on Update")
	}
	if len(cal.updatedEvents) != 1 {
		t.Fatalf("expected 1 calendar update on reschedule, got %d", len(cal.updatedEvents))
	}
	if len(tg.messages) != 0 {
		t.Fatalf("expected no telegram messages on Update, got %d", len(tg.messages))
	}
}

func TestAppointmentUseCase_Update_notesOnlySkipsCalendar(t *testing.T) {
	pet := basePet()
	cal := &calendarFake{}
	tg := &telegramFake{}
	apptSvc := &appointmentServiceFake{stored: baseAppt()}
	uc := newTestAppointmentUC(apptSvc, cal, tg, pet)

	notes := "updated notes"
	_, err := uc.Update(context.Background(), "p1", "appt-1", service.UpdateAppointmentInput{
		Notes: &notes,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cal.updatedEvents) != 0 {
		t.Fatalf("expected no calendar update for notes-only patch, got %d", len(cal.updatedEvents))
	}
}

func TestAppointmentUseCase_Update_calendarFailureSkipsDB(t *testing.T) {
	pet := basePet()
	cal := &failingCalendarFake{updateEventErr: errors.New("calendar update failed")}
	tg := &telegramFake{}
	apptSvc := &appointmentServiceFake{stored: baseAppt()}
	uc := app.NewAppointmentUseCase(apptSvc, &fakePetGetterWithCalendar{pet: pet}, cal, tg, "America/Sao_Paulo", zap.NewNop())

	newTime := time.Date(2026, 5, 2, 11, 0, 0, 0, time.UTC)
	errAppt, err := uc.Update(context.Background(), "p1", "appt-1", service.UpdateAppointmentInput{
		ScheduledAt: &newTime,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if errAppt != nil {
		t.Fatalf("expected nil appointment on error, got %#v", errAppt)
	}
	if apptSvc.updated != nil {
		t.Fatal("expected DB update to be skipped when calendar update fails")
	}
}

func TestAppointmentUseCase_Update_dbFailureCompensatesCalendar(t *testing.T) {
	pet := basePet()
	cal := &calendarFake{}
	tg := &telegramFake{}
	dbErr := errors.New("db update failed")
	apptSvc := &appointmentServiceFake{stored: baseAppt(), updateErr: dbErr}
	uc := newTestAppointmentUC(apptSvc, cal, tg, pet)

	newTime := time.Date(2026, 5, 2, 11, 0, 0, 0, time.UTC)
	_, err := uc.Update(context.Background(), "p1", "appt-1", service.UpdateAppointmentInput{
		ScheduledAt: &newTime,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, dbErr) {
		t.Fatalf("expected db error, got %v", err)
	}
	if len(cal.updatedEvents) != 2 {
		t.Fatalf("expected calendar update and compensation, got %d updates", len(cal.updatedEvents))
	}
	if !cal.updatedEvents[1].StartTime.Equal(baseAppt().ScheduledAt) {
		t.Fatalf("expected compensation to restore original time, got %v", cal.updatedEvents[1].StartTime)
	}
}

// 9. GetByID not found: returns domain.ErrNotFound.
func TestAppointmentUseCase_GetByID_notFound(t *testing.T) {
	pet := basePet()
	cal := &calendarFake{}
	tg := &telegramFake{}
	apptSvc := &appointmentServiceFake{getErr: domain.ErrNotFound}
	uc := newTestAppointmentUC(apptSvc, cal, tg, pet)

	_, err := uc.GetByID(context.Background(), "p1", "appt-missing")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected domain.ErrNotFound, got %v", err)
	}
}

// 10. List empty: returns empty slice, no error.
func TestAppointmentUseCase_List_empty(t *testing.T) {
	pet := basePet()
	cal := &calendarFake{}
	tg := &telegramFake{}
	apptSvc := &appointmentServiceFake{} // stored is nil → returns empty slice
	uc := newTestAppointmentUC(apptSvc, cal, tg, pet)

	appts, err := uc.List(context.Background(), "p1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(appts) != 0 {
		t.Fatalf("expected empty slice, got %d appointments", len(appts))
	}
}

// --- helper fakes for error scenarios ---

// failingCalendarFake returns errors for CreateEvent or DeleteEvent.
type failingCalendarFake struct {
	createEventErr error
	updateEventErr error
	deleteEventErr error
}

func (f *failingCalendarFake) CreateCalendar(context.Context, string) (string, error) {
	return "", nil
}
func (f *failingCalendarFake) DeleteCalendar(context.Context, string) error { return nil }
func (f *failingCalendarFake) CreateEvent(_ context.Context, _ string, _ gcalendar.Event) (string, error) {
	return "", f.createEventErr
}
func (f *failingCalendarFake) UpdateEvent(_ context.Context, _, _ string, _ gcalendar.Event) error {
	return f.updateEventErr
}
func (f *failingCalendarFake) DeleteEvent(_ context.Context, _, _ string) error {
	return f.deleteEventErr
}
func (f *failingCalendarFake) CreateRecurringEvent(_ context.Context, _ string, _ gcalendar.Event, _ int) (string, error) {
	return "", nil
}
func (f *failingCalendarFake) StopRecurringEvent(_ context.Context, _, _ string, _ time.Time) error {
	return nil
}

// failCompensationCalendarFake: CreateEvent succeeds, DeleteEvent fails.
type failCompensationCalendarFake struct {
	createEventID  string
	deleteEventErr error
}

func (f *failCompensationCalendarFake) CreateCalendar(context.Context, string) (string, error) {
	return "", nil
}
func (f *failCompensationCalendarFake) DeleteCalendar(context.Context, string) error { return nil }
func (f *failCompensationCalendarFake) CreateEvent(_ context.Context, _ string, _ gcalendar.Event) (string, error) {
	return f.createEventID, nil
}
func (f *failCompensationCalendarFake) UpdateEvent(_ context.Context, _, _ string, _ gcalendar.Event) error {
	return nil
}
func (f *failCompensationCalendarFake) DeleteEvent(_ context.Context, _, _ string) error {
	return f.deleteEventErr
}
func (f *failCompensationCalendarFake) CreateRecurringEvent(_ context.Context, _ string, _ gcalendar.Event, _ int) (string, error) {
	return "", nil
}
func (f *failCompensationCalendarFake) StopRecurringEvent(_ context.Context, _, _ string, _ time.Time) error {
	return nil
}
