package domain

type WorkoutExercise struct {
	ID        string
	WorkoutID string
	Name      string
	Equipment *string
	OrderIdx  int
	Sets      []WorkoutSet
}

type WorkoutSet struct {
	ID           string
	ExerciseID   string
	SetNumber    int
	Reps         *int
	WeightKg     *float64
	DurationSecs *int
	Notes        *string
}
