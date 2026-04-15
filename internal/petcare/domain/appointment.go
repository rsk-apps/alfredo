package domain

import "time"

// AppointmentType classifies the kind of appointment.
type AppointmentType string

const (
	AppointmentTypeVet      AppointmentType = "vet"
	AppointmentTypeGrooming AppointmentType = "grooming"
	AppointmentTypeOther    AppointmentType = "other"
)

// Appointment represents a scheduled interaction with an external provider.
type Appointment struct {
	ID                    string
	PetID                 string
	Type                  AppointmentType
	ScheduledAt           time.Time
	Provider              *string
	Location              *string
	Notes                 *string
	GoogleCalendarEventID string
	CreatedAt             time.Time
}
