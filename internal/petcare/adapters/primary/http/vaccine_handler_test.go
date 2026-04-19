package http

import (
	"net/http"
	"testing"
)

func TestVaccineHandlerRegisterRoutes(t *testing.T) {
	NewVaccineHandler(&handlerVaccineSvc{}, testLocation(t)).Register(newTestGroup())
}

func TestVaccineHandlerSuccessRoutes(t *testing.T) {
	h := NewVaccineHandler(&handlerVaccineSvc{}, testLocation(t))

	assertStatus(t, doHandlerRequest(t, http.MethodGet, "/vaccines", "", map[string]string{"id": testPetID}, h.ListVaccines), http.StatusOK)
	assertStatus(t, doHandlerRequest(t, http.MethodPost, "/vaccines", `{"name":"V10","date":"2026-04-17T10:00:00","recurrence_days":30}`, map[string]string{"id": testPetID}, h.RecordVaccine), http.StatusCreated)
	assertStatus(t, doHandlerRequest(t, http.MethodDelete, "/vaccines/"+testResourceID, "", map[string]string{"id": testPetID, "vid": testResourceID}, h.DeleteVaccine), http.StatusNoContent)
}
