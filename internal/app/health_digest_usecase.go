package app

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	healthdomain "github.com/rafaelsoares/alfredo/internal/health/domain"
	healthservice "github.com/rafaelsoares/alfredo/internal/health/service"
	"github.com/rafaelsoares/alfredo/internal/telegram"
)

type HealthDigestUseCase struct {
	profile  HealthProfileQuerier
	metrics  HealthMetricsQuerier
	workouts HealthWorkoutsQuerier
	insight  *healthservice.InsightService
	telegram TelegramPort
	tz       *time.Location
	logger   *zap.Logger
}

func NewHealthDigestUseCase(
	profile HealthProfileQuerier,
	metrics HealthMetricsQuerier,
	workouts HealthWorkoutsQuerier,
	insight *healthservice.InsightService,
	telegram TelegramPort,
	timezone *time.Location,
	logger *zap.Logger,
) *HealthDigestUseCase {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &HealthDigestUseCase{
		profile:  profile,
		metrics:  metrics,
		workouts: workouts,
		insight:  insight,
		telegram: telegram,
		tz:       timezone,
		logger:   logger,
	}
}

func (uc *HealthDigestUseCase) Compute(ctx context.Context, days int) (healthdomain.HealthInsight, error) {
	now := time.Now().In(uc.tz)
	from := now.AddDate(0, 0, -days)

	profile, err := uc.profile.Get(ctx)
	if err != nil && !isNotFound(err) {
		return healthdomain.HealthInsight{}, fmt.Errorf("fetch profile: %w", err)
	}
	var profilePtr *healthdomain.HealthProfile
	if err == nil {
		profilePtr = &profile
	}

	metricTypes := []string{"weight", "restingHeartRate", "sleepTime", "vo2Max"}
	metricsByType := make(map[string][]healthdomain.DailyMetric)

	for _, typ := range metricTypes {
		list, err := uc.metrics.List(ctx, typ, from, now)
		if err != nil {
			return healthdomain.HealthInsight{}, fmt.Errorf("fetch metrics %q: %w", typ, err)
		}
		if list != nil {
			metricsByType[typ] = list
		}
	}

	thisWindow, err := uc.workouts.List(ctx, from, now)
	if err != nil {
		return healthdomain.HealthInsight{}, fmt.Errorf("fetch workouts this window: %w", err)
	}

	priorFrom := from.AddDate(0, 0, -days)
	priorWindow, err := uc.workouts.List(ctx, priorFrom, from)
	if err != nil {
		return healthdomain.HealthInsight{}, fmt.Errorf("fetch workouts prior window: %w", err)
	}

	allWorkouts := append(thisWindow, priorWindow...)

	result := uc.insight.Compute(profilePtr, metricsByType, allWorkouts, days)
	result.Workouts.CountThisWindow = len(thisWindow)
	result.Workouts.CountPriorWindow = len(priorWindow)

	return result, nil
}

func (uc *HealthDigestUseCase) RunDigest(ctx context.Context, days int) error {
	insight, err := uc.Compute(ctx, days)
	if err != nil {
		uc.logger.Warn("compute insight failed", zap.Error(err))
		insight = healthdomain.HealthInsight{WindowDays: days}
	}

	msg := formatTelegramMessage(insight)
	if err := uc.telegram.Send(ctx, telegram.Message{Text: msg}); err != nil {
		uc.logger.Warn("telegram send failed", zap.Error(err))
	}

	return nil
}

func formatTelegramMessage(insight healthdomain.HealthInsight) string {
	hasAnyData := insight.Weight.HasData || insight.RestingHR.HasData ||
		insight.Sleep.HasData || insight.Workouts.HasData || insight.VO2Max.HasData

	if !hasAnyData {
		return "📊 <b>Resumo de Saúde — últimos " + fmt.Sprintf("%d", insight.WindowDays) + " dias</b>\n\n" +
			"Nenhum dado de saúde registrado para o período."
	}

	msg := "📊 <b>Resumo de Saúde — últimos " + fmt.Sprintf("%d", insight.WindowDays) + " dias</b>\n\n"

	if insight.Weight.HasData {
		trendEmoji := trendEmojiForDirection(insight.Weight.Trend)
		msg += fmt.Sprintf("<b>Peso:</b> %s %.1f kg (últimos: %.1f kg)\n",
			trendEmoji, insight.Weight.DeltaKg, insight.Weight.LatestKg)
	}

	if insight.BMI.HasData {
		msg += fmt.Sprintf("<b>IMC:</b> %.1f\n", insight.BMI.Value)
	}

	if insight.RestingHR.HasData {
		trendEmoji := trendEmojiForDirection(insight.RestingHR.Trend)
		msg += fmt.Sprintf("<b>FC Repouso:</b> %s %.0f bpm\n", trendEmoji, insight.RestingHR.AverageBPM)
	}

	if insight.Sleep.HasData {
		msg += fmt.Sprintf("<b>Sono:</b> %.1f h/noite\n", insight.Sleep.AverageHours)
	}

	if insight.Workouts.HasData {
		msg += fmt.Sprintf("<b>Treinos:</b> %d nesta janela (vs %d na anterior)\n",
			insight.Workouts.CountThisWindow, insight.Workouts.CountPriorWindow)
	}

	if insight.VO2Max.HasData {
		msg += fmt.Sprintf("<b>VO2Max:</b> %.1f (%s)\n", insight.VO2Max.Value, insight.VO2Max.NormCategory)
	}

	if len(insight.Flags) > 0 {
		msg += "\n⚠️ <b>Alertas</b>\n"
		for _, flag := range insight.Flags {
			msg += "• " + flag.Message + "\n"
		}
	}

	return msg
}

func trendEmojiForDirection(trend healthdomain.TrendDirection) string {
	switch trend {
	case healthdomain.TrendUp:
		return "📈"
	case healthdomain.TrendDown:
		return "📉"
	case healthdomain.TrendStable:
		return "➡️"
	case healthdomain.TrendNoData:
		return "❓"
	default:
		return "❓"
	}
}

func isNotFound(err error) bool {
	return err != nil && err.Error() == healthdomain.ErrNotFound.Error()
}
