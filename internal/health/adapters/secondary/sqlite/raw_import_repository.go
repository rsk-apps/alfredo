package sqlite

import (
	"context"
	"fmt"
	"time"
)

type RawImportRepository struct {
	db dbtx
}

func NewRawImportRepository(db dbtx) *RawImportRepository {
	return &RawImportRepository{db: db}
}

func (r *RawImportRepository) Store(ctx context.Context, importType string, payload string, importedAt time.Time) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO health_raw_imports
		(import_type, payload, imported_at, created_at)
		VALUES (?, ?, ?, ?)
	`,
		importType,
		payload,
		importedAt.Format(time.RFC3339Nano),
		now.Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("store raw import: %w", err)
	}
	return nil
}
