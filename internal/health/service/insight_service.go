package service

import (
	"math"
	"sort"
	"strconv"
	"time"

	healthdomain "github.com/rafaelsoares/alfredo/internal/health/domain"
)

const (
	sleepThresholdHours         = 6.0
	consecutivePoorSleepTrigger = 3
	significantWeightDeltaKg    = 2.0
	minDataPointsForHRTrend     = 3
)

type InsightService struct{}

func NewInsightService() *InsightService {
	return &InsightService{}
}

func (s *InsightService) Compute(
	profile *healthdomain.HealthProfile,
	metricsByType map[string][]healthdomain.DailyMetric,
	workouts []healthdomain.WorkoutSession,
	days int,
) healthdomain.HealthInsight {
	weightMetrics := s.sortedMetricsByDate(metricsByType["weight"])
	restingHRMetrics := s.sortedMetricsByDate(metricsByType["restingHeartRate"])
	sleepMetrics := s.sortedMetricsByDate(metricsByType["sleepTime"])
	vo2MaxMetrics := s.sortedMetricsByDate(metricsByType["vo2Max"])

	insight := healthdomain.HealthInsight{
		WindowDays: days,
		Weight:     s.computeWeightInsight(weightMetrics),
		BMI:        s.computeBMIInsight(profile, weightMetrics),
		RestingHR:  s.computeRestingHRInsight(restingHRMetrics),
		Sleep:      s.computeSleepInsight(sleepMetrics),
		Workouts:   s.computeWorkoutInsight(workouts, days),
		VO2Max:     s.computeVO2MaxInsight(profile, vo2MaxMetrics),
	}
	insight.Flags = s.computeFlags(insight, profile)
	return insight
}

func (s *InsightService) computeWeightInsight(metrics []healthdomain.DailyMetric) healthdomain.WeightInsight {
	if len(metrics) == 0 {
		return healthdomain.WeightInsight{HasData: false}
	}
	insight := healthdomain.WeightInsight{HasData: true, LatestKg: metrics[len(metrics)-1].Value}

	if len(metrics) < 2 {
		return insight
	}

	midpoint := len(metrics) / 2
	firstHalf := metrics[:midpoint]
	secondHalf := metrics[midpoint:]

	firstAvg := s.average(firstHalf)
	secondAvg := s.average(secondHalf)

	delta := secondAvg - firstAvg
	insight.DeltaKg = metrics[len(metrics)-1].Value - metrics[0].Value

	if math.Abs(delta) < 0.5 {
		insight.Trend = healthdomain.TrendStable
	} else if delta > 0 {
		insight.Trend = healthdomain.TrendUp
	} else {
		insight.Trend = healthdomain.TrendDown
	}
	return insight
}

func (s *InsightService) computeBMIInsight(profile *healthdomain.HealthProfile, metrics []healthdomain.DailyMetric) healthdomain.BMIInsight {
	if profile == nil || profile.HeightCM <= 0 || len(metrics) == 0 {
		return healthdomain.BMIInsight{HasData: false}
	}
	weight := metrics[len(metrics)-1].Value
	heightM := profile.HeightCM / 100.0
	bmi := weight / (heightM * heightM)
	return healthdomain.BMIInsight{HasData: true, Value: bmi}
}

func (s *InsightService) computeRestingHRInsight(metrics []healthdomain.DailyMetric) healthdomain.RestingHRInsight {
	if len(metrics) < minDataPointsForHRTrend {
		return healthdomain.RestingHRInsight{HasData: false}
	}
	insight := healthdomain.RestingHRInsight{HasData: true}
	midpoint := len(metrics) / 2
	firstHalf := metrics[:midpoint]
	secondHalf := metrics[midpoint:]

	firstAvg := s.average(firstHalf)
	secondAvg := s.average(secondHalf)

	delta := secondAvg - firstAvg
	insight.AverageBPM = secondAvg

	if math.Abs(delta) <= 2 {
		insight.Trend = healthdomain.TrendStable
	} else if delta > 2 {
		insight.Trend = healthdomain.TrendUp
	} else {
		insight.Trend = healthdomain.TrendDown
	}
	return insight
}

func (s *InsightService) computeSleepInsight(metrics []healthdomain.DailyMetric) healthdomain.SleepInsight {
	if len(metrics) == 0 {
		return healthdomain.SleepInsight{HasData: false}
	}
	sleepByDate := s.groupSleepByDate(metrics)

	totalHours := 0.0
	for _, hours := range sleepByDate {
		totalHours += hours
	}
	avgHours := totalHours / float64(len(sleepByDate))

	maxConsecutive := s.maxConsecutiveBelowThreshold(sleepByDate)

	return healthdomain.SleepInsight{
		HasData:                   true,
		AverageHours:              avgHours,
		ConsecutiveBelowThreshold: maxConsecutive,
	}
}

func (s *InsightService) computeWorkoutInsight(workouts []healthdomain.WorkoutSession, days int) healthdomain.WorkoutInsight {
	if len(workouts) == 0 {
		return healthdomain.WorkoutInsight{HasData: false}
	}
	return healthdomain.WorkoutInsight{
		HasData:          true,
		CountThisWindow:  len(workouts),
		CountPriorWindow: 0,
	}
}

func (s *InsightService) computeVO2MaxInsight(profile *healthdomain.HealthProfile, metrics []healthdomain.DailyMetric) healthdomain.VO2MaxInsight {
	if len(metrics) == 0 {
		return healthdomain.VO2MaxInsight{HasData: false}
	}
	if profile == nil || profile.BirthDate == "" || profile.Sex == "" {
		return healthdomain.VO2MaxInsight{HasData: false}
	}
	vo2 := metrics[len(metrics)-1].Value
	age := s.ageFromBirthDate(profile.BirthDate)
	category := s.vo2MaxCategory(vo2, age, profile.Sex)
	return healthdomain.VO2MaxInsight{HasData: true, Value: vo2, NormCategory: category}
}

func (s *InsightService) computeFlags(insight healthdomain.HealthInsight, profile *healthdomain.HealthProfile) []healthdomain.NotableFlag {
	var flags []healthdomain.NotableFlag

	if insight.Sleep.HasData && insight.Sleep.ConsecutiveBelowThreshold >= consecutivePoorSleepTrigger {
		flags = append(flags, healthdomain.NotableFlag{
			Code:    "poor_sleep_streak",
			Message: "Sono ruim: " + strconv.Itoa(insight.Sleep.ConsecutiveBelowThreshold) + " noites seguidas abaixo do limiar",
		})
	}

	if insight.Weight.HasData && math.Abs(insight.Weight.DeltaKg) >= significantWeightDeltaKg {
		direction := "redução"
		if insight.Weight.DeltaKg > 0 {
			direction = "ganho"
		}
		flags = append(flags, healthdomain.NotableFlag{
			Code:    "significant_weight_change",
			Message: direction + " de peso: " + strconv.FormatFloat(math.Abs(insight.Weight.DeltaKg), 'f', 1, 64) + " kg",
		})
	}

	if insight.RestingHR.HasData && insight.RestingHR.Trend == healthdomain.TrendUp {
		flags = append(flags, healthdomain.NotableFlag{
			Code:    "resting_hr_trending_up",
			Message: "Frequência cardíaca de repouso em alta",
		})
	}

	return flags
}

func (s *InsightService) average(metrics []healthdomain.DailyMetric) float64 {
	if len(metrics) == 0 {
		return 0
	}
	sum := 0.0
	for _, m := range metrics {
		sum += m.Value
	}
	return sum / float64(len(metrics))
}

func (s *InsightService) sortedMetricsByDate(metrics []healthdomain.DailyMetric) []healthdomain.DailyMetric {
	if len(metrics) < 2 {
		return metrics
	}
	sorted := append([]healthdomain.DailyMetric(nil), metrics...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Date < sorted[j].Date
	})
	return sorted
}

func (s *InsightService) groupSleepByDate(metrics []healthdomain.DailyMetric) map[string]float64 {
	sleepByDate := make(map[string]float64)
	for _, m := range metrics {
		sleepByDate[m.Date] += m.Value
	}
	return sleepByDate
}

func (s *InsightService) maxConsecutiveBelowThreshold(sleepByDate map[string]float64) int {
	if len(sleepByDate) == 0 {
		return 0
	}
	dates := make([]string, 0, len(sleepByDate))
	for d := range sleepByDate {
		dates = append(dates, d)
	}
	sort.Strings(dates)

	maxConsecutive := 0
	currentConsecutive := 0
	var previousDate time.Time

	for _, date := range dates {
		currentDate, err := time.Parse("2006-01-02", date)
		if err != nil {
			currentConsecutive = 0
			continue
		}
		if !previousDate.IsZero() && currentDate.Sub(previousDate) != 24*time.Hour {
			currentConsecutive = 0
		}
		if sleepByDate[date] < sleepThresholdHours {
			currentConsecutive++
			if currentConsecutive > maxConsecutive {
				maxConsecutive = currentConsecutive
			}
		} else {
			currentConsecutive = 0
		}
		previousDate = currentDate
	}
	return maxConsecutive
}

func (s *InsightService) ageFromBirthDate(birthDateStr string) int {
	layout := "2006-01-02"
	birthDate, err := time.Parse(layout, birthDateStr)
	if err != nil {
		return 0
	}
	now := time.Now()
	age := now.Year() - birthDate.Year()
	if now.Month() < birthDate.Month() || (now.Month() == birthDate.Month() && now.Day() < birthDate.Day()) {
		age--
	}
	return age
}

func (s *InsightService) vo2MaxCategory(vo2 float64, age int, sex string) string {
	categories := s.vo2MaxRangesForAge(age)
	categoryNames := []string{"muito baixo", "baixo", "abaixo da média", "na média", "acima da média", "excelente"}

	ageGroupKey := sex
	if ageGroupKey != "M" && ageGroupKey != "F" {
		ageGroupKey = "M"
	}

	ranges, ok := categories[ageGroupKey]
	if !ok {
		return "desconhecido"
	}

	for i, r := range ranges {
		if vo2 >= r[0] && vo2 <= r[1] {
			if i < len(categoryNames) {
				return categoryNames[i]
			}
		}
	}
	return "desconhecido"
}

func (s *InsightService) vo2MaxRangesForAge(age int) map[string][][2]float64 {
	switch {
	case age < 30:
		return map[string][][2]float64{
			"M": {{0, 24.9}, {25.0, 33.9}, {34.0, 42.4}, {42.5, 46.4}, {46.5, 52.4}, {52.5, 100}},
			"F": {{0, 23.5}, {23.6, 28.9}, {29.0, 32.9}, {33.0, 36.9}, {37.0, 41.0}, {41.1, 100}},
		}
	case age < 40:
		return map[string][][2]float64{
			"M": {{0, 22.9}, {23.0, 30.9}, {31.0, 38.4}, {38.5, 44.0}, {44.1, 49.4}, {49.5, 100}},
			"F": {{0, 20.9}, {21.0, 26.9}, {27.0, 31.4}, {31.5, 35.6}, {35.7, 40.0}, {40.1, 100}},
		}
	case age < 50:
		return map[string][][2]float64{
			"M": {{0, 19.9}, {20.0, 26.9}, {27.0, 35.9}, {36.0, 41.0}, {41.1, 45.9}, {46.0, 100}},
			"F": {{0, 19.4}, {19.5, 24.4}, {24.5, 30.2}, {30.3, 33.9}, {34.0, 38.9}, {39.0, 100}},
		}
	case age < 60:
		return map[string][][2]float64{
			"M": {{0, 17.9}, {18.0, 24.9}, {25.0, 33.4}, {33.5, 39.2}, {39.3, 43.0}, {43.1, 100}},
			"F": {{0, 17.9}, {18.0, 22.7}, {22.8, 27.2}, {27.3, 31.4}, {31.5, 35.7}, {35.8, 100}},
		}
	default:
		return map[string][][2]float64{
			"M": {{0, 15.9}, {16.0, 21.9}, {22.0, 30.4}, {30.5, 35.2}, {35.3, 39.9}, {40.0, 100}},
			"F": {{0, 16.9}, {17.0, 21.4}, {21.5, 24.9}, {25.0, 30.2}, {30.3, 31.4}, {31.5, 100}},
		}
	}
}
