package http

import (
	"net/http"
	"testing"
)

func TestSummaryHandlerRegisterRoutes(t *testing.T) {
	NewSummaryHandler(&handlerSummarySvc{}, nil).Register(newTestGroup())
}

func TestSummaryHandlerGetSummary(t *testing.T) {
	summary := NewSummaryHandler(&handlerSummarySvc{}, testLocation(t))
	assertStatus(t, doHandlerRequest(t, http.MethodGet, "/summary", "", nil, summary.GetSummary), http.StatusOK)
}
