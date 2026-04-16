package http

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
	"github.com/rafaelsoares/alfredo/internal/petcare/service"
)

type vaccineSvcStub struct{}

func (v *vaccineSvcStub) ListVaccines(context.Context, string) ([]domain.Vaccine, error) {
	return nil, nil
}
func (v *vaccineSvcStub) RecordVaccine(_ context.Context, in service.RecordVaccineInput) (*domain.Vaccine, error) {
	return &domain.Vaccine{ID: "v1", PetID: in.PetID, Name: in.Name, AdministeredAt: in.AdministeredAt}, nil
}
func (v *vaccineSvcStub) DeleteVaccine(context.Context, string, string) error { return nil }

type treatmentSvcStub struct{}

func (t *treatmentSvcStub) Create(_ context.Context, in service.CreateTreatmentInput) (*domain.Treatment, []domain.Dose, error) {
	return &domain.Treatment{ID: "t1", PetID: in.PetID, Name: in.Name, StartedAt: in.StartedAt}, nil, nil
}
func (t *treatmentSvcStub) GetByID(context.Context, string, string) (*domain.Treatment, []domain.Dose, error) {
	return nil, nil, nil
}
func (t *treatmentSvcStub) List(context.Context, string) ([]domain.Treatment, map[string][]domain.Dose, error) {
	return nil, nil, nil
}
func (t *treatmentSvcStub) Stop(context.Context, string, string) error { return nil }

type observationSvcStub struct {
	created *domain.Observation
}

func (o *observationSvcStub) Create(_ context.Context, in service.CreateObservationInput) (*domain.Observation, error) {
	observation := &domain.Observation{
		ID:          "123e4567-e89b-12d3-a456-426614174001",
		PetID:       in.PetID,
		ObservedAt:  in.ObservedAt,
		Description: in.Description,
		CreatedAt:   time.Now().UTC(),
	}
	o.created = observation
	return observation, nil
}
func (o *observationSvcStub) ListByPet(context.Context, string) ([]domain.Observation, error) {
	return nil, nil
}
func (o *observationSvcStub) GetByID(context.Context, string, string) (*domain.Observation, error) {
	return nil, nil
}

type petSvcStub struct{}

func (p *petSvcStub) List(context.Context) ([]domain.Pet, error) { return nil, nil }
func (p *petSvcStub) Create(_ context.Context, in service.CreatePetInput) (*domain.Pet, error) {
	return &domain.Pet{ID: "p1", Name: in.Name, Species: in.Species, BirthDate: in.BirthDate}, nil
}
func (p *petSvcStub) GetByID(context.Context, string) (*domain.Pet, error) { return nil, nil }
func (p *petSvcStub) Update(context.Context, string, service.UpdatePetInput) (*domain.Pet, error) {
	return nil, nil
}
func (p *petSvcStub) Delete(context.Context, string) error { return nil }

func TestVaccineHandlerRejectsDateOnly(t *testing.T) {
	e := echo.New()
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/pets/123e4567-e89b-12d3-a456-426614174000/vaccines", bytes.NewBufferString(`{"name":"Rabies","date":"2026-04-12"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/v1/pets/:id/vaccines")
	c.SetParamNames("id")
	c.SetParamValues("123e4567-e89b-12d3-a456-426614174000")

	h := NewVaccineHandler(&vaccineSvcStub{}, loc)
	if err := h.RecordVaccine(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestTreatmentHandlerRejectsDateOnly(t *testing.T) {
	e := echo.New()
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/pets/123e4567-e89b-12d3-a456-426614174000/treatments", bytes.NewBufferString(`{"name":"Amoxicillin","dosage_amount":1,"dosage_unit":"ml","route":"oral","interval_hours":12,"started_at":"2026-04-12"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/v1/pets/:id/treatments")
	c.SetParamNames("id")
	c.SetParamValues("123e4567-e89b-12d3-a456-426614174000")

	h := NewTreatmentHandler(&treatmentSvcStub{}, loc)
	if err := h.StartTreatment(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestPetHandlerAcceptsDateOnlyBirthDate(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/pets", bytes.NewBufferString(`{"name":"Luna","species":"dog","birth_date":"2020-01-01"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := NewPetHandler(&petSvcStub{})
	if err := h.Create(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
}

func TestObservationHandlerRejectsDateOnly(t *testing.T) {
	e := echo.New()
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/pets/123e4567-e89b-12d3-a456-426614174000/observations", bytes.NewBufferString(`{"observed_at":"2026-04-12","description":"Vomited"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/v1/pets/:id/observations")
	c.SetParamNames("id")
	c.SetParamValues("123e4567-e89b-12d3-a456-426614174000")

	h := NewObservationHandler(&observationSvcStub{}, loc)
	if err := h.CreateObservation(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestObservationHandlerParsesNaiveTimeWithConfiguredLocation(t *testing.T) {
	e := echo.New()
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	svc := &observationSvcStub{}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/pets/123e4567-e89b-12d3-a456-426614174000/observations", bytes.NewBufferString(`{"observed_at":"2026-04-12T09:30:00","description":"Vomited"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/v1/pets/:id/observations")
	c.SetParamNames("id")
	c.SetParamValues("123e4567-e89b-12d3-a456-426614174000")

	h := NewObservationHandler(svc, loc)
	if err := h.CreateObservation(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	if got := svc.created.ObservedAt.Format(time.RFC3339); got != "2026-04-12T09:30:00-03:00" {
		t.Fatalf("observed_at = %s, want 2026-04-12T09:30:00-03:00", got)
	}
}
