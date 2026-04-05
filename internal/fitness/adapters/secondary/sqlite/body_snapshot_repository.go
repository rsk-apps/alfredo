// internal/fitness/adapters/secondary/sqlite/body_snapshot_repository.go
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

type BodySnapshotRepository struct{ db *sql.DB }

func NewBodySnapshotRepository(db *sql.DB) *BodySnapshotRepository {
	return &BodySnapshotRepository{db: db}
}

func (r *BodySnapshotRepository) Create(ctx context.Context, s domain.BodySnapshot) (*domain.BodySnapshot, error) {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO fitness_body_snapshots
		 (id, date, weight_kg, waist_cm, hip_cm, neck_cm, body_fat_pct, photo_path, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.Date.Format("2006-01-02"),
		s.WeightKg, s.WaistCm, s.HipCm, s.NeckCm, s.BodyFatPct, s.PhotoPath,
		s.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, domain.ErrAlreadyExists
		}
		return nil, err
	}
	return &s, nil
}

func (r *BodySnapshotRepository) GetByID(ctx context.Context, id string) (*domain.BodySnapshot, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, date, weight_kg, waist_cm, hip_cm, neck_cm, body_fat_pct, photo_path, created_at
		 FROM fitness_body_snapshots WHERE id = ?`, id)
	s, err := scanBodySnapshot(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return s, err
}

func (r *BodySnapshotRepository) List(ctx context.Context, from, to *time.Time) ([]domain.BodySnapshot, error) {
	query := `SELECT id, date, weight_kg, waist_cm, hip_cm, neck_cm, body_fat_pct, photo_path, created_at
	          FROM fitness_body_snapshots`
	args := []any{}
	clauses := []string{}
	if from != nil {
		clauses = append(clauses, "date >= ?")
		args = append(args, from.Format("2006-01-02"))
	}
	if to != nil {
		clauses = append(clauses, "date <= ?")
		args = append(args, to.Format("2006-01-02"))
	}
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY date DESC"
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	var snapshots []domain.BodySnapshot
	for rows.Next() {
		s, err := scanBodySnapshot(rows)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, *s)
	}
	return snapshots, rows.Err()
}

func (r *BodySnapshotRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM fitness_body_snapshots WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func scanBodySnapshot(s scanner) (*domain.BodySnapshot, error) {
	var snap domain.BodySnapshot
	var date, createdAt string
	err := s.Scan(&snap.ID, &date, &snap.WeightKg, &snap.WaistCm, &snap.HipCm,
		&snap.NeckCm, &snap.BodyFatPct, &snap.PhotoPath, &createdAt)
	if err != nil {
		return nil, err
	}
	snap.Date, err = time.Parse("2006-01-02", date)
	if err != nil {
		return nil, fmt.Errorf("parse date %q: %w", date, err)
	}
	snap.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at %q: %w", createdAt, err)
	}
	return &snap, nil
}
