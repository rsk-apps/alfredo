package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
	"github.com/rafaelsoares/alfredo/internal/petcare/service"
)

type mockDoseRepo struct {
	created []domain.Dose
	latest  *domain.Dose
}

func (m *mockDoseRepo) CreateBatch(_ context.Context, doses []domain.Dose) error {
	m.created = append(m.created, doses...)
	return nil
}
func (m *mockDoseRepo) ListByTreatment(_ context.Context, _ string) ([]domain.Dose, error) {
	return nil, nil
}
func (m *mockDoseRepo) DeleteFutureDoses(_ context.Context, _ string, _ time.Time) ([]string, error) {
	return nil, nil
}
func (m *mockDoseRepo) ListOpenEndedActiveTreatments(_ context.Context) ([]domain.Treatment, error) {
	return nil, nil
}
func (m *mockDoseRepo) LatestDoseFor(_ context.Context, _ string) (*domain.Dose, error) {
	return m.latest, nil
}

func TestDoseService_GenerateDoses_Finite(t *testing.T) {
	start := time.Date(2026, 4, 3, 8, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour) // 1 day later
	tr := domain.Treatment{
		ID: "t1", PetID: "p1",
		IntervalHours: 12,
		StartedAt:     start,
		EndedAt:       &end,
	}
	svc := service.NewDoseService(&mockDoseRepo{})
	doses := svc.GenerateDoses(tr, end)
	// Expect doses at: start, start+12h (start+24h == end, excluded since it equals end exactly)
	if len(doses) != 2 {
		t.Errorf("got %d doses, want 2", len(doses))
	}
	if !doses[0].ScheduledFor.Equal(start) {
		t.Errorf("first dose at %v, want %v", doses[0].ScheduledFor, start)
	}
	if !doses[1].ScheduledFor.Equal(start.Add(12 * time.Hour)) {
		t.Errorf("second dose at %v, want %v", doses[1].ScheduledFor, start.Add(12*time.Hour))
	}
}

func TestDoseService_GenerateDoses_UpTo(t *testing.T) {
	start := time.Date(2026, 4, 3, 8, 0, 0, 0, time.UTC)
	upTo := start.Add(48 * time.Hour)
	tr := domain.Treatment{
		ID: "t1", PetID: "p1",
		IntervalHours: 24,
		StartedAt:     start,
		// EndedAt is nil (open-ended)
	}
	svc := service.NewDoseService(&mockDoseRepo{})
	doses := svc.GenerateDoses(tr, upTo)
	// Doses at: start, start+24h (start+48h == upTo, excluded)
	if len(doses) != 2 {
		t.Errorf("got %d doses, want 2", len(doses))
	}
}

func TestDoseService_GenerateDoses_EachHasUniqueID(t *testing.T) {
	start := time.Date(2026, 4, 3, 8, 0, 0, 0, time.UTC)
	end := start.Add(48 * time.Hour)
	tr := domain.Treatment{ID: "t1", PetID: "p1", IntervalHours: 24, StartedAt: start, EndedAt: &end}
	svc := service.NewDoseService(&mockDoseRepo{})
	doses := svc.GenerateDoses(tr, end)
	ids := map[string]bool{}
	for _, d := range doses {
		if ids[d.ID] {
			t.Errorf("duplicate dose ID: %s", d.ID)
		}
		ids[d.ID] = true
	}
}

func TestDoseService_ExtendOpenEnded_StartsAfterLatest(t *testing.T) {
	start := time.Date(2026, 4, 3, 8, 0, 0, 0, time.UTC)
	latest := start.Add(24 * time.Hour)
	repo := &mockDoseRepo{latest: &domain.Dose{ID: "d0", TreatmentID: "t1", ScheduledFor: latest}}
	tr := domain.Treatment{ID: "t1", PetID: "p1", IntervalHours: 24, StartedAt: start}
	svc := service.NewDoseService(repo)
	windowEnd := start.Add(72 * time.Hour)
	doses, err := svc.ExtendOpenEnded(context.Background(), tr, windowEnd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Latest is at start+24h. Next should be start+48h, then start+72h is excluded (== windowEnd).
	if len(doses) != 1 {
		t.Errorf("got %d doses, want 1", len(doses))
	}
	if !doses[0].ScheduledFor.Equal(start.Add(48 * time.Hour)) {
		t.Errorf("dose at %v, want %v", doses[0].ScheduledFor, start.Add(48*time.Hour))
	}
}
