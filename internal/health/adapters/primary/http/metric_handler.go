package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rafaelsoares/alfredo/internal/health/domain"
)

type MetricUseCaser interface {
	Import(ctx context.Context, metrics []domain.DailyMetric, payload string, importedAt time.Time) (int, error)
	List(ctx context.Context, metricType string, from, to time.Time) ([]domain.DailyMetric, error)
}

type MetricHandler struct {
	uc MetricUseCaser
}

func NewMetricHandler(uc MetricUseCaser) *MetricHandler {
	return &MetricHandler{uc: uc}
}

func (h *MetricHandler) Register(g *echo.Group) {
	g.POST("/health/metrics/import", h.ImportMetrics)
	g.GET("/health/metrics", h.ListMetrics)
}

type metricImportResponse struct {
	Imported int `json:"imported"`
}

type dailyMetricResponse struct {
	ID          int                 `json:"id"`
	Date        string              `json:"date"`
	MetricType  string              `json:"metric_type"`
	Value       float64             `json:"value"`
	Unit        string              `json:"unit"`
	SleepStages *domain.SleepStages `json:"sleep_stages,omitempty"`
	CreatedAt   string              `json:"created_at"`
	UpdatedAt   string              `json:"updated_at"`
}

// Health Exporter format: {metricType: [{date, value, unit, stages}, ...], ...}.
type healthExporterMetric struct {
	Date   string      `json:"date"`
	Value  float64     `json:"value"`
	Unit   string      `json:"unit"`
	Stages interface{} `json:"stages"`
}

func (h *MetricHandler) ImportMetrics(c echo.Context) error {
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	// Parse Health Exporter JSON: map of metric arrays plus metadata objects such as exportInfo.
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid JSON format: %v", err)})
	}

	var metrics []domain.DailyMetric
	importedAt := time.Now().UTC()

	// Iterate all top-level keys except "exportInfo"
	for metricType, rawEntries := range payload {
		if metricType == "exportInfo" {
			continue
		}

		var entries []healthExporterMetric
		if err := json.Unmarshal(rawEntries, &entries); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": fmt.Sprintf("invalid metric section %q: expected an array of metric entries: %v", metricType, err),
			})
		}

		for _, entry := range entries {
			m := domain.DailyMetric{
				Date:       entry.Date,
				MetricType: metricType,
				Value:      entry.Value,
				Unit:       entry.Unit,
			}

			// Handle sleep stages for sleepTime metric
			if metricType == "sleepTime" && entry.Stages != nil {
				// Parse stages object
				stagesData, _ := json.Marshal(entry.Stages)
				var stages domain.SleepStages
				if err := json.Unmarshal(stagesData, &stages); err == nil {
					m.SleepStages = &stages
				}
			}

			metrics = append(metrics, m)
		}
	}

	count, err := h.uc.Import(c.Request().Context(), metrics, string(body), importedAt)
	if err != nil {
		return mapError(c, err)
	}

	return c.JSON(http.StatusOK, metricImportResponse{Imported: count})
}

func (h *MetricHandler) ListMetrics(c echo.Context) error {
	metricType := c.QueryParam("type")
	fromStr := c.QueryParam("from")
	toStr := c.QueryParam("to")

	if metricType == "" || fromStr == "" || toStr == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "type, from, and to query params required"})
	}

	from, err := time.Parse("2006-01-02", fromStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "from date format must be YYYY-MM-DD"})
	}

	to, err := time.Parse("2006-01-02", toStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "to date format must be YYYY-MM-DD"})
	}

	metrics, err := h.uc.List(c.Request().Context(), metricType, from, to)
	if err != nil {
		return mapError(c, err)
	}

	responses := make([]dailyMetricResponse, 0, len(metrics))
	for _, m := range metrics {
		responses = append(responses, dailyMetricResponse{
			ID:          m.ID,
			Date:        m.Date,
			MetricType:  m.MetricType,
			Value:       m.Value,
			Unit:        m.Unit,
			SleepStages: m.SleepStages,
			CreatedAt:   m.CreatedAt.Format(time.RFC3339Nano),
			UpdatedAt:   m.UpdatedAt.Format(time.RFC3339Nano),
		})
	}

	return c.JSON(http.StatusOK, responses)
}
