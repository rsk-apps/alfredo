package http

import (
	"net/http"
	"testing"
)

func TestHealthHTTPHandlerStatusCodes(t *testing.T) {
	healthy := NewHealthHTTPHandler(handlerHealthChecker{status: "healthy"})
	assertStatus(t, doHandlerRequest(t, http.MethodGet, "/health", "", nil, healthy.Health), http.StatusOK)

	degraded := NewHealthHTTPHandler(handlerHealthChecker{status: "degraded"})
	assertStatus(t, doHandlerRequest(t, http.MethodGet, "/health", "", nil, degraded.Health), http.StatusServiceUnavailable)
}
