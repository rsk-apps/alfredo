package domain

import "time"

// SleepStages represents the breakdown of sleep stages from Apple Health.
type SleepStages struct {
	Awake       float64 `json:"awake"`
	Core        float64 `json:"core"`
	Deep        float64 `json:"deep"`
	REM         float64 `json:"rem"`
	Unspecified float64 `json:"unspecified"`
}

// DailyMetric represents a single health metric recorded on a specific date.
type DailyMetric struct {
	ID          int
	Date        string        // YYYY-MM-DD
	MetricType  string        // e.g. "weight", "restingHeartRate", "sleepTime"
	Value       float64
	Unit        string
	SleepStages *SleepStages  // non-nil only when MetricType == "sleepTime"
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
