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

// --- request types ---

type heartRateRequest struct {
	Avg      *float64 `json:"avg"`
	Max      *float64 `json:"max"`
	Zone1Pct *float64 `json:"zone1_pct"`
	Zone2Pct *float64 `json:"zone2_pct"`
	Zone3Pct *float64 `json:"zone3_pct"`
	Zone4Pct *float64 `json:"zone4_pct"`
	Zone5Pct *float64 `json:"zone5_pct"`
}

type cardioRequest struct {
	DistanceMeters  *float64 `json:"distance_meters"`
	AvgPaceSecPerKm *float64 `json:"avg_pace_sec_per_km"`
}

type setRequest struct {
	SetNumber    int      `json:"set_number"    validate:"required,gt=0"`
	Reps         *int     `json:"reps"`
	WeightKg     *float64 `json:"weight_kg"`
	DurationSecs *int     `json:"duration_secs"`
	Notes        *string  `json:"notes"`
}

type exerciseRequest struct {
	Name      string       `json:"name"      validate:"required,min=1"`
	Equipment *string      `json:"equipment"`
	OrderIdx  int          `json:"order_idx"`
	Sets      []setRequest `json:"sets"      validate:"required,min=1,dive"`
}

type strengthRequest struct {
	Exercises []exerciseRequest `json:"exercises" validate:"required,min=1,dive"`
}

type workoutRequest struct {
	ExternalID      string           `json:"external_id"      validate:"required"`
	Type            string           `json:"type"             validate:"required,min=1,max=50"`
	StartedAt       string           `json:"started_at"       validate:"required"`
	DurationSeconds int              `json:"duration_seconds" validate:"required,gt=0"`
	ActiveCalories  float64          `json:"active_calories"  validate:"gte=0"`
	TotalCalories   float64          `json:"total_calories"   validate:"gte=0"`
	HeartRate       *heartRateRequest `json:"heart_rate"`
	Cardio          *cardioRequest   `json:"cardio"`
	Strength        *strengthRequest `json:"strength"`
	Source          string           `json:"source"           validate:"required,min=1,max=50"`
}

func parseWorkoutRequest(req workoutRequest) (domain.Workout, error) {
	startedAt, err := time.Parse(time.RFC3339, req.StartedAt)
	if err != nil {
		return domain.Workout{}, err
	}
	w := domain.Workout{
		ExternalID:      req.ExternalID,
		Type:            req.Type,
		StartedAt:       startedAt,
		DurationSeconds: req.DurationSeconds,
		ActiveCalories:  req.ActiveCalories,
		TotalCalories:   req.TotalCalories,
		Source:          req.Source,
	}
	if req.HeartRate != nil {
		w.HeartRate = &domain.WorkoutHeartRate{
			Avg:      req.HeartRate.Avg,
			Max:      req.HeartRate.Max,
			Zone1Pct: req.HeartRate.Zone1Pct,
			Zone2Pct: req.HeartRate.Zone2Pct,
			Zone3Pct: req.HeartRate.Zone3Pct,
			Zone4Pct: req.HeartRate.Zone4Pct,
			Zone5Pct: req.HeartRate.Zone5Pct,
		}
	}
	if req.Cardio != nil {
		w.Cardio = &domain.CardioData{
			DistanceMeters:  req.Cardio.DistanceMeters,
			AvgPaceSecPerKm: req.Cardio.AvgPaceSecPerKm,
		}
	}
	if req.Strength != nil {
		exercises := make([]domain.WorkoutExercise, 0, len(req.Strength.Exercises))
		for _, er := range req.Strength.Exercises {
			sets := make([]domain.WorkoutSet, 0, len(er.Sets))
			for _, sr := range er.Sets {
				sets = append(sets, domain.WorkoutSet{
					SetNumber:    sr.SetNumber,
					Reps:         sr.Reps,
					WeightKg:     sr.WeightKg,
					DurationSecs: sr.DurationSecs,
					Notes:        sr.Notes,
				})
			}
			exercises = append(exercises, domain.WorkoutExercise{
				Name:      er.Name,
				Equipment: er.Equipment,
				OrderIdx:  er.OrderIdx,
				Sets:      sets,
			})
		}
		w.Strength = &domain.StrengthData{Exercises: exercises}
	}
	return w, nil
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

type heartRateResponse struct {
	Avg      *float64 `json:"avg,omitempty"`
	Max      *float64 `json:"max,omitempty"`
	Zone1Pct *float64 `json:"zone1_pct,omitempty"`
	Zone2Pct *float64 `json:"zone2_pct,omitempty"`
	Zone3Pct *float64 `json:"zone3_pct,omitempty"`
	Zone4Pct *float64 `json:"zone4_pct,omitempty"`
	Zone5Pct *float64 `json:"zone5_pct,omitempty"`
}

type cardioResponse struct {
	DistanceMeters  *float64 `json:"distance_meters,omitempty"`
	AvgPaceSecPerKm *float64 `json:"avg_pace_sec_per_km,omitempty"`
}

type setResponse struct {
	ID           string   `json:"id"`
	SetNumber    int      `json:"set_number"`
	Reps         *int     `json:"reps,omitempty"`
	WeightKg     *float64 `json:"weight_kg,omitempty"`
	DurationSecs *int     `json:"duration_secs,omitempty"`
	Notes        *string  `json:"notes,omitempty"`
}

type exerciseResponse struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Equipment *string       `json:"equipment,omitempty"`
	OrderIdx  int           `json:"order_idx"`
	Sets      []setResponse `json:"sets"`
}

type strengthResponse struct {
	Exercises []exerciseResponse `json:"exercises"`
}

type workoutResponse struct {
	ID              string             `json:"id"`
	ExternalID      string             `json:"external_id"`
	Type            string             `json:"type"`
	StartedAt       string             `json:"started_at"`
	DurationSeconds int                `json:"duration_seconds"`
	ActiveCalories  float64            `json:"active_calories"`
	TotalCalories   float64            `json:"total_calories"`
	HeartRate       *heartRateResponse `json:"heart_rate,omitempty"`
	Cardio          *cardioResponse    `json:"cardio,omitempty"`
	Strength        *strengthResponse  `json:"strength,omitempty"`
	Source          string             `json:"source"`
	CreatedAt       string             `json:"created_at"`
}

func toWorkoutResponse(w domain.Workout) workoutResponse {
	resp := workoutResponse{
		ID:              w.ID,
		ExternalID:      w.ExternalID,
		Type:            w.Type,
		StartedAt:       w.StartedAt.Format(time.RFC3339),
		DurationSeconds: w.DurationSeconds,
		ActiveCalories:  w.ActiveCalories,
		TotalCalories:   w.TotalCalories,
		Source:          w.Source,
		CreatedAt:       w.CreatedAt.Format(time.RFC3339),
	}
	if w.HeartRate != nil {
		resp.HeartRate = &heartRateResponse{
			Avg:      w.HeartRate.Avg,
			Max:      w.HeartRate.Max,
			Zone1Pct: w.HeartRate.Zone1Pct,
			Zone2Pct: w.HeartRate.Zone2Pct,
			Zone3Pct: w.HeartRate.Zone3Pct,
			Zone4Pct: w.HeartRate.Zone4Pct,
			Zone5Pct: w.HeartRate.Zone5Pct,
		}
	}
	if w.Cardio != nil {
		resp.Cardio = &cardioResponse{
			DistanceMeters:  w.Cardio.DistanceMeters,
			AvgPaceSecPerKm: w.Cardio.AvgPaceSecPerKm,
		}
	}
	if w.Strength != nil {
		exercises := make([]exerciseResponse, 0, len(w.Strength.Exercises))
		for _, ex := range w.Strength.Exercises {
			sets := make([]setResponse, 0, len(ex.Sets))
			for _, s := range ex.Sets {
				sets = append(sets, setResponse{
					ID:           s.ID,
					SetNumber:    s.SetNumber,
					Reps:         s.Reps,
					WeightKg:     s.WeightKg,
					DurationSecs: s.DurationSecs,
					Notes:        s.Notes,
				})
			}
			exercises = append(exercises, exerciseResponse{
				ID:        ex.ID,
				Name:      ex.Name,
				Equipment: ex.Equipment,
				OrderIdx:  ex.OrderIdx,
				Sets:      sets,
			})
		}
		resp.Strength = &strengthResponse{Exercises: exercises}
	}
	return resp
}
