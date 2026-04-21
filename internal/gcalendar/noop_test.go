package gcalendar

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestNoopAdapterReturnsDeterministicIDs(t *testing.T) {
	adapter := NewNoopAdapter(zap.NewNop())
	ctx := context.Background()
	event := Event{
		Title:       "Rabies",
		StartTime:   time.Date(2026, 4, 12, 9, 0, 0, 0, time.UTC),
		EndTime:     time.Date(2026, 4, 12, 9, 0, 0, 0, time.UTC),
		ReminderMins: []int{10},
		TimeZone:    "America/Sao_Paulo",
	}

	id1, err := adapter.CreateEvent(ctx, "cal-1", event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	id2, err := adapter.CreateEvent(ctx, "cal-1", event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id1 != id2 {
		t.Fatalf("expected deterministic ids, got %q and %q", id1, id2)
	}
}
