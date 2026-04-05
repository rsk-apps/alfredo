// internal/fitness/domain/workout.go
package domain

import "time"

type Workout struct {
	ID              string
	ExternalID      string
	Type            string
	StartedAt       time.Time
	DurationSeconds int
	ActiveCalories  float64
	TotalCalories   float64
	DistanceMeters  *float64
	AvgPaceSecPerKm *float64
	AvgHeartRate    *float64
	MaxHeartRate    *float64
	HRZone1Pct      *float64
	HRZone2Pct      *float64
	HRZone3Pct      *float64
	HRZone4Pct      *float64
	HRZone5Pct      *float64
	Source          string
	CreatedAt       time.Time
}
