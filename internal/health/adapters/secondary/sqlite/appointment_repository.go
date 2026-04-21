package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/rafaelsoares/alfredo/internal/health/domain"
)

// HealthAppointmentRepository is the SQLite implementation of the appointment repository.
type HealthAppointmentRepository struct {
	db dbtx
}

// NewHealthAppointmentRepository creates a new health appointment repository.
func NewHealthAppointmentRepository(db dbtx) *HealthAppointmentRepository {
	return &HealthAppointmentRepository{db: db}
}

// Create creates a new health appointment.
func (r *HealthAppointmentRepository) Create(ctx context.Context, a domain.HealthAppointment) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO health_appointments (id, specialty, scheduled_at, doctor, notes, google_calendar_event_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`,
		a.ID,
		a.Specialty,
		a.ScheduledAt.Format(time.RFC3339),
		a.Doctor,
		a.Notes,
		a.GoogleCalendarEventID,
		a.CreatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("create health appointment: %w", err)
	}
	return nil
}

// GetByID retrieves a health appointment by ID.
func (r *HealthAppointmentRepository) GetByID(ctx context.Context, id string) (*domain.HealthAppointment, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, specialty, scheduled_at, doctor, notes, google_calendar_event_id, created_at
		FROM health_appointments
		WHERE id = ?
	`, id)
	appt, err := scanAppointment(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get health appointment: %w", err)
	}
	return appt, nil
}

// List retrieves all health appointments ordered by scheduled_at.
func (r *HealthAppointmentRepository) List(ctx context.Context) ([]domain.HealthAppointment, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, specialty, scheduled_at, doctor, notes, google_calendar_event_id, created_at
		FROM health_appointments
		ORDER BY scheduled_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list health appointments: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var appts []domain.HealthAppointment
	for rows.Next() {
		appt, err := scanAppointment(rows)
		if err != nil {
			return nil, fmt.Errorf("scan health appointment: %w", err)
		}
		appts = append(appts, *appt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate health appointments: %w", err)
	}
	return appts, nil
}

// Delete deletes a health appointment.
func (r *HealthAppointmentRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `
		DELETE FROM health_appointments
		WHERE id = ?
	`, id)
	if err != nil {
		return fmt.Errorf("delete health appointment: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func scanAppointment(s scanner) (*domain.HealthAppointment, error) {
	var appt domain.HealthAppointment
	var scheduledAtStr string
	var createdAtStr string
	if err := s.Scan(&appt.ID, &appt.Specialty, &scheduledAtStr, &appt.Doctor, &appt.Notes, &appt.GoogleCalendarEventID, &createdAtStr); err != nil {
		return nil, err
	}
	scheduledAt, err := time.Parse(time.RFC3339, scheduledAtStr)
	if err != nil {
		return nil, fmt.Errorf("parse scheduled_at %q: %w", scheduledAtStr, err)
	}
	createdAt, err := time.Parse(time.RFC3339Nano, createdAtStr)
	if err != nil {
		return nil, fmt.Errorf("parse created_at %q: %w", createdAtStr, err)
	}
	appt.ScheduledAt = scheduledAt
	appt.CreatedAt = createdAt
	return &appt, nil
}
