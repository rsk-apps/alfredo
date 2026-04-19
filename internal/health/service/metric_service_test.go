package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rafaelsoares/alfredo/internal/health/domain"
)

var errMetricRepoFail = errors.New("metric repo error")
var errRawImportFail = errors.New("raw import error")

type metricRepoStub struct {
	bulkUpsertFn func(context.Context, []domain.DailyMetric) (int, error)
	listFn       func(context.Context, string, time.Time, time.Time) ([]domain.DailyMetric, error)
}

func (r *metricRepoStub) BulkUpsert(ctx context.Context, metrics []domain.DailyMetric) (int, error) {
	if r.bulkUpsertFn != nil {
		return r.bulkUpsertFn(ctx, metrics)
	}
	return 0, nil
}

func (r *metricRepoStub) List(ctx context.Context, metricType string, from, to time.Time) ([]domain.DailyMetric, error) {
	if r.listFn != nil {
		return r.listFn(ctx, metricType, from, to)
	}
	return nil, nil
}

type rawImportRepoStub struct {
	storeFn func(context.Context, string, string, time.Time) error
}

func (r *rawImportRepoStub) Store(ctx context.Context, importType string, payload string, importedAt time.Time) error {
	if r.storeFn != nil {
		return r.storeFn(ctx, importType, payload, importedAt)
	}
	return nil
}

func TestMetricServiceImportDelegatesAndWrapsRepositoryErrors(t *testing.T) {
	svc := NewMetricService(
		&metricRepoStub{
			bulkUpsertFn: func(context.Context, []domain.DailyMetric) (int, error) {
				return 0, errMetricRepoFail
			},
		},
		&rawImportRepoStub{},
	)

	_, err := svc.Import(context.Background(), nil, "", time.Now())
	if !errors.Is(err, errMetricRepoFail) {
		t.Fatalf("Import error = %v, want wrapped metric repo error", err)
	}
}

func TestMetricServiceImportWritesToBothRepositories(t *testing.T) {
	metrics := []domain.DailyMetric{{Date: "2026-04-18", MetricType: "weight", Value: 80.5}}
	payload := `{"test": "payload"}`
	importedAt := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)

	metricCalled := false
	rawCalled := false

	svc := NewMetricService(
		&metricRepoStub{
			bulkUpsertFn: func(_ context.Context, m []domain.DailyMetric) (int, error) {
				metricCalled = true
				if len(m) != 1 || m[0].MetricType != "weight" {
					t.Fatalf("BulkUpsert got %#v", m)
				}
				return 1, nil
			},
		},
		&rawImportRepoStub{
			storeFn: func(_ context.Context, importType, p string, ia time.Time) error {
				rawCalled = true
				if importType != "metrics" || p != payload || ia != importedAt {
					t.Fatalf("Store got importType=%s, payload=%s, importedAt=%v", importType, p, ia)
				}
				return nil
			},
		},
	)

	count, err := svc.Import(context.Background(), metrics, payload, importedAt)
	if err != nil {
		t.Fatalf("Import error = %v", err)
	}
	if !metricCalled || !rawCalled {
		t.Fatalf("metricCalled=%v, rawCalled=%v", metricCalled, rawCalled)
	}
	if count != 1 {
		t.Fatalf("Import count = %d, want 1", count)
	}
}

func TestMetricServiceImportWrapsRawImportErrors(t *testing.T) {
	svc := NewMetricService(
		&metricRepoStub{
			bulkUpsertFn: func(context.Context, []domain.DailyMetric) (int, error) {
				return 1, nil
			},
		},
		&rawImportRepoStub{
			storeFn: func(context.Context, string, string, time.Time) error {
				return errRawImportFail
			},
		},
	)

	_, err := svc.Import(context.Background(), nil, "", time.Now())
	if !errors.Is(err, errRawImportFail) {
		t.Fatalf("Import error = %v, want wrapped raw import error", err)
	}
}

func TestMetricServiceListDelegatesAndWrapsErrors(t *testing.T) {
	svc := NewMetricService(
		&metricRepoStub{
			listFn: func(context.Context, string, time.Time, time.Time) ([]domain.DailyMetric, error) {
				return nil, errMetricRepoFail
			},
		},
		&rawImportRepoStub{},
	)

	_, err := svc.List(context.Background(), "weight", time.Now(), time.Now())
	if !errors.Is(err, errMetricRepoFail) {
		t.Fatalf("List error = %v, want wrapped error", err)
	}
}
