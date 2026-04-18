package domain

import "time"

// VaccineSummary carries deterministic urgency metadata for a vaccine.
type VaccineSummary struct {
	Vaccine      Vaccine `json:"vaccine"`
	DaysUntilDue int     `json:"days_until_due"`
	Overdue      bool    `json:"overdue"`
}

// PetDigest groups all actionable pet-care signals for one pet.
type PetDigest struct {
	Pet                    Pet              `json:"pet"`
	VaccinesDueSoon        []VaccineSummary `json:"vaccines_due_soon"`
	ActiveTreatments       []Treatment      `json:"active_treatments"`
	UpcomingAppointments   []Appointment    `json:"upcoming_appointments"`
	RecentObservations     []Observation    `json:"recent_observations"`
	SuppliesNeedingReorder []Supply         `json:"supplies_needing_reorder"`
}

// AllPetsSummary is the all-pets aggregate used by the daily digest flow.
type AllPetsSummary struct {
	GeneratedAt time.Time   `json:"generated_at"`
	Pets        []PetDigest `json:"pets"`
}
