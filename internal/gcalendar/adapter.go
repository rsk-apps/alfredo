package gcalendar

import (
	"context"
	"fmt"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	calendar "google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

type AdapterConfig struct {
	ClientID     string
	ClientSecret string
	RefreshToken string
}

type Adapter struct {
	service *calendar.Service
}

func NewAdapter(ctx context.Context, cfg AdapterConfig) (*Adapter, error) {
	if cfg.ClientID == "" || cfg.ClientSecret == "" || cfg.RefreshToken == "" {
		return nil, fmt.Errorf("gcalendar credentials are required")
	}

	token := &oauth2.Token{RefreshToken: cfg.RefreshToken}
	oauthCfg := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Endpoint:     google.Endpoint,
		Scopes:       []string{calendar.CalendarScope},
	}
	source := oauth2.ReuseTokenSource(nil, oauthCfg.TokenSource(ctx, token))

	svc, err := calendar.NewService(ctx, option.WithTokenSource(source))
	if err != nil {
		return nil, fmt.Errorf("create google calendar service: %w", err)
	}
	return &Adapter{service: svc}, nil
}

func (a *Adapter) CreateCalendar(ctx context.Context, name string) (string, error) {
	cal, err := a.service.Calendars.Insert(&calendar.Calendar{Summary: name}).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("create calendar %q: %w", name, err)
	}
	return cal.Id, nil
}

func (a *Adapter) DeleteCalendar(ctx context.Context, calendarID string) error {
	if calendarID == "" {
		return fmt.Errorf("delete calendar: empty calendar id")
	}
	if err := a.service.Calendars.Delete(calendarID).Context(ctx).Do(); err != nil {
		return fmt.Errorf("delete calendar %q: %w", calendarID, err)
	}
	return nil
}

func (a *Adapter) CreateEvent(ctx context.Context, calendarID string, event Event) (string, error) {
	gEvent, err := toGoogleEvent(event, 0)
	if err != nil {
		return "", err
	}
	inserted, err := a.service.Events.Insert(calendarID, gEvent).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("create event on calendar %q: %w", calendarID, err)
	}
	return inserted.Id, nil
}

func (a *Adapter) UpdateEvent(ctx context.Context, calendarID, eventID string, event Event) error {
	if calendarID == "" || eventID == "" {
		return fmt.Errorf("update event: calendar and event ids are required")
	}
	gEvent, err := toGoogleEvent(event, 0)
	if err != nil {
		return err
	}
	if event.Location == "" {
		gEvent.ForceSendFields = append(gEvent.ForceSendFields, "Location")
	}
	if _, err := a.service.Events.Patch(calendarID, eventID, gEvent).Context(ctx).Do(); err != nil {
		return fmt.Errorf("update event %q on calendar %q: %w", eventID, calendarID, err)
	}
	return nil
}

func (a *Adapter) CreateRecurringEvent(ctx context.Context, calendarID string, event Event, intervalHours int) (string, error) {
	if intervalHours <= 0 {
		return "", fmt.Errorf("create recurring event: interval_hours must be > 0")
	}
	gEvent, err := toGoogleEvent(event, intervalHours)
	if err != nil {
		return "", err
	}
	inserted, err := a.service.Events.Insert(calendarID, gEvent).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("create recurring event on calendar %q: %w", calendarID, err)
	}
	return inserted.Id, nil
}

func (a *Adapter) StopRecurringEvent(ctx context.Context, calendarID, eventID string, until time.Time) error {
	if calendarID == "" || eventID == "" {
		return fmt.Errorf("stop recurring event: calendar and event ids are required")
	}
	ev, err := a.service.Events.Get(calendarID, eventID).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("load recurring event %q: %w", eventID, err)
	}
	if len(ev.Recurrence) == 0 {
		return fmt.Errorf("stop recurring event %q: missing recurrence rule", eventID)
	}
	untilUTC := until.UTC().Format("20060102T150405Z")
	updated := make([]string, 0, len(ev.Recurrence))
	foundRule := false
	for _, line := range ev.Recurrence {
		if !strings.HasPrefix(line, "RRULE:") {
			updated = append(updated, line)
			continue
		}
		foundRule = true
		rule := strings.TrimPrefix(line, "RRULE:")
		parts := strings.Split(rule, ";")
		next := make([]string, 0, len(parts)+1)
		replaced := false
		for _, part := range parts {
			if strings.HasPrefix(part, "UNTIL=") {
				next = append(next, "UNTIL="+untilUTC)
				replaced = true
				continue
			}
			next = append(next, part)
		}
		if !replaced {
			next = append(next, "UNTIL="+untilUTC)
		}
		updated = append(updated, "RRULE:"+strings.Join(next, ";"))
	}
	if !foundRule {
		return fmt.Errorf("stop recurring event %q: recurrence rule not found", eventID)
	}
	_, err = a.service.Events.Patch(calendarID, eventID, &calendar.Event{
		Recurrence: updated,
	}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("update recurring event %q: %w", eventID, err)
	}
	return nil
}

func (a *Adapter) DeleteEvent(ctx context.Context, calendarID, eventID string) error {
	if calendarID == "" || eventID == "" {
		return fmt.Errorf("delete event: calendar and event ids are required")
	}
	if err := a.service.Events.Delete(calendarID, eventID).Context(ctx).Do(); err != nil {
		return fmt.Errorf("delete event %q from calendar %q: %w", eventID, calendarID, err)
	}
	return nil
}

func toGoogleEvent(event Event, intervalHours int) (*calendar.Event, error) {
	if event.TimeZone == "" {
		return nil, fmt.Errorf("create calendar event %q: timezone is required", event.Title)
	}
	end := event.EndTime
	if !end.After(event.StartTime) {
		end = event.StartTime.Add(time.Minute)
	}
	gEvent := &calendar.Event{
		Summary:     event.Title,
		Description: event.Description,
		Start: &calendar.EventDateTime{
			DateTime: event.StartTime.Format(time.RFC3339),
			TimeZone: event.TimeZone,
		},
		End: &calendar.EventDateTime{
			DateTime: end.Format(time.RFC3339),
			TimeZone: event.TimeZone,
		},
		Reminders: &calendar.EventReminders{
			UseDefault:      false,
			ForceSendFields: []string{"UseDefault"},
			Overrides: []*calendar.EventReminder{
				{Method: "popup", Minutes: int64(event.ReminderMin), ForceSendFields: []string{"Minutes"}},
			},
		},
	}
	if event.Location != "" {
		gEvent.Location = event.Location
	}
	if intervalHours > 0 {
		gEvent.Recurrence = []string{fmt.Sprintf("RRULE:FREQ=HOURLY;INTERVAL=%d", intervalHours)}
	}
	return gEvent, nil
}
