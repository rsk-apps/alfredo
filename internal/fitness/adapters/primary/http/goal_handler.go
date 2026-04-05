// internal/fitness/adapters/primary/http/goal_handler.go
package http

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
	fitnesssvc "github.com/rafaelsoares/alfredo/internal/fitness/service"
	"github.com/rafaelsoares/alfredo/internal/logger"
)

type GoalServicer interface {
	CreateGoal(ctx context.Context, in fitnesssvc.CreateGoalInput) (*domain.Goal, error)
	GetGoal(ctx context.Context, id string) (*domain.Goal, error)
	ListGoals(ctx context.Context) ([]domain.Goal, error)
	UpdateGoal(ctx context.Context, id string, in fitnesssvc.UpdateGoalInput) (*domain.Goal, error)
	DeleteGoal(ctx context.Context, id string) error
	AchieveGoal(ctx context.Context, id string) (*domain.Goal, error)
}

type GoalHandler struct{ svc GoalServicer }

func NewGoalHandler(svc GoalServicer) *GoalHandler { return &GoalHandler{svc: svc} }

func (h *GoalHandler) Register(g *echo.Group) {
	g.POST("/fitness/goals", h.CreateGoal)
	g.GET("/fitness/goals", h.ListGoals)
	g.GET("/fitness/goals/:id", h.GetGoal)
	g.PUT("/fitness/goals/:id", h.UpdateGoal)
	g.DELETE("/fitness/goals/:id", h.DeleteGoal)
	g.POST("/fitness/goals/:id/achieve", h.AchieveGoal)
}

func (h *GoalHandler) CreateGoal(c echo.Context) error {
	var req struct {
		Name        string   `json:"name"         validate:"required,min=1,max=200"`
		Description *string  `json:"description"  validate:"omitempty,max=1000"`
		TargetValue *float64 `json:"target_value"`
		TargetUnit  *string  `json:"target_unit"  validate:"omitempty,max=50"`
		Deadline    *string  `json:"deadline"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, newErrorResponse("invalid_request_body", "Request body is invalid JSON", nil))
	}
	if !validateRequest(c, &req) {
		return nil
	}
	in := fitnesssvc.CreateGoalInput{
		Name:        req.Name,
		Description: req.Description,
		TargetValue: req.TargetValue,
		TargetUnit:  req.TargetUnit,
	}
	if req.Deadline != nil {
		t, err := time.Parse("2006-01-02", *req.Deadline)
		if err != nil {
			return c.JSON(http.StatusBadRequest, newErrorResponse("validation_failed", "Request validation failed",
				[]fieldError{{Field: "deadline", Issue: "must be YYYY-MM-DD format"}}))
		}
		in.Deadline = &t
	}
	g, err := h.svc.CreateGoal(c.Request().Context(), in)
	if err != nil {
		return mapError(c, err)
	}
	logger.FromEcho(c).Info("fitness goal created", zap.String("goal_id", g.ID))
	return c.JSON(http.StatusCreated, toGoalResponse(*g))
}

func (h *GoalHandler) ListGoals(c echo.Context) error {
	goals, err := h.svc.ListGoals(c.Request().Context())
	if err != nil {
		return mapError(c, err)
	}
	resp := make([]goalResponse, 0, len(goals))
	for _, g := range goals {
		resp = append(resp, toGoalResponse(g))
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *GoalHandler) GetGoal(c echo.Context) error {
	id, ok := parseUUID(c, "id")
	if !ok {
		return nil
	}
	g, err := h.svc.GetGoal(c.Request().Context(), id)
	if err != nil {
		return mapError(c, err)
	}
	return c.JSON(http.StatusOK, toGoalResponse(*g))
}

func (h *GoalHandler) UpdateGoal(c echo.Context) error {
	id, ok := parseUUID(c, "id")
	if !ok {
		return nil
	}
	var req struct {
		Name        *string  `json:"name"         validate:"omitempty,min=1,max=200"`
		Description *string  `json:"description"  validate:"omitempty,max=1000"`
		TargetValue *float64 `json:"target_value"`
		TargetUnit  *string  `json:"target_unit"  validate:"omitempty,max=50"`
		Deadline    *string  `json:"deadline"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, newErrorResponse("invalid_request_body", "Request body is invalid JSON", nil))
	}
	if !validateRequest(c, &req) {
		return nil
	}
	in := fitnesssvc.UpdateGoalInput{
		Name:        req.Name,
		Description: req.Description,
		TargetValue: req.TargetValue,
		TargetUnit:  req.TargetUnit,
	}
	if req.Deadline != nil {
		t, err := time.Parse("2006-01-02", *req.Deadline)
		if err != nil {
			return c.JSON(http.StatusBadRequest, newErrorResponse("validation_failed", "Request validation failed",
				[]fieldError{{Field: "deadline", Issue: "must be YYYY-MM-DD format"}}))
		}
		in.Deadline = &t
	}
	g, err := h.svc.UpdateGoal(c.Request().Context(), id, in)
	if err != nil {
		return mapError(c, err)
	}
	return c.JSON(http.StatusOK, toGoalResponse(*g))
}

func (h *GoalHandler) DeleteGoal(c echo.Context) error {
	id, ok := parseUUID(c, "id")
	if !ok {
		return nil
	}
	if err := h.svc.DeleteGoal(c.Request().Context(), id); err != nil {
		return mapError(c, err)
	}
	logger.FromEcho(c).Info("fitness goal deleted", zap.String("goal_id", id))
	return c.NoContent(http.StatusNoContent)
}

func (h *GoalHandler) AchieveGoal(c echo.Context) error {
	id, ok := parseUUID(c, "id")
	if !ok {
		return nil
	}
	g, err := h.svc.AchieveGoal(c.Request().Context(), id)
	if err != nil {
		return mapError(c, err)
	}
	logger.FromEcho(c).Info("fitness goal achieved", zap.String("goal_id", id))
	return c.JSON(http.StatusOK, toGoalResponse(*g))
}

// --- response types ---

type goalResponse struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description *string  `json:"description,omitempty"`
	TargetValue *float64 `json:"target_value,omitempty"`
	TargetUnit  *string  `json:"target_unit,omitempty"`
	Deadline    *string  `json:"deadline,omitempty"`
	AchievedAt  *string  `json:"achieved_at,omitempty"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

func toGoalResponse(g domain.Goal) goalResponse {
	r := goalResponse{
		ID:          g.ID,
		Name:        g.Name,
		Description: g.Description,
		TargetValue: g.TargetValue,
		TargetUnit:  g.TargetUnit,
		CreatedAt:   g.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   g.UpdatedAt.Format(time.RFC3339),
	}
	if g.Deadline != nil {
		s := g.Deadline.Format("2006-01-02")
		r.Deadline = &s
	}
	if g.AchievedAt != nil {
		s := g.AchievedAt.Format(time.RFC3339)
		r.AchievedAt = &s
	}
	return r
}
