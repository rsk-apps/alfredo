package http

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/rafaelsoares/alfredo/internal/shared/health"
)

// HealthChecker is the narrow interface consumed by HealthHTTPHandler.
// Satisfied by app.HealthAggregator.
type HealthChecker interface {
	Check(ctx context.Context) health.HealthResult
}

type HealthHTTPHandler struct {
	agg HealthChecker
}

func NewHealthHTTPHandler(agg HealthChecker) *HealthHTTPHandler {
	return &HealthHTTPHandler{agg: agg}
}

func (h *HealthHTTPHandler) Health(c echo.Context) error {
	result := h.agg.Check(c.Request().Context())
	if result.Status == "healthy" {
		return c.JSON(http.StatusOK, result)
	}
	return c.JSON(http.StatusServiceUnavailable, result)
}
