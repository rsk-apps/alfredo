package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	mw "github.com/rafaelsoares/alfredo/internal/petcare/adapters/primary/http/middleware"
)

func TestAPIKeyAuth(t *testing.T) {
	const validKey = "test-secret"
	okHandler := func(c echo.Context) error { return c.String(http.StatusOK, "ok") }

	apply := func(req *http.Request) (*httptest.ResponseRecorder, error) {
		e := echo.New()
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		return rec, mw.APIKeyAuth(validKey)(okHandler)(c)
	}

	t.Run("valid Bearer token passes", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		req.Header.Set("Authorization", "Bearer "+validKey)
		rec, err := apply(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("want 200, got %d", rec.Code)
		}
	})

	t.Run("valid X-Api-Key passes", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		req.Header.Set("X-Api-Key", validKey)
		_, err := apply(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("missing key returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		_, err := apply(req)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		he, ok := err.(*echo.HTTPError)
		if !ok {
			t.Fatalf("want *echo.HTTPError, got %T", err)
		}
		if he.Code != http.StatusUnauthorized {
			t.Errorf("want 401, got %d", he.Code)
		}
	})

	t.Run("wrong key returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		req.Header.Set("Authorization", "Bearer wrong-key")
		_, err := apply(req)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		he, ok := err.(*echo.HTTPError)
		if !ok {
			t.Fatalf("want *echo.HTTPError, got %T", err)
		}
		if he.Code != http.StatusUnauthorized {
			t.Errorf("want 401, got %d", he.Code)
		}
	})

	t.Run("Bearer prefix required — key in wrong format returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		req.Header.Set("Authorization", validKey) // no "Bearer " prefix
		_, err := apply(req)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		he, ok := err.(*echo.HTTPError)
		if !ok {
			t.Fatalf("want *echo.HTTPError, got %T", err)
		}
		if he.Code != http.StatusUnauthorized {
			t.Errorf("want 401, got %d", he.Code)
		}
	})
}
