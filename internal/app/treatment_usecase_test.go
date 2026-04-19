package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/rafaelsoares/alfredo/internal/app"
	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
	"github.com/rafaelsoares/alfredo/internal/petcare/service"
)

func TestTreatmentUseCaseCreateFinitePersistsDosesWithCalendarEvents(t *testing.T) {
	pet := &domain.Pet{ID: "pet-1", Name: "Luna", GoogleCalendarID: "cal-1"}
	treatmentRepo := &treatmentRepoStub{}
	doseRepo := &doseRepoStub{}
	calendar := &calendarFake{createEventIDs: []string{"evt-1", "evt-2", "evt-3"}}
	telegramRecorder := &telegramFake{err: errors.New("telegram down")}
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
		telegramRecorder,
		"America/Sao_Paulo",
		zap.NewNop(),
	)

	startedAt := time.Date(2026, 4, 17, 8, 0, 0, 0, time.UTC)
	endedAt := startedAt.Add(24 * time.Hour)
	treatment, doses, err := uc.Create(context.Background(), service.CreateTreatmentInput{
		PetID:         pet.ID,
		Name:          "Antibiotico",
		DosageAmount:  2.5,
		DosageUnit:    "ml",
		Route:         "oral",
		IntervalHours: 12,
		StartedAt:     startedAt,
		EndedAt:       &endedAt,
	})
	if err != nil {
		t.Fatalf("Create = %v", err)
	}
	if treatment == nil {
		t.Fatal("expected persisted treatment")
	}
	if len(doses) != 2 {
		t.Fatalf("doses = %d, want 2", len(doses))
	}
	if len(doseRepo.created) != 2 {
		t.Fatalf("persisted doses = %d, want 2", len(doseRepo.created))
	}
	if len(calendar.createdEvents) != 2 {
		t.Fatalf("calendar events = %d, want 2", len(calendar.createdEvents))
	}
	for i, dose := range doseRepo.created {
		if dose.GoogleCalendarEventID == "" {
			t.Fatalf("dose %d missing google calendar event id", i)
		}
	}
	if len(telegramRecorder.messages) != 1 {
		t.Fatalf("telegram attempts = %d, want 1 even when sending fails", len(telegramRecorder.messages))
	}
}

func TestTreatmentUseCaseRejectsPetWithoutCalendarBeforeCreatingTreatment(t *testing.T) {
	pet := &domain.Pet{ID: "pet-1", Name: "Luna"}
	treatmentRepo := &treatmentRepoStub{}
	uc := app.NewTreatmentUseCase(
		&treatmentServiceStub{repo: treatmentRepo},
		&doseServiceStub{repo: &doseRepoStub{}},
		&fakePetGetterWithCalendar{pet: pet},
		serviceTxRunner{
			petRepo:       &petRepoStub{pet: pet},
			vaccineRepo:   &vaccineRepoStub{},
			treatmentRepo: treatmentRepo,
			doseRepo:      &doseRepoStub{},
		},
		&calendarFake{},
		&telegramFake{},
		"America/Sao_Paulo",
		zap.NewNop(),
	)

	_, _, err := uc.Create(context.Background(), service.CreateTreatmentInput{
		PetID:         pet.ID,
		Name:          "Antibiotico",
		DosageAmount:  2.5,
		DosageUnit:    "ml",
		Route:         "oral",
		IntervalHours: 12,
		StartedAt:     time.Now(),
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if treatmentRepo.last != nil {
		t.Fatalf("created treatment = %#v, want none without calendar id", treatmentRepo.last)
	}
}

func TestTreatmentUseCaseReturnsPetLookupErrorBeforeCalendarWork(t *testing.T) {
	treatmentRepo := &treatmentRepoStub{}
	uc := app.NewTreatmentUseCase(
		&treatmentServiceStub{repo: treatmentRepo},
		&doseServiceStub{repo: &doseRepoStub{}},
		petGetterErr{err: domain.ErrNotFound},
		serviceTxRunner{
			petRepo:       &petRepoStub{},
			vaccineRepo:   &vaccineRepoStub{},
			treatmentRepo: treatmentRepo,
			doseRepo:      &doseRepoStub{},
		},
		&calendarFake{createRecurringID: "series-1"},
		&telegramFake{},
		"America/Sao_Paulo",
		zap.NewNop(),
	)

	_, _, err := uc.Create(context.Background(), service.CreateTreatmentInput{
		PetID:         "missing",
		Name:          "Antibiotico",
		DosageAmount:  2.5,
		DosageUnit:    "ml",
		Route:         "oral",
		IntervalHours: 12,
		StartedAt:     time.Now(),
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if treatmentRepo.last != nil {
		t.Fatalf("created treatment = %#v, want none when pet lookup fails", treatmentRepo.last)
	}
}

func TestTreatmentUseCaseCreateRecurringStoresSeriesWithoutTelegramAdapter(t *testing.T) {
	pet := &domain.Pet{ID: "pet-1", Name: "Luna", GoogleCalendarID: "cal-1"}
	treatmentRepo := &treatmentRepoStub{}
	calendar := &calendarFake{createRecurringID: "series-1"}
	uc := app.NewTreatmentUseCase(
		&treatmentServiceStub{repo: treatmentRepo},
		&doseServiceStub{repo: &doseRepoStub{}},
		&fakePetGetterWithCalendar{pet: pet},
		serviceTxRunner{
			petRepo:       &petRepoStub{pet: pet},
			vaccineRepo:   &vaccineRepoStub{},
			treatmentRepo: treatmentRepo,
			doseRepo:      &doseRepoStub{},
		},
		calendar,
		nil,
		"America/Sao_Paulo",
		nil,
	)

	treatment, doses, err := uc.Create(context.Background(), service.CreateTreatmentInput{
		PetID:         pet.ID,
		Name:          "Antibiotico",
		DosageAmount:  2.5,
		DosageUnit:    "ml",
		Route:         "oral",
		IntervalHours: 12,
		StartedAt:     time.Now(),
	})
	if err != nil {
		t.Fatalf("Create = %v", err)
	}
	if treatment.GoogleCalendarEventID != "series-1" {
		t.Fatalf("calendar event id = %q, want series-1", treatment.GoogleCalendarEventID)
	}
	if len(doses) != 0 {
		t.Fatalf("doses = %#v, want none for recurring treatment", doses)
	}
}

func TestTreatmentUseCaseListAndGetReturnDoseDetails(t *testing.T) {
	treatment := domain.Treatment{ID: "tr-1", PetID: "pet-1", Name: "Antibiotico"}
	dose := domain.Dose{ID: "dose-1", TreatmentID: treatment.ID, PetID: treatment.PetID}
	treatmentRepo := &treatmentRepoStub{last: &treatment}
	doseRepo := &doseRepoStub{created: []domain.Dose{dose}}
	pet := &domain.Pet{ID: "pet-1", GoogleCalendarID: "cal-1"}
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
		&calendarFake{},
		&telegramFake{},
		"America/Sao_Paulo",
		zap.NewNop(),
	)

	listed, doseMap, err := uc.List(context.Background(), pet.ID)
	if err != nil {
		t.Fatalf("List = %v", err)
	}
	if len(listed) != 1 || len(doseMap[treatment.ID]) != 1 {
		t.Fatalf("listed = %#v doseMap = %#v, want one treatment with one dose", listed, doseMap)
	}
	got, doses, err := uc.GetByID(context.Background(), pet.ID, treatment.ID)
	if err != nil {
		t.Fatalf("GetByID = %v", err)
	}
	if got.ID != treatment.ID || len(doses) != 1 || doses[0].ID != dose.ID {
		t.Fatalf("got = %#v doses = %#v, want tr-1 with dose-1", got, doses)
	}
}

func TestTreatmentUseCaseStopRecurringStopsSeriesBeforeMarkingStopped(t *testing.T) {
	pet := &domain.Pet{ID: "pet-1", Name: "Luna", GoogleCalendarID: "cal-1"}
	treatmentRepo := &treatmentRepoStub{last: &domain.Treatment{
		ID:                    "tr-recurring",
		PetID:                 pet.ID,
		Name:                  "Antibiotico",
		StartedAt:             time.Date(2026, 4, 17, 8, 0, 0, 0, time.UTC),
		GoogleCalendarEventID: "series-1",
	}}
	doseRepo := &doseRepoStub{}
	calendar := &calendarFake{}
	telegramRecorder := &telegramFake{}
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
		telegramRecorder,
		"America/Sao_Paulo",
		zap.NewNop(),
	)

	err := uc.Stop(context.Background(), pet.ID, "tr-recurring")
	if err != nil {
		t.Fatalf("Stop = %v", err)
	}
	if len(calendar.stoppedRecurring) != 1 || calendar.stoppedRecurring[0] != "series-1" {
		t.Fatalf("stopped recurring events = %#v, want [series-1]", calendar.stoppedRecurring)
	}
	if len(telegramRecorder.messages) != 1 {
		t.Fatalf("telegram messages = %d, want 1", len(telegramRecorder.messages))
	}
}

func TestTreatmentUseCaseStopFiniteDeletesOnlyFutureDoseEvents(t *testing.T) {
	pet := &domain.Pet{ID: "pet-1", Name: "Luna", GoogleCalendarID: "cal-1"}
	endedAt := time.Date(2026, 4, 19, 8, 0, 0, 0, time.UTC)
	treatmentRepo := &treatmentRepoStub{last: &domain.Treatment{
		ID:        "tr-finite",
		PetID:     pet.ID,
		Name:      "Colirio",
		StartedAt: time.Date(2026, 4, 17, 8, 0, 0, 0, time.UTC),
		EndedAt:   &endedAt,
	}}
	doseRepo := &doseRepoStub{
		created: []domain.Dose{
			{ID: "dose-past", TreatmentID: "tr-finite"},
			{ID: "dose-future", TreatmentID: "tr-finite", GoogleCalendarEventID: "evt-future"},
		},
		future: []domain.Dose{
			{ID: "dose-future", TreatmentID: "tr-finite", GoogleCalendarEventID: "evt-future"},
		},
	}
	calendar := &calendarFake{}
	telegramRecorder := &telegramFake{}
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
		telegramRecorder,
		"America/Sao_Paulo",
		zap.NewNop(),
	)

	err := uc.Stop(context.Background(), pet.ID, "tr-finite")
	if err != nil {
		t.Fatalf("Stop = %v", err)
	}
	if len(calendar.deletedEvents) != 1 || calendar.deletedEvents[0] != "evt-future" {
		t.Fatalf("deleted calendar events = %#v, want [evt-future]", calendar.deletedEvents)
	}
	if len(doseRepo.deletedFuture) != 1 || doseRepo.deletedFuture[0] != "tr-finite" {
		t.Fatalf("deleted future doses = %#v, want [tr-finite]", doseRepo.deletedFuture)
	}
	if len(telegramRecorder.messages) != 1 {
		t.Fatalf("telegram messages = %d, want 1", len(telegramRecorder.messages))
	}
}
