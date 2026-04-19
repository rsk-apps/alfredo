package app_test

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/rafaelsoares/alfredo/internal/app"
	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
	"github.com/rafaelsoares/alfredo/internal/petcare/service"
)

func TestVaccineUseCaseRecordWithoutRecurrenceCreatesOneEventAndNotifies(t *testing.T) {
	pet := &domain.Pet{ID: "pet-1", Name: "Luna", GoogleCalendarID: "cal-1"}
	vaccineRepo := &vaccineRepoStub{}
	calendar := &calendarFake{createEventIDs: []string{"evt-admin"}}
	telegramRecorder := &telegramFake{}
	uc := app.NewVaccineUseCase(
		service.NewVaccineService(vaccineRepo, &petRepoStub{pet: pet}),
		&fakePetGetterWithCalendar{pet: pet},
		serviceTxRunner{
			petRepo:       &petRepoStub{pet: pet},
			vaccineRepo:   vaccineRepo,
			treatmentRepo: &treatmentRepoStub{},
			doseRepo:      &doseRepoStub{},
		},
		calendar,
		telegramRecorder,
		"America/Sao_Paulo",
		zap.NewNop(),
	)

	administeredAt := time.Date(2026, 4, 17, 10, 0, 0, 0, time.FixedZone("BRT", -3*60*60))
	recorded, err := uc.RecordVaccine(context.Background(), service.RecordVaccineInput{
		PetID:          pet.ID,
		Name:           "V10",
		AdministeredAt: administeredAt,
	})
	if err != nil {
		t.Fatalf("RecordVaccine = %v", err)
	}
	if recorded.GoogleCalendarEventID != "evt-admin" {
		t.Fatalf("recorded event id = %q, want evt-admin", recorded.GoogleCalendarEventID)
	}
	if recorded.GoogleCalendarNextDueEventID != "" {
		t.Fatalf("next due event id = %q, want empty without recurrence", recorded.GoogleCalendarNextDueEventID)
	}
	if len(calendar.createdEvents) != 1 {
		t.Fatalf("calendar events = %d, want 1", len(calendar.createdEvents))
	}
	if !calendar.createdEvents[0].StartTime.Equal(administeredAt) {
		t.Fatalf("event start = %s, want %s", calendar.createdEvents[0].StartTime, administeredAt)
	}
	if len(telegramRecorder.messages) != 1 {
		t.Fatalf("telegram messages = %d, want 1", len(telegramRecorder.messages))
	}
}
