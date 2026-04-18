package http

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/rafaelsoares/alfredo/internal/logger"
	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
	"github.com/rafaelsoares/alfredo/internal/petcare/service"
)

// SupplyUseCaser is the consumer-defined interface consumed by SupplyHandler.
type SupplyUseCaser interface {
	Create(ctx context.Context, in service.CreateSupplyInput) (*domain.Supply, error)
	GetByID(ctx context.Context, petID, supplyID string) (*domain.Supply, error)
	List(ctx context.Context, petID string) ([]domain.Supply, error)
	Update(ctx context.Context, petID, supplyID string, in service.UpdateSupplyInput) (*domain.Supply, error)
	Delete(ctx context.Context, petID, supplyID string) error
}

type SupplyHandler struct {
	uc SupplyUseCaser
}

func NewSupplyHandler(uc SupplyUseCaser) *SupplyHandler {
	return &SupplyHandler{uc: uc}
}

func (h *SupplyHandler) Register(g *echo.Group) {
	g.POST("/pets/:id/supplies", h.create)
	g.GET("/pets/:id/supplies", h.list)
	g.GET("/pets/:id/supplies/:sid", h.getByID)
	g.PATCH("/pets/:id/supplies/:sid", h.update)
	g.DELETE("/pets/:id/supplies/:sid", h.delete)
}

func (h *SupplyHandler) create(c echo.Context) error {
	petID, ok := parseUUID(c, "id")
	if !ok {
		return nil
	}
	var req createSupplyRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, newErrorResponse("invalid_request_body", "Request body is invalid JSON", nil))
	}
	if strings.TrimSpace(req.Name) == "" {
		return validationError(c, "name", "required")
	}
	if req.EstimatedDaysSupply <= 0 {
		return validationError(c, "estimated_days_supply", "must be greater than zero")
	}
	lastPurchasedAt, ok := parseSupplyDate(c, "last_purchased_at", req.LastPurchasedAt)
	if !ok {
		return nil
	}
	supply, err := h.uc.Create(c.Request().Context(), service.CreateSupplyInput{
		PetID:               petID,
		Name:                req.Name,
		LastPurchasedAt:     lastPurchasedAt,
		EstimatedDaysSupply: req.EstimatedDaysSupply,
		Notes:               req.Notes,
	})
	if err != nil {
		return mapError(c, err)
	}
	logger.FromEcho(c).Info("supply created",
		zap.String("pet_id", petID),
		zap.String("supply_id", supply.ID),
	)
	return c.JSON(http.StatusCreated, toSupplyResponse(*supply))
}

func (h *SupplyHandler) list(c echo.Context) error {
	petID, ok := parseUUID(c, "id")
	if !ok {
		return nil
	}
	supplies, err := h.uc.List(c.Request().Context(), petID)
	if err != nil {
		return mapError(c, err)
	}
	resp := make([]supplyResponse, 0, len(supplies))
	for _, supply := range supplies {
		resp = append(resp, toSupplyResponse(supply))
	}
	logger.FromEcho(c).Info("supplies listed", zap.String("pet_id", petID), zap.Int("count", len(supplies)))
	return c.JSON(http.StatusOK, resp)
}

func (h *SupplyHandler) getByID(c echo.Context) error {
	petID, ok := parseUUID(c, "id")
	if !ok {
		return nil
	}
	supplyID, ok := parseUUID(c, "sid")
	if !ok {
		return nil
	}
	supply, err := h.uc.GetByID(c.Request().Context(), petID, supplyID)
	if err != nil {
		return mapError(c, err)
	}
	return c.JSON(http.StatusOK, toSupplyResponse(*supply))
}

func (h *SupplyHandler) update(c echo.Context) error {
	petID, ok := parseUUID(c, "id")
	if !ok {
		return nil
	}
	supplyID, ok := parseUUID(c, "sid")
	if !ok {
		return nil
	}
	var req updateSupplyRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, newErrorResponse("invalid_request_body", "Request body is invalid JSON", nil))
	}
	if req.Name == nil && req.LastPurchasedAt == nil && req.EstimatedDaysSupply == nil && req.Notes == nil {
		return validationError(c, "body", "at least one field must be provided")
	}
	if req.Name != nil && strings.TrimSpace(*req.Name) == "" {
		return validationError(c, "name", "required")
	}
	if req.EstimatedDaysSupply != nil && *req.EstimatedDaysSupply <= 0 {
		return validationError(c, "estimated_days_supply", "must be greater than zero")
	}
	in := service.UpdateSupplyInput{
		Name:                req.Name,
		EstimatedDaysSupply: req.EstimatedDaysSupply,
		Notes:               req.Notes,
	}
	if req.LastPurchasedAt != nil {
		lastPurchasedAt, ok := parseSupplyDate(c, "last_purchased_at", *req.LastPurchasedAt)
		if !ok {
			return nil
		}
		in.LastPurchasedAt = &lastPurchasedAt
	}
	supply, err := h.uc.Update(c.Request().Context(), petID, supplyID, in)
	if err != nil {
		return mapError(c, err)
	}
	logger.FromEcho(c).Info("supply updated",
		zap.String("pet_id", petID),
		zap.String("supply_id", supplyID),
	)
	return c.JSON(http.StatusOK, toSupplyResponse(*supply))
}

func (h *SupplyHandler) delete(c echo.Context) error {
	petID, ok := parseUUID(c, "id")
	if !ok {
		return nil
	}
	supplyID, ok := parseUUID(c, "sid")
	if !ok {
		return nil
	}
	if err := h.uc.Delete(c.Request().Context(), petID, supplyID); err != nil {
		return mapError(c, err)
	}
	logger.FromEcho(c).Info("supply deleted",
		zap.String("pet_id", petID),
		zap.String("supply_id", supplyID),
	)
	return c.NoContent(http.StatusNoContent)
}

type createSupplyRequest struct {
	Name                string  `json:"name"`
	LastPurchasedAt     string  `json:"last_purchased_at"`
	EstimatedDaysSupply int     `json:"estimated_days_supply"`
	Notes               *string `json:"notes"`
}

type updateSupplyRequest struct {
	Name                *string `json:"name"`
	LastPurchasedAt     *string `json:"last_purchased_at"`
	EstimatedDaysSupply *int    `json:"estimated_days_supply"`
	Notes               *string `json:"notes"`
}

type supplyResponse struct {
	ID                  string  `json:"id"`
	PetID               string  `json:"pet_id"`
	Name                string  `json:"name"`
	LastPurchasedAt     string  `json:"last_purchased_at"`
	EstimatedDaysSupply int     `json:"estimated_days_supply"`
	NextReorderAt       string  `json:"next_reorder_at"`
	Notes               *string `json:"notes"`
	CreatedAt           string  `json:"created_at"`
	UpdatedAt           string  `json:"updated_at"`
}

func toSupplyResponse(supply domain.Supply) supplyResponse {
	return supplyResponse{
		ID:                  supply.ID,
		PetID:               supply.PetID,
		Name:                supply.Name,
		LastPurchasedAt:     supply.LastPurchasedAt.Format("2006-01-02"),
		EstimatedDaysSupply: supply.EstimatedDaysSupply,
		NextReorderAt:       supply.NextReorderAt().Format("2006-01-02"),
		Notes:               supply.Notes,
		CreatedAt:           supply.CreatedAt.Format(time.RFC3339),
		UpdatedAt:           supply.UpdatedAt.Format(time.RFC3339),
	}
}

func parseSupplyDate(c echo.Context, field, value string) (time.Time, bool) {
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		_ = validationError(c, field, "must be YYYY-MM-DD format")
		return time.Time{}, false
	}
	return parsed, true
}

func validationError(c echo.Context, field, issue string) error {
	return c.JSON(http.StatusBadRequest, newErrorResponse(
		"validation_failed",
		"Request validation failed",
		[]fieldError{{Field: field, Issue: issue}},
	))
}
