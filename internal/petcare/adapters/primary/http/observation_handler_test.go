package http

import (
	"net/http"
	"testing"
)

func TestObservationHandlerRegisterRoutes(t *testing.T) {
	NewObservationHandler(&handlerObservationSvc{}, testLocation(t)).Register(newTestGroup())
}

func TestObservationHandlerSuccessRoutes(t *testing.T) {
	h := NewObservationHandler(&handlerObservationSvc{}, testLocation(t))

	assertStatus(t, doHandlerRequest(t, http.MethodPost, "/observations", `{"observed_at":"2026-04-17T09:30:00","description":"Vomitou"}`, map[string]string{"id": testPetID}, h.CreateObservation), http.StatusCreated)
	assertStatus(t, doHandlerRequest(t, http.MethodGet, "/observations", "", map[string]string{"id": testPetID}, h.ListObservations), http.StatusOK)
	assertStatus(t, doHandlerRequest(t, http.MethodGet, "/observations/"+testResourceID, "", map[string]string{"id": testPetID, "oid": testResourceID}, h.GetObservation), http.StatusOK)
}

func TestObservationHandlerValidationErrors(t *testing.T) {
	h := NewObservationHandler(&handlerObservationSvc{}, testLocation(t))

	assertStatus(t, doHandlerRequest(t, http.MethodPost, "/observations", `{`, map[string]string{"id": testPetID}, h.CreateObservation), http.StatusBadRequest)
	assertStatus(t, doHandlerRequest(t, http.MethodPost, "/observations", `{"observed_at":"2026-04-17T09:30:00"}`, map[string]string{"id": testPetID}, h.CreateObservation), http.StatusBadRequest)
	assertStatus(t, doHandlerRequest(t, http.MethodPost, "/observations", `{"observed_at":"2026-04-17","description":"Vomitou"}`, map[string]string{"id": testPetID}, h.CreateObservation), http.StatusBadRequest)
	assertStatus(t, doHandlerRequest(t, http.MethodGet, "/observations/not-a-uuid", "", map[string]string{"id": testPetID, "oid": "not-a-uuid"}, h.GetObservation), http.StatusBadRequest)
}
