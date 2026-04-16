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

// ObservationServicer is the consumer-defined interface consumed by ObservationHandler.
type ObservationServicer interface {
	Create(ctx context.Context, in service.CreateObservationInput) (*domain.Observation, error)
	ListByPet(ctx context.Context, petID string) ([]domain.Observation, error)
	GetByID(ctx context.Context, petID, observationID string) (*domain.Observation, error)
}

type ObservationHandler struct {
	svc ObservationServicer
	loc *time.Location
}

func NewObservationHandler(svc ObservationServicer, loc *time.Location) *ObservationHandler {
	return &ObservationHandler{svc: svc, loc: loc}
}

func (h *ObservationHandler) Register(g *echo.Group) {
	g.POST("/pets/:id/observations", h.CreateObservation)
	g.GET("/pets/:id/observations", h.ListObservations)
	g.GET("/pets/:id/observations/:oid", h.GetObservation)
}

func (h *ObservationHandler) CreateObservation(c echo.Context) error {
	petID, ok := parseUUID(c, "id")
	if !ok {
		return nil
	}
	var req struct {
		ObservedAt  string `json:"observed_at" validate:"required"`
		Description string `json:"description" validate:"required,min=1,max=1000"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, newErrorResponse("invalid_request_body", "Request body is invalid JSON", nil))
	}
	if !validateRequest(c, &req) {
		return nil
	}
	observedAt, err := timeutil.ParseUserTime(req.ObservedAt, h.loc)
	if err != nil {
		return c.JSON(http.StatusBadRequest, newErrorResponse(
			"validation_failed",
			"Request validation failed",
			[]fieldError{{Field: "observed_at", Issue: "must be RFC3339 with offset or YYYY-MM-DDTHH:MM:SS"}},
		))
	}
	observation, err := h.svc.Create(c.Request().Context(), service.CreateObservationInput{
		PetID:       petID,
		ObservedAt:  observedAt,
		Description: req.Description,
	})
	if err != nil {
		return mapError(c, err)
	}
	logger.FromEcho(c).Info("observation created",
		zap.String("pet_id", observation.PetID),
		zap.String("observation_id", observation.ID),
	)
	return c.JSON(http.StatusCreated, toObservationResponse(*observation))
}

func (h *ObservationHandler) ListObservations(c echo.Context) error {
	petID, ok := parseUUID(c, "id")
	if !ok {
		return nil
	}
	observations, err := h.svc.ListByPet(c.Request().Context(), petID)
	if err != nil {
		return mapError(c, err)
	}
	resp := make([]observationResponse, 0, len(observations))
	for _, observation := range observations {
		resp = append(resp, toObservationResponse(observation))
	}
	logger.FromEcho(c).Info("observations listed", zap.String("pet_id", petID), zap.Int("count", len(observations)))
	return c.JSON(http.StatusOK, resp)
}

func (h *ObservationHandler) GetObservation(c echo.Context) error {
	petID, ok := parseUUID(c, "id")
	if !ok {
		return nil
	}
	observationID, ok := parseUUID(c, "oid")
	if !ok {
		return nil
	}
	observation, err := h.svc.GetByID(c.Request().Context(), petID, observationID)
	if err != nil {
		return mapError(c, err)
	}
	return c.JSON(http.StatusOK, toObservationResponse(*observation))
}

type observationResponse struct {
	ID          string `json:"id"`
	PetID       string `json:"pet_id"`
	ObservedAt  string `json:"observed_at"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
}

func toObservationResponse(observation domain.Observation) observationResponse {
	return observationResponse{
		ID:          observation.ID,
		PetID:       observation.PetID,
		ObservedAt:  observation.ObservedAt.Format(time.RFC3339),
		Description: observation.Description,
		CreatedAt:   observation.CreatedAt.Format(time.RFC3339),
	}
}
