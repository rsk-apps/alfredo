// internal/fitness/adapters/secondary/sqlite/goal_repository.go
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
)

type GoalRepository struct{ db *sql.DB }

func NewGoalRepository(db *sql.DB) *GoalRepository { return &GoalRepository{db: db} }

func (r *GoalRepository) Create(ctx context.Context, g domain.Goal) (*domain.Goal, error) {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO fitness_goals
		 (id, name, description, target_value, target_unit, deadline, achieved_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		g.ID, g.Name, g.Description, g.TargetValue, g.TargetUnit,
		formatOptionalRFC3339(g.Deadline), formatOptionalRFC3339(g.AchievedAt),
		g.CreatedAt.Format(time.RFC3339), g.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, err
	}
	return &g, nil
}

func (r *GoalRepository) GetByID(ctx context.Context, id string) (*domain.Goal, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, name, description, target_value, target_unit, deadline, achieved_at, created_at, updated_at
		 FROM fitness_goals WHERE id = ?`, id)
	g, err := scanGoal(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return g, err
}

func (r *GoalRepository) List(ctx context.Context) ([]domain.Goal, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, description, target_value, target_unit, deadline, achieved_at, created_at, updated_at
		 FROM fitness_goals ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	var goals []domain.Goal
	for rows.Next() {
		g, err := scanGoal(rows)
		if err != nil {
			return nil, err
		}
		goals = append(goals, *g)
	}
	return goals, rows.Err()
}

func (r *GoalRepository) Update(ctx context.Context, g domain.Goal) (*domain.Goal, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE fitness_goals
		 SET name=?, description=?, target_value=?, target_unit=?, deadline=?, achieved_at=?, updated_at=?
		 WHERE id=?`,
		g.Name, g.Description, g.TargetValue, g.TargetUnit,
		formatOptionalRFC3339(g.Deadline), formatOptionalRFC3339(g.AchievedAt),
		g.UpdatedAt.Format(time.RFC3339), g.ID,
	)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, domain.ErrNotFound
	}
	return &g, nil
}

func (r *GoalRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM fitness_goals WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func scanGoal(s scanner) (*domain.Goal, error) {
	var g domain.Goal
	var createdAt, updatedAt string
	var deadline, achievedAt sql.NullString
	err := s.Scan(&g.ID, &g.Name, &g.Description, &g.TargetValue, &g.TargetUnit,
		&deadline, &achievedAt, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	g.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at %q: %w", createdAt, err)
	}
	g.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at %q: %w", updatedAt, err)
	}
	if deadline.Valid && deadline.String != "" {
		t, err := time.Parse(time.RFC3339, deadline.String)
		if err != nil {
			return nil, fmt.Errorf("parse deadline %q: %w", deadline.String, err)
		}
		g.Deadline = &t
	}
	if achievedAt.Valid && achievedAt.String != "" {
		t, err := time.Parse(time.RFC3339, achievedAt.String)
		if err != nil {
			return nil, fmt.Errorf("parse achieved_at %q: %w", achievedAt.String, err)
		}
		g.AchievedAt = &t
	}
	return &g, nil
}

// formatOptionalRFC3339 formats a *time.Time as RFC3339, returning nil if t is nil.
func formatOptionalRFC3339(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.RFC3339)
	return &s
}
