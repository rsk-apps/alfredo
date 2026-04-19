package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
	"github.com/rafaelsoares/alfredo/internal/petcare/service"
	"github.com/rafaelsoares/alfredo/internal/shared/health"
)

const (
	testPetID         = "123e4567-e89b-12d3-a456-426614174000"
	testResourceID    = "123e4567-e89b-12d3-a456-426614174001"
	testOtherResource = "123e4567-e89b-12d3-a456-426614174002"
)

var handlerNow = time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)

type handlerPetSvc struct{}

func (s *handlerPetSvc) List(context.Context) ([]domain.Pet, error) {
	return []domain.Pet{{ID: testPetID, Name: "Luna", Species: "dog", CreatedAt: handlerNow}}, nil
}
func (s *handlerPetSvc) Create(_ context.Context, in service.CreatePetInput) (*domain.Pet, error) {
	return &domain.Pet{ID: testPetID, Name: in.Name, Species: in.Species, BirthDate: in.BirthDate, CreatedAt: handlerNow}, nil
}
func (s *handlerPetSvc) GetByID(context.Context, string) (*domain.Pet, error) {
	return &domain.Pet{ID: testPetID, Name: "Luna", Species: "dog", CreatedAt: handlerNow}, nil
}
func (s *handlerPetSvc) Update(_ context.Context, id string, in service.UpdatePetInput) (*domain.Pet, error) {
	return &domain.Pet{ID: id, Name: in.Name, Species: in.Species, BirthDate: in.BirthDate, CreatedAt: handlerNow}, nil
}
func (s *handlerPetSvc) Delete(context.Context, string) error { return nil }

type handlerVaccineSvc struct{}

func (s *handlerVaccineSvc) ListVaccines(context.Context, string) ([]domain.Vaccine, error) {
	nextDue := time.Date(2026, 5, 17, 0, 0, 0, 0, time.UTC)
	return []domain.Vaccine{{ID: testResourceID, PetID: testPetID, Name: "V10", AdministeredAt: handlerNow, NextDueAt: &nextDue}}, nil
}
func (s *handlerVaccineSvc) RecordVaccine(_ context.Context, in service.RecordVaccineInput) (*domain.Vaccine, error) {
	return &domain.Vaccine{ID: testResourceID, PetID: in.PetID, Name: in.Name, AdministeredAt: in.AdministeredAt}, nil
}
func (s *handlerVaccineSvc) DeleteVaccine(context.Context, string, string) error { return nil }

type handlerTreatmentSvc struct{}

func (s *handlerTreatmentSvc) Create(_ context.Context, in service.CreateTreatmentInput) (*domain.Treatment, []domain.Dose, error) {
	tr := &domain.Treatment{
		ID:            testResourceID,
		PetID:         in.PetID,
		Name:          in.Name,
		DosageAmount:  in.DosageAmount,
		DosageUnit:    in.DosageUnit,
		Route:         in.Route,
		IntervalHours: in.IntervalHours,
		StartedAt:     in.StartedAt,
		EndedAt:       in.EndedAt,
		CreatedAt:     handlerNow,
	}
	return tr, []domain.Dose{{ID: testOtherResource, ScheduledFor: in.StartedAt, GoogleCalendarEventID: "evt-1"}}, nil
}
func (s *handlerTreatmentSvc) GetByID(context.Context, string, string) (*domain.Treatment, []domain.Dose, error) {
	tr := sampleTreatment()
	return &tr, []domain.Dose{{ID: testOtherResource, ScheduledFor: handlerNow}}, nil
}
func (s *handlerTreatmentSvc) List(context.Context, string) ([]domain.Treatment, map[string][]domain.Dose, error) {
	tr := sampleTreatment()
	return []domain.Treatment{tr}, map[string][]domain.Dose{tr.ID: {{ID: testOtherResource, ScheduledFor: handlerNow}}}, nil
}
func (s *handlerTreatmentSvc) Stop(context.Context, string, string) error { return nil }

type handlerObservationSvc struct{}

func (s *handlerObservationSvc) Create(_ context.Context, in service.CreateObservationInput) (*domain.Observation, error) {
	return &domain.Observation{ID: testResourceID, PetID: in.PetID, ObservedAt: in.ObservedAt, Description: in.Description, CreatedAt: handlerNow}, nil
}
func (s *handlerObservationSvc) ListByPet(context.Context, string) ([]domain.Observation, error) {
	return []domain.Observation{{ID: testResourceID, PetID: testPetID, ObservedAt: handlerNow, Description: "Vomitou", CreatedAt: handlerNow}}, nil
}
func (s *handlerObservationSvc) GetByID(context.Context, string, string) (*domain.Observation, error) {
	return &domain.Observation{ID: testResourceID, PetID: testPetID, ObservedAt: handlerNow, Description: "Vomitou", CreatedAt: handlerNow}, nil
}

type handlerSupplySvc struct{}

func (s *handlerSupplySvc) Create(_ context.Context, in service.CreateSupplyInput) (*domain.Supply, error) {
	return sampleSupply(in.PetID, in.Name, in.LastPurchasedAt, in.EstimatedDaysSupply), nil
}
func (s *handlerSupplySvc) GetByID(context.Context, string, string) (*domain.Supply, error) {
	return sampleSupply(testPetID, "Racao", time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), 30), nil
}
func (s *handlerSupplySvc) List(context.Context, string) ([]domain.Supply, error) {
	return []domain.Supply{*sampleSupply(testPetID, "Racao", time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), 30)}, nil
}
func (s *handlerSupplySvc) Update(_ context.Context, petID, _ string, in service.UpdateSupplyInput) (*domain.Supply, error) {
	name := "Racao"
	if in.Name != nil {
		name = *in.Name
	}
	purchasedAt := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	if in.LastPurchasedAt != nil {
		purchasedAt = *in.LastPurchasedAt
	}
	days := 30
	if in.EstimatedDaysSupply != nil {
		days = *in.EstimatedDaysSupply
	}
	return sampleSupply(petID, name, purchasedAt, days), nil
}
func (s *handlerSupplySvc) Delete(context.Context, string, string) error { return nil }

type handlerSummarySvc struct{}

func (s *handlerSummarySvc) AllPets(context.Context) (domain.AllPetsSummary, error) {
	return domain.AllPetsSummary{
		GeneratedAt: handlerNow,
		Pets: []domain.PetDigest{{
			Pet:                    domain.Pet{ID: testPetID, Name: "Luna", Species: "dog", CreatedAt: handlerNow},
			VaccinesDueSoon:        []domain.VaccineSummary{{Vaccine: domain.Vaccine{ID: testResourceID, PetID: testPetID, Name: "V10", AdministeredAt: handlerNow}, DaysUntilDue: 3}},
			ActiveTreatments:       []domain.Treatment{sampleTreatment()},
			UpcomingAppointments:   []domain.Appointment{{ID: testResourceID, PetID: testPetID, Type: domain.AppointmentTypeVet, ScheduledAt: handlerNow, CreatedAt: handlerNow}},
			RecentObservations:     []domain.Observation{{ID: testResourceID, PetID: testPetID, ObservedAt: handlerNow, Description: "Vomitou", CreatedAt: handlerNow}},
			SuppliesNeedingReorder: []domain.Supply{*sampleSupply(testPetID, "Racao", time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), 10)},
		}},
	}, nil
}

type handlerHealthChecker struct{ status string }

func (h handlerHealthChecker) Check(context.Context) health.HealthResult {
	return health.HealthResult{Status: h.status}
}

type handlerAppointmentSvc struct{}

func (s *handlerAppointmentSvc) Create(_ context.Context, in service.CreateAppointmentInput) (*domain.Appointment, error) {
	return &domain.Appointment{ID: testResourceID, PetID: in.PetID, Type: in.Type, ScheduledAt: in.ScheduledAt, Provider: in.Provider, Location: in.Location, Notes: in.Notes, CreatedAt: handlerNow}, nil
}
func (s *handlerAppointmentSvc) GetByID(context.Context, string, string) (*domain.Appointment, error) {
	return &domain.Appointment{ID: testResourceID, PetID: testPetID, Type: domain.AppointmentTypeVet, ScheduledAt: handlerNow, CreatedAt: handlerNow}, nil
}
func (s *handlerAppointmentSvc) List(context.Context, string) ([]domain.Appointment, error) {
	return []domain.Appointment{{ID: testResourceID, PetID: testPetID, Type: domain.AppointmentTypeVet, ScheduledAt: handlerNow, CreatedAt: handlerNow}}, nil
}
func (s *handlerAppointmentSvc) Update(_ context.Context, petID, appointmentID string, in service.UpdateAppointmentInput) (*domain.Appointment, error) {
	scheduledAt := handlerNow
	if in.ScheduledAt != nil {
		scheduledAt = *in.ScheduledAt
	}
	return &domain.Appointment{ID: appointmentID, PetID: petID, Type: domain.AppointmentTypeVet, ScheduledAt: scheduledAt, Provider: in.Provider, Location: in.Location, Notes: in.Notes, CreatedAt: handlerNow}, nil
}
func (s *handlerAppointmentSvc) Delete(context.Context, string, string) error { return nil }

func doHandlerRequest(t *testing.T, method, path, body string, params map[string]string, handler echo.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if len(params) > 0 {
		names := make([]string, 0, len(params))
		values := make([]string, 0, len(params))
		for name, value := range params {
			names = append(names, name)
			values = append(values, value)
		}
		c.SetParamNames(names...)
		c.SetParamValues(values...)
	}
	if err := handler(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	return rec
}

func newTestGroup() *echo.Group {
	return echo.New().Group("/api/v1")
}

func assertStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status = %d body = %s, want %d", rec.Code, rec.Body.String(), want)
	}
}

func sampleTreatment() domain.Treatment {
	endedAt := handlerNow.Add(24 * time.Hour)
	stoppedAt := handlerNow.Add(2 * time.Hour)
	return domain.Treatment{
		ID:            testResourceID,
		PetID:         testPetID,
		Name:          "Antibiotico",
		DosageAmount:  2.5,
		DosageUnit:    "ml",
		Route:         "oral",
		IntervalHours: 12,
		StartedAt:     handlerNow,
		EndedAt:       &endedAt,
		StoppedAt:     &stoppedAt,
		CreatedAt:     handlerNow,
	}
}

func sampleSupply(petID, name string, purchasedAt time.Time, days int) *domain.Supply {
	return &domain.Supply{
		ID:                  testResourceID,
		PetID:               petID,
		Name:                name,
		LastPurchasedAt:     purchasedAt,
		EstimatedDaysSupply: days,
		CreatedAt:           handlerNow,
		UpdatedAt:           handlerNow,
	}
}

func testLocation(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	return loc
}
