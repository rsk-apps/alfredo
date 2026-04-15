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
	ReminderMin int
	TimeZone    string
}

// Port is the outbound calendar interface used by app use cases.
type Port interface {
	CreateCalendar(ctx context.Context, name string) (calendarID string, err error)
	DeleteCalendar(ctx context.Context, calendarID string) error
	CreateEvent(ctx context.Context, calendarID string, event Event) (eventID string, err error)
	CreateRecurringEvent(ctx context.Context, calendarID string, event Event, intervalHours int) (eventID string, err error)
	StopRecurringEvent(ctx context.Context, calendarID string, eventID string, until time.Time) error
	DeleteEvent(ctx context.Context, calendarID string, eventID string) error
}
