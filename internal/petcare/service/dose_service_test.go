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
	listed  []domain.Dose
	future  []domain.Dose
	err     error

	createBatchCalled       bool
	listByTreatmentCalled   bool
	listFutureCalled        bool
	deleteFutureCalled      bool
	listByTreatmentID       string
	listFutureTreatmentID   string
	listFutureAfter         time.Time
	deleteFutureTreatmentID string
	deleteFutureAfter       time.Time
}

func (m *mockDoseRepo) CreateBatch(_ context.Context, doses []domain.Dose) error {
	m.createBatchCalled = true
	m.created = append(m.created, doses...)
	return m.err
}
func (m *mockDoseRepo) ListByTreatment(_ context.Context, treatmentID string) ([]domain.Dose, error) {
	m.listByTreatmentCalled = true
	m.listByTreatmentID = treatmentID
	return m.listed, m.err
}
func (m *mockDoseRepo) ListFutureByTreatment(_ context.Context, treatmentID string, after time.Time) ([]domain.Dose, error) {
	m.listFutureCalled = true
	m.listFutureTreatmentID = treatmentID
	m.listFutureAfter = after
	return m.future, m.err
}
func (m *mockDoseRepo) DeleteFutureByTreatment(_ context.Context, treatmentID string, after time.Time) error {
	m.deleteFutureCalled = true
	m.deleteFutureTreatmentID = treatmentID
	m.deleteFutureAfter = after
	return m.err
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

func TestDoseService_CreateBatch_DelegatesToRepository(t *testing.T) {
	repo := &mockDoseRepo{}
	svc := service.NewDoseService(repo)
	doses := []domain.Dose{{ID: "dose-1"}, {ID: "dose-2"}}

	if err := svc.CreateBatch(context.Background(), doses); err != nil {
		t.Fatalf("CreateBatch error = %v", err)
	}
	if !repo.createBatchCalled {
		t.Fatal("expected CreateBatch to call repository")
	}
	if got := len(repo.created); got != len(doses) {
		t.Fatalf("CreateBatch stored %d doses, want %d", got, len(doses))
	}
}

func TestDoseService_ListByTreatment_DelegatesToRepository(t *testing.T) {
	want := []domain.Dose{{ID: "dose-1", TreatmentID: "t1"}}
	repo := &mockDoseRepo{listed: want}
	svc := service.NewDoseService(repo)

	got, err := svc.ListByTreatment(context.Background(), "t1")
	if err != nil {
		t.Fatalf("ListByTreatment error = %v", err)
	}
	if !repo.listByTreatmentCalled {
		t.Fatal("expected ListByTreatment to call repository")
	}
	if len(got) != len(want) || got[0].ID != want[0].ID {
		t.Fatalf("ListByTreatment = %#v, want %#v", got, want)
	}
}

func TestDoseService_ListFutureByTreatment_DelegatesToRepository(t *testing.T) {
	after := time.Date(2026, 4, 4, 8, 0, 0, 0, time.UTC)
	want := []domain.Dose{{ID: "dose-1", TreatmentID: "t1"}}
	repo := &mockDoseRepo{future: want}
	svc := service.NewDoseService(repo)

	got, err := svc.ListFutureByTreatment(context.Background(), "t1", after)
	if err != nil {
		t.Fatalf("ListFutureByTreatment error = %v", err)
	}
	if !repo.listFutureCalled {
		t.Fatal("expected ListFutureByTreatment to call repository")
	}
	if repo.listFutureTreatmentID != "t1" || !repo.listFutureAfter.Equal(after) {
		t.Fatalf("ListFutureByTreatment arguments = %q %v, want %q %v", repo.listFutureTreatmentID, repo.listFutureAfter, "t1", after)
	}
	if len(got) != len(want) || got[0].ID != want[0].ID {
		t.Fatalf("ListFutureByTreatment = %#v, want %#v", got, want)
	}
}

func TestDoseService_DeleteFutureByTreatment_DelegatesToRepository(t *testing.T) {
	after := time.Date(2026, 4, 4, 8, 0, 0, 0, time.UTC)
	repo := &mockDoseRepo{}
	svc := service.NewDoseService(repo)

	if err := svc.DeleteFutureByTreatment(context.Background(), "t1", after); err != nil {
		t.Fatalf("DeleteFutureByTreatment error = %v", err)
	}
	if !repo.deleteFutureCalled {
		t.Fatal("expected DeleteFutureByTreatment to call repository")
	}
	if repo.deleteFutureTreatmentID != "t1" || !repo.deleteFutureAfter.Equal(after) {
		t.Fatalf("DeleteFutureByTreatment arguments = %q %v, want %q %v", repo.deleteFutureTreatmentID, repo.deleteFutureAfter, "t1", after)
	}
}
