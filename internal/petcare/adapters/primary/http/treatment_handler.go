// internal/petcare/adapters/primary/http/treatment_handler.go
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
)

// TreatmentServicer is the consumer-defined interface consumed by TreatmentHandler.
type TreatmentServicer interface {
	Create(ctx context.Context, in service.CreateTreatmentInput) (*domain.Treatment, []domain.Dose, error)
	GetByID(ctx context.Context, petID, treatmentID string) (*domain.Treatment, []domain.Dose, error)
	List(ctx context.Context, petID string) ([]domain.Treatment, map[string][]domain.Dose, error)
	Stop(ctx context.Context, petID, treatmentID string) error
}

type TreatmentHandler struct {
	svc TreatmentServicer
}

func NewTreatmentHandler(svc TreatmentServicer) *TreatmentHandler {
	return &TreatmentHandler{svc: svc}
}

func (h *TreatmentHandler) Register(g *echo.Group) {
	g.POST("/pets/:id/treatments", h.StartTreatment)
	g.GET("/pets/:id/treatments", h.ListTreatments)
	g.GET("/pets/:id/treatments/:tid", h.GetTreatment)
	g.DELETE("/pets/:id/treatments/:tid", h.StopTreatment)
}

func (h *TreatmentHandler) StartTreatment(c echo.Context) error {
	petID, ok := parseUUID(c, "id")
	if !ok {
		return nil
	}
	var req struct {
		Name          string  `json:"name" validate:"required,min=1,max=100"`
		DosageAmount  float64 `json:"dosage_amount" validate:"required,gt=0"`
		DosageUnit    string  `json:"dosage_unit" validate:"required,min=1,max=20"`
		Route         string  `json:"route" validate:"required,min=1,max=50"`
		IntervalHours int     `json:"interval_hours" validate:"required,min=1"`
		StartedAt     string  `json:"started_at" validate:"required"`
		EndedAt       *string `json:"ended_at"`
		VetName       *string `json:"vet_name" validate:"omitempty,max=100"`
		Notes         *string `json:"notes" validate:"omitempty,max=500"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, newErrorResponse("invalid_request_body", "Request body is invalid JSON", nil))
	}
	if !validateRequest(c, &req) {
		return nil
	}
	startedAt, err := time.Parse(time.RFC3339, req.StartedAt)
	if err != nil {
		return c.JSON(http.StatusBadRequest, newErrorResponse(
			"validation_failed", "Request validation failed",
			[]fieldError{{Field: "started_at", Issue: "must be RFC3339 format"}},
		))
	}
	in := service.CreateTreatmentInput{
		PetID:         petID,
		Name:          req.Name,
		DosageAmount:  req.DosageAmount,
		DosageUnit:    req.DosageUnit,
		Route:         req.Route,
		IntervalHours: req.IntervalHours,
		StartedAt:     startedAt,
		VetName:       req.VetName,
		Notes:         req.Notes,
	}
	if req.EndedAt != nil {
		endedAt, err := time.Parse(time.RFC3339, *req.EndedAt)
		if err != nil {
			return c.JSON(http.StatusBadRequest, newErrorResponse(
				"validation_failed", "Request validation failed",
				[]fieldError{{Field: "ended_at", Issue: "must be RFC3339 format"}},
			))
		}
		in.EndedAt = &endedAt
	}
	tr, doses, err := h.svc.Create(c.Request().Context(), in)
	if err != nil {
		return mapError(c, err)
	}
	logger.FromEcho(c).Info("treatment started",
		zap.String("pet_id", petID),
		zap.String("treatment_id", tr.ID),
		zap.Int("doses_generated", len(doses)),
	)
	return c.JSON(http.StatusCreated, toTreatmentResponse(*tr, doses))
}

func (h *TreatmentHandler) ListTreatments(c echo.Context) error {
	petID, ok := parseUUID(c, "id")
	if !ok {
		return nil
	}
	ts, doseMap, err := h.svc.List(c.Request().Context(), petID)
	if err != nil {
		return mapError(c, err)
	}
	resp := make([]treatmentResponse, 0, len(ts))
	for _, t := range ts {
		resp = append(resp, toTreatmentResponse(t, doseMap[t.ID]))
	}
	logger.FromEcho(c).Info("treatments listed", zap.String("pet_id", petID), zap.Int("count", len(ts)))
	return c.JSON(http.StatusOK, resp)
}

func (h *TreatmentHandler) GetTreatment(c echo.Context) error {
	petID, ok := parseUUID(c, "id")
	if !ok {
		return nil
	}
	treatmentID, ok := parseUUID(c, "tid")
	if !ok {
		return nil
	}
	tr, doses, err := h.svc.GetByID(c.Request().Context(), petID, treatmentID)
	if err != nil {
		return mapError(c, err)
	}
	return c.JSON(http.StatusOK, toTreatmentResponse(*tr, doses))
}

func (h *TreatmentHandler) StopTreatment(c echo.Context) error {
	petID, ok := parseUUID(c, "id")
	if !ok {
		return nil
	}
	treatmentID, ok := parseUUID(c, "tid")
	if !ok {
		return nil
	}
	if err := h.svc.Stop(c.Request().Context(), petID, treatmentID); err != nil {
		return mapError(c, err)
	}
	logger.FromEcho(c).Info("treatment stopped",
		zap.String("pet_id", petID),
		zap.String("treatment_id", treatmentID),
	)
	return c.NoContent(http.StatusNoContent)
}

// --- response types ---

type doseResponse struct {
	ID           string `json:"id"`
	ScheduledFor string `json:"scheduled_for"`
}

type treatmentResponse struct {
	ID            string         `json:"id"`
	PetID         string         `json:"pet_id"`
	Name          string         `json:"name"`
	DosageAmount  float64        `json:"dosage_amount"`
	DosageUnit    string         `json:"dosage_unit"`
	Route         string         `json:"route"`
	IntervalHours int            `json:"interval_hours"`
	StartedAt     string         `json:"started_at"`
	EndedAt       *string        `json:"ended_at,omitempty"`
	StoppedAt     *string        `json:"stopped_at,omitempty"`
	VetName       *string        `json:"vet_name,omitempty"`
	Notes         *string        `json:"notes,omitempty"`
	CreatedAt     string         `json:"created_at"`
	Doses         []doseResponse `json:"doses"`
}

func toTreatmentResponse(t domain.Treatment, doses []domain.Dose) treatmentResponse {
	r := treatmentResponse{
		ID:            t.ID,
		PetID:         t.PetID,
		Name:          t.Name,
		DosageAmount:  t.DosageAmount,
		DosageUnit:    t.DosageUnit,
		Route:         t.Route,
		IntervalHours: t.IntervalHours,
		StartedAt:     t.StartedAt.Format(time.RFC3339),
		VetName:       t.VetName,
		Notes:         t.Notes,
		CreatedAt:     t.CreatedAt.Format(time.RFC3339),
		Doses:         make([]doseResponse, 0, len(doses)),
	}
	if t.EndedAt != nil {
		s := t.EndedAt.Format(time.RFC3339)
		r.EndedAt = &s
	}
	if t.StoppedAt != nil {
		s := t.StoppedAt.Format(time.RFC3339)
		r.StoppedAt = &s
	}
	for _, d := range doses {
		r.Doses = append(r.Doses, doseResponse{
			ID:           d.ID,
			ScheduledFor: d.ScheduledFor.Format(time.RFC3339),
		})
	}
	return r
}
