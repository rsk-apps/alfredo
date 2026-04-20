package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	healthdomain "github.com/rafaelsoares/alfredo/internal/health/domain"
	healthservice "github.com/rafaelsoares/alfredo/internal/health/service"
	"github.com/rafaelsoares/alfredo/internal/telegram"
)

type mockHealthProfileQuerier struct {
	profile *healthdomain.HealthProfile
	err     error
}

func (m *mockHealthProfileQuerier) Get(ctx context.Context) (healthdomain.HealthProfile, error) {
	if m.err != nil {
		return healthdomain.HealthProfile{}, m.err
	}
	if m.profile != nil {
		return *m.profile, nil
	}
	return healthdomain.HealthProfile{}, healthdomain.ErrNotFound
}

type mockHealthMetricsQuerier struct {
	metrics map[string][]healthdomain.DailyMetric
	err     error
}

func (m *mockHealthMetricsQuerier) List(ctx context.Context, metricType string, from, to time.Time) ([]healthdomain.DailyMetric, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.metrics[metricType], nil
}

type mockHealthWorkoutsQuerier struct {
	workouts []healthdomain.WorkoutSession
	err      error
}

func (m *mockHealthWorkoutsQuerier) List(ctx context.Context, from, to time.Time) ([]healthdomain.WorkoutSession, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.workouts, nil
}

type recordingTelegramPort struct {
	messages []string
	err      error
}

func (r *recordingTelegramPort) Send(ctx context.Context, msg telegram.Message) error {
	if r.err != nil {
		return r.err
	}
	r.messages = append(r.messages, msg.Text)
	return nil
}

func TestHealthDigestUseCaseComputeWithValidData(t *testing.T) {
	profile := &healthdomain.HealthProfile{HeightCM: 180, BirthDate: "1990-01-01", Sex: "M"}
	metrics := map[string][]healthdomain.DailyMetric{
		"weight": {
			{Date: "2026-04-01", MetricType: "weight", Value: 75.0},
			{Date: "2026-04-02", MetricType: "weight", Value: 75.5},
		},
		"restingHeartRate": {
			{Date: "2026-04-01", MetricType: "restingHeartRate", Value: 62},
			{Date: "2026-04-02", MetricType: "restingHeartRate", Value: 63},
			{Date: "2026-04-03", MetricType: "restingHeartRate", Value: 64},
		},
	}
	workouts := []healthdomain.WorkoutSession{}

	profileQuerier := &mockHealthProfileQuerier{profile: profile}
	metricsQuerier := &mockHealthMetricsQuerier{metrics: metrics}
	workoutsQuerier := &mockHealthWorkoutsQuerier{workouts: workouts}

	uc := NewHealthDigestUseCase(profileQuerier, metricsQuerier, workoutsQuerier, healthservice.NewInsightService(), nil, time.UTC, zap.NewNop())

	insight, err := uc.Compute(context.Background(), 14)
	if err != nil {
		t.Fatalf("Compute failed: %v", err)
	}

	if !insight.Weight.HasData {
		t.Error("expected weight data")
	}
	if !insight.RestingHR.HasData {
		t.Error("expected resting HR data")
	}
	if !insight.BMI.HasData {
		t.Error("expected BMI data")
	}
}

func TestHealthDigestUseCaseComputeWithoutProfile(t *testing.T) {
	profileQuerier := &mockHealthProfileQuerier{err: healthdomain.ErrNotFound}
	metricsQuerier := &mockHealthMetricsQuerier{metrics: map[string][]healthdomain.DailyMetric{}}
	workoutsQuerier := &mockHealthWorkoutsQuerier{workouts: []healthdomain.WorkoutSession{}}

	uc := NewHealthDigestUseCase(profileQuerier, metricsQuerier, workoutsQuerier, healthservice.NewInsightService(), nil, time.UTC, zap.NewNop())

	insight, err := uc.Compute(context.Background(), 14)
	if err != nil {
		t.Fatalf("Compute failed: %v", err)
	}

	if insight.BMI.HasData {
		t.Error("expected BMI without data when profile is missing")
	}
}

func TestHealthDigestUseCaseRunDigestSendsTelegram(t *testing.T) {
	profile := &healthdomain.HealthProfile{HeightCM: 180, BirthDate: "1990-01-01", Sex: "M"}
	metrics := map[string][]healthdomain.DailyMetric{
		"weight": {
			{Date: "2026-04-01", MetricType: "weight", Value: 75.0},
		},
	}
	workouts := []healthdomain.WorkoutSession{}

	profileQuerier := &mockHealthProfileQuerier{profile: profile}
	metricsQuerier := &mockHealthMetricsQuerier{metrics: metrics}
	workoutsQuerier := &mockHealthWorkoutsQuerier{workouts: workouts}
	telegram := &recordingTelegramPort{}

	uc := NewHealthDigestUseCase(profileQuerier, metricsQuerier, workoutsQuerier, healthservice.NewInsightService(), telegram, time.UTC, zap.NewNop())

	err := uc.RunDigest(context.Background(), 14)
	if err != nil {
		t.Fatalf("RunDigest failed: %v", err)
	}

	if len(telegram.messages) == 0 {
		t.Fatal("expected Telegram message to be sent")
	}
}

func TestHealthDigestUseCaseRunDigestSwallowsTelegramError(t *testing.T) {
	profile := &healthdomain.HealthProfile{HeightCM: 180}
	metrics := map[string][]healthdomain.DailyMetric{
		"weight": {
			{Date: "2026-04-01", MetricType: "weight", Value: 75.0},
		},
	}
	workouts := []healthdomain.WorkoutSession{}

	profileQuerier := &mockHealthProfileQuerier{profile: profile}
	metricsQuerier := &mockHealthMetricsQuerier{metrics: metrics}
	workoutsQuerier := &mockHealthWorkoutsQuerier{workouts: workouts}
	telegram := &recordingTelegramPort{err: errors.New("telegram down")}

	uc := NewHealthDigestUseCase(profileQuerier, metricsQuerier, workoutsQuerier, healthservice.NewInsightService(), telegram, time.UTC, zap.NewNop())

	err := uc.RunDigest(context.Background(), 14)
	if err != nil {
		t.Fatalf("RunDigest should swallow telegram error, got: %v", err)
	}
}

func TestHealthDigestUseCaseRunDigestWithNoData(t *testing.T) {
	profileQuerier := &mockHealthProfileQuerier{err: healthdomain.ErrNotFound}
	metricsQuerier := &mockHealthMetricsQuerier{metrics: map[string][]healthdomain.DailyMetric{}}
	workoutsQuerier := &mockHealthWorkoutsQuerier{workouts: []healthdomain.WorkoutSession{}}
	telegram := &recordingTelegramPort{}

	uc := NewHealthDigestUseCase(profileQuerier, metricsQuerier, workoutsQuerier, healthservice.NewInsightService(), telegram, time.UTC, zap.NewNop())

	err := uc.RunDigest(context.Background(), 14)
	if err != nil {
		t.Fatalf("RunDigest failed: %v", err)
	}

	if len(telegram.messages) == 0 {
		t.Fatal("expected Telegram message even when no data")
	}

	msg := telegram.messages[0]
	if msg == "" {
		t.Fatal("expected non-empty message")
	}
}

func TestHealthDigestUseCaseComputeProfileQueryError(t *testing.T) {
	profileQuerier := &mockHealthProfileQuerier{err: errors.New("db error")}
	metricsQuerier := &mockHealthMetricsQuerier{metrics: map[string][]healthdomain.DailyMetric{}}
	workoutsQuerier := &mockHealthWorkoutsQuerier{workouts: []healthdomain.WorkoutSession{}}

	uc := NewHealthDigestUseCase(profileQuerier, metricsQuerier, workoutsQuerier, healthservice.NewInsightService(), nil, time.UTC, zap.NewNop())

	_, err := uc.Compute(context.Background(), 14)
	if err == nil {
		t.Fatal("expected error from profile query failure")
	}
}

func TestHealthDigestUseCaseComputeMetricsQueryError(t *testing.T) {
	profileQuerier := &mockHealthProfileQuerier{err: healthdomain.ErrNotFound}
	metricsQuerier := &mockHealthMetricsQuerier{err: errors.New("db error")}
	workoutsQuerier := &mockHealthWorkoutsQuerier{workouts: []healthdomain.WorkoutSession{}}

	uc := NewHealthDigestUseCase(profileQuerier, metricsQuerier, workoutsQuerier, healthservice.NewInsightService(), nil, time.UTC, zap.NewNop())

	_, err := uc.Compute(context.Background(), 14)
	if err == nil {
		t.Fatal("expected error from metrics query failure")
	}
}

func TestHealthDigestUseCaseComputeWorkoutsQueryError(t *testing.T) {
	profileQuerier := &mockHealthProfileQuerier{err: healthdomain.ErrNotFound}
	metricsQuerier := &mockHealthMetricsQuerier{metrics: map[string][]healthdomain.DailyMetric{}}
	workoutsQuerier := &mockHealthWorkoutsQuerier{err: errors.New("db error")}

	uc := NewHealthDigestUseCase(profileQuerier, metricsQuerier, workoutsQuerier, healthservice.NewInsightService(), nil, time.UTC, zap.NewNop())

	_, err := uc.Compute(context.Background(), 14)
	if err == nil {
		t.Fatal("expected error from workouts query failure")
	}
}

func TestHealthDigestUseCaseComputeWithWorkoutData(t *testing.T) {
	profileQuerier := &mockHealthProfileQuerier{err: healthdomain.ErrNotFound}
	metricsQuerier := &mockHealthMetricsQuerier{metrics: map[string][]healthdomain.DailyMetric{}}
	workouts := []healthdomain.WorkoutSession{
		{StartDate: time.Now(), EndDate: time.Now()},
		{StartDate: time.Now(), EndDate: time.Now()},
	}
	workoutsQuerier := &mockHealthWorkoutsQuerier{workouts: workouts}

	uc := NewHealthDigestUseCase(profileQuerier, metricsQuerier, workoutsQuerier, healthservice.NewInsightService(), nil, time.UTC, zap.NewNop())

	insight, err := uc.Compute(context.Background(), 14)
	if err != nil {
		t.Fatalf("Compute failed: %v", err)
	}

	if !insight.Workouts.HasData {
		t.Error("expected workout data")
	}
	if insight.Workouts.CountThisWindow != 2 {
		t.Errorf("expected 2 workouts, got %d", insight.Workouts.CountThisWindow)
	}
}

func TestHealthDigestUseCaseTelegramMessageFormatting(t *testing.T) {
	profile := &healthdomain.HealthProfile{HeightCM: 180, BirthDate: "1990-01-01", Sex: "M"}
	metrics := map[string][]healthdomain.DailyMetric{
		"weight": {{Date: "2026-04-01", MetricType: "weight", Value: 75.0}, {Date: "2026-04-02", MetricType: "weight", Value: 76.0}},
		"restingHeartRate": {{Date: "2026-04-01", MetricType: "restingHeartRate", Value: 60}, {Date: "2026-04-02", MetricType: "restingHeartRate", Value: 65}, {Date: "2026-04-03", MetricType: "restingHeartRate", Value: 70}},
		"sleepTime": {{Date: "2026-04-01", MetricType: "sleepTime", Value: 5.0}, {Date: "2026-04-02", MetricType: "sleepTime", Value: 5.5}, {Date: "2026-04-03", MetricType: "sleepTime", Value: 6.0}},
		"vo2Max": {{Date: "2026-04-03", MetricType: "vo2Max", Value: 45.0}},
	}
	telegram := &recordingTelegramPort{}

	profileQuerier := &mockHealthProfileQuerier{profile: profile}
	metricsQuerier := &mockHealthMetricsQuerier{metrics: metrics}
	workoutsQuerier := &mockHealthWorkoutsQuerier{workouts: []healthdomain.WorkoutSession{{StartDate: time.Now()}}}

	uc := NewHealthDigestUseCase(profileQuerier, metricsQuerier, workoutsQuerier, healthservice.NewInsightService(), telegram, time.UTC, zap.NewNop())

	err := uc.RunDigest(context.Background(), 14)
	if err != nil {
		t.Fatalf("RunDigest failed: %v", err)
	}

	msg := telegram.messages[0]
	requiredStrings := []string{"Resumo de Saúde", "Peso", "IMC", "FC Repouso", "Sono", "Treinos", "VO2Max"}
	for _, required := range requiredStrings {
		if !strings.Contains(msg, required) {
			t.Errorf("expected message to contain %q, got: %s", required, msg)
		}
	}
}

func TestHealthDigestUseCaseEmptyDataMessage(t *testing.T) {
	profileQuerier := &mockHealthProfileQuerier{err: healthdomain.ErrNotFound}
	metricsQuerier := &mockHealthMetricsQuerier{metrics: map[string][]healthdomain.DailyMetric{}}
	workoutsQuerier := &mockHealthWorkoutsQuerier{workouts: []healthdomain.WorkoutSession{}}
	telegram := &recordingTelegramPort{}

	uc := NewHealthDigestUseCase(profileQuerier, metricsQuerier, workoutsQuerier, healthservice.NewInsightService(), telegram, time.UTC, zap.NewNop())

	err := uc.RunDigest(context.Background(), 14)
	if err != nil {
		t.Fatalf("RunDigest failed: %v", err)
	}

	msg := telegram.messages[0]
	if !strings.Contains(msg, "Nenhum dado") {
		t.Errorf("expected empty-data message, got: %s", msg)
	}
}
