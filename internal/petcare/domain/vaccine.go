package domain

import "time"

type Vaccine struct {
	ID                           string
	PetID                        string
	Name                         string
	AdministeredAt               time.Time // Date accepts past, present, or future dates
	NextDueAt                    *time.Time
	VetName                      *string
	BatchNumber                  *string
	Notes                        *string
	GoogleCalendarEventID        string
	GoogleCalendarNextDueEventID string
}
