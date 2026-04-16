package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
)

type AppointmentRepository struct{ db dbtx }

func NewAppointmentRepository(db dbtx) *AppointmentRepository {
	return &AppointmentRepository{db: db}
}

func (r *AppointmentRepository) Create(ctx context.Context, a domain.Appointment) (*domain.Appointment, error) {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO appointments (id, pet_id, type, scheduled_at, provider, location, notes, google_calendar_event_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.PetID, string(a.Type), a.ScheduledAt.Format(time.RFC3339),
		a.Provider, a.Location, a.Notes, a.GoogleCalendarEventID,
		a.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *AppointmentRepository) GetByID(ctx context.Context, petID, appointmentID string) (*domain.Appointment, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, pet_id, type, scheduled_at, provider, location, notes, google_calendar_event_id, created_at
		FROM appointments WHERE id = ? AND pet_id = ?`, appointmentID, petID)
	a, err := scanAppointment(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return a, err
}

func (r *AppointmentRepository) List(ctx context.Context, petID string) ([]domain.Appointment, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, pet_id, type, scheduled_at, provider, location, notes, google_calendar_event_id, created_at
		FROM appointments WHERE pet_id = ? ORDER BY scheduled_at`, petID)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck

	var as []domain.Appointment
	for rows.Next() {
		a, err := scanAppointment(rows)
		if err != nil {
			return nil, err
		}
		as = append(as, *a)
	}
	return as, rows.Err()
}

func (r *AppointmentRepository) Update(ctx context.Context, a domain.Appointment) (*domain.Appointment, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE appointments SET scheduled_at=?, provider=?, location=?, notes=?
		WHERE id=? AND pet_id=?`,
		a.ScheduledAt.Format(time.RFC3339),
		a.Provider, a.Location, a.Notes,
		a.ID, a.PetID,
	)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, domain.ErrNotFound
	}
	return &a, nil
}

func (r *AppointmentRepository) Delete(ctx context.Context, petID, appointmentID string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM appointments WHERE id = ? AND pet_id = ?`, appointmentID, petID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func scanAppointment(s scanner) (*domain.Appointment, error) {
	var a domain.Appointment
	var typ string
	var scheduledAt string
	var createdAt string
	var provider sql.NullString
	var location sql.NullString
	var notes sql.NullString

	err := s.Scan(&a.ID, &a.PetID, &typ, &scheduledAt, &provider, &location, &notes, &a.GoogleCalendarEventID, &createdAt)
	if err != nil {
		return nil, err
	}
	a.Type = domain.AppointmentType(typ)
	a.ScheduledAt, err = time.Parse(time.RFC3339, scheduledAt)
	if err != nil {
		return nil, fmt.Errorf("parse scheduled_at %q: %w", scheduledAt, err)
	}
	a.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at %q: %w", createdAt, err)
	}
	if provider.Valid {
		a.Provider = &provider.String
	}
	if location.Valid {
		a.Location = &location.String
	}
	if notes.Valid {
		a.Notes = &notes.String
	}
	return &a, nil
}
