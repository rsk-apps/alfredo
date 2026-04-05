// internal/fitness/domain/profile.go
package domain

import "time"

type Profile struct {
	ID        string
	FirstName string
	LastName  string
	BirthDate time.Time
	Gender    string // "male", "female", "other"
	HeightCm  float64
	CreatedAt time.Time
	UpdatedAt time.Time
}
