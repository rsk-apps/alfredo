package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
)

type VaccineRepository struct{ db *sql.DB }

func NewVaccineRepository(db *sql.DB) *VaccineRepository { return &VaccineRepository{db: db} }

func (r *VaccineRepository) ListVaccines(ctx context.Context, petID string) ([]domain.Vaccine, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, pet_id, name, administered_at, next_due_at, vet_name, batch_number, notes FROM vaccines WHERE pet_id = ? ORDER BY administered_at DESC`, petID)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	var vs []domain.Vaccine
	for rows.Next() {
		v, err := scanVaccine(rows)
		if err != nil {
			return nil, err
		}
		vs = append(vs, *v)
	}
	return vs, rows.Err()
}

func (r *VaccineRepository) CreateVaccine(ctx context.Context, v domain.Vaccine) (*domain.Vaccine, error) {
	_, err := r.db.ExecContext(ctx, `INSERT INTO vaccines (id, pet_id, name, administered_at, next_due_at, vet_name, batch_number, notes) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		v.ID, v.PetID, v.Name, v.AdministeredAt.Format(time.RFC3339),
		formatDate(v.NextDueAt), v.VetName, v.BatchNumber, v.Notes)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *VaccineRepository) GetVaccine(ctx context.Context, petID, vaccineID string) (*domain.Vaccine, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, pet_id, name, administered_at, next_due_at, vet_name, batch_number, notes FROM vaccines WHERE id = ? AND pet_id = ?`, vaccineID, petID)
	v, err := scanVaccine(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return v, err
}

func (r *VaccineRepository) DeleteVaccine(ctx context.Context, petID, vaccineID string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM vaccines WHERE id = ? AND pet_id = ?`, vaccineID, petID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func scanVaccine(s scanner) (*domain.Vaccine, error) {
	var v domain.Vaccine
	var adminAt string
	var nextDue sql.NullString
	err := s.Scan(&v.ID, &v.PetID, &v.Name, &adminAt, &nextDue, &v.VetName, &v.BatchNumber, &v.Notes)
	if err != nil {
		return nil, err
	}
	v.AdministeredAt, err = time.Parse(time.RFC3339, adminAt)
	if err != nil {
		return nil, fmt.Errorf("parse administered_at %q: %w", adminAt, err)
	}
	if nextDue.Valid && nextDue.String != "" {
		t, err := time.Parse("2006-01-02", nextDue.String)
		if err != nil {
			return nil, fmt.Errorf("parse next_due_at %q: %w", nextDue.String, err)
		}
		v.NextDueAt = &t
	}
	return &v, nil
}
