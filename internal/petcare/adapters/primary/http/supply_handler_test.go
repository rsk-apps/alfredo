package http

import (
	"net/http"
	"testing"
)

func TestSupplyHandlerRegisterRoutes(t *testing.T) {
	NewSupplyHandler(&handlerSupplySvc{}).Register(newTestGroup())
}

func TestSupplyHandlerSuccessRoutes(t *testing.T) {
	h := NewSupplyHandler(&handlerSupplySvc{})

	assertStatus(t, doHandlerRequest(t, http.MethodPost, "/supplies", `{"name":"Racao","last_purchased_at":"2026-04-01","estimated_days_supply":30}`, map[string]string{"id": testPetID}, h.create), http.StatusCreated)
	assertStatus(t, doHandlerRequest(t, http.MethodGet, "/supplies", "", map[string]string{"id": testPetID}, h.list), http.StatusOK)
	assertStatus(t, doHandlerRequest(t, http.MethodGet, "/supplies/"+testResourceID, "", map[string]string{"id": testPetID, "sid": testResourceID}, h.getByID), http.StatusOK)
	assertStatus(t, doHandlerRequest(t, http.MethodPatch, "/supplies/"+testResourceID, `{"name":"Racao senior","last_purchased_at":"2026-04-02","estimated_days_supply":45}`, map[string]string{"id": testPetID, "sid": testResourceID}, h.update), http.StatusOK)
	assertStatus(t, doHandlerRequest(t, http.MethodDelete, "/supplies/"+testResourceID, "", map[string]string{"id": testPetID, "sid": testResourceID}, h.delete), http.StatusNoContent)
}

func TestSupplyHandlerValidationErrors(t *testing.T) {
	h := NewSupplyHandler(&handlerSupplySvc{})

	assertStatus(t, doHandlerRequest(t, http.MethodPost, "/supplies", `{`, map[string]string{"id": testPetID}, h.create), http.StatusBadRequest)
	assertStatus(t, doHandlerRequest(t, http.MethodPost, "/supplies", `{"name":"Racao","last_purchased_at":"2026-04-01","estimated_days_supply":0}`, map[string]string{"id": testPetID}, h.create), http.StatusBadRequest)
	assertStatus(t, doHandlerRequest(t, http.MethodPost, "/supplies", `{"name":"Racao","last_purchased_at":"2026/04/01","estimated_days_supply":30}`, map[string]string{"id": testPetID}, h.create), http.StatusBadRequest)
	assertStatus(t, doHandlerRequest(t, http.MethodPatch, "/supplies/"+testResourceID, `{`, map[string]string{"id": testPetID, "sid": testResourceID}, h.update), http.StatusBadRequest)
	assertStatus(t, doHandlerRequest(t, http.MethodPatch, "/supplies/"+testResourceID, `{"name":" "}`, map[string]string{"id": testPetID, "sid": testResourceID}, h.update), http.StatusBadRequest)
	assertStatus(t, doHandlerRequest(t, http.MethodPatch, "/supplies/"+testResourceID, `{"estimated_days_supply":0}`, map[string]string{"id": testPetID, "sid": testResourceID}, h.update), http.StatusBadRequest)
	assertStatus(t, doHandlerRequest(t, http.MethodPatch, "/supplies/"+testResourceID, `{"last_purchased_at":"2026/04/01"}`, map[string]string{"id": testPetID, "sid": testResourceID}, h.update), http.StatusBadRequest)
	assertStatus(t, doHandlerRequest(t, http.MethodGet, "/supplies/not-a-uuid", "", map[string]string{"id": testPetID, "sid": "not-a-uuid"}, h.getByID), http.StatusBadRequest)
}
