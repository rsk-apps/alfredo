package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	mw "github.com/rafaelsoares/alfredo/internal/petcare/adapters/primary/http/middleware"
)

func TestRequestLogger_includesClientIP(t *testing.T) {
	core, logs := observer.New(zapcore.InfoLevel)
	log := zap.New(core)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/pets", nil)
	req.RemoteAddr = "1.2.3.4:5678"
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := mw.RequestLogger(log)(func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})
	if err := handler(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if logs.Len() == 0 {
		t.Fatal("no log entries recorded")
	}
	entry := logs.All()[0]
	var found bool
	for _, f := range entry.Context {
		if f.Key == "client_ip" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("client_ip field missing from log entry; got fields: %+v", entry.Context)
	}
}

func TestRequestLoggerLogsErrorResponsesWithDebugPayload(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	log := zap.New(core)

	for _, tc := range []struct {
		name   string
		status int
		method string
		target string
		body   string
	}{
		{name: "warn", status: http.StatusBadRequest, method: http.MethodPost, target: "/api/v1/pets", body: `{"bad":true}`},
		{name: "error", status: http.StatusInternalServerError, method: http.MethodGet, target: "/api/v1/pets?limit=1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			var body *strings.Reader
			if tc.body != "" {
				body = strings.NewReader(tc.body)
			} else {
				body = strings.NewReader("")
			}
			req := httptest.NewRequest(tc.method, tc.target, body)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			handler := mw.RequestLogger(log)(func(c echo.Context) error {
				c.Set("log_error", "mapped_error")
				return c.NoContent(tc.status)
			})
			if err := handler(c); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
	if logs.Len() < 2 {
		t.Fatalf("logs = %d, want at least 2", logs.Len())
	}
}
