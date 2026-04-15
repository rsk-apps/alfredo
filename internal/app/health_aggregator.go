package app

import (
	"context"
	"time"

	"github.com/rafaelsoares/alfredo/internal/shared/health"
)

// HealthAggregator combines health checks from all registered dependencies
// into a single /api/v1/health response.
type HealthAggregator struct {
	checkers map[string]HealthPinger
}

func NewHealthAggregator(checkers map[string]HealthPinger) *HealthAggregator {
	return &HealthAggregator{checkers: checkers}
}

func (h *HealthAggregator) Check(ctx context.Context) health.HealthResult {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	deps := make(map[string]health.DependencyStatus, len(h.checkers))
	allHealthy := true

	for name, checker := range h.checkers {
		if err := checker.Ping(ctx); err != nil {
			// Return a generic error string to avoid leaking internal details
			// (e.g. DB file paths) on the unauthenticated /health endpoint.
			deps[name] = health.DependencyStatus{Status: "down", Error: "unavailable"}
			allHealthy = false
		} else {
			deps[name] = health.DependencyStatus{Status: "up"}
		}
	}

	status := "healthy"
	if !allHealthy {
		status = "degraded"
	}
	return health.HealthResult{Status: status, Dependencies: deps}
}
