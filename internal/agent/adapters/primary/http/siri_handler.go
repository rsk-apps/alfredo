package http

import (
	"context"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

const maxSiriTextLength = 2000

type SiriRequest struct {
	Text string `json:"text"`
}

type SiriResponse struct {
	Reply string `json:"reply"`
}

type SiriAgentUseCaser interface {
	Handle(ctx context.Context, inputText string) (string, error)
}

type SiriHandler struct {
	uc SiriAgentUseCaser
}

func NewSiriHandler(uc SiriAgentUseCaser) *SiriHandler {
	return &SiriHandler{uc: uc}
}

func (h *SiriHandler) Register(g *echo.Group) {
	g.POST("/agent/siri", h.handle)
}

func (h *SiriHandler) handle(c echo.Context) error {
	var req SiriRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid json"})
	}
	text := strings.TrimSpace(req.Text)
	if text == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "text is required"})
	}
	if len(text) > maxSiriTextLength {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "text is too long"})
	}
	reply, err := h.uc.Handle(c.Request().Context(), text)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, SiriResponse{Reply: reply})
}
