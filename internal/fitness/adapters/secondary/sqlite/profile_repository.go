// internal/fitness/adapters/secondary/sqlite/profile_repository.go
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

type scanner interface {
	Scan(dest ...any) error
}

type ProfileRepository struct{ db *sql.DB }

func NewProfileRepository(db *sql.DB) *ProfileRepository { return &ProfileRepository{db: db} }

func (r *ProfileRepository) Create(ctx context.Context, p domain.Profile) (*domain.Profile, error) {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO fitness_profiles (id, first_name, last_name, birth_date, gender, height_cm, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.FirstName, p.LastName,
		p.BirthDate.Format("2006-01-02"), p.Gender, p.HeightCm,
		p.CreatedAt.Format(time.RFC3339), p.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, domain.ErrAlreadyExists
		}
		return nil, err
	}
	return &p, nil
}

func (r *ProfileRepository) Get(ctx context.Context) (*domain.Profile, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, first_name, last_name, birth_date, gender, height_cm, created_at, updated_at
		 FROM fitness_profiles LIMIT 1`)
	p, err := scanProfile(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return p, err
}

func (r *ProfileRepository) Update(ctx context.Context, p domain.Profile) (*domain.Profile, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE fitness_profiles SET first_name=?, last_name=?, birth_date=?, gender=?, height_cm=?, updated_at=?
		 WHERE id=?`,
		p.FirstName, p.LastName, p.BirthDate.Format("2006-01-02"),
		p.Gender, p.HeightCm, p.UpdatedAt.Format(time.RFC3339), p.ID,
	)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, domain.ErrNotFound
	}
	return &p, nil
}

func scanProfile(s scanner) (*domain.Profile, error) {
	var p domain.Profile
	var birthDate, createdAt, updatedAt string
	err := s.Scan(&p.ID, &p.FirstName, &p.LastName, &birthDate, &p.Gender, &p.HeightCm, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	p.BirthDate, err = time.Parse("2006-01-02", birthDate)
	if err != nil {
		return nil, fmt.Errorf("parse birth_date %q: %w", birthDate, err)
	}
	p.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at %q: %w", createdAt, err)
	}
	p.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at %q: %w", updatedAt, err)
	}
	return &p, nil
}

// isUniqueViolation reports whether err is a SQLite UNIQUE constraint violation.
func isUniqueViolation(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "UNIQUE constraint failed") ||
		strings.Contains(err.Error(), "unique constraint"))
}
