package http

import (
	"net/http"
	"testing"
)

func TestAppointmentHandlerRegisterRoutes(t *testing.T) {
	NewAppointmentHandler(&handlerAppointmentSvc{}, testLocation(t)).Register(newTestGroup())
}

func TestAppointmentHandlerSuccessRoutes(t *testing.T) {
	h := NewAppointmentHandler(&handlerAppointmentSvc{}, testLocation(t))
	body := `{"type":"vet","scheduled_at":"2026-04-17T09:30:00","provider":"Vet","location":"Clinic","notes":"checkup"}`

	assertStatus(t, doHandlerRequest(t, http.MethodPost, "/appointments", body, map[string]string{"id": testPetID}, h.create), http.StatusCreated)
	assertStatus(t, doHandlerRequest(t, http.MethodGet, "/appointments", "", map[string]string{"id": testPetID}, h.list), http.StatusOK)
	assertStatus(t, doHandlerRequest(t, http.MethodGet, "/appointments/"+testResourceID, "", map[string]string{"id": testPetID, "aid": testResourceID}, h.getByID), http.StatusOK)
	assertStatus(t, doHandlerRequest(t, http.MethodPatch, "/appointments/"+testResourceID, `{"scheduled_at":"2026-04-18T09:30:00"}`, map[string]string{"id": testPetID, "aid": testResourceID}, h.update), http.StatusOK)
	assertStatus(t, doHandlerRequest(t, http.MethodDelete, "/appointments/"+testResourceID, "", map[string]string{"id": testPetID, "aid": testResourceID}, h.delete), http.StatusNoContent)
}

func TestAppointmentHandlerValidationErrors(t *testing.T) {
	h := NewAppointmentHandler(&handlerAppointmentSvc{}, testLocation(t))

	assertStatus(t, doHandlerRequest(t, http.MethodPost, "/appointments", `{`, map[string]string{"id": testPetID}, h.create), http.StatusBadRequest)
	assertStatus(t, doHandlerRequest(t, http.MethodPost, "/appointments", `{"type":"bath","scheduled_at":"2026-04-17T09:30:00"}`, map[string]string{"id": testPetID}, h.create), http.StatusBadRequest)
	assertStatus(t, doHandlerRequest(t, http.MethodPost, "/appointments", `{"type":"vet","scheduled_at":"2026-04-17"}`, map[string]string{"id": testPetID}, h.create), http.StatusBadRequest)
	assertStatus(t, doHandlerRequest(t, http.MethodPatch, "/appointments/"+testResourceID, `{}`, map[string]string{"id": testPetID, "aid": testResourceID}, h.update), http.StatusBadRequest)
	assertStatus(t, doHandlerRequest(t, http.MethodPatch, "/appointments/"+testResourceID, `{"scheduled_at":"2026-04-17"}`, map[string]string{"id": testPetID, "aid": testResourceID}, h.update), http.StatusBadRequest)
	assertStatus(t, doHandlerRequest(t, http.MethodGet, "/appointments/not-a-uuid", "", map[string]string{"id": testPetID, "aid": "not-a-uuid"}, h.getByID), http.StatusBadRequest)
}
