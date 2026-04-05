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
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO fitness_workouts
		 (id, external_id, type, started_at, duration_seconds, active_calories, total_calories,
		  distance_meters, avg_pace_sec_per_km, avg_heart_rate, max_heart_rate,
		  hr_zone1_pct, hr_zone2_pct, hr_zone3_pct, hr_zone4_pct, hr_zone5_pct, source, created_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		w.ID, w.ExternalID, w.Type, w.StartedAt.Format(time.RFC3339),
		w.DurationSeconds, w.ActiveCalories, w.TotalCalories,
		w.DistanceMeters, w.AvgPaceSecPerKm, w.AvgHeartRate, w.MaxHeartRate,
		w.HRZone1Pct, w.HRZone2Pct, w.HRZone3Pct, w.HRZone4Pct, w.HRZone5Pct,
		w.Source, w.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, domain.ErrAlreadyExists
		}
		return nil, err
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
	return w, err
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
	err := s.Scan(
		&w.ID, &w.ExternalID, &w.Type, &startedAt, &w.DurationSeconds,
		&w.ActiveCalories, &w.TotalCalories,
		&w.DistanceMeters, &w.AvgPaceSecPerKm, &w.AvgHeartRate, &w.MaxHeartRate,
		&w.HRZone1Pct, &w.HRZone2Pct, &w.HRZone3Pct, &w.HRZone4Pct, &w.HRZone5Pct,
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
	return &w, nil
}
