package port

import (
	"context"
	"time"

	"github.com/rafaelsoares/alfredo/internal/health/domain"
)

type MetricRepository interface {
	BulkUpsert(ctx context.Context, metrics []domain.DailyMetric) (int, error)
	List(ctx context.Context, metricType string, from, to time.Time) ([]domain.DailyMetric, error)
}
