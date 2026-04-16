package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
)

type ObservationRepository struct{ db dbtx }

func NewObservationRepository(db dbtx) *ObservationRepository {
	return &ObservationRepository{db: db}
}

func (r *ObservationRepository) Create(ctx context.Context, observation domain.Observation) (*domain.Observation, error) {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO pet_observations (id, pet_id, observed_at, description, created_at) VALUES (?, ?, ?, ?, ?)`,
		observation.ID,
		observation.PetID,
		observation.ObservedAt.Format(time.RFC3339),
		observation.Description,
		observation.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, err
	}
	return &observation, nil
}

func (r *ObservationRepository) ListByPet(ctx context.Context, petID string) ([]domain.Observation, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, pet_id, observed_at, description, created_at
		 FROM pet_observations
		 WHERE pet_id = ?
		 ORDER BY observed_at DESC, created_at DESC`, petID)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck

	observations := make([]domain.Observation, 0)
	for rows.Next() {
		observation, err := scanObservation(rows)
		if err != nil {
			return nil, err
		}
		observations = append(observations, *observation)
	}
	return observations, rows.Err()
}

func (r *ObservationRepository) GetByID(ctx context.Context, petID, observationID string) (*domain.Observation, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, pet_id, observed_at, description, created_at
		 FROM pet_observations
		 WHERE id = ? AND pet_id = ?`, observationID, petID)
	observation, err := scanObservation(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return observation, err
}

func scanObservation(s scanner) (*domain.Observation, error) {
	var observation domain.Observation
	var observedAt, createdAt string
	err := s.Scan(&observation.ID, &observation.PetID, &observedAt, &observation.Description, &createdAt)
	if err != nil {
		return nil, err
	}
	observation.ObservedAt, err = time.Parse(time.RFC3339, observedAt)
	if err != nil {
		return nil, fmt.Errorf("parse observed_at %q: %w", observedAt, err)
	}
	observation.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at %q: %w", createdAt, err)
	}
	return &observation, nil
}
