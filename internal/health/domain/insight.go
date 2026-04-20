package domain

type TrendDirection string

const (
	TrendUp     TrendDirection = "up"
	TrendDown   TrendDirection = "down"
	TrendStable TrendDirection = "stable"
	TrendNoData TrendDirection = "no_data"
)

type WeightInsight struct {
	HasData  bool
	Trend    TrendDirection
	DeltaKg  float64
	LatestKg float64
}

type BMIInsight struct {
	HasData bool
	Value   float64
}

type RestingHRInsight struct {
	HasData    bool
	Trend      TrendDirection
	AverageBPM float64
}

type SleepInsight struct {
	HasData                   bool
	AverageHours              float64
	ConsecutiveBelowThreshold int
}

type WorkoutInsight struct {
	HasData          bool
	CountThisWindow  int
	CountPriorWindow int
}

type VO2MaxInsight struct {
	HasData      bool
	Value        float64
	NormCategory string
}

type NotableFlag struct {
	Code    string
	Message string
}

type HealthInsight struct {
	WindowDays int
	Weight     WeightInsight
	BMI        BMIInsight
	RestingHR  RestingHRInsight
	Sleep      SleepInsight
	Workouts   WorkoutInsight
	VO2Max     VO2MaxInsight
	Flags      []NotableFlag
}
