package app

import (
	"context"
	"errors"
	"testing"
)

type healthPingerFake struct {
	err    error
	called bool
}

func (p *healthPingerFake) Ping(context.Context) error {
	p.called = true
	return p.err
}

func TestHealthAggregatorCheckReportsEveryDependencyWithoutLeakingErrors(t *testing.T) {
	sqlite := &healthPingerFake{}
	calendar := &healthPingerFake{err: errors.New("sqlite path /private/data.db unavailable")}
	agg := NewHealthAggregator(map[string]HealthPinger{
		"sqlite":   sqlite,
		"calendar": calendar,
	})

	result := agg.Check(context.Background())

	if result.Status != "degraded" {
		t.Fatalf("status = %q, want degraded", result.Status)
	}
	if !sqlite.called || !calendar.called {
		t.Fatal("expected all dependency checks to run even when one fails")
	}
	if got := result.Dependencies["sqlite"].Status; got != "up" {
		t.Fatalf("sqlite status = %q, want up", got)
	}
	calendarStatus := result.Dependencies["calendar"]
	if calendarStatus.Status != "down" {
		t.Fatalf("calendar status = %q, want down", calendarStatus.Status)
	}
	if calendarStatus.Error != "unavailable" {
		t.Fatalf("calendar error = %q, want sanitized unavailable", calendarStatus.Error)
	}
}
