package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

type siriAgentStub struct {
	input string
	err   error
}

func (s *siriAgentStub) Handle(_ context.Context, inputText string) (string, error) {
	s.input = inputText
	return "resposta", s.err
}

func TestSiriHandlerValidatesAndDispatchesText(t *testing.T) {
	agent := &siriAgentStub{}
	rec := doSiriRequest(t, `{"text":"  resumo dos pets  "}`, agent)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s, want 200", rec.Code, rec.Body.String())
	}
	if agent.input != "resumo dos pets" {
		t.Fatalf("input = %q, want trimmed text", agent.input)
	}
}

func TestSiriHandlerRejectsInvalidRequests(t *testing.T) {
	for _, body := range []string{
		`{`,
		`{"text":"   "}`,
		`{"text":"` + strings.Repeat("x", maxSiriTextLength+1) + `"}`,
	} {
		rec := doSiriRequest(t, body, &siriAgentStub{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %q status = %d, want 400", body, rec.Code)
		}
	}
}

func TestSiriHandlerRegister(t *testing.T) {
	e := echo.New()
	NewSiriHandler(&siriAgentStub{}).Register(e.Group("/api/v1"))
}

func doSiriRequest(t *testing.T, body string, agent *siriAgentStub) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent/siri", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := NewSiriHandler(agent).handle(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	return rec
}
