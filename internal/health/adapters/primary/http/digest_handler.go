package http

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
)

type DigestUseCaser interface {
	RunDigest(ctx context.Context, days int) error
}

type DigestHandler struct {
	usecase DigestUseCaser
}

func NewDigestHandler(usecase DigestUseCaser) *DigestHandler {
	return &DigestHandler{usecase: usecase}
}

func (h *DigestHandler) Register(g *echo.Group) {
	g.POST("/health/digest", h.RunDigest)
}

type DigestRequest struct {
	Days *int `json:"days"`
}

func (h *DigestHandler) RunDigest(c echo.Context) error {
	var req DigestRequest
	if err := c.Bind(&req); err != nil {
		req = DigestRequest{}
	}

	days := 14
	if req.Days != nil && *req.Days > 0 {
		days = *req.Days
	}

	_ = h.usecase.RunDigest(c.Request().Context(), days)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status": "sent",
		"days":   days,
	})
}
