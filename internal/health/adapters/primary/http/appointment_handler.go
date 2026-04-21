package http

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/rafaelsoares/alfredo/internal/health/domain"
	"github.com/rafaelsoares/alfredo/internal/logger"
	"github.com/rafaelsoares/alfredo/internal/timeutil"
)

// HealthAppointmentUseCaser is the interface consumed by HealthAppointmentHandler.
type HealthAppointmentUseCaser interface {
	Create(ctx context.Context, specialty string, scheduledAt time.Time, doctor, notes *string) (*domain.HealthAppointment, error)
	GetByID(ctx context.Context, id string) (*domain.HealthAppointment, error)
	List(ctx context.Context) ([]domain.HealthAppointment, error)
	Delete(ctx context.Context, id string) error
}

// HealthAppointmentHandler handles HTTP requests for health appointments.
type HealthAppointmentHandler struct {
	uc  HealthAppointmentUseCaser
	loc *time.Location
}

// NewHealthAppointmentHandler creates a new health appointment handler.
func NewHealthAppointmentHandler(uc HealthAppointmentUseCaser, loc *time.Location) *HealthAppointmentHandler {
	return &HealthAppointmentHandler{uc: uc, loc: loc}
}

// Register registers the appointment routes.
func (h *HealthAppointmentHandler) Register(g *echo.Group) {
	g.POST("/health/appointments", h.create)
	g.GET("/health/appointments", h.list)
	g.GET("/health/appointments/:id", h.getByID)
	g.DELETE("/health/appointments/:id", h.delete)
}

func (h *HealthAppointmentHandler) create(c echo.Context) error {
	var req struct {
		Specialty   string  `json:"specialty" validate:"required"`
		ScheduledAt string  `json:"scheduled_at" validate:"required"`
		Doctor      *string `json:"doctor" validate:"omitempty,max=100"`
		Notes       *string `json:"notes" validate:"omitempty,max=1000"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, newErrorResponse("invalid_request_body", "Request body is invalid JSON", nil))
	}

	if req.Specialty == "" {
		return c.JSON(http.StatusBadRequest, newErrorResponse(
			"validation_failed", "Request validation failed",
			[]fieldError{{Field: "specialty", Issue: "specialty must not be empty"}},
		))
	}

	scheduledAt, err := timeutil.ParseUserTime(req.ScheduledAt, h.loc)
	if err != nil {
		return c.JSON(http.StatusBadRequest, newErrorResponse(
			"validation_failed", "Request validation failed",
			[]fieldError{{Field: "scheduled_at", Issue: "must be RFC3339 with offset or YYYY-MM-DDTHH:MM:SS"}},
		))
	}

	appt, err := h.uc.Create(c.Request().Context(), req.Specialty, scheduledAt, req.Doctor, req.Notes)
	if err != nil {
		return mapError(c, err)
	}

	logger.FromEcho(c).Info("health appointment created",
		zap.String("appointment_id", appt.ID),
		zap.String("specialty", appt.Specialty),
	)

	return c.JSON(http.StatusCreated, toAppointmentResponse(*appt, h.loc))
}

func (h *HealthAppointmentHandler) list(c echo.Context) error {
	appts, err := h.uc.List(c.Request().Context())
	if err != nil {
		return mapError(c, err)
	}

	resp := make([]appointmentResponse, 0, len(appts))
	for _, a := range appts {
		resp = append(resp, toAppointmentResponse(a, h.loc))
	}

	logger.FromEcho(c).Info("health appointments listed", zap.Int("count", len(appts)))

	return c.JSON(http.StatusOK, resp)
}

func (h *HealthAppointmentHandler) getByID(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, newErrorResponse("invalid_request", "appointment id is required", nil))
	}

	appt, err := h.uc.GetByID(c.Request().Context(), id)
	if err != nil {
		return mapError(c, err)
	}

	return c.JSON(http.StatusOK, toAppointmentResponse(*appt, h.loc))
}

func (h *HealthAppointmentHandler) delete(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, newErrorResponse("invalid_request", "appointment id is required", nil))
	}

	err := h.uc.Delete(c.Request().Context(), id)
	if err != nil {
		return mapError(c, err)
	}

	logger.FromEcho(c).Info("health appointment deleted", zap.String("appointment_id", id))

	return c.NoContent(http.StatusNoContent)
}

type appointmentResponse struct {
	ID                    string  `json:"id"`
	Specialty             string  `json:"specialty"`
	ScheduledAt           string  `json:"scheduled_at"`
	Doctor                *string `json:"doctor,omitempty"`
	Notes                 *string `json:"notes,omitempty"`
	GoogleCalendarEventID string  `json:"google_calendar_event_id"`
	CreatedAt             string  `json:"created_at"`
}

func toAppointmentResponse(a domain.HealthAppointment, loc *time.Location) appointmentResponse {
	// Format scheduled_at in the user's timezone
	scheduledAtStr := a.ScheduledAt.In(loc).Format("2006-01-02T15:04:05-07:00")
	return appointmentResponse{
		ID:                    a.ID,
		Specialty:             a.Specialty,
		ScheduledAt:           scheduledAtStr,
		Doctor:                a.Doctor,
		Notes:                 a.Notes,
		GoogleCalendarEventID: a.GoogleCalendarEventID,
		CreatedAt:             a.CreatedAt.Format(time.RFC3339),
	}
}
