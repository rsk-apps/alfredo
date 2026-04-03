package http

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rafaelsoares/alfredo/internal/logger"
	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
	"github.com/rafaelsoares/alfredo/internal/petcare/service"
	"go.uber.org/zap"
)

// VaccineServicer is the consumer-defined interface consumed by VaccineHandler.
type VaccineServicer interface {
	ListVaccines(ctx context.Context, petID string) ([]domain.Vaccine, error)
	RecordVaccine(ctx context.Context, in service.RecordVaccineInput) (*domain.Vaccine, error)
	DeleteVaccine(ctx context.Context, petID, vaccineID string) error
}

type VaccineHandler struct {
	svc VaccineServicer
}

func NewVaccineHandler(svc VaccineServicer) *VaccineHandler {
	return &VaccineHandler{svc: svc}
}

func (h *VaccineHandler) Register(g *echo.Group) {
	g.GET("/pets/:id/vaccines", h.ListVaccines)
	g.POST("/pets/:id/vaccines", h.RecordVaccine)
	g.DELETE("/pets/:id/vaccines/:vid", h.DeleteVaccine)
}

func (h *VaccineHandler) ListVaccines(c echo.Context) error {
	id, ok := parseUUID(c, "id")
	if !ok {
		return nil
	}
	vs, err := h.svc.ListVaccines(c.Request().Context(), id)
	if err != nil {
		return mapError(c, err)
	}
	resp := make([]vaccineResponse, 0, len(vs))
	for _, v := range vs {
		resp = append(resp, toVaccineResponse(v))
	}
	logger.FromEcho(c).Info("vaccines listed", zap.String("pet_id", id), zap.Int("count", len(vs)))
	return c.JSON(http.StatusOK, resp)
}

func (h *VaccineHandler) RecordVaccine(c echo.Context) error {
	id, ok := parseUUID(c, "id")
	if !ok {
		return nil
	}
	var req struct {
		Name           string `json:"name" validate:"required,min=1,max=100"`
		Date           string `json:"date" validate:"required"`
		RecurrenceDays *int   `json:"recurrence_days" validate:"omitempty,min=1"`
		VetName        *string `json:"vet_name" validate:"omitempty,max=100"`
		BatchNumber    *string `json:"batch_number" validate:"omitempty,max=50"`
		Notes          *string `json:"notes" validate:"omitempty,max=500"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, newErrorResponse("invalid_request_body", "Request body is invalid JSON", nil))
	}
	if !validateRequest(c, &req) {
		return nil
	}
	adminAt, err := time.Parse(time.RFC3339, req.Date)
	if err != nil {
		return c.JSON(http.StatusBadRequest, newErrorResponse(
			"validation_failed",
			"Request validation failed",
			[]fieldError{{Field: "date", Issue: "must be RFC3339 format"}},
		))
	}
	v, err := h.svc.RecordVaccine(c.Request().Context(), service.RecordVaccineInput{
		PetID: id, Name: req.Name, AdministeredAt: adminAt,
		RecurrenceDays: req.RecurrenceDays, VetName: req.VetName, BatchNumber: req.BatchNumber, Notes: req.Notes,
	})
	if err != nil {
		return mapError(c, err)
	}
	logger.FromEcho(c).Info("vaccine recorded", zap.String("pet_id", v.PetID), zap.String("vaccine_id", v.ID), zap.String("name", v.Name))
	return c.JSON(http.StatusCreated, toVaccineResponse(*v))
}

func (h *VaccineHandler) DeleteVaccine(c echo.Context) error {
	petID, ok := parseUUID(c, "id")
	if !ok {
		return nil
	}
	vaccineID, ok := parseUUID(c, "vid")
	if !ok {
		return nil
	}
	if err := h.svc.DeleteVaccine(c.Request().Context(), petID, vaccineID); err != nil {
		return mapError(c, err)
	}
	logger.FromEcho(c).Info("vaccine deleted", zap.String("pet_id", petID), zap.String("vaccine_id", vaccineID))
	return c.NoContent(http.StatusNoContent)
}

// --- response types ---

type vaccineResponse struct {
	ID          string  `json:"id"`
	PetID       string  `json:"pet_id"`
	Name        string  `json:"name"`
	Date        string  `json:"date"`
	NextDueAt   *string `json:"next_due_at,omitempty"`
	VetName     *string `json:"vet_name,omitempty"`
	BatchNumber *string `json:"batch_number,omitempty"`
	Notes       *string `json:"notes,omitempty"`
}

func toVaccineResponse(v domain.Vaccine) vaccineResponse {
	r := vaccineResponse{
		ID:          v.ID,
		PetID:       v.PetID,
		Name:        v.Name,
		Date:        v.AdministeredAt.Format(time.RFC3339),
		VetName:     v.VetName,
		BatchNumber: v.BatchNumber,
		Notes:       v.Notes,
	}
	if v.NextDueAt != nil {
		s := v.NextDueAt.Format("2006-01-02")
		r.NextDueAt = &s
	}
	return r
}
