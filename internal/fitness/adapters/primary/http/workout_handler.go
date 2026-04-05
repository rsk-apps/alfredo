// internal/fitness/adapters/primary/http/workout_handler.go
package http

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
	"github.com/rafaelsoares/alfredo/internal/logger"
)

type WorkoutServicer interface {
	IngestWorkout(ctx context.Context, w domain.Workout) (*domain.Workout, error)
	IngestWorkoutBatch(ctx context.Context, ws []domain.Workout) ([]domain.Workout, error)
	GetByID(ctx context.Context, id string) (*domain.Workout, error)
	List(ctx context.Context, from, to *time.Time) ([]domain.Workout, error)
	Delete(ctx context.Context, id string) error
}

type WorkoutHandler struct{ svc WorkoutServicer }

func NewWorkoutHandler(svc WorkoutServicer) *WorkoutHandler { return &WorkoutHandler{svc: svc} }

func (h *WorkoutHandler) Register(g *echo.Group) {
	g.POST("/fitness/workouts", h.IngestWorkout)
	g.POST("/fitness/workouts/batch", h.IngestWorkoutBatch)
	g.GET("/fitness/workouts", h.ListWorkouts)
	g.GET("/fitness/workouts/:id", h.GetWorkout)
	g.DELETE("/fitness/workouts/:id", h.DeleteWorkout)
}

type workoutRequest struct {
	ExternalID      string   `json:"external_id"        validate:"required"`
	Type            string   `json:"type"               validate:"required,min=1,max=50"`
	StartedAt       string   `json:"started_at"         validate:"required"`
	DurationSeconds int      `json:"duration_seconds"   validate:"required,gt=0"`
	ActiveCalories  float64  `json:"active_calories"    validate:"gte=0"`
	TotalCalories   float64  `json:"total_calories"     validate:"gte=0"`
	DistanceMeters  *float64 `json:"distance_meters"`
	AvgPaceSecPerKm *float64 `json:"avg_pace_sec_per_km"`
	AvgHeartRate    *float64 `json:"avg_heart_rate"`
	MaxHeartRate    *float64 `json:"max_heart_rate"`
	HRZone1Pct      *float64 `json:"hr_zone1_pct"`
	HRZone2Pct      *float64 `json:"hr_zone2_pct"`
	HRZone3Pct      *float64 `json:"hr_zone3_pct"`
	HRZone4Pct      *float64 `json:"hr_zone4_pct"`
	HRZone5Pct      *float64 `json:"hr_zone5_pct"`
	Source          string   `json:"source"             validate:"required,min=1,max=50"`
}

func parseWorkoutRequest(req workoutRequest) (domain.Workout, error) {
	startedAt, err := time.Parse(time.RFC3339, req.StartedAt)
	if err != nil {
		return domain.Workout{}, err
	}
	return domain.Workout{
		ExternalID:      req.ExternalID,
		Type:            req.Type,
		StartedAt:       startedAt,
		DurationSeconds: req.DurationSeconds,
		ActiveCalories:  req.ActiveCalories,
		TotalCalories:   req.TotalCalories,
		DistanceMeters:  req.DistanceMeters,
		AvgPaceSecPerKm: req.AvgPaceSecPerKm,
		AvgHeartRate:    req.AvgHeartRate,
		MaxHeartRate:    req.MaxHeartRate,
		HRZone1Pct:      req.HRZone1Pct,
		HRZone2Pct:      req.HRZone2Pct,
		HRZone3Pct:      req.HRZone3Pct,
		HRZone4Pct:      req.HRZone4Pct,
		HRZone5Pct:      req.HRZone5Pct,
		Source:          req.Source,
	}, nil
}

func (h *WorkoutHandler) IngestWorkout(c echo.Context) error {
	var req workoutRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, newErrorResponse("invalid_request_body", "Request body is invalid JSON", nil))
	}
	if !validateRequest(c, &req) {
		return nil
	}
	w, err := parseWorkoutRequest(req)
	if err != nil {
		return c.JSON(http.StatusBadRequest, newErrorResponse("validation_failed", "Request validation failed",
			[]fieldError{{Field: "started_at", Issue: "must be RFC3339 format"}}))
	}
	saved, err := h.svc.IngestWorkout(c.Request().Context(), w)
	if err != nil {
		return mapError(c, err)
	}
	logger.FromEcho(c).Info("workout ingested", zap.String("workout_id", saved.ID))
	return c.JSON(http.StatusCreated, toWorkoutResponse(*saved))
}

func (h *WorkoutHandler) IngestWorkoutBatch(c echo.Context) error {
	var reqs []workoutRequest
	if err := c.Bind(&reqs); err != nil {
		return c.JSON(http.StatusBadRequest, newErrorResponse("invalid_request_body", "Request body is invalid JSON", nil))
	}
	var workouts []domain.Workout
	for _, req := range reqs {
		if !validateRequest(c, &req) {
			return nil
		}
		w, err := parseWorkoutRequest(req)
		if err != nil {
			return c.JSON(http.StatusBadRequest, newErrorResponse("validation_failed", "Request validation failed",
				[]fieldError{{Field: "started_at", Issue: "must be RFC3339 format"}}))
		}
		workouts = append(workouts, w)
	}
	saved, err := h.svc.IngestWorkoutBatch(c.Request().Context(), workouts)
	if err != nil {
		return mapError(c, err)
	}
	logger.FromEcho(c).Info("workout batch ingested", zap.Int("count", len(saved)))
	resp := make([]workoutResponse, 0, len(saved))
	for _, w := range saved {
		resp = append(resp, toWorkoutResponse(w))
	}
	return c.JSON(http.StatusCreated, resp)
}

func (h *WorkoutHandler) ListWorkouts(c echo.Context) error {
	from, to := parseDateRangeParams(c)
	ws, err := h.svc.List(c.Request().Context(), from, to)
	if err != nil {
		return mapError(c, err)
	}
	resp := make([]workoutResponse, 0, len(ws))
	for _, w := range ws {
		resp = append(resp, toWorkoutResponse(w))
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *WorkoutHandler) GetWorkout(c echo.Context) error {
	id, ok := parseUUID(c, "id")
	if !ok {
		return nil
	}
	w, err := h.svc.GetByID(c.Request().Context(), id)
	if err != nil {
		return mapError(c, err)
	}
	return c.JSON(http.StatusOK, toWorkoutResponse(*w))
}

func (h *WorkoutHandler) DeleteWorkout(c echo.Context) error {
	id, ok := parseUUID(c, "id")
	if !ok {
		return nil
	}
	if err := h.svc.Delete(c.Request().Context(), id); err != nil {
		return mapError(c, err)
	}
	logger.FromEcho(c).Info("workout deleted", zap.String("workout_id", id))
	return c.NoContent(http.StatusNoContent)
}

// parseDateRangeParams reads optional ?from= and ?to= query params as RFC3339 strings.
func parseDateRangeParams(c echo.Context) (*time.Time, *time.Time) {
	var from, to *time.Time
	if s := c.QueryParam("from"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			from = &t
		}
	}
	if s := c.QueryParam("to"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			to = &t
		}
	}
	return from, to
}

// --- response types ---

type workoutResponse struct {
	ID              string   `json:"id"`
	ExternalID      string   `json:"external_id"`
	Type            string   `json:"type"`
	StartedAt       string   `json:"started_at"`
	DurationSeconds int      `json:"duration_seconds"`
	ActiveCalories  float64  `json:"active_calories"`
	TotalCalories   float64  `json:"total_calories"`
	DistanceMeters  *float64 `json:"distance_meters,omitempty"`
	AvgPaceSecPerKm *float64 `json:"avg_pace_sec_per_km,omitempty"`
	AvgHeartRate    *float64 `json:"avg_heart_rate,omitempty"`
	MaxHeartRate    *float64 `json:"max_heart_rate,omitempty"`
	HRZone1Pct      *float64 `json:"hr_zone1_pct,omitempty"`
	HRZone2Pct      *float64 `json:"hr_zone2_pct,omitempty"`
	HRZone3Pct      *float64 `json:"hr_zone3_pct,omitempty"`
	HRZone4Pct      *float64 `json:"hr_zone4_pct,omitempty"`
	HRZone5Pct      *float64 `json:"hr_zone5_pct,omitempty"`
	Source          string   `json:"source"`
	CreatedAt       string   `json:"created_at"`
}

func toWorkoutResponse(w domain.Workout) workoutResponse {
	return workoutResponse{
		ID:              w.ID,
		ExternalID:      w.ExternalID,
		Type:            w.Type,
		StartedAt:       w.StartedAt.Format(time.RFC3339),
		DurationSeconds: w.DurationSeconds,
		ActiveCalories:  w.ActiveCalories,
		TotalCalories:   w.TotalCalories,
		DistanceMeters:  w.DistanceMeters,
		AvgPaceSecPerKm: w.AvgPaceSecPerKm,
		AvgHeartRate:    w.AvgHeartRate,
		MaxHeartRate:    w.MaxHeartRate,
		HRZone1Pct:      w.HRZone1Pct,
		HRZone2Pct:      w.HRZone2Pct,
		HRZone3Pct:      w.HRZone3Pct,
		HRZone4Pct:      w.HRZone4Pct,
		HRZone5Pct:      w.HRZone5Pct,
		Source:          w.Source,
		CreatedAt:       w.CreatedAt.Format(time.RFC3339),
	}
}
