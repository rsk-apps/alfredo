package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
	"github.com/rafaelsoares/alfredo/internal/petcare/service"
)

func TestSummaryUseCaseAllPetsAppliesThresholds(t *testing.T) {
	now := time.Now().In(time.UTC)
	pet := domain.Pet{ID: "pet-1", Name: "Nutella", Species: "dog"}
	pets := &summaryPetService{pets: []domain.Pet{pet}}
	vaccines := &summaryVaccineService{byPet: map[string][]domain.Vaccine{
		pet.ID: {
			{ID: "vaccine-due", PetID: pet.ID, Name: "V10", NextDueAt: ptrTime(now.AddDate(0, 0, 25))},
			{ID: "vaccine-far", PetID: pet.ID, Name: "Raiva", NextDueAt: ptrTime(now.AddDate(0, 0, 35))},
			{ID: "vaccine-overdue", PetID: pet.ID, Name: "Giardia", NextDueAt: ptrTime(now.AddDate(0, 0, -3))},
		},
	}}
	treatments := &summaryTreatmentService{byPet: map[string][]domain.Treatment{
		pet.ID: {
			{ID: "ongoing", PetID: pet.ID, Name: "Antibiótico", StartedAt: now.Add(-time.Hour)},
			{ID: "finished", PetID: pet.ID, Name: "Colírio", StartedAt: now.AddDate(0, 0, -10), EndedAt: ptrTime(now.AddDate(0, 0, -1))},
			{ID: "stopped", PetID: pet.ID, Name: "Pomada", StartedAt: now.Add(-time.Hour), StoppedAt: ptrTime(now)},
		},
	}}
	appointments := &summaryAppointmentService{byPet: map[string][]domain.Appointment{
		pet.ID: {
			{ID: "appointment-soon", PetID: pet.ID, ScheduledAt: now.AddDate(0, 0, 14)},
			{ID: "appointment-far", PetID: pet.ID, ScheduledAt: now.AddDate(0, 0, 15)},
			{ID: "appointment-past", PetID: pet.ID, ScheduledAt: now.AddDate(0, 0, -1)},
		},
	}}
	observations := &summaryObservationService{byPet: map[string][]domain.Observation{
		pet.ID: {
			{ID: "observation-recent", PetID: pet.ID, ObservedAt: now.AddDate(0, 0, -7)},
			{ID: "observation-old", PetID: pet.ID, ObservedAt: now.AddDate(0, 0, -8)},
			{ID: "observation-future", PetID: pet.ID, ObservedAt: now.Add(time.Hour)},
		},
	}}
	supplies := &summarySupplyService{byPet: map[string][]domain.Supply{
		pet.ID: {
			{ID: "supply-soon", PetID: pet.ID, Name: "Ração", LastPurchasedAt: dateDaysFrom(now, -25), EstimatedDaysSupply: 30},
			{ID: "supply-far", PetID: pet.ID, Name: "Sachê", LastPurchasedAt: dateDaysFrom(now, -20), EstimatedDaysSupply: 30},
		},
	}}
	uc := NewSummaryUseCase(pets, vaccines, treatments, appointments, observations, supplies, time.UTC)

	summary, err := uc.AllPets(context.Background())
	if err != nil {
		t.Fatalf("AllPets returned error: %v", err)
	}
	if len(summary.Pets) != 1 {
		t.Fatalf("summary pets len = %d", len(summary.Pets))
	}
	digest := summary.Pets[0]

	if got := idsFromVaccineSummaries(digest.VaccinesDueSoon); !sameIDs(got, []string{"vaccine-due", "vaccine-overdue"}) {
		t.Fatalf("vaccines due soon = %v", got)
	}
	assertVaccineSummary(t, digest.VaccinesDueSoon, "vaccine-due", 25, false)
	assertVaccineSummary(t, digest.VaccinesDueSoon, "vaccine-overdue", -3, true)
	if got := treatmentIDs(digest.ActiveTreatments); !sameIDs(got, []string{"ongoing"}) {
		t.Fatalf("active treatments = %v", got)
	}
	if got := appointmentIDs(digest.UpcomingAppointments); !sameIDs(got, []string{"appointment-soon"}) {
		t.Fatalf("upcoming appointments = %v", got)
	}
	if got := observationIDs(digest.RecentObservations); !sameIDs(got, []string{"observation-recent"}) {
		t.Fatalf("recent observations = %v", got)
	}
	if got := supplyIDs(digest.SuppliesNeedingReorder); !sameIDs(got, []string{"supply-soon"}) {
		t.Fatalf("supplies needing reorder = %v", got)
	}
}

func TestSummaryUseCaseAllPetsWrapsListErrors(t *testing.T) {
	want := errors.New("database down")
	uc := NewSummaryUseCase(&summaryPetService{err: want}, nil, nil, nil, nil, nil, time.UTC)

	_, err := uc.AllPets(context.Background())
	if err == nil {
		t.Fatal("AllPets returned nil error")
	}
	if !errors.Is(err, want) {
		t.Fatalf("AllPets error = %v, want wrapping %v", err, want)
	}
}

func TestSummaryUseCaseVaccinesDueSoonUsesDateOnlyDueDateInConfiguredTimezone(t *testing.T) {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	uc := NewSummaryUseCase(nil, nil, nil, nil, nil, nil, loc)
	now := time.Date(2026, 4, 18, 10, 0, 0, 0, loc)
	dueTomorrowFromSQLite := time.Date(2026, 4, 19, 0, 0, 0, 0, time.UTC)
	dueTodayFromSQLite := time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC)

	summaries := uc.vaccinesDueSoon([]domain.Vaccine{
		{ID: "due-tomorrow", Name: "V10", NextDueAt: &dueTomorrowFromSQLite},
		{ID: "due-today", Name: "Raiva", NextDueAt: &dueTodayFromSQLite},
	}, now)

	assertVaccineSummary(t, summaries, "due-tomorrow", 1, false)
	assertVaccineSummary(t, summaries, "due-today", 0, false)
}

func TestSuppliesNeedingReorderUsesDateOnlyReorderDateInConfiguredTimezone(t *testing.T) {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	now := time.Date(2026, 4, 18, 23, 30, 0, 0, loc)

	supplies := suppliesNeedingReorder([]domain.Supply{
		{ID: "reorder-within-buffer", LastPurchasedAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), EstimatedDaysSupply: 24},
		{ID: "reorder-outside-buffer", LastPurchasedAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), EstimatedDaysSupply: 25},
	}, now, loc)

	if got := supplyIDs(supplies); !sameIDs(got, []string{"reorder-within-buffer"}) {
		t.Fatalf("supplies needing reorder = %v", got)
	}
}

type summaryPetService struct {
	pets []domain.Pet
	err  error
}

func (s *summaryPetService) List(context.Context) ([]domain.Pet, error) {
	return s.pets, s.err
}

func (s *summaryPetService) Create(context.Context, service.CreatePetInput) (*domain.Pet, error) {
	panic("not used")
}

func (s *summaryPetService) GetByID(context.Context, string) (*domain.Pet, error) {
	panic("not used")
}

func (s *summaryPetService) Update(context.Context, string, service.UpdatePetInput) (*domain.Pet, error) {
	panic("not used")
}

func (s *summaryPetService) Delete(context.Context, string) error {
	panic("not used")
}

type summaryVaccineService struct {
	byPet map[string][]domain.Vaccine
}

func (s *summaryVaccineService) ListVaccines(_ context.Context, petID string) ([]domain.Vaccine, error) {
	return s.byPet[petID], nil
}

func (s *summaryVaccineService) RecordVaccine(context.Context, service.RecordVaccineInput) (*domain.Vaccine, error) {
	panic("not used")
}

func (s *summaryVaccineService) GetVaccine(context.Context, string, string) (*domain.Vaccine, error) {
	panic("not used")
}

func (s *summaryVaccineService) DeleteVaccine(context.Context, string, string) error {
	panic("not used")
}

type summaryTreatmentService struct {
	byPet map[string][]domain.Treatment
}

func (s *summaryTreatmentService) List(_ context.Context, petID string) ([]domain.Treatment, error) {
	return s.byPet[petID], nil
}

func (s *summaryTreatmentService) Create(context.Context, service.CreateTreatmentInput) (*domain.Treatment, error) {
	panic("not used")
}

func (s *summaryTreatmentService) GetByID(context.Context, string, string) (*domain.Treatment, error) {
	panic("not used")
}

func (s *summaryTreatmentService) Stop(context.Context, string, string) error {
	panic("not used")
}

type summaryAppointmentService struct {
	byPet map[string][]domain.Appointment
}

func (s *summaryAppointmentService) List(_ context.Context, petID string) ([]domain.Appointment, error) {
	return s.byPet[petID], nil
}

func (s *summaryAppointmentService) Create(context.Context, service.CreateAppointmentInput) (*domain.Appointment, error) {
	panic("not used")
}

func (s *summaryAppointmentService) GetByID(context.Context, string, string) (*domain.Appointment, error) {
	panic("not used")
}

func (s *summaryAppointmentService) Update(context.Context, string, string, service.UpdateAppointmentInput) (*domain.Appointment, error) {
	panic("not used")
}

func (s *summaryAppointmentService) Delete(context.Context, string, string) error {
	panic("not used")
}

type summaryObservationService struct {
	byPet map[string][]domain.Observation
}

func (s *summaryObservationService) ListByPet(_ context.Context, petID string) ([]domain.Observation, error) {
	return s.byPet[petID], nil
}

func (s *summaryObservationService) Create(context.Context, service.CreateObservationInput) (*domain.Observation, error) {
	panic("not used")
}

func (s *summaryObservationService) GetByID(context.Context, string, string) (*domain.Observation, error) {
	panic("not used")
}

type summarySupplyService struct {
	byPet map[string][]domain.Supply
}

func (s *summarySupplyService) List(_ context.Context, petID string) ([]domain.Supply, error) {
	return s.byPet[petID], nil
}

func (s *summarySupplyService) Create(context.Context, service.CreateSupplyInput) (*domain.Supply, error) {
	panic("not used")
}

func (s *summarySupplyService) GetByID(context.Context, string, string) (*domain.Supply, error) {
	panic("not used")
}

func (s *summarySupplyService) Update(context.Context, string, string, service.UpdateSupplyInput) (*domain.Supply, error) {
	panic("not used")
}

func (s *summarySupplyService) Delete(context.Context, string, string) error {
	panic("not used")
}

func ptrTime(t time.Time) *time.Time {
	return &t
}

func dateDaysFrom(t time.Time, days int) time.Time {
	y, m, d := t.AddDate(0, 0, days).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func idsFromVaccineSummaries(summaries []domain.VaccineSummary) []string {
	out := make([]string, 0, len(summaries))
	for _, summary := range summaries {
		out = append(out, summary.Vaccine.ID)
	}
	return out
}

func treatmentIDs(treatments []domain.Treatment) []string {
	out := make([]string, 0, len(treatments))
	for _, treatment := range treatments {
		out = append(out, treatment.ID)
	}
	return out
}

func appointmentIDs(appointments []domain.Appointment) []string {
	out := make([]string, 0, len(appointments))
	for _, appointment := range appointments {
		out = append(out, appointment.ID)
	}
	return out
}

func observationIDs(observations []domain.Observation) []string {
	out := make([]string, 0, len(observations))
	for _, observation := range observations {
		out = append(out, observation.ID)
	}
	return out
}

func supplyIDs(supplies []domain.Supply) []string {
	out := make([]string, 0, len(supplies))
	for _, supply := range supplies {
		out = append(out, supply.ID)
	}
	return out
}

func sameIDs(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	seen := make(map[string]int, len(got))
	for _, id := range got {
		seen[id]++
	}
	for _, id := range want {
		seen[id]--
		if seen[id] < 0 {
			return false
		}
	}
	return true
}

func assertVaccineSummary(t *testing.T, summaries []domain.VaccineSummary, id string, daysUntil int, overdue bool) {
	t.Helper()
	for _, summary := range summaries {
		if summary.Vaccine.ID != id {
			continue
		}
		if summary.DaysUntilDue != daysUntil {
			t.Fatalf("%s days_until_due = %d, want %d", id, summary.DaysUntilDue, daysUntil)
		}
		if summary.Overdue != overdue {
			t.Fatalf("%s overdue = %v, want %v", id, summary.Overdue, overdue)
		}
		return
	}
	t.Fatalf("summary for vaccine %q not found", id)
}
