// internal/fitness/domain/errors.go
package domain

import "errors"

var (
	// ErrNotFound indicates the requested resource does not exist.
	ErrNotFound = errors.New("not found")

	// ErrValidation indicates the request failed domain validation.
	ErrValidation = errors.New("validation failed")

	// ErrAlreadyExists indicates the resource already exists.
	ErrAlreadyExists = errors.New("already exists")

	// ErrAlreadyAchieved is returned when a goal has already been achieved.
	ErrAlreadyAchieved = errors.New("already achieved")
)
