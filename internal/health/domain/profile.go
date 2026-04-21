package domain

import "time"

// HealthProfile stores Rafael's personal health profile.
type HealthProfile struct {
	ID               int
	HeightCM         float64
	BirthDate        string
	Sex              string
	GoogleCalendarID string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
