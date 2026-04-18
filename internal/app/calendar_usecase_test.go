package app_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/rafaelsoares/alfredo/internal/app"
	"github.com/rafaelsoares/alfredo/internal/gcalendar"
	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
	"github.com/rafaelsoares/alfredo/internal/petcare/port"
	"github.com/rafaelsoares/alfredo/internal/petcare/service"
	"github.com/rafaelsoares/alfredo/internal/telegram"
)

type failTxRunner struct{ err error }

func (r failTxRunner) WithinTx(_ context.Context, _ func(*service.PetService, *service.VaccineService, *service.TreatmentService, *service.DoseService) error) error {
	return r.err
}

type serviceTxRunner struct {
	petRepo       port.PetRepository
	vaccineRepo   port.VaccineRepository
	treatmentRepo port.TreatmentRepository
	doseRepo      port.DoseRepository
}

func (r serviceTxRunner) WithinTx(ctx context.Context, fn func(*service.PetService, *service.VaccineService, *service.TreatmentService, *service.DoseService) error) error {
	return fn(
		service.NewPetService(r.petRepo),
		service.NewVaccineService(r.vaccineRepo, r.petRepo),
		service.NewTreatmentService(r.treatmentRepo),
		service.NewDoseService(r.doseRepo),
	)
}

type calendarFake struct {
	createCalendarID  string
	createEventIDs    []string
	createRecurringID string
	deletedCalendars  []string
	deletedEvents     []string
	createdEvents     []gcalendar.Event
	updatedEvents     []gcalendar.Event
	createEventCalls  int
}

func (c *calendarFake) CreateCalendar(context.Context, string) (string, error) {
	return c.createCalendarID, nil
}
func (c *calendarFake) DeleteCalendar(_ context.Context, calendarID string) error {
	c.deletedCalendars = append(c.deletedCalendars, calendarID)
	return nil
}
func (c *calendarFake) CreateEvent(_ context.Context, _ string, event gcalendar.Event) (string, error) {
	c.createdEvents = append(c.createdEvents, event)
	if c.createEventCalls >= len(c.createEventIDs) {
		return "", fmt.Errorf("calendarFake: unexpected CreateEvent call %d", c.createEventCalls)
	}
	id := c.createEventIDs[c.createEventCalls]
	c.createEventCalls++
	return id, nil
}
func (c *calendarFake) UpdateEvent(_ context.Context, _, _ string, event gcalendar.Event) error {
	c.updatedEvents = append(c.updatedEvents, event)
	return nil
}
func (c *calendarFake) CreateRecurringEvent(context.Context, string, gcalendar.Event, int) (string, error) {
	return c.createRecurringID, nil
}
func (c *calendarFake) StopRecurringEvent(context.Context, string, string, time.Time) error {
	return nil
}
func (c *calendarFake) DeleteEvent(_ context.Context, _, eventID string) error {
	c.deletedEvents = append(c.deletedEvents, eventID)
	return nil
}

type telegramFake struct {
	messages []telegram.Message
	err      error
}

func (t *telegramFake) Send(_ context.Context, msg telegram.Message) error {
	t.messages = append(t.messages, msg)
	return t.err
}

type petRepoStub struct {
	pet       *domain.Pet
	createErr error
}

func (r *petRepoStub) List(context.Context) ([]domain.Pet, error) { return nil, nil }
func (r *petRepoStub) Create(context.Context, domain.Pet) (*domain.Pet, error) {
	return nil, r.createErr
}
func (r *petRepoStub) GetByID(context.Context, string) (*domain.Pet, error)    { return r.pet, nil }
func (r *petRepoStub) Update(context.Context, domain.Pet) (*domain.Pet, error) { return nil, nil }
func (r *petRepoStub) Delete(context.Context, string) error                    { return nil }

type vaccineRepoStub struct {
	createErr error
	last      *domain.Vaccine
}

func (r *vaccineRepoStub) ListVaccines(context.Context, string) ([]domain.Vaccine, error) {
	return nil, nil
}
func (r *vaccineRepoStub) CreateVaccine(_ context.Context, v domain.Vaccine) (*domain.Vaccine, error) {
	if r.createErr != nil {
		return nil, r.createErr
	}
	vc := v
	r.last = &vc
	return &vc, nil
}
func (r *vaccineRepoStub) GetVaccine(context.Context, string, string) (*domain.Vaccine, error) {
	return r.last, nil
}
func (r *vaccineRepoStub) DeleteVaccine(context.Context, string, string) error { return nil }

type treatmentRepoStub struct {
	last *domain.Treatment
}

func (r *treatmentRepoStub) Create(_ context.Context, t domain.Treatment) (*domain.Treatment, error) {
	tc := t
	r.last = &tc
	return &tc, nil
}
func (r *treatmentRepoStub) GetByID(context.Context, string, string) (*domain.Treatment, error) {
	return r.last, nil
}
func (r *treatmentRepoStub) List(context.Context, string) ([]domain.Treatment, error) {
	if r.last == nil {
		return nil, nil
	}
	return []domain.Treatment{*r.last}, nil
}
func (r *treatmentRepoStub) Stop(context.Context, string, time.Time) error { return nil }

type doseRepoStub struct {
	createBatchErr error
	created        []domain.Dose
}

func (r *doseRepoStub) CreateBatch(_ context.Context, doses []domain.Dose) error {
	r.created = append(r.created, doses...)
	return r.createBatchErr
}
func (r *doseRepoStub) ListByTreatment(context.Context, string) ([]domain.Dose, error) {
	return r.created, nil
}
func (r *doseRepoStub) ListFutureByTreatment(context.Context, string, time.Time) ([]domain.Dose, error) {
	return nil, nil
}
func (r *doseRepoStub) DeleteFutureByTreatment(context.Context, string, time.Time) error { return nil }

func TestPetUseCaseCreateCompensatesCalendarOnTxError(t *testing.T) {
	calendar := &calendarFake{createCalendarID: "cal-1"}
	uc := app.NewPetUseCase(&stubPetService{}, failTxRunner{err: errors.New("db failed")}, calendar, zap.NewNop())

	_, err := uc.Create(context.Background(), service.CreatePetInput{Name: "Luna", Species: "dog"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if len(calendar.deletedCalendars) != 1 || calendar.deletedCalendars[0] != "cal-1" {
		t.Fatalf("unexpected calendar compensation: %#v", calendar.deletedCalendars)
	}
}

func TestVaccineUseCaseCreateCompensatesEventOnTxError(t *testing.T) {
	pet := &domain.Pet{ID: "p1", Name: "Luna", GoogleCalendarID: "cal-1"}
	calendar := &calendarFake{createEventIDs: []string{"evt-1"}}
	telegramRecorder := &telegramFake{}
	uc := app.NewVaccineUseCase(&stubVaccineService{}, &fakePetGetterWithCalendar{pet: pet}, failTxRunner{err: errors.New("db failed")}, calendar, telegramRecorder, "America/Sao_Paulo", zap.NewNop())

	_, err := uc.RecordVaccine(context.Background(), service.RecordVaccineInput{
		PetID:          "p1",
		Name:           "Rabies",
		AdministeredAt: time.Now(),
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if len(calendar.deletedEvents) != 1 || calendar.deletedEvents[0] != "evt-1" {
		t.Fatalf("unexpected event compensation: %#v", calendar.deletedEvents)
	}
	if len(telegramRecorder.messages) != 0 {
		t.Fatalf("expected no telegram message on tx error, got %#v", telegramRecorder.messages)
	}
}

func TestVaccineUseCaseRecordsEventAtAdministeredTimeWhenRecurring(t *testing.T) {
	pet := &domain.Pet{ID: "p1", Name: "Luna", GoogleCalendarID: "cal-1"}
	calendar := &calendarFake{createEventIDs: []string{"evt-admin", "evt-next"}}
	vaccineRepo := &vaccineRepoStub{}
	uc := app.NewVaccineUseCase(
		&stubVaccineService{},
		&fakePetGetterWithCalendar{pet: pet},
		serviceTxRunner{
			petRepo:       &petRepoStub{pet: pet},
			vaccineRepo:   vaccineRepo,
			treatmentRepo: &treatmentRepoStub{},
			doseRepo:      &doseRepoStub{},
		},
		calendar,
		&telegramFake{},
		"America/Sao_Paulo",
		zap.NewNop(),
	)

	administeredAt := time.Date(2026, 3, 27, 9, 0, 0, 0, time.FixedZone("BRT", -3*60*60))
	recurrenceDays := 365
	vaccine, err := uc.RecordVaccine(context.Background(), service.RecordVaccineInput{
		PetID:          "p1",
		Name:           "Rabies",
		AdministeredAt: administeredAt,
		RecurrenceDays: &recurrenceDays,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(calendar.createdEvents) != 2 {
		t.Fatalf("expected 2 calendar events, got %d", len(calendar.createdEvents))
	}
	if !calendar.createdEvents[0].StartTime.Equal(administeredAt) {
		t.Fatalf("expected event at administered time %s, got %s", administeredAt, calendar.createdEvents[0].StartTime)
	}
	if vaccine.NextDueAt == nil {
		t.Fatal("expected next_due_at to be computed")
	}
	wantNextDue := administeredAt.AddDate(0, 0, recurrenceDays)
	if !vaccine.NextDueAt.Equal(wantNextDue) {
		t.Fatalf("expected next_due_at %s, got %s", wantNextDue, *vaccine.NextDueAt)
	}
	if !calendar.createdEvents[1].StartTime.Equal(wantNextDue) {
		t.Fatalf("expected next due event at %s, got %s", wantNextDue, calendar.createdEvents[1].StartTime)
	}
	if calendar.createdEvents[1].ReminderMin != 7*24*60 {
		t.Fatalf("expected next due reminder 10080 minutes, got %d", calendar.createdEvents[1].ReminderMin)
	}
	if vaccine.GoogleCalendarEventID != "evt-admin" {
		t.Fatalf("expected administered event id evt-admin, got %q", vaccine.GoogleCalendarEventID)
	}
	if vaccine.GoogleCalendarNextDueEventID != "evt-next" {
		t.Fatalf("expected next due event id evt-next, got %q", vaccine.GoogleCalendarNextDueEventID)
	}
}

func TestVaccineUseCaseDeleteRemovesAdministeredAndNextDueEvents(t *testing.T) {
	pet := &domain.Pet{ID: "p1", Name: "Luna", GoogleCalendarID: "cal-1"}
	vaccineRepo := &vaccineRepoStub{last: &domain.Vaccine{
		ID:                           "v1",
		PetID:                        "p1",
		Name:                         "Rabies",
		AdministeredAt:               time.Now(),
		GoogleCalendarEventID:        "evt-admin",
		GoogleCalendarNextDueEventID: "evt-next",
	}}
	calendar := &calendarFake{}
	uc := app.NewVaccineUseCase(
		&stubVaccineService{},
		&fakePetGetterWithCalendar{pet: pet},
		serviceTxRunner{
			petRepo:       &petRepoStub{pet: pet},
			vaccineRepo:   vaccineRepo,
			treatmentRepo: &treatmentRepoStub{},
			doseRepo:      &doseRepoStub{},
		},
		calendar,
		&telegramFake{},
		"America/Sao_Paulo",
		zap.NewNop(),
	)

	err := uc.DeleteVaccine(context.Background(), "p1", "v1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(calendar.deletedEvents) != 2 {
		t.Fatalf("expected 2 deleted events, got %#v", calendar.deletedEvents)
	}
	if calendar.deletedEvents[0] != "evt-admin" || calendar.deletedEvents[1] != "evt-next" {
		t.Fatalf("unexpected deleted events: %#v", calendar.deletedEvents)
	}
}

func TestTreatmentUseCaseCreateFiniteCompensatesCreatedEvents(t *testing.T) {
	pet := &domain.Pet{ID: "p1", Name: "Luna", GoogleCalendarID: "cal-1"}
	calendar := &calendarFake{createEventIDs: []string{"evt-1", "evt-2"}}
	treatmentRepo := &treatmentRepoStub{}
	doseRepo := &doseRepoStub{createBatchErr: errors.New("insert doses failed")}
	uc := app.NewTreatmentUseCase(
		&treatmentServiceStub{repo: treatmentRepo},
		&doseServiceStub{repo: doseRepo},
		&fakePetGetterWithCalendar{pet: pet},
		serviceTxRunner{
			petRepo:       &petRepoStub{pet: pet},
			vaccineRepo:   &vaccineRepoStub{},
			treatmentRepo: treatmentRepo,
			doseRepo:      doseRepo,
		},
		calendar,
		&telegramFake{},
		"America/Sao_Paulo",
		zap.NewNop(),
	)

	start := time.Date(2026, 4, 12, 8, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Hour)
	_, _, err := uc.Create(context.Background(), service.CreateTreatmentInput{
		PetID:         "p1",
		Name:          "Amoxicillin",
		DosageAmount:  1,
		DosageUnit:    "ml",
		Route:         "oral",
		IntervalHours: 1,
		StartedAt:     start,
		EndedAt:       &end,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if len(calendar.deletedEvents) != 2 {
		t.Fatalf("expected 2 compensated events, got %#v", calendar.deletedEvents)
	}
}

func TestTreatmentUseCaseCreateRecurringStoresSeriesIDWithoutDoses(t *testing.T) {
	pet := &domain.Pet{ID: "p1", Name: "Luna", GoogleCalendarID: "cal-1"}
	calendar := &calendarFake{createRecurringID: "series-1"}
	treatmentRepo := &treatmentRepoStub{}
	doseRepo := &doseRepoStub{}
	uc := app.NewTreatmentUseCase(
		&treatmentServiceStub{repo: treatmentRepo},
		&doseServiceStub{repo: doseRepo},
		&fakePetGetterWithCalendar{pet: pet},
		serviceTxRunner{
			petRepo:       &petRepoStub{pet: pet},
			vaccineRepo:   &vaccineRepoStub{},
			treatmentRepo: treatmentRepo,
			doseRepo:      doseRepo,
		},
		calendar,
		&telegramFake{},
		"America/Sao_Paulo",
		zap.NewNop(),
	)

	tr, doses, err := uc.Create(context.Background(), service.CreateTreatmentInput{
		PetID:         "p1",
		Name:          "Daily med",
		DosageAmount:  1,
		DosageUnit:    "ml",
		Route:         "oral",
		IntervalHours: 24,
		StartedAt:     time.Now(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tr.GoogleCalendarEventID != "series-1" {
		t.Fatalf("expected series id, got %q", tr.GoogleCalendarEventID)
	}
	if doses != nil {
		t.Fatalf("expected no doses, got %#v", doses)
	}
}

type fakePetGetterWithCalendar struct{ pet *domain.Pet }

func (f *fakePetGetterWithCalendar) GetByID(context.Context, string) (*domain.Pet, error) {
	return f.pet, nil
}

type stubPetService struct{}

func (s *stubPetService) List(context.Context) ([]domain.Pet, error) { return nil, nil }
func (s *stubPetService) Create(context.Context, service.CreatePetInput) (*domain.Pet, error) {
	return nil, nil
}
func (s *stubPetService) GetByID(context.Context, string) (*domain.Pet, error) { return nil, nil }
func (s *stubPetService) Update(context.Context, string, service.UpdatePetInput) (*domain.Pet, error) {
	return nil, nil
}
func (s *stubPetService) Delete(context.Context, string) error { return nil }

type stubVaccineService struct{}

func (s *stubVaccineService) ListVaccines(context.Context, string) ([]domain.Vaccine, error) {
	return nil, nil
}
func (s *stubVaccineService) RecordVaccine(context.Context, service.RecordVaccineInput) (*domain.Vaccine, error) {
	return nil, nil
}
func (s *stubVaccineService) GetVaccine(context.Context, string, string) (*domain.Vaccine, error) {
	return nil, nil
}
func (s *stubVaccineService) DeleteVaccine(context.Context, string, string) error { return nil }

type treatmentServiceStub struct{ repo port.TreatmentRepository }

func (s *treatmentServiceStub) Create(ctx context.Context, in service.CreateTreatmentInput) (*domain.Treatment, error) {
	return service.NewTreatmentService(s.repo).Create(ctx, in)
}
func (s *treatmentServiceStub) GetByID(ctx context.Context, petID, treatmentID string) (*domain.Treatment, error) {
	return service.NewTreatmentService(s.repo).GetByID(ctx, petID, treatmentID)
}
func (s *treatmentServiceStub) List(ctx context.Context, petID string) ([]domain.Treatment, error) {
	return service.NewTreatmentService(s.repo).List(ctx, petID)
}
func (s *treatmentServiceStub) Stop(ctx context.Context, petID, treatmentID string) error {
	return service.NewTreatmentService(s.repo).Stop(ctx, petID, treatmentID)
}

type doseServiceStub struct{ repo port.DoseRepository }

func (s *doseServiceStub) GenerateDoses(t domain.Treatment, upTo time.Time) []domain.Dose {
	return service.NewDoseService(s.repo).GenerateDoses(t, upTo)
}
func (s *doseServiceStub) CreateBatch(ctx context.Context, doses []domain.Dose) error {
	return service.NewDoseService(s.repo).CreateBatch(ctx, doses)
}
func (s *doseServiceStub) ListByTreatment(ctx context.Context, treatmentID string) ([]domain.Dose, error) {
	return service.NewDoseService(s.repo).ListByTreatment(ctx, treatmentID)
}
func (s *doseServiceStub) ListFutureByTreatment(ctx context.Context, treatmentID string, after time.Time) ([]domain.Dose, error) {
	return service.NewDoseService(s.repo).ListFutureByTreatment(ctx, treatmentID, after)
}
func (s *doseServiceStub) DeleteFutureByTreatment(ctx context.Context, treatmentID string, after time.Time) error {
	return service.NewDoseService(s.repo).DeleteFutureByTreatment(ctx, treatmentID, after)
}
