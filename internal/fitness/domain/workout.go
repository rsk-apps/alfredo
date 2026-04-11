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
	HeartRate       *WorkoutHeartRate // nil if no HR data recorded
	Cardio          *CardioData       // nil for non-cardio workouts
	Strength        *StrengthData     // nil for non-strength workouts
	Source          string
	CreatedAt       time.Time
}

type WorkoutHeartRate struct {
	Avg      *float64
	Max      *float64
	Zone1Pct *float64
	Zone2Pct *float64
	Zone3Pct *float64
	Zone4Pct *float64
	Zone5Pct *float64
}

type CardioData struct {
	DistanceMeters  *float64
	AvgPaceSecPerKm *float64
}

type StrengthData struct {
	Exercises []WorkoutExercise
}
