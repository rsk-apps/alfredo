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

type workoutUseCaseStub struct {
	importFn func(context.Context, []domain.WorkoutSession, string, time.Time) (int, error)
	listFn   func(context.Context, time.Time, time.Time) ([]domain.WorkoutSession, error)
}

func (s *workoutUseCaseStub) Import(ctx context.Context, sessions []domain.WorkoutSession, payload string, importedAt time.Time) (int, error) {
	if s.importFn != nil {
		return s.importFn(ctx, sessions, payload, importedAt)
	}
	return len(sessions), nil
}

func (s *workoutUseCaseStub) List(ctx context.Context, from, to time.Time) ([]domain.WorkoutSession, error) {
	if s.listFn != nil {
		return s.listFn(ctx, from, to)
	}
	return nil, nil
}

func doWorkoutRequest(t *testing.T, method, path, body string, uc WorkoutUseCaser) *httptest.ResponseRecorder {
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
	c.SetPath(path)

	// Parse query params for GET requests
	if method == http.MethodGet {
		u := req.URL
		q := u.Query()
		for k, v := range q {
			if len(v) > 0 {
				_ = k
			}
		}
	}

	h := NewWorkoutHandler(uc)
	switch method {
	case http.MethodPost:
		if err := h.ImportWorkouts(c); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case http.MethodGet:
		if err := h.ListWorkouts(c); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	default:
		t.Fatalf("unsupported method: %s", method)
	}
	return rec
}

func TestWorkoutHandlerRejectsMalformedStartDate(t *testing.T) {
	rec := doWorkoutRequest(t, http.MethodPost, "/api/v1/health/workouts/import", `{
		"workouts": [
			{
				"activityName": "Running",
				"startDate": "not-a-date",
				"endDate": "2026-04-18T10:30:00Z",
				"duration": 1800
			}
		]
	}`, &workoutUseCaseStub{})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for malformed startDate", rec.Code)
	}
}

func TestWorkoutHandlerRejectsMalformedEndDate(t *testing.T) {
	rec := doWorkoutRequest(t, http.MethodPost, "/api/v1/health/workouts/import", `{
		"workouts": [
			{
				"activityName": "Running",
				"startDate": "2026-04-18T10:00:00Z",
				"endDate": "bad-end",
				"duration": 1800
			}
		]
	}`, &workoutUseCaseStub{})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for malformed endDate", rec.Code)
	}
}

func TestWorkoutHandlerListToDateIsInclusive(t *testing.T) {
	var capturedTo time.Time

	stub := &workoutUseCaseStub{
		listFn: func(_ context.Context, _, to time.Time) ([]domain.WorkoutSession, error) {
			capturedTo = to
			return nil, nil
		},
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health/workouts?from=2026-04-01&to=2026-04-30", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/v1/health/workouts")
	c.QueryParams().Set("from", "2026-04-01")
	c.QueryParams().Set("to", "2026-04-30")

	h := NewWorkoutHandler(stub)
	if err := h.ListWorkouts(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	// to should be end of 2026-04-30, not midnight (start of day)
	if capturedTo.Day() != 30 || capturedTo.Month() != 4 || capturedTo.Year() != 2026 {
		t.Fatalf("to date = %v, want 2026-04-30", capturedTo)
	}
	if capturedTo.Hour() == 0 && capturedTo.Minute() == 0 && capturedTo.Second() == 0 {
		t.Fatalf("to = %v is midnight (start of day), want end of day so workouts on April 30 are included", capturedTo)
	}
}

func TestWorkoutHandlerImportHappyPathWithStatistics(t *testing.T) {
	var captured []domain.WorkoutSession
	stub := &workoutUseCaseStub{
		importFn: func(_ context.Context, sessions []domain.WorkoutSession, _ string, _ time.Time) (int, error) {
			captured = sessions
			return len(sessions), nil
		},
	}

	rec := doWorkoutRequest(t, http.MethodPost, "/api/v1/health/workouts/import", `{
		"workouts": [{
			"activityName": "Running",
			"startDate": "2026-04-18T10:00:00Z",
			"endDate": "2026-04-18T10:30:00Z",
			"duration": 1800,
			"statistics": {
				"HKQuantityTypeIdentifierActiveEnergyBurned": {"value": 350.0},
				"HKQuantityTypeIdentifierBasalEnergyBurned": {"value": 100.0},
				"HKQuantityTypeIdentifierHeartRate": {"average": 155.0, "minimum": 120.0, "maximum": 180.0},
				"HKQuantityTypeIdentifierDistanceWalkingRunning": {"value": 5000.0}
			}
		}]
	}`, stub)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if len(captured) != 1 {
		t.Fatalf("captured %d sessions, want 1", len(captured))
	}
	s := captured[0]
	if s.ActivityType != "Running" {
		t.Fatalf("ActivityType = %s, want Running", s.ActivityType)
	}
	if s.ActiveCaloriesKcal == nil || *s.ActiveCaloriesKcal != 350.0 {
		t.Fatalf("ActiveCaloriesKcal = %v, want 350.0", s.ActiveCaloriesKcal)
	}
	if s.BasalCaloriesKcal == nil || *s.BasalCaloriesKcal != 100.0 {
		t.Fatalf("BasalCaloriesKcal = %v, want 100.0", s.BasalCaloriesKcal)
	}
	if s.HRAvgBPM == nil || *s.HRAvgBPM != 155.0 {
		t.Fatalf("HRAvgBPM = %v, want 155.0", s.HRAvgBPM)
	}
	if s.HRMinBPM == nil || *s.HRMinBPM != 120.0 {
		t.Fatalf("HRMinBPM = %v, want 120.0", s.HRMinBPM)
	}
	if s.HRMaxBPM == nil || *s.HRMaxBPM != 180.0 {
		t.Fatalf("HRMaxBPM = %v, want 180.0", s.HRMaxBPM)
	}
	if s.DistanceM == nil || *s.DistanceM != 5000.0 {
		t.Fatalf("DistanceM = %v, want 5000.0", s.DistanceM)
	}
}

func TestWorkoutHandlerImportRejectsInvalidJSON(t *testing.T) {
	rec := doWorkoutRequest(t, http.MethodPost, "/api/v1/health/workouts/import", "not-json", &workoutUseCaseStub{})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for invalid JSON", rec.Code)
	}
}

func TestWorkoutHandlerImportEmptyWorkoutsSucceeds(t *testing.T) {
	rec := doWorkoutRequest(t, http.MethodPost, "/api/v1/health/workouts/import", `{"workouts":[]}`, &workoutUseCaseStub{})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 for empty workouts list", rec.Code)
	}
}

func TestWorkoutHandlerImportUseCaseError(t *testing.T) {
	stub := &workoutUseCaseStub{
		importFn: func(_ context.Context, _ []domain.WorkoutSession, _ string, _ time.Time) (int, error) {
			return 0, errors.New("db failure")
		},
	}
	rec := doWorkoutRequest(t, http.MethodPost, "/api/v1/health/workouts/import", `{
		"workouts": [{"activityName":"Running","startDate":"2026-04-18T10:00:00Z","endDate":"2026-04-18T10:30:00Z","duration":1800}]
	}`, stub)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestWorkoutHandlerListRequiresFromAndTo(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"missing both", "/api/v1/health/workouts"},
		{"missing from", "/api/v1/health/workouts?to=2026-04-30"},
		{"missing to", "/api/v1/health/workouts?from=2026-04-01"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := doWorkoutRequest(t, http.MethodGet, tc.path, "", &workoutUseCaseStub{})
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
		})
	}
}

func TestWorkoutHandlerListRejectsBadDates(t *testing.T) {
	t.Run("bad from", func(t *testing.T) {
		rec := doWorkoutRequest(t, http.MethodGet, "/api/v1/health/workouts?from=not-a-date&to=2026-04-30", "", &workoutUseCaseStub{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
	})
	t.Run("bad to", func(t *testing.T) {
		rec := doWorkoutRequest(t, http.MethodGet, "/api/v1/health/workouts?from=2026-04-01&to=not-a-date", "", &workoutUseCaseStub{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
	})
}

func TestWorkoutHandlerListUseCaseError(t *testing.T) {
	stub := &workoutUseCaseStub{
		listFn: func(_ context.Context, _, _ time.Time) ([]domain.WorkoutSession, error) {
			return nil, errors.New("db failure")
		},
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health/workouts?from=2026-04-01&to=2026-04-30", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.QueryParams().Set("from", "2026-04-01")
	c.QueryParams().Set("to", "2026-04-30")

	h := NewWorkoutHandler(stub)
	if err := h.ListWorkouts(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestWorkoutHandlerListReturnsSessionsResponse(t *testing.T) {
	now := time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)
	active := 350.0
	stub := &workoutUseCaseStub{
		listFn: func(_ context.Context, _, _ time.Time) ([]domain.WorkoutSession, error) {
			return []domain.WorkoutSession{
				{
					ID:                 1,
					ActivityType:       "Running",
					StartDate:          now,
					EndDate:            now.Add(30 * time.Minute),
					DurationSeconds:    1800,
					ActiveCaloriesKcal: &active,
					Source:             "Apple Watch",
					CreatedAt:          now,
					UpdatedAt:          now,
				},
			}, nil
		},
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health/workouts?from=2026-04-01&to=2026-04-30", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.QueryParams().Set("from", "2026-04-01")
	c.QueryParams().Set("to", "2026-04-30")

	h := NewWorkoutHandler(stub)
	if err := h.ListWorkouts(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"Running"`) || !strings.Contains(body, `"Apple Watch"`) {
		t.Fatalf("body = %s, want session data", body)
	}
}
