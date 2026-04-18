package gcalendar

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// NoopAdapter logs calls and returns deterministic fake IDs for local development.
type NoopAdapter struct {
	logger *zap.Logger
}

func NewNoopAdapter(logger *zap.Logger) *NoopAdapter {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &NoopAdapter{logger: logger}
}

func (a *NoopAdapter) CreateCalendar(_ context.Context, name string) (string, error) {
	id := deterministicID("calendar", name)
	a.logger.Info("gcalendar noop create calendar", zap.String("name", name), zap.String("calendar_id", id))
	return id, nil
}

func (a *NoopAdapter) DeleteCalendar(_ context.Context, calendarID string) error {
	a.logger.Info("gcalendar noop delete calendar", zap.String("calendar_id", calendarID))
	return nil
}

func (a *NoopAdapter) CreateEvent(_ context.Context, calendarID string, event Event) (string, error) {
	id := deterministicID("event", calendarID, event.Title, event.StartTime.Format(time.RFC3339Nano), event.EndTime.Format(time.RFC3339Nano), event.TimeZone, fmt.Sprintf("%d", event.ReminderMin))
	a.logger.Info("gcalendar noop create event",
		zap.String("calendar_id", calendarID),
		zap.String("event_id", id),
		zap.String("title", event.Title),
	)
	return id, nil
}

func (a *NoopAdapter) UpdateEvent(_ context.Context, calendarID, eventID string, event Event) error {
	a.logger.Info("gcalendar noop update event",
		zap.String("calendar_id", calendarID),
		zap.String("event_id", eventID),
		zap.String("title", event.Title),
	)
	return nil
}

func (a *NoopAdapter) CreateRecurringEvent(_ context.Context, calendarID string, event Event, intervalHours int) (string, error) {
	id := deterministicID("recurring", calendarID, event.Title, event.StartTime.Format(time.RFC3339Nano), event.EndTime.Format(time.RFC3339Nano), event.TimeZone, fmt.Sprintf("%d", intervalHours))
	a.logger.Info("gcalendar noop create recurring event",
		zap.String("calendar_id", calendarID),
		zap.String("event_id", id),
		zap.String("title", event.Title),
		zap.Int("interval_hours", intervalHours),
	)
	return id, nil
}

func (a *NoopAdapter) StopRecurringEvent(_ context.Context, calendarID, eventID string, until time.Time) error {
	a.logger.Info("gcalendar noop stop recurring event",
		zap.String("calendar_id", calendarID),
		zap.String("event_id", eventID),
		zap.Time("until", until),
	)
	return nil
}

func (a *NoopAdapter) DeleteEvent(_ context.Context, calendarID, eventID string) error {
	a.logger.Info("gcalendar noop delete event",
		zap.String("calendar_id", calendarID),
		zap.String("event_id", eventID),
	)
	return nil
}

func deterministicID(parts ...string) string {
	key := ""
	for _, part := range parts {
		key += "|" + part
	}
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(key)).String()
}
