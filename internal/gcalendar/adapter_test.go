package gcalendar

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestToGoogleEventRequiresTimezone(t *testing.T) {
	_, err := toGoogleEvent(Event{
		Title:     "Dose",
		StartTime: time.Now(),
		EndTime:   time.Now(),
	}, 0)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestToGoogleEventRecurringRuleAndTimezones(t *testing.T) {
	ev, err := toGoogleEvent(Event{
		Title:       "Amoxicillin",
		StartTime:   time.Date(2026, 4, 12, 12, 0, 0, 0, time.UTC),
		EndTime:     time.Date(2026, 4, 12, 12, 0, 0, 0, time.UTC),
		ReminderMins: nil,
		TimeZone:    "America/Sao_Paulo",
	}, 12)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Start.TimeZone != "America/Sao_Paulo" || ev.End.TimeZone != "America/Sao_Paulo" {
		t.Fatalf("unexpected timezones: start=%q end=%q", ev.Start.TimeZone, ev.End.TimeZone)
	}
	if len(ev.Recurrence) != 1 || ev.Recurrence[0] != "RRULE:FREQ=HOURLY;INTERVAL=12" {
		t.Fatalf("unexpected recurrence: %#v", ev.Recurrence)
	}
}

func TestToGoogleEventIncludesZeroValueReminderFields(t *testing.T) {
	ev, err := toGoogleEvent(Event{
		Title:       "Dose",
		StartTime:   time.Date(2026, 4, 12, 12, 0, 0, 0, time.UTC),
		EndTime:     time.Date(2026, 4, 12, 12, 0, 0, 0, time.UTC),
		ReminderMins: nil,
		TimeZone:    "America/Sao_Paulo",
	}, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	body, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	got := string(body)
	// When ReminderMins is nil, Overrides should be an empty slice
	if !strings.Contains(got, `"useDefault":false`) {
		t.Fatalf("expected marshaled event to contain useDefault:false, got %s", got)
	}
}
