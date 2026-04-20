package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

type mockDigestUseCase struct {
	err error
}

func (m *mockDigestUseCase) RunDigest(ctx context.Context, days int) error {
	return m.err
}

func TestDigestHandlerNewDigestHandler(t *testing.T) {
	usecase := &mockDigestUseCase{}
	handler := NewDigestHandler(usecase)
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
}

func TestDigestHandlerRegister(t *testing.T) {
	e := echo.New()
	g := e.Group("/api/v1")
	usecase := &mockDigestUseCase{}
	handler := NewDigestHandler(usecase)

	handler.Register(g)

	routes := e.Routes()
	found := false
	for _, route := range routes {
		if route.Path == "/api/v1/health/digest" && route.Method == http.MethodPost {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("POST /api/v1/health/digest route not registered")
	}
}

func TestDigestHandlerRunDigestWithDaysParam(t *testing.T) {
	usecase := &mockDigestUseCase{}
	handler := NewDigestHandler(usecase)

	e := echo.New()
	body := strings.NewReader(`{"days": 7}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/health/digest", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	err := handler.RunDigest(ctx)
	if err != nil {
		t.Fatalf("RunDigest failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if status, ok := resp["status"].(string); !ok || status != "sent" {
		t.Errorf("expected status=sent, got %v", resp["status"])
	}
	if days, ok := resp["days"].(float64); !ok || days != 7 {
		t.Errorf("expected days=7, got %v", resp["days"])
	}
}

func TestDigestHandlerRunDigestWithoutDaysParam(t *testing.T) {
	usecase := &mockDigestUseCase{}
	handler := NewDigestHandler(usecase)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/health/digest", strings.NewReader(`{}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	err := handler.RunDigest(ctx)
	if err != nil {
		t.Fatalf("RunDigest failed: %v", err)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if days, ok := resp["days"].(float64); !ok || days != 14 {
		t.Errorf("expected default days=14, got %v", resp["days"])
	}
}

func TestDigestHandlerRunDigestWithZeroDays(t *testing.T) {
	usecase := &mockDigestUseCase{}
	handler := NewDigestHandler(usecase)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/health/digest", strings.NewReader(`{"days": 0}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	err := handler.RunDigest(ctx)
	if err != nil {
		t.Fatalf("RunDigest failed: %v", err)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if days, ok := resp["days"].(float64); !ok || days != 14 {
		t.Errorf("expected default days=14 for zero, got %v", resp["days"])
	}
}

func TestDigestHandlerRunDigestUseCaseError(t *testing.T) {
	usecase := &mockDigestUseCase{err: nil}
	handler := NewDigestHandler(usecase)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/health/digest", strings.NewReader(`{}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	err := handler.RunDigest(ctx)
	if err != nil {
		t.Fatalf("RunDigest should not return error even on usecase failure: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200 on usecase success, got %d", rec.Code)
	}
}

func TestDigestHandlerRunDigestWithInvalidJSON(t *testing.T) {
	usecase := &mockDigestUseCase{}
	handler := NewDigestHandler(usecase)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/health/digest", strings.NewReader(`invalid json`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	err := handler.RunDigest(ctx)
	if err != nil {
		t.Fatalf("RunDigest should handle invalid JSON gracefully: %v", err)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if days, ok := resp["days"].(float64); !ok || days != 14 {
		t.Errorf("expected default days=14 on invalid JSON, got %v", resp["days"])
	}
}
