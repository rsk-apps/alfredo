package domain

import "time"

// HealthAppointment represents a medical appointment.
type HealthAppointment struct {
	ID                    string
	Specialty             string
	ScheduledAt           time.Time
	Doctor                *string
	Notes                 *string
	GoogleCalendarEventID string
	CreatedAt             time.Time
}
