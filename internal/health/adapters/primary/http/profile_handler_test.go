package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rafaelsoares/alfredo/internal/health/domain"
)

type profileSvcStub struct {
	getFn    func(context.Context) (domain.HealthProfile, error)
	upsertFn func(context.Context, domain.HealthProfile) (domain.HealthProfile, error)
}

func (s *profileSvcStub) Get(ctx context.Context) (domain.HealthProfile, error) {
	if s.getFn != nil {
		return s.getFn(ctx)
	}
	return domain.HealthProfile{}, nil
}

func (s *profileSvcStub) Upsert(ctx context.Context, profile domain.HealthProfile) (domain.HealthProfile, error) {
	if s.upsertFn != nil {
		return s.upsertFn(ctx, profile)
	}
	return profile, nil
}

func TestProfileHandlerRejectsInvalidInput(t *testing.T) {
	t.Run("invalid sex", func(t *testing.T) {
		rec := doProfileRequest(t, http.MethodPut, `/api/v1/health/profile`, `{"height_cm":178,"birth_date":"1993-06-15","sex":"unknown"}`, &profileSvcStub{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
	})

	t.Run("zero height", func(t *testing.T) {
		rec := doProfileRequest(t, http.MethodPut, `/api/v1/health/profile`, `{"height_cm":0,"birth_date":"1993-06-15","sex":"male"}`, &profileSvcStub{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
	})

	t.Run("datetime birth_date", func(t *testing.T) {
		rec := doProfileRequest(t, http.MethodPut, `/api/v1/health/profile`, `{"height_cm":178,"birth_date":"1993-06-15T00:00:00","sex":"male"}`, &profileSvcStub{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
	})
}

func TestProfileHandlerRoundTrip(t *testing.T) {
	svc := &profileSvcStub{
		upsertFn: func(_ context.Context, profile domain.HealthProfile) (domain.HealthProfile, error) {
			profile.ID = 1
			profile.CreatedAt = time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
			profile.UpdatedAt = time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
			return profile, nil
		},
		getFn: func(context.Context) (domain.HealthProfile, error) {
			return domain.HealthProfile{
				ID:        1,
				HeightCM:  180,
				BirthDate: "1993-06-15",
				Sex:       "male",
				CreatedAt: time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC),
				UpdatedAt: time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC),
			}, nil
		},
	}

	putRec := doProfileRequest(t, http.MethodPut, `/api/v1/health/profile`, `{"height_cm":180,"birth_date":"1993-06-15","sex":"male"}`, svc)
	if putRec.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, want 200", putRec.Code)
	}
	var created profileResponse
	if err := json.Unmarshal(putRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode PUT response: %v", err)
	}
	if created.ID != 1 || created.HeightCM != 180 || created.BirthDate != "1993-06-15" || created.Sex != "male" {
		t.Fatalf("PUT response = %#v", created)
	}

	getRec := doProfileRequest(t, http.MethodGet, `/api/v1/health/profile`, "", svc)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want 200", getRec.Code)
	}
	var fetched profileResponse
	if err := json.Unmarshal(getRec.Body.Bytes(), &fetched); err != nil {
		t.Fatalf("decode GET response: %v", err)
	}
	if fetched.ID != 1 || fetched.HeightCM != 180 || fetched.BirthDate != "1993-06-15" || fetched.Sex != "male" {
		t.Fatalf("GET response = %#v", fetched)
	}
}

func TestProfileHandlerGetNotFound(t *testing.T) {
	rec := doProfileRequest(t, http.MethodGet, `/api/v1/health/profile`, "", &profileSvcStub{
		getFn: func(context.Context) (domain.HealthProfile, error) {
			return domain.HealthProfile{}, domain.ErrNotFound
		},
	})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func doProfileRequest(t *testing.T, method, path, body string, svc ProfileUseCaser) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, http.NoBody)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	h := NewProfileHandler(svc)
	switch method {
	case http.MethodGet:
		if err := h.GetProfile(c); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case http.MethodPut:
		if err := h.UpsertProfile(c); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	default:
		t.Fatalf("unsupported method: %s", method)
	}
	return rec
}
