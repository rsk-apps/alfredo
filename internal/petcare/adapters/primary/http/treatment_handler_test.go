package http

import (
	"net/http"
	"testing"
)

func TestTreatmentHandlerRegisterRoutes(t *testing.T) {
	NewTreatmentHandler(&handlerTreatmentSvc{}, testLocation(t)).Register(newTestGroup())
}

func TestTreatmentHandlerSuccessRoutes(t *testing.T) {
	h := NewTreatmentHandler(&handlerTreatmentSvc{}, testLocation(t))
	body := `{"name":"Antibiotico","dosage_amount":2.5,"dosage_unit":"ml","route":"oral","interval_hours":12,"started_at":"2026-04-17T08:00:00","ended_at":"2026-04-18T08:00:00"}`

	assertStatus(t, doHandlerRequest(t, http.MethodPost, "/treatments", body, map[string]string{"id": testPetID}, h.StartTreatment), http.StatusCreated)
	assertStatus(t, doHandlerRequest(t, http.MethodGet, "/treatments", "", map[string]string{"id": testPetID}, h.ListTreatments), http.StatusOK)
	assertStatus(t, doHandlerRequest(t, http.MethodGet, "/treatments/"+testResourceID, "", map[string]string{"id": testPetID, "tid": testResourceID}, h.GetTreatment), http.StatusOK)
	assertStatus(t, doHandlerRequest(t, http.MethodDelete, "/treatments/"+testResourceID, "", map[string]string{"id": testPetID, "tid": testResourceID}, h.StopTreatment), http.StatusNoContent)
}
