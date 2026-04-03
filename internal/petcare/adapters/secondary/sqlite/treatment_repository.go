// internal/petcare/adapters/secondary/sqlite/treatment_repository.go
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
)

type TreatmentRepository struct{ db *sql.DB }

func NewTreatmentRepository(db *sql.DB) *TreatmentRepository {
	return &TreatmentRepository{db: db}
}

func (r *TreatmentRepository) Create(ctx context.Context, t domain.Treatment) (*domain.Treatment, error) {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO treatments (id, pet_id, name, dosage_amount, dosage_unit, route, interval_hours, started_at, ended_at, stopped_at, vet_name, notes, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.PetID, t.Name, t.DosageAmount, t.DosageUnit, t.Route, t.IntervalHours,
		t.StartedAt.Format(time.RFC3339), formatOptionalRFC3339(t.EndedAt), formatOptionalRFC3339(t.StoppedAt),
		t.VetName, t.Notes, t.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *TreatmentRepository) GetByID(ctx context.Context, petID, treatmentID string) (*domain.Treatment, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, pet_id, name, dosage_amount, dosage_unit, route, interval_hours, started_at, ended_at, stopped_at, vet_name, notes, created_at
		 FROM treatments WHERE id = ? AND pet_id = ?`, treatmentID, petID)
	t, err := scanTreatment(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return t, err
}

func (r *TreatmentRepository) List(ctx context.Context, petID string) ([]domain.Treatment, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, pet_id, name, dosage_amount, dosage_unit, route, interval_hours, started_at, ended_at, stopped_at, vet_name, notes, created_at
		 FROM treatments WHERE pet_id = ? ORDER BY created_at DESC`, petID)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	var ts []domain.Treatment
	for rows.Next() {
		t, err := scanTreatment(rows)
		if err != nil {
			return nil, err
		}
		ts = append(ts, *t)
	}
	return ts, rows.Err()
}

func (r *TreatmentRepository) Stop(ctx context.Context, treatmentID string, stoppedAt time.Time) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE treatments SET stopped_at = ? WHERE id = ? AND stopped_at IS NULL`,
		stoppedAt.Format(time.RFC3339), treatmentID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func scanTreatment(s scanner) (*domain.Treatment, error) {
	var t domain.Treatment
	var startedAt, createdAt string
	var endedAt, stoppedAt sql.NullString
	err := s.Scan(
		&t.ID, &t.PetID, &t.Name, &t.DosageAmount, &t.DosageUnit, &t.Route, &t.IntervalHours,
		&startedAt, &endedAt, &stoppedAt, &t.VetName, &t.Notes, &createdAt,
	)
	if err != nil {
		return nil, err
	}
	t.StartedAt, err = time.Parse(time.RFC3339, startedAt)
	if err != nil {
		return nil, fmt.Errorf("parse started_at %q: %w", startedAt, err)
	}
	t.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at %q: %w", createdAt, err)
	}
	if endedAt.Valid && endedAt.String != "" {
		ts, err := time.Parse(time.RFC3339, endedAt.String)
		if err != nil {
			return nil, fmt.Errorf("parse ended_at %q: %w", endedAt.String, err)
		}
		t.EndedAt = &ts
	}
	if stoppedAt.Valid && stoppedAt.String != "" {
		ts, err := time.Parse(time.RFC3339, stoppedAt.String)
		if err != nil {
			return nil, fmt.Errorf("parse stopped_at %q: %w", stoppedAt.String, err)
		}
		t.StoppedAt = &ts
	}
	return &t, nil
}

// formatOptionalRFC3339 formats a *time.Time as RFC3339, returning nil if t is nil.
func formatOptionalRFC3339(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.RFC3339)
	return &s
}
