package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/rafaelsoares/alfredo/internal/health/domain"
)

type ProfileRepository struct {
	db dbtx
}

func NewProfileRepository(db dbtx) *ProfileRepository {
	return &ProfileRepository{db: db}
}

func (r *ProfileRepository) Get(ctx context.Context) (domain.HealthProfile, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, height_cm, birth_date, sex, google_calendar_id, created_at, updated_at
		FROM health_profiles
		WHERE id = 1
	`)
	profile, err := scanProfile(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.HealthProfile{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.HealthProfile{}, fmt.Errorf("select health profile: %w", err)
	}
	return profile, nil
}

func (r *ProfileRepository) Upsert(ctx context.Context, profile domain.HealthProfile) (domain.HealthProfile, error) {
	now := time.Now().UTC()
	profile.ID = 1
	profile.CreatedAt = now
	profile.UpdatedAt = now
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO health_profiles (id, height_cm, birth_date, sex, google_calendar_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			height_cm = excluded.height_cm,
			birth_date = excluded.birth_date,
			sex = excluded.sex,
			google_calendar_id = excluded.google_calendar_id,
			updated_at = excluded.updated_at
		RETURNING id, height_cm, birth_date, sex, google_calendar_id, created_at, updated_at
	`,
		profile.ID,
		profile.HeightCM,
		profile.BirthDate,
		profile.Sex,
		profile.GoogleCalendarID,
		profile.CreatedAt.Format(time.RFC3339Nano),
		profile.UpdatedAt.Format(time.RFC3339Nano),
	)
	profile, err := scanProfile(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.HealthProfile{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.HealthProfile{}, fmt.Errorf("upsert health profile: %w", err)
	}
	return profile, nil
}

func scanProfile(s scanner) (domain.HealthProfile, error) {
	var profile domain.HealthProfile
	var createdAt string
	var updatedAt string
	if err := s.Scan(&profile.ID, &profile.HeightCM, &profile.BirthDate, &profile.Sex, &profile.GoogleCalendarID, &createdAt, &updatedAt); err != nil {
		return domain.HealthProfile{}, err
	}
	created, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return domain.HealthProfile{}, fmt.Errorf("parse created_at %q: %w", createdAt, err)
	}
	updated, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return domain.HealthProfile{}, fmt.Errorf("parse updated_at %q: %w", updatedAt, err)
	}
	profile.CreatedAt = created
	profile.UpdatedAt = updated
	return profile, nil
}

func (r *ProfileRepository) GetCalendarID(ctx context.Context) (string, error) {
	var id string
	err := r.db.QueryRowContext(ctx, `SELECT google_calendar_id FROM health_profiles WHERE id = 1`).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return id, err
}

func (r *ProfileRepository) SetCalendarID(ctx context.Context, calendarID string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO health_profiles (id, google_calendar_id) VALUES (1, ?) ON CONFLICT(id) DO UPDATE SET google_calendar_id = excluded.google_calendar_id`,
		calendarID)
	return err
}

type scanner interface {
	Scan(dest ...any) error
}
