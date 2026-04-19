package http

import (
	"context"
	"net/http"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
)

func TestHandlerHelpersMapDomainErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want int
	}{
		{"not found", domain.ErrNotFound, http.StatusNotFound},
		{"validation", domain.ErrValidation, http.StatusBadRequest},
		{"already stopped", domain.ErrAlreadyStopped, http.StatusConflict},
		{"unexpected", context.Canceled, http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := doHandlerRequest(t, http.MethodGet, "/x", "", nil, func(c echo.Context) error {
				return mapError(c, tc.err)
			})
			assertStatus(t, rec, tc.want)
		})
	}
}

func TestHandlerHelpersRejectInvalidInputs(t *testing.T) {
	assertStatus(t, doHandlerRequest(t, http.MethodGet, "/pets/not-a-uuid", "", map[string]string{"id": "not-a-uuid"}, func(c echo.Context) error {
		_, _ = parseUUID(c, "id")
		return nil
	}), http.StatusBadRequest)

	assertStatus(t, doHandlerRequest(t, http.MethodPost, "/pets", `{"name":"","species":"dog"}`, nil, NewPetHandler(&handlerPetSvc{}).Create), http.StatusBadRequest)
	assertStatus(t, doHandlerRequest(t, http.MethodPost, "/pets", `{"name":"Luna","species":"dog","birth_date":"2026/04/01"}`, nil, NewPetHandler(&handlerPetSvc{}).Create), http.StatusBadRequest)
	assertStatus(t, doHandlerRequest(t, http.MethodPost, "/supplies", `{"name":" ","last_purchased_at":"2026-04-01","estimated_days_supply":30}`, map[string]string{"id": testPetID}, NewSupplyHandler(&handlerSupplySvc{}).create), http.StatusBadRequest)
	assertStatus(t, doHandlerRequest(t, http.MethodPatch, "/supplies/"+testResourceID, `{}`, map[string]string{"id": testPetID, "sid": testResourceID}, NewSupplyHandler(&handlerSupplySvc{}).update), http.StatusBadRequest)
}
