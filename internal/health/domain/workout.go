package domain

import "time"

// WorkoutSession represents a single workout recorded by Apple Health.
type WorkoutSession struct {
	ID                 int
	ActivityType       string
	StartDate          time.Time
	EndDate            time.Time
	DurationSeconds    float64
	ActiveCaloriesKcal *float64
	BasalCaloriesKcal  *float64
	HRAvgBPM           *float64
	HRMinBPM           *float64
	HRMaxBPM           *float64
	DistanceM          *float64
	Source             string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}
