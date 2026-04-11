// internal/fitness/adapters/secondary/sqlite/workout_repository.go
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
)

type WorkoutRepository struct{ db *sql.DB }

func NewWorkoutRepository(db *sql.DB) *WorkoutRepository { return &WorkoutRepository{db: db} }

func (r *WorkoutRepository) Create(ctx context.Context, w domain.Workout) (*domain.Workout, error) {
	// Extract flat HR/cardio values for the fitness_workouts table (schema unchanged).
	var avgHR, maxHR, z1, z2, z3, z4, z5 *float64
	if w.HeartRate != nil {
		avgHR = w.HeartRate.Avg
		maxHR = w.HeartRate.Max
		z1 = w.HeartRate.Zone1Pct
		z2 = w.HeartRate.Zone2Pct
		z3 = w.HeartRate.Zone3Pct
		z4 = w.HeartRate.Zone4Pct
		z5 = w.HeartRate.Zone5Pct
	}
	var distanceMeters, avgPace *float64
	if w.Cardio != nil {
		distanceMeters = w.Cardio.DistanceMeters
		avgPace = w.Cardio.AvgPaceSecPerKm
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	_, err = tx.ExecContext(ctx,
		`INSERT INTO fitness_workouts
		 (id, external_id, type, started_at, duration_seconds, active_calories, total_calories,
		  distance_meters, avg_pace_sec_per_km, avg_heart_rate, max_heart_rate,
		  hr_zone1_pct, hr_zone2_pct, hr_zone3_pct, hr_zone4_pct, hr_zone5_pct, source, created_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		w.ID, w.ExternalID, w.Type, w.StartedAt.Format(time.RFC3339),
		w.DurationSeconds, w.ActiveCalories, w.TotalCalories,
		distanceMeters, avgPace, avgHR, maxHR,
		z1, z2, z3, z4, z5,
		w.Source, w.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, domain.ErrAlreadyExists
		}
		return nil, err
	}

	if w.Strength != nil {
		for _, ex := range w.Strength.Exercises {
			_, err = tx.ExecContext(ctx,
				`INSERT INTO fitness_workout_exercises (id, workout_id, name, equipment, order_idx)
				 VALUES (?,?,?,?,?)`,
				ex.ID, ex.WorkoutID, ex.Name, ex.Equipment, ex.OrderIdx,
			)
			if err != nil {
				return nil, fmt.Errorf("insert exercise %q: %w", ex.Name, err)
			}
			for _, s := range ex.Sets {
				_, err = tx.ExecContext(ctx,
					`INSERT INTO fitness_workout_sets (id, exercise_id, set_number, reps, weight_kg, duration_secs, notes)
					 VALUES (?,?,?,?,?,?,?)`,
					s.ID, s.ExerciseID, s.SetNumber, s.Reps, s.WeightKg, s.DurationSecs, s.Notes,
				)
				if err != nil {
					return nil, fmt.Errorf("insert set %d for exercise %q: %w", s.SetNumber, ex.Name, err)
				}
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return &w, nil
}

func (r *WorkoutRepository) GetByID(ctx context.Context, id string) (*domain.Workout, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, external_id, type, started_at, duration_seconds, active_calories, total_calories,
		        distance_meters, avg_pace_sec_per_km, avg_heart_rate, max_heart_rate,
		        hr_zone1_pct, hr_zone2_pct, hr_zone3_pct, hr_zone4_pct, hr_zone5_pct, source, created_at
		 FROM fitness_workouts WHERE id = ?`, id)
	w, err := scanWorkout(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	exercises, err := r.loadExercises(ctx, id)
	if err != nil {
		return nil, err
	}
	if len(exercises) > 0 {
		w.Strength = &domain.StrengthData{Exercises: exercises}
	}
	return w, nil
}

func (r *WorkoutRepository) List(ctx context.Context, from, to *time.Time) ([]domain.Workout, error) {
	query := `SELECT id, external_id, type, started_at, duration_seconds, active_calories, total_calories,
	                 distance_meters, avg_pace_sec_per_km, avg_heart_rate, max_heart_rate,
	                 hr_zone1_pct, hr_zone2_pct, hr_zone3_pct, hr_zone4_pct, hr_zone5_pct, source, created_at
	          FROM fitness_workouts`
	args := []any{}
	clauses := []string{}
	if from != nil {
		clauses = append(clauses, "started_at >= ?")
		args = append(args, from.Format(time.RFC3339))
	}
	if to != nil {
		clauses = append(clauses, "started_at <= ?")
		args = append(args, to.Format(time.RFC3339))
	}
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY started_at DESC"
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	var ws []domain.Workout
	for rows.Next() {
		w, err := scanWorkout(rows)
		if err != nil {
			return nil, err
		}
		ws = append(ws, *w)
	}
	return ws, rows.Err()
}

func (r *WorkoutRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM fitness_workouts WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func scanWorkout(s scanner) (*domain.Workout, error) {
	var w domain.Workout
	var startedAt, createdAt string
	var avgHR, maxHR, z1, z2, z3, z4, z5 *float64
	var distanceMeters, avgPace *float64

	err := s.Scan(
		&w.ID, &w.ExternalID, &w.Type, &startedAt, &w.DurationSeconds,
		&w.ActiveCalories, &w.TotalCalories,
		&distanceMeters, &avgPace, &avgHR, &maxHR,
		&z1, &z2, &z3, &z4, &z5,
		&w.Source, &createdAt,
	)
	if err != nil {
		return nil, err
	}
	w.StartedAt, err = time.Parse(time.RFC3339, startedAt)
	if err != nil {
		return nil, fmt.Errorf("parse started_at %q: %w", startedAt, err)
	}
	w.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at %q: %w", createdAt, err)
	}

	// Reconstruct nested HeartRate if any HR field is non-nil.
	if avgHR != nil || maxHR != nil || z1 != nil || z2 != nil || z3 != nil || z4 != nil || z5 != nil {
		w.HeartRate = &domain.WorkoutHeartRate{
			Avg:      avgHR,
			Max:      maxHR,
			Zone1Pct: z1,
			Zone2Pct: z2,
			Zone3Pct: z3,
			Zone4Pct: z4,
			Zone5Pct: z5,
		}
	}

	// Reconstruct nested CardioData if any cardio field is non-nil.
	if distanceMeters != nil || avgPace != nil {
		w.Cardio = &domain.CardioData{
			DistanceMeters:  distanceMeters,
			AvgPaceSecPerKm: avgPace,
		}
	}

	// Strength is always nil from scanWorkout; GetByID loads it separately.
	return &w, nil
}

func (r *WorkoutRepository) loadExercises(ctx context.Context, workoutID string) ([]domain.WorkoutExercise, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, workout_id, name, equipment, order_idx
		 FROM fitness_workout_exercises
		 WHERE workout_id = ?
		 ORDER BY order_idx`,
		workoutID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck

	var exercises []domain.WorkoutExercise
	for rows.Next() {
		var ex domain.WorkoutExercise
		if err := rows.Scan(&ex.ID, &ex.WorkoutID, &ex.Name, &ex.Equipment, &ex.OrderIdx); err != nil {
			return nil, err
		}
		sets, err := r.loadSets(ctx, ex.ID)
		if err != nil {
			return nil, err
		}
		ex.Sets = sets
		exercises = append(exercises, ex)
	}
	return exercises, rows.Err()
}

func (r *WorkoutRepository) loadSets(ctx context.Context, exerciseID string) ([]domain.WorkoutSet, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, exercise_id, set_number, reps, weight_kg, duration_secs, notes
		 FROM fitness_workout_sets
		 WHERE exercise_id = ?
		 ORDER BY set_number`,
		exerciseID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck

	var sets []domain.WorkoutSet
	for rows.Next() {
		var s domain.WorkoutSet
		if err := rows.Scan(&s.ID, &s.ExerciseID, &s.SetNumber, &s.Reps, &s.WeightKg, &s.DurationSecs, &s.Notes); err != nil {
			return nil, err
		}
		sets = append(sets, s)
	}
	return sets, rows.Err()
}
