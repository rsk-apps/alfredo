package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rafaelsoares/alfredo/internal/health/domain"
)

type metricUseCaseStub struct {
	importFn func(context.Context, []domain.DailyMetric, string, time.Time) (int, error)
	listFn   func(context.Context, string, time.Time, time.Time) ([]domain.DailyMetric, error)
}

func (s *metricUseCaseStub) Import(ctx context.Context, metrics []domain.DailyMetric, payload string, importedAt time.Time) (int, error) {
	if s.importFn != nil {
		return s.importFn(ctx, metrics, payload, importedAt)
	}
	return len(metrics), nil
}

func (s *metricUseCaseStub) List(ctx context.Context, metricType string, from, to time.Time) ([]domain.DailyMetric, error) {
	if s.listFn != nil {
		return s.listFn(ctx, metricType, from, to)
	}
	return nil, nil
}

func doMetricRequest(t *testing.T, method, path, body string, uc MetricUseCaser) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	h := NewMetricHandler(uc)
	switch method {
	case http.MethodPost:
		if err := h.ImportMetrics(c); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case http.MethodGet:
		if err := h.ListMetrics(c); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	default:
		t.Fatalf("unsupported method: %s", method)
	}
	return rec
}

func TestMetricHandlerImportHappyPath(t *testing.T) {
	var captured []domain.DailyMetric
	stub := &metricUseCaseStub{
		importFn: func(_ context.Context, metrics []domain.DailyMetric, _ string, _ time.Time) (int, error) {
			captured = metrics
			return len(metrics), nil
		},
	}

	rec := doMetricRequest(t, http.MethodPost, "/api/v1/health/metrics/import",
		`{"weight":[{"date":"2026-04-18","value":80.5,"unit":"kg"}]}`, stub)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if len(captured) != 1 {
		t.Fatalf("captured %d metrics, want 1", len(captured))
	}
	if captured[0].MetricType != "weight" || captured[0].Value != 80.5 || captured[0].Date != "2026-04-18" {
		t.Fatalf("captured = %#v", captured[0])
	}
}

func TestMetricHandlerImportSkipsExportInfo(t *testing.T) {
	var captured []domain.DailyMetric
	stub := &metricUseCaseStub{
		importFn: func(_ context.Context, metrics []domain.DailyMetric, _ string, _ time.Time) (int, error) {
			captured = metrics
			return len(metrics), nil
		},
	}

	rec := doMetricRequest(t, http.MethodPost, "/api/v1/health/metrics/import",
		`{"weight":[{"date":"2026-04-18","value":80.5,"unit":"kg"}],"exportInfo":[{"date":"2026-04-18","value":1,"unit":""}]}`, stub)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if len(captured) != 1 {
		t.Fatalf("captured %d metrics, want 1 (exportInfo must be skipped)", len(captured))
	}
}

func TestMetricHandlerImportSleepStagesAreParsed(t *testing.T) {
	var captured []domain.DailyMetric
	stub := &metricUseCaseStub{
		importFn: func(_ context.Context, metrics []domain.DailyMetric, _ string, _ time.Time) (int, error) {
			captured = metrics
			return len(metrics), nil
		},
	}

	rec := doMetricRequest(t, http.MethodPost, "/api/v1/health/metrics/import",
		`{"sleepTime":[{"date":"2026-04-18","value":260,"unit":"min","stages":{"awake":30,"core":120,"deep":60,"rem":45,"unspecified":5}}]}`, stub)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if len(captured) != 1 {
		t.Fatalf("captured %d metrics, want 1", len(captured))
	}
	if captured[0].SleepStages == nil {
		t.Fatal("SleepStages = nil, want non-nil for sleepTime metric")
	}
	if captured[0].SleepStages.Awake != 30 || captured[0].SleepStages.Core != 120 || captured[0].SleepStages.Deep != 60 {
		t.Fatalf("SleepStages = %#v", captured[0].SleepStages)
	}
}

func TestMetricHandlerImportRejectsInvalidJSON(t *testing.T) {
	rec := doMetricRequest(t, http.MethodPost, "/api/v1/health/metrics/import", "not-json", &metricUseCaseStub{})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for invalid JSON", rec.Code)
	}
}

func TestMetricHandlerImportUseCaseError(t *testing.T) {
	stub := &metricUseCaseStub{
		importFn: func(_ context.Context, _ []domain.DailyMetric, _ string, _ time.Time) (int, error) {
			return 0, errors.New("db failure")
		},
	}
	rec := doMetricRequest(t, http.MethodPost, "/api/v1/health/metrics/import",
		`{"weight":[{"date":"2026-04-18","value":80.5,"unit":"kg"}]}`, stub)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestMetricHandlerListRequiresAllParams(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"missing type", "/api/v1/health/metrics?from=2026-04-01&to=2026-04-30"},
		{"missing from", "/api/v1/health/metrics?type=weight&to=2026-04-30"},
		{"missing to", "/api/v1/health/metrics?type=weight&from=2026-04-01"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := doMetricRequest(t, http.MethodGet, tc.path, "", &metricUseCaseStub{})
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
		})
	}
}

func TestMetricHandlerListRejectsBadDates(t *testing.T) {
	t.Run("bad from", func(t *testing.T) {
		rec := doMetricRequest(t, http.MethodGet, "/api/v1/health/metrics?type=weight&from=not-a-date&to=2026-04-30", "", &metricUseCaseStub{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
	})
	t.Run("bad to", func(t *testing.T) {
		rec := doMetricRequest(t, http.MethodGet, "/api/v1/health/metrics?type=weight&from=2026-04-01&to=not-a-date", "", &metricUseCaseStub{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
	})
}

func TestMetricHandlerListUseCaseError(t *testing.T) {
	stub := &metricUseCaseStub{
		listFn: func(_ context.Context, _ string, _, _ time.Time) ([]domain.DailyMetric, error) {
			return nil, errors.New("db failure")
		},
	}
	rec := doMetricRequest(t, http.MethodGet, "/api/v1/health/metrics?type=weight&from=2026-04-01&to=2026-04-30", "", stub)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestMetricHandlerListHappyPath(t *testing.T) {
	now := time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC)
	stub := &metricUseCaseStub{
		listFn: func(_ context.Context, metricType string, _, _ time.Time) ([]domain.DailyMetric, error) {
			return []domain.DailyMetric{
				{ID: 1, Date: "2026-04-18", MetricType: metricType, Value: 80.5, Unit: "kg", CreatedAt: now, UpdatedAt: now},
			}, nil
		},
	}
	rec := doMetricRequest(t, http.MethodGet, "/api/v1/health/metrics?type=weight&from=2026-04-01&to=2026-04-30", "", stub)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"weight"`) || !strings.Contains(body, `"2026-04-18"`) {
		t.Fatalf("body = %s, want metric data", body)
	}
}
