package domain

import "time"

// Observation records an anomalous pet event Rafael may need to review later.
type Observation struct {
	ID          string
	PetID       string
	ObservedAt  time.Time
	Description string
	CreatedAt   time.Time
}
