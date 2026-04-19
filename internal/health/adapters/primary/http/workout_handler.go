package http

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rafaelsoares/alfredo/internal/health/domain"
)

type WorkoutUseCaser interface {
	Import(ctx context.Context, sessions []domain.WorkoutSession, payload string, importedAt time.Time) (int, error)
	List(ctx context.Context, from, to time.Time) ([]domain.WorkoutSession, error)
}

type WorkoutHandler struct {
	uc WorkoutUseCaser
}

func NewWorkoutHandler(uc WorkoutUseCaser) *WorkoutHandler {
	return &WorkoutHandler{uc: uc}
}

func (h *WorkoutHandler) Register(g *echo.Group) {
	g.POST("/health/workouts/import", h.ImportWorkouts)
	g.GET("/health/workouts", h.ListWorkouts)
}

type workoutImportResponse struct {
	Imported int `json:"imported"`
}

type workoutSessionResponse struct {
	ID                 int     `json:"id"`
	ActivityType       string  `json:"activity_type"`
	StartDate          string  `json:"start_date"`
	EndDate            string  `json:"end_date"`
	DurationSeconds    float64 `json:"duration_seconds"`
	ActiveCaloriesKcal *float64 `json:"active_calories_kcal,omitempty"`
	BasalCaloriesKcal  *float64 `json:"basal_calories_kcal,omitempty"`
	HRAvgBPM           *float64 `json:"hr_avg_bpm,omitempty"`
	HRMinBPM           *float64 `json:"hr_min_bpm,omitempty"`
	HRMaxBPM           *float64 `json:"hr_max_bpm,omitempty"`
	DistanceM          *float64 `json:"distance_m,omitempty"`
	Source             string  `json:"source"`
	CreatedAt          string  `json:"created_at"`
	UpdatedAt          string  `json:"updated_at"`
}

// Apple Health Exporter workouts format
type healthExporterWorkout struct {
	ActivityName string                 `json:"activityName"`
	StartDate    string                 `json:"startDate"`
	EndDate      string                 `json:"endDate"`
	Duration     float64                `json:"duration"`
	Statistics   map[string]interface{} `json:"statistics"`
}

type workoutStatistic struct {
	Value float64 `json:"value"`
}

func (h *WorkoutHandler) ImportWorkouts(c echo.Context) error {
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	// Parse workouts export JSON
	var payload struct {
		Workouts []healthExporterWorkout `json:"workouts"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid JSON format"})
	}

	var sessions []domain.WorkoutSession
	importedAt := time.Now().UTC()

	for _, w := range payload.Workouts {
		startDate, _ := time.Parse(time.RFC3339, w.StartDate)
		endDate, _ := time.Parse(time.RFC3339, w.EndDate)

		session := domain.WorkoutSession{
			ActivityType:    w.ActivityName,
			StartDate:       startDate,
			EndDate:         endDate,
			DurationSeconds: w.Duration,
			Source:          "Apple Watch",
		}

		// Map HKQuantityTypeIdentifier keys to domain fields
		if stats := w.Statistics; stats != nil {
			if val, ok := stats["HKQuantityTypeIdentifierActiveEnergyBurned"]; ok {
				if stat, ok := val.(map[string]interface{}); ok {
					if v, ok := stat["value"].(float64); ok {
						session.ActiveCaloriesKcal = &v
					}
				}
			}
			if val, ok := stats["HKQuantityTypeIdentifierBasalEnergyBurned"]; ok {
				if stat, ok := val.(map[string]interface{}); ok {
					if v, ok := stat["value"].(float64); ok {
						session.BasalCaloriesKcal = &v
					}
				}
			}
			if val, ok := stats["HKQuantityTypeIdentifierHeartRate"]; ok {
				if stat, ok := val.(map[string]interface{}); ok {
					if avgV, ok := stat["average"].(float64); ok {
						session.HRAvgBPM = &avgV
					}
					if minV, ok := stat["minimum"].(float64); ok {
						session.HRMinBPM = &minV
					}
					if maxV, ok := stat["maximum"].(float64); ok {
						session.HRMaxBPM = &maxV
					}
				}
			}
			if val, ok := stats["HKQuantityTypeIdentifierDistanceWalkingRunning"]; ok {
				if stat, ok := val.(map[string]interface{}); ok {
					if v, ok := stat["value"].(float64); ok {
						session.DistanceM = &v
					}
				}
			}
		}

		sessions = append(sessions, session)
	}

	count, err := h.uc.Import(c.Request().Context(), sessions, string(body), importedAt)
	if err != nil {
		return mapError(c, err)
	}

	return c.JSON(http.StatusOK, workoutImportResponse{Imported: count})
}

func (h *WorkoutHandler) ListWorkouts(c echo.Context) error {
	fromStr := c.QueryParam("from")
	toStr := c.QueryParam("to")

	if fromStr == "" || toStr == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "from and to query params required"})
	}

	from, err := time.Parse("2006-01-02", fromStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "from date format must be YYYY-MM-DD"})
	}

	to, err := time.Parse("2006-01-02", toStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "to date format must be YYYY-MM-DD"})
	}

	sessions, err := h.uc.List(c.Request().Context(), from, to)
	if err != nil {
		return mapError(c, err)
	}

	responses := make([]workoutSessionResponse, 0, len(sessions))
	for _, s := range sessions {
		responses = append(responses, workoutSessionResponse{
			ID:                 s.ID,
			ActivityType:       s.ActivityType,
			StartDate:          s.StartDate.Format(time.RFC3339Nano),
			EndDate:            s.EndDate.Format(time.RFC3339Nano),
			DurationSeconds:    s.DurationSeconds,
			ActiveCaloriesKcal: s.ActiveCaloriesKcal,
			BasalCaloriesKcal:  s.BasalCaloriesKcal,
			HRAvgBPM:           s.HRAvgBPM,
			HRMinBPM:           s.HRMinBPM,
			HRMaxBPM:           s.HRMaxBPM,
			DistanceM:          s.DistanceM,
			Source:             s.Source,
			CreatedAt:          s.CreatedAt.Format(time.RFC3339Nano),
			UpdatedAt:          s.UpdatedAt.Format(time.RFC3339Nano),
		})
	}

	return c.JSON(http.StatusOK, responses)
}
