package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/rafaelsoares/alfredo/internal/health/domain"
)

func TestMapErrorMapsHealthDomainErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want int
	}{
		{"not found", domain.ErrNotFound, http.StatusNotFound},
		{"validation", domain.ErrValidation, http.StatusBadRequest},
		{"unexpected", context.Canceled, http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			rec := httptest.NewRecorder()
			c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), rec)
			if err := mapError(c, tc.err); err != nil {
				t.Fatalf("mapError returned error: %v", err)
			}
			if rec.Code != tc.want {
				t.Fatalf("status = %d body = %s, want %d", rec.Code, rec.Body.String(), tc.want)
			}
		})
	}
}
