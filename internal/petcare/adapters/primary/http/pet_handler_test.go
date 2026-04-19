package http

import (
	"net/http"
	"testing"
)

func TestPetHandlerRegisterRoutes(t *testing.T) {
	NewPetHandler(&handlerPetSvc{}).Register(newTestGroup())
}

func TestPetHandlerSuccessRoutes(t *testing.T) {
	h := NewPetHandler(&handlerPetSvc{})

	assertStatus(t, doHandlerRequest(t, http.MethodGet, "/pets", "", nil, h.List), http.StatusOK)
	assertStatus(t, doHandlerRequest(t, http.MethodPost, "/pets", `{"name":"Luna","species":"dog","birth_date":"2020-01-01"}`, nil, h.Create), http.StatusCreated)
	assertStatus(t, doHandlerRequest(t, http.MethodGet, "/pets/"+testPetID, "", map[string]string{"id": testPetID}, h.GetByID), http.StatusOK)
	assertStatus(t, doHandlerRequest(t, http.MethodPut, "/pets/"+testPetID, `{"name":"Lua","species":"dog"}`, map[string]string{"id": testPetID}, h.Update), http.StatusOK)
	assertStatus(t, doHandlerRequest(t, http.MethodDelete, "/pets/"+testPetID, "", map[string]string{"id": testPetID}, h.Delete), http.StatusNoContent)
}
