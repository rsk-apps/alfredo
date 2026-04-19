package port

import (
	"context"
	"time"
)

type RawImportRepository interface {
	Store(ctx context.Context, importType string, payload string, importedAt time.Time) error
}
