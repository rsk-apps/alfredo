package service

import (
	"testing"
	"time"

	healthdomain "github.com/rafaelsoares/alfredo/internal/health/domain"
)

func TestComputeWeightTrend(t *testing.T) {
	svc := NewInsightService()

	t.Run("uptrend", func(t *testing.T) {
		metrics := []healthdomain.DailyMetric{
			{Date: "2026-01-01", MetricType: "weight", Value: 70.0},
			{Date: "2026-01-02", MetricType: "weight", Value: 70.1},
			{Date: "2026-01-03", MetricType: "weight", Value: 70.2},
			{Date: "2026-01-04", MetricType: "weight", Value: 71.0},
			{Date: "2026-01-05", MetricType: "weight", Value: 71.5},
		}
		insight := svc.computeWeightInsight(metrics)
		if !insight.HasData {
			t.Fatal("expected HasData=true")
		}
		if insight.Trend != healthdomain.TrendUp {
			t.Errorf("expected TrendUp, got %v", insight.Trend)
		}
		if insight.LatestKg != 71.5 {
			t.Errorf("expected LatestKg=71.5, got %v", insight.LatestKg)
		}
	})

	t.Run("downtrend", func(t *testing.T) {
		metrics := []healthdomain.DailyMetric{
			{Date: "2026-01-01", MetricType: "weight", Value: 75.0},
			{Date: "2026-01-02", MetricType: "weight", Value: 74.9},
			{Date: "2026-01-03", MetricType: "weight", Value: 74.8},
			{Date: "2026-01-04", MetricType: "weight", Value: 73.0},
			{Date: "2026-01-05", MetricType: "weight", Value: 72.5},
		}
		insight := svc.computeWeightInsight(metrics)
		if insight.Trend != healthdomain.TrendDown {
			t.Errorf("expected TrendDown, got %v", insight.Trend)
		}
	})

	t.Run("stable", func(t *testing.T) {
		metrics := []healthdomain.DailyMetric{
			{Date: "2026-01-01", MetricType: "weight", Value: 70.0},
			{Date: "2026-01-02", MetricType: "weight", Value: 70.1},
			{Date: "2026-01-03", MetricType: "weight", Value: 70.0},
			{Date: "2026-01-04", MetricType: "weight", Value: 70.15},
			{Date: "2026-01-05", MetricType: "weight", Value: 70.05},
		}
		insight := svc.computeWeightInsight(metrics)
		if insight.Trend != healthdomain.TrendStable {
			t.Errorf("expected TrendStable, got %v", insight.Trend)
		}
	})

	t.Run("no data", func(t *testing.T) {
		insight := svc.computeWeightInsight(nil)
		if insight.HasData {
			t.Fatal("expected HasData=false for nil metrics")
		}
	})
}

func TestComputeBMI(t *testing.T) {
	svc := NewInsightService()

	t.Run("with profile and weight", func(t *testing.T) {
		profile := &healthdomain.HealthProfile{HeightCM: 180}
		metrics := []healthdomain.DailyMetric{
			{Date: "2026-01-02", MetricType: "weight", Value: 75.0},
			{Date: "2026-01-01", MetricType: "weight", Value: 74.0},
		}
		insight := svc.Compute(profile, map[string][]healthdomain.DailyMetric{"weight": metrics}, nil, 14).BMI
		if !insight.HasData {
			t.Fatal("expected HasData=true")
		}
		expected := 75.0 / (1.8 * 1.8)
		if insight.Value < expected-0.01 || insight.Value > expected+0.01 {
			t.Errorf("expected BMI≈%.2f, got %.2f", expected, insight.Value)
		}
	})

	t.Run("nil profile", func(t *testing.T) {
		metrics := []healthdomain.DailyMetric{
			{Date: "2026-01-01", MetricType: "weight", Value: 75.0},
		}
		insight := svc.computeBMIInsight(nil, metrics)
		if insight.HasData {
			t.Fatal("expected HasData=false for nil profile")
		}
	})

	t.Run("no weight data", func(t *testing.T) {
		profile := &healthdomain.HealthProfile{HeightCM: 180}
		insight := svc.computeBMIInsight(profile, nil)
		if insight.HasData {
			t.Fatal("expected HasData=false for no metrics")
		}
	})
}

func TestComputeRestingHRTrend(t *testing.T) {
	svc := NewInsightService()

	t.Run("uptrend", func(t *testing.T) {
		metrics := []healthdomain.DailyMetric{
			{Date: "2026-01-01", MetricType: "restingHeartRate", Value: 60},
			{Date: "2026-01-02", MetricType: "restingHeartRate", Value: 61},
			{Date: "2026-01-03", MetricType: "restingHeartRate", Value: 65},
			{Date: "2026-01-04", MetricType: "restingHeartRate", Value: 67},
			{Date: "2026-01-05", MetricType: "restingHeartRate", Value: 68},
		}
		insight := svc.computeRestingHRInsight(metrics)
		if !insight.HasData {
			t.Fatal("expected HasData=true")
		}
		if insight.Trend != healthdomain.TrendUp {
			t.Errorf("expected TrendUp, got %v", insight.Trend)
		}
	})

	t.Run("descending input still computes chronologically", func(t *testing.T) {
		metrics := []healthdomain.DailyMetric{
			{Date: "2026-01-05", MetricType: "restingHeartRate", Value: 68},
			{Date: "2026-01-04", MetricType: "restingHeartRate", Value: 67},
			{Date: "2026-01-03", MetricType: "restingHeartRate", Value: 65},
			{Date: "2026-01-02", MetricType: "restingHeartRate", Value: 61},
			{Date: "2026-01-01", MetricType: "restingHeartRate", Value: 60},
		}
		insight := svc.Compute(nil, map[string][]healthdomain.DailyMetric{"restingHeartRate": metrics}, nil, 14).RestingHR
		if insight.Trend != healthdomain.TrendUp {
			t.Errorf("expected TrendUp for descending input, got %v", insight.Trend)
		}
		expected := (65.0 + 67.0 + 68.0) / 3.0
		if insight.AverageBPM < expected-0.001 || insight.AverageBPM > expected+0.001 {
			t.Errorf("expected later-window average %.3f, got %v", expected, insight.AverageBPM)
		}
	})

	t.Run("insufficient data", func(t *testing.T) {
		metrics := []healthdomain.DailyMetric{
			{Date: "2026-01-01", MetricType: "restingHeartRate", Value: 60},
			{Date: "2026-01-02", MetricType: "restingHeartRate", Value: 61},
		}
		insight := svc.computeRestingHRInsight(metrics)
		if insight.HasData {
			t.Fatal("expected HasData=false for < 3 data points")
		}
	})
}

func TestComputeSleepInsight(t *testing.T) {
	svc := NewInsightService()

	t.Run("with poor sleep streak", func(t *testing.T) {
		metrics := []healthdomain.DailyMetric{
			{Date: "2026-01-01", MetricType: "sleepTime", Value: 5.5},
			{Date: "2026-01-02", MetricType: "sleepTime", Value: 5.0},
			{Date: "2026-01-03", MetricType: "sleepTime", Value: 5.2},
			{Date: "2026-01-04", MetricType: "sleepTime", Value: 7.0},
			{Date: "2026-01-05", MetricType: "sleepTime", Value: 8.0},
		}
		insight := svc.computeSleepInsight(metrics)
		if !insight.HasData {
			t.Fatal("expected HasData=true")
		}
		if insight.ConsecutiveBelowThreshold != 3 {
			t.Errorf("expected 3 consecutive nights below threshold, got %d", insight.ConsecutiveBelowThreshold)
		}
	})

	t.Run("no data", func(t *testing.T) {
		insight := svc.computeSleepInsight(nil)
		if insight.HasData {
			t.Fatal("expected HasData=false for nil metrics")
		}
	})

	t.Run("gaps break the consecutive streak", func(t *testing.T) {
		metrics := []healthdomain.DailyMetric{
			{Date: "2026-01-01", MetricType: "sleepTime", Value: 5.5},
			{Date: "2026-01-03", MetricType: "sleepTime", Value: 5.0},
			{Date: "2026-01-05", MetricType: "sleepTime", Value: 5.2},
		}
		insight := svc.computeSleepInsight(metrics)
		if insight.ConsecutiveBelowThreshold != 1 {
			t.Errorf("expected gaps to reset streak, got %d", insight.ConsecutiveBelowThreshold)
		}
	})
}

func TestComputeVO2MaxCategory(t *testing.T) {
	svc := NewInsightService()

	tests := []struct {
		name     string
		vo2      float64
		age      int
		sex      string
		expected string
	}{
		{"male very low", 10.0, 30, "M", "muito baixo"},
		{"male low", 28.0, 30, "M", "baixo"},
		{"male excelente", 55.0, 30, "M", "excelente"},
		{"female very low", 10.0, 30, "F", "muito baixo"},
		{"female excelente", 45.0, 30, "F", "excelente"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			category := svc.vo2MaxCategory(tt.vo2, tt.age, tt.sex)
			if category != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, category)
			}
		})
	}

	t.Run("age affects the norm category", func(t *testing.T) {
		younger := svc.vo2MaxCategory(41.0, 25, "M")
		older := svc.vo2MaxCategory(41.0, 60, "M")
		if younger == older {
			t.Fatalf("expected age-specific categories, got %q for both", younger)
		}
	})
}

func TestComputeFlags(t *testing.T) {
	svc := NewInsightService()

	t.Run("poor sleep flag", func(t *testing.T) {
		insight := healthdomain.HealthInsight{
			Sleep: healthdomain.SleepInsight{
				HasData:                   true,
				ConsecutiveBelowThreshold: 3,
			},
		}
		flags := svc.computeFlags(insight, nil)
		if len(flags) == 0 {
			t.Fatal("expected poor_sleep_streak flag")
		}
		if flags[0].Code != "poor_sleep_streak" {
			t.Errorf("expected poor_sleep_streak, got %s", flags[0].Code)
		}
	})

	t.Run("significant weight change flag", func(t *testing.T) {
		insight := healthdomain.HealthInsight{
			Weight: healthdomain.WeightInsight{
				HasData: true,
				DeltaKg: 2.5,
			},
		}
		flags := svc.computeFlags(insight, nil)
		if len(flags) == 0 {
			t.Fatal("expected significant_weight_change flag")
		}
		if flags[0].Code != "significant_weight_change" {
			t.Errorf("expected significant_weight_change, got %s", flags[0].Code)
		}
	})

	t.Run("resting hr trending up flag", func(t *testing.T) {
		insight := healthdomain.HealthInsight{
			RestingHR: healthdomain.RestingHRInsight{
				HasData: true,
				Trend:   healthdomain.TrendUp,
			},
		}
		flags := svc.computeFlags(insight, nil)
		if len(flags) == 0 {
			t.Fatal("expected resting_hr_trending_up flag")
		}
		if flags[0].Code != "resting_hr_trending_up" {
			t.Errorf("expected resting_hr_trending_up, got %s", flags[0].Code)
		}
	})

	t.Run("no flags when all normal", func(t *testing.T) {
		insight := healthdomain.HealthInsight{
			Weight:    healthdomain.WeightInsight{HasData: true, DeltaKg: 0.5},
			RestingHR: healthdomain.RestingHRInsight{HasData: true, Trend: healthdomain.TrendStable},
			Sleep:     healthdomain.SleepInsight{HasData: true, ConsecutiveBelowThreshold: 1},
		}
		flags := svc.computeFlags(insight, nil)
		if len(flags) > 0 {
			t.Errorf("expected no flags, got %d", len(flags))
		}
	})
}

func TestAgeFromBirthDate(t *testing.T) {
	svc := NewInsightService()

	age := svc.ageFromBirthDate("1990-04-20")
	if age <= 0 {
		t.Errorf("expected positive age, got %d", age)
	}
}

func TestComputeFullInsight(t *testing.T) {
	svc := NewInsightService()

	profile := &healthdomain.HealthProfile{
		HeightCM:  180,
		BirthDate: "1990-01-01",
		Sex:       "M",
	}

	metricsByType := map[string][]healthdomain.DailyMetric{
		"weight": {
			{Date: "2026-04-01", MetricType: "weight", Value: 75.0},
			{Date: "2026-04-02", MetricType: "weight", Value: 75.2},
			{Date: "2026-04-03", MetricType: "weight", Value: 76.0},
		},
		"restingHeartRate": {
			{Date: "2026-04-01", MetricType: "restingHeartRate", Value: 62},
			{Date: "2026-04-02", MetricType: "restingHeartRate", Value: 63},
			{Date: "2026-04-03", MetricType: "restingHeartRate", Value: 64},
		},
		"sleepTime": {
			{Date: "2026-04-01", MetricType: "sleepTime", Value: 7.0},
			{Date: "2026-04-02", MetricType: "sleepTime", Value: 7.5},
			{Date: "2026-04-03", MetricType: "sleepTime", Value: 8.0},
		},
		"vo2Max": {
			{Date: "2026-04-03", MetricType: "vo2Max", Value: 45.0},
		},
	}

	workouts := []healthdomain.WorkoutSession{
		{StartDate: time.Now(), EndDate: time.Now()},
		{StartDate: time.Now(), EndDate: time.Now()},
	}

	insight := svc.Compute(profile, metricsByType, workouts, 14)

	if !insight.Weight.HasData {
		t.Fatal("expected weight data")
	}
	if !insight.BMI.HasData {
		t.Fatal("expected BMI data")
	}
	if !insight.RestingHR.HasData {
		t.Fatal("expected resting HR data")
	}
	if !insight.Sleep.HasData {
		t.Fatal("expected sleep data")
	}
	if !insight.VO2Max.HasData {
		t.Fatal("expected VO2Max data")
	}
}

func TestComputeFullInsightSortsDescendingMetricSlices(t *testing.T) {
	svc := NewInsightService()

	profile := &healthdomain.HealthProfile{
		HeightCM:  180,
		BirthDate: "1990-01-01",
		Sex:       "M",
	}

	metricsByType := map[string][]healthdomain.DailyMetric{
		"weight": {
			{Date: "2026-04-03", MetricType: "weight", Value: 76.0},
			{Date: "2026-04-02", MetricType: "weight", Value: 75.2},
			{Date: "2026-04-01", MetricType: "weight", Value: 75.0},
		},
	}

	insight := svc.Compute(profile, metricsByType, nil, 14)

	if insight.Weight.Trend != healthdomain.TrendUp {
		t.Fatalf("expected upward trend after sorting descending input, got %v", insight.Weight.Trend)
	}
	if insight.Weight.DeltaKg != 1.0 {
		t.Fatalf("expected delta of 1.0kg, got %v", insight.Weight.DeltaKg)
	}
	expectedBMI := 76.0 / (1.8 * 1.8)
	if insight.BMI.Value < expectedBMI-0.01 || insight.BMI.Value > expectedBMI+0.01 {
		t.Fatalf("expected BMI to use latest weight %.2f, got %.2f", expectedBMI, insight.BMI.Value)
	}
}
