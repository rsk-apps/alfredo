package gcalendar

import (
	"context"
	"time"
)

// Event describes the domain-level information needed to create a calendar entry.
type Event struct {
	Title       string
	Description string
	Location    string // optional — omitted from Google Calendar request when empty
	StartTime   time.Time
	EndTime     time.Time
	ReminderMins []int
	TimeZone    string
}

// Port is the outbound calendar interface used by app use cases.
type Port interface {
	CreateCalendar(ctx context.Context, name string) (calendarID string, err error)
	DeleteCalendar(ctx context.Context, calendarID string) error
	CreateEvent(ctx context.Context, calendarID string, event Event) (eventID string, err error)
	UpdateEvent(ctx context.Context, calendarID string, eventID string, event Event) error
	CreateRecurringEvent(ctx context.Context, calendarID string, event Event, intervalHours int) (eventID string, err error)
	// StopRecurringEvent truncates the recurrence rule so the series ends at or before until
	// (inclusive per RFC 5545). Pass the timestamp of the last desired occurrence, not the
	// deletion moment, to avoid emitting a phantom final event.
	StopRecurringEvent(ctx context.Context, calendarID string, eventID string, until time.Time) error
	DeleteEvent(ctx context.Context, calendarID string, eventID string) error
}
