package http

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/rafaelsoares/alfredo/internal/logger"
	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
)

type SummaryHandlerUseCaser interface {
	AllPets(ctx context.Context) (domain.AllPetsSummary, error)
}

type SummaryHandler struct {
	uc  SummaryHandlerUseCaser
	loc *time.Location
}

func NewSummaryHandler(uc SummaryHandlerUseCaser, loc *time.Location) *SummaryHandler {
	if loc == nil {
		loc = time.UTC
	}
	return &SummaryHandler{uc: uc, loc: loc}
}

func (h *SummaryHandler) Register(g *echo.Group) {
	g.GET("/pets/summary", h.GetSummary)
}

func (h *SummaryHandler) GetSummary(c echo.Context) error {
	summary, err := h.uc.AllPets(c.Request().Context())
	if err != nil {
		return mapError(c, err)
	}
	logger.FromEcho(c).Info("pet summary generated", zap.Int("pet_count", len(summary.Pets)))
	return c.JSON(http.StatusOK, h.toSummaryResponse(summary))
}

type allPetsSummaryResponse struct {
	GeneratedAt string              `json:"generated_at"`
	Pets        []petDigestResponse `json:"pets"`
}

type petDigestResponse struct {
	Pet                    petResponse              `json:"pet"`
	VaccinesDueSoon        []vaccineSummaryResponse `json:"vaccines_due_soon"`
	ActiveTreatments       []treatmentResponse      `json:"active_treatments"`
	UpcomingAppointments   []appointmentResponse    `json:"upcoming_appointments"`
	RecentObservations     []observationResponse    `json:"recent_observations"`
	SuppliesNeedingReorder []supplyResponse         `json:"supplies_needing_reorder"`
}

type vaccineSummaryResponse struct {
	Vaccine      vaccineResponse `json:"vaccine"`
	DaysUntilDue int             `json:"days_until_due"`
	Overdue      bool            `json:"overdue"`
}

func (h *SummaryHandler) toSummaryResponse(summary domain.AllPetsSummary) allPetsSummaryResponse {
	resp := allPetsSummaryResponse{
		GeneratedAt: summary.GeneratedAt.Format(time.RFC3339),
		Pets:        make([]petDigestResponse, 0, len(summary.Pets)),
	}
	for _, digest := range summary.Pets {
		resp.Pets = append(resp.Pets, h.toPetDigestResponse(digest))
	}
	return resp
}

func (h *SummaryHandler) toPetDigestResponse(digest domain.PetDigest) petDigestResponse {
	resp := petDigestResponse{
		Pet:                    toPetResponse(digest.Pet),
		VaccinesDueSoon:        make([]vaccineSummaryResponse, 0, len(digest.VaccinesDueSoon)),
		ActiveTreatments:       make([]treatmentResponse, 0, len(digest.ActiveTreatments)),
		UpcomingAppointments:   make([]appointmentResponse, 0, len(digest.UpcomingAppointments)),
		RecentObservations:     make([]observationResponse, 0, len(digest.RecentObservations)),
		SuppliesNeedingReorder: make([]supplyResponse, 0, len(digest.SuppliesNeedingReorder)),
	}
	for _, vaccine := range digest.VaccinesDueSoon {
		resp.VaccinesDueSoon = append(resp.VaccinesDueSoon, vaccineSummaryResponse{
			Vaccine:      toVaccineResponse(vaccine.Vaccine),
			DaysUntilDue: vaccine.DaysUntilDue,
			Overdue:      vaccine.Overdue,
		})
	}
	for _, treatment := range digest.ActiveTreatments {
		resp.ActiveTreatments = append(resp.ActiveTreatments, toTreatmentResponse(treatment, nil))
	}
	for _, appointment := range digest.UpcomingAppointments {
		resp.UpcomingAppointments = append(resp.UpcomingAppointments, toAppointmentResponse(appointment, h.loc))
	}
	for _, observation := range digest.RecentObservations {
		resp.RecentObservations = append(resp.RecentObservations, toObservationResponse(observation))
	}
	for _, supply := range digest.SuppliesNeedingReorder {
		resp.SuppliesNeedingReorder = append(resp.SuppliesNeedingReorder, toSupplyResponse(supply))
	}
	return resp
}
