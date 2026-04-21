package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rafaelsoares/alfredo/internal/health/domain"
)

type mockAppointmentUseCase struct {
	appts map[string]*domain.HealthAppointment
	err   error
}

func (m *mockAppointmentUseCase) Create(ctx context.Context, specialty string, scheduledAt time.Time, doctor, notes *string) (*domain.HealthAppointment, error) {
	if m.err != nil {
		return nil, m.err
	}
	appt := &domain.HealthAppointment{
		ID:          "appt-1",
		Specialty:   specialty,
		ScheduledAt: scheduledAt,
		Doctor:      doctor,
		Notes:       notes,
		CreatedAt:   time.Now().UTC(),
	}
	if m.appts == nil {
		m.appts = make(map[string]*domain.HealthAppointment)
	}
	m.appts[appt.ID] = appt
	return appt, nil
}

func (m *mockAppointmentUseCase) GetByID(ctx context.Context, id string) (*domain.HealthAppointment, error) {
	if m.err != nil {
		return nil, m.err
	}
	a, ok := m.appts[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return a, nil
}

func (m *mockAppointmentUseCase) List(ctx context.Context) ([]domain.HealthAppointment, error) {
	if m.err != nil {
		return nil, m.err
	}
	var out []domain.HealthAppointment
	for _, a := range m.appts {
		out = append(out, *a)
	}
	return out, nil
}

func (m *mockAppointmentUseCase) Delete(ctx context.Context, id string) error {
	if m.err != nil {
		return m.err
	}
	if _, ok := m.appts[id]; !ok {
		return domain.ErrNotFound
	}
	delete(m.appts, id)
	return nil
}

func TestHealthAppointmentHandlerCreate(t *testing.T) {
	uc := &mockAppointmentUseCase{appts: make(map[string]*domain.HealthAppointment)}
	handler := NewHealthAppointmentHandler(uc, time.UTC)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/health/appointments", strings.NewReader(`{
		"specialty": "Cardiologia",
		"scheduled_at": "2026-05-10T09:00:00",
		"doctor": "Dr. Silva"
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.create(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rec.Code)
	}
}

func TestHealthAppointmentHandlerCreateValidationError(t *testing.T) {
	uc := &mockAppointmentUseCase{appts: make(map[string]*domain.HealthAppointment)}
	handler := NewHealthAppointmentHandler(uc, time.UTC)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/health/appointments", strings.NewReader(`{
		"specialty": "",
		"scheduled_at": "2026-05-10T09:00:00"
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.create(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestHealthAppointmentHandlerCreateRejectsInvalidJSONAndDate(t *testing.T) {
	uc := &mockAppointmentUseCase{appts: make(map[string]*domain.HealthAppointment)}
	handler := NewHealthAppointmentHandler(uc, time.UTC)
	e := echo.New()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/health/appointments", strings.NewReader(`{`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	if err := handler.create(e.NewContext(req, rec)); err != nil {
		t.Fatalf("invalid json error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid json status = %d, want 400", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/health/appointments", strings.NewReader(`{
		"specialty": "Cardiologia",
		"scheduled_at": "2026-05-10"
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	if err := handler.create(e.NewContext(req, rec)); err != nil {
		t.Fatalf("invalid date error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid date status = %d, want 400", rec.Code)
	}
}

func TestHealthAppointmentHandlerList(t *testing.T) {
	appt := &domain.HealthAppointment{
		ID:        "appt-1",
		Specialty: "Dentista",
		CreatedAt: time.Now().UTC(),
	}
	uc := &mockAppointmentUseCase{appts: map[string]*domain.HealthAppointment{"appt-1": appt}}
	handler := NewHealthAppointmentHandler(uc, time.UTC)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health/appointments", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.list(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
}

func TestHealthAppointmentHandlerListAndCreatePropagateUseCaseErrors(t *testing.T) {
	uc := &mockAppointmentUseCase{err: domain.ErrValidation}
	handler := NewHealthAppointmentHandler(uc, time.UTC)
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health/appointments", nil)
	rec := httptest.NewRecorder()
	if err := handler.list(e.NewContext(req, rec)); err != nil {
		t.Fatalf("list error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("list status = %d, want 400", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/health/appointments", strings.NewReader(`{
		"specialty": "Cardiologia",
		"scheduled_at": "2026-05-10T09:00:00"
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	if err := handler.create(e.NewContext(req, rec)); err != nil {
		t.Fatalf("create error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("create status = %d, want 400", rec.Code)
	}
}

func TestHealthAppointmentHandlerGetByID(t *testing.T) {
	appt := &domain.HealthAppointment{
		ID:        "appt-1",
		Specialty: "Oftalmologia",
		CreatedAt: time.Now().UTC(),
	}
	uc := &mockAppointmentUseCase{appts: map[string]*domain.HealthAppointment{"appt-1": appt}}
	handler := NewHealthAppointmentHandler(uc, time.UTC)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health/appointments/appt-1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("appt-1")

	if err := handler.getByID(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
}

func TestHealthAppointmentHandlerGetByIDNotFound(t *testing.T) {
	uc := &mockAppointmentUseCase{appts: make(map[string]*domain.HealthAppointment)}
	handler := NewHealthAppointmentHandler(uc, time.UTC)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health/appointments/nonexistent", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("nonexistent")

	if err := handler.getByID(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}

func TestHealthAppointmentHandlerGetByIDRequiresID(t *testing.T) {
	uc := &mockAppointmentUseCase{appts: make(map[string]*domain.HealthAppointment)}
	handler := NewHealthAppointmentHandler(uc, time.UTC)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health/appointments", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.getByID(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHealthAppointmentHandlerDelete(t *testing.T) {
	appt := &domain.HealthAppointment{
		ID:        "appt-1",
		Specialty: "Neurologia",
		CreatedAt: time.Now().UTC(),
	}
	uc := &mockAppointmentUseCase{appts: map[string]*domain.HealthAppointment{"appt-1": appt}}
	handler := NewHealthAppointmentHandler(uc, time.UTC)

	e := echo.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/health/appointments/appt-1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("appt-1")

	if err := handler.delete(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", rec.Code)
	}
}

func TestHealthAppointmentHandlerDeleteNotFound(t *testing.T) {
	uc := &mockAppointmentUseCase{appts: make(map[string]*domain.HealthAppointment)}
	handler := NewHealthAppointmentHandler(uc, time.UTC)

	e := echo.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/health/appointments/nonexistent", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("nonexistent")

	if err := handler.delete(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}

func TestHealthAppointmentHandlerDeleteRequiresID(t *testing.T) {
	uc := &mockAppointmentUseCase{appts: make(map[string]*domain.HealthAppointment)}
	handler := NewHealthAppointmentHandler(uc, time.UTC)

	e := echo.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/health/appointments", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.delete(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
