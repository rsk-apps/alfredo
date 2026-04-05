// internal/fitness/domain/body_snapshot.go
package domain

import "time"

type BodySnapshot struct {
	ID         string
	Date       time.Time // date only (no time component)
	WeightKg   *float64
	WaistCm    *float64
	HipCm      *float64
	NeckCm     *float64
	BodyFatPct *float64
	PhotoPath  *string
	CreatedAt  time.Time
}
