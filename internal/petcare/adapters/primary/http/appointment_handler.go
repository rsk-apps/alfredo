// internal/petcare/adapters/primary/http/appointment_handler.go
package http

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/rafaelsoares/alfredo/internal/logger"
	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
	"github.com/rafaelsoares/alfredo/internal/petcare/service"
	"github.com/rafaelsoares/alfredo/internal/timeutil"
)

// AppointmentUseCaser is the consumer-defined interface consumed by AppointmentHandler.
type AppointmentUseCaser interface {
	Create(ctx context.Context, in service.CreateAppointmentInput) (*domain.Appointment, error)
	GetByID(ctx context.Context, petID, appointmentID string) (*domain.Appointment, error)
	List(ctx context.Context, petID string) ([]domain.Appointment, error)
	Update(ctx context.Context, petID, appointmentID string, in service.UpdateAppointmentInput) (*domain.Appointment, error)
	Delete(ctx context.Context, petID, appointmentID string) error
}

type AppointmentHandler struct {
	uc  AppointmentUseCaser
	loc *time.Location
}

func NewAppointmentHandler(uc AppointmentUseCaser, loc *time.Location) *AppointmentHandler {
	return &AppointmentHandler{uc: uc, loc: loc}
}

func (h *AppointmentHandler) Register(g *echo.Group) {
	g.POST("/pets/:id/appointments", h.create)
	g.GET("/pets/:id/appointments", h.list)
	g.GET("/pets/:id/appointments/:aid", h.getByID)
	g.PATCH("/pets/:id/appointments/:aid", h.update)
	g.DELETE("/pets/:id/appointments/:aid", h.delete)
}

func (h *AppointmentHandler) create(c echo.Context) error {
	petID, ok := parseUUID(c, "id")
	if !ok {
		return nil
	}
	var req struct {
		Type        string  `json:"type" validate:"required"`
		ScheduledAt string  `json:"scheduled_at" validate:"required"`
		Provider    *string `json:"provider" validate:"omitempty,max=100"`
		Location    *string `json:"location" validate:"omitempty,max=200"`
		Notes       *string `json:"notes" validate:"omitempty,max=1000"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, newErrorResponse("invalid_request_body", "Request body is invalid JSON", nil))
	}
	if !validateRequest(c, &req) {
		return nil
	}
	switch domain.AppointmentType(req.Type) {
	case domain.AppointmentTypeVet, domain.AppointmentTypeGrooming, domain.AppointmentTypeOther:
		// valid
	default:
		return c.JSON(http.StatusBadRequest, newErrorResponse(
			"validation_failed", "Request validation failed",
			[]fieldError{{Field: "type", Issue: "must be one of: vet, grooming, other"}},
		))
	}
	scheduledAt, err := timeutil.ParseUserTime(req.ScheduledAt, h.loc)
	if err != nil {
		return c.JSON(http.StatusBadRequest, newErrorResponse(
			"validation_failed", "Request validation failed",
			[]fieldError{{Field: "scheduled_at", Issue: "must be RFC3339 with offset or YYYY-MM-DDTHH:MM:SS"}},
		))
	}
	appt, err := h.uc.Create(c.Request().Context(), service.CreateAppointmentInput{
		PetID:       petID,
		Type:        domain.AppointmentType(req.Type),
		ScheduledAt: scheduledAt,
		Provider:    req.Provider,
		Location:    req.Location,
		Notes:       req.Notes,
	})
	if err != nil {
		return mapError(c, err)
	}
	logger.FromEcho(c).Info("appointment created",
		zap.String("pet_id", petID),
		zap.String("appointment_id", appt.ID),
		zap.String("type", string(appt.Type)),
	)
	return c.JSON(http.StatusCreated, toAppointmentResponse(*appt, h.loc))
}

func (h *AppointmentHandler) list(c echo.Context) error {
	petID, ok := parseUUID(c, "id")
	if !ok {
		return nil
	}
	appts, err := h.uc.List(c.Request().Context(), petID)
	if err != nil {
		return mapError(c, err)
	}
	resp := make([]appointmentResponse, 0, len(appts))
	for _, a := range appts {
		resp = append(resp, toAppointmentResponse(a, h.loc))
	}
	logger.FromEcho(c).Info("appointments listed", zap.String("pet_id", petID), zap.Int("count", len(appts)))
	return c.JSON(http.StatusOK, resp)
}

func (h *AppointmentHandler) getByID(c echo.Context) error {
	petID, ok := parseUUID(c, "id")
	if !ok {
		return nil
	}
	appointmentID, ok := parseUUID(c, "aid")
	if !ok {
		return nil
	}
	appt, err := h.uc.GetByID(c.Request().Context(), petID, appointmentID)
	if err != nil {
		return mapError(c, err)
	}
	return c.JSON(http.StatusOK, toAppointmentResponse(*appt, h.loc))
}

func (h *AppointmentHandler) update(c echo.Context) error {
	petID, ok := parseUUID(c, "id")
	if !ok {
		return nil
	}
	appointmentID, ok := parseUUID(c, "aid")
	if !ok {
		return nil
	}
	var req struct {
		ScheduledAt *string `json:"scheduled_at"`
		Provider    *string `json:"provider" validate:"omitempty,max=100"`
		Location    *string `json:"location" validate:"omitempty,max=200"`
		Notes       *string `json:"notes" validate:"omitempty,max=1000"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, newErrorResponse("invalid_request_body", "Request body is invalid JSON", nil))
	}
	if !validateRequest(c, &req) {
		return nil
	}
	if req.ScheduledAt == nil && req.Provider == nil && req.Location == nil && req.Notes == nil {
		return c.JSON(http.StatusBadRequest, newErrorResponse(
			"validation_failed", "Request validation failed",
			[]fieldError{{Field: "body", Issue: "at least one field must be provided"}},
		))
	}
	in := service.UpdateAppointmentInput{
		Provider: req.Provider,
		Location: req.Location,
		Notes:    req.Notes,
	}
	if req.ScheduledAt != nil {
		scheduledAt, err := timeutil.ParseUserTime(*req.ScheduledAt, h.loc)
		if err != nil {
			return c.JSON(http.StatusBadRequest, newErrorResponse(
				"validation_failed", "Request validation failed",
				[]fieldError{{Field: "scheduled_at", Issue: "must be RFC3339 with offset or YYYY-MM-DDTHH:MM:SS"}},
			))
		}
		in.ScheduledAt = &scheduledAt
	}
	appt, err := h.uc.Update(c.Request().Context(), petID, appointmentID, in)
	if err != nil {
		return mapError(c, err)
	}
	logger.FromEcho(c).Info("appointment updated",
		zap.String("pet_id", petID),
		zap.String("appointment_id", appointmentID),
	)
	return c.JSON(http.StatusOK, toAppointmentResponse(*appt, h.loc))
}

func (h *AppointmentHandler) delete(c echo.Context) error {
	petID, ok := parseUUID(c, "id")
	if !ok {
		return nil
	}
	appointmentID, ok := parseUUID(c, "aid")
	if !ok {
		return nil
	}
	if err := h.uc.Delete(c.Request().Context(), petID, appointmentID); err != nil {
		return mapError(c, err)
	}
	logger.FromEcho(c).Info("appointment deleted",
		zap.String("pet_id", petID),
		zap.String("appointment_id", appointmentID),
	)
	return c.NoContent(http.StatusNoContent)
}

// --- response types ---

type appointmentResponse struct {
	ID                    string  `json:"id"`
	PetID                 string  `json:"pet_id"`
	Type                  string  `json:"type"`
	ScheduledAt           string  `json:"scheduled_at"`
	Provider              *string `json:"provider,omitempty"`
	Location              *string `json:"location,omitempty"`
	Notes                 *string `json:"notes,omitempty"`
	GoogleCalendarEventID string  `json:"google_calendar_event_id"`
	CreatedAt             string  `json:"created_at"`
}

func toAppointmentResponse(a domain.Appointment, loc *time.Location) appointmentResponse {
	return appointmentResponse{
		ID:                    a.ID,
		PetID:                 a.PetID,
		Type:                  string(a.Type),
		ScheduledAt:           a.ScheduledAt.In(loc).Format(time.RFC3339),
		Provider:              a.Provider,
		Location:              a.Location,
		Notes:                 a.Notes,
		GoogleCalendarEventID: a.GoogleCalendarEventID,
		CreatedAt:             a.CreatedAt.Format(time.RFC3339),
	}
}
