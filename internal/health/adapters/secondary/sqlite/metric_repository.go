package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rafaelsoares/alfredo/internal/health/domain"
)

type MetricRepository struct {
	db dbtx
}

func NewMetricRepository(db dbtx) *MetricRepository {
	return &MetricRepository{db: db}
}

func (r *MetricRepository) BulkUpsert(ctx context.Context, metrics []domain.DailyMetric) (int, error) {
	if len(metrics) == 0 {
		return 0, nil
	}

	now := time.Now().UTC()
	count := 0

	for _, m := range metrics {
		var sleepStagesJSON *string
		if m.SleepStages != nil {
			data, _ := json.Marshal(m.SleepStages)
			str := string(data)
			sleepStagesJSON = &str
		}

		_, err := r.db.ExecContext(ctx, `
			INSERT OR REPLACE INTO health_daily_metrics
			(date, metric_type, value, unit, sleep_stages, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`,
			m.Date,
			m.MetricType,
			m.Value,
			m.Unit,
			sleepStagesJSON,
			now.Format(time.RFC3339Nano),
			now.Format(time.RFC3339Nano),
		)
		if err != nil {
			return 0, fmt.Errorf("bulk upsert metrics: %w", err)
		}
		count++
	}

	return count, nil
}

func (r *MetricRepository) List(ctx context.Context, metricType string, from, to time.Time) ([]domain.DailyMetric, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, date, metric_type, value, unit, sleep_stages, created_at, updated_at
		FROM health_daily_metrics
		WHERE metric_type = ? AND date >= ? AND date <= ?
		ORDER BY date DESC
	`,
		metricType,
		from.Format("2006-01-02"),
		to.Format("2006-01-02"),
	)
	if err != nil {
		return nil, fmt.Errorf("query metrics: %w", err)
	}
	defer rows.Close()

	var metrics []domain.DailyMetric
	for rows.Next() {
		var m domain.DailyMetric
		var sleepStagesJSON *string
		var createdAt, updatedAt string

		err := rows.Scan(
			&m.ID,
			&m.Date,
			&m.MetricType,
			&m.Value,
			&m.Unit,
			&sleepStagesJSON,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan metric: %w", err)
		}

		if sleepStagesJSON != nil {
			var stages domain.SleepStages
			if err := json.Unmarshal([]byte(*sleepStagesJSON), &stages); err != nil {
				return nil, fmt.Errorf("unmarshal sleep stages: %w", err)
			}
			m.SleepStages = &stages
		}

		m.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		m.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)

		metrics = append(metrics, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate metrics: %w", err)
	}

	return metrics, nil
}
