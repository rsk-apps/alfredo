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
	ChestCm    *float64
	BicepsCm   *float64
	TricepsCm  *float64
	BodyFatPct *float64
	// Pollock 7-site skinfold measurements (mm)
	ChestSkinfoldMm       *float64
	MidaxillarySkinfoldMm *float64
	TricepsSkinfoldMm     *float64
	SubscapularSkinfoldMm *float64
	AbdominalSkinfoldMm   *float64
	SuprailiacSkinfoldMm  *float64
	ThighSkinfoldMm       *float64
	PhotoPath  *string
	CreatedAt  time.Time
}
