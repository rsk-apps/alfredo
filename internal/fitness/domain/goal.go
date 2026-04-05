// internal/fitness/domain/goal.go
package domain

import "time"

type Goal struct {
	ID          string
	Name        string
	Description *string
	TargetValue *float64
	TargetUnit  *string
	Deadline    *time.Time
	AchievedAt  *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
