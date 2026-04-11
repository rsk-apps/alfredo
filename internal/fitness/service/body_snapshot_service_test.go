package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
	"github.com/rafaelsoares/alfredo/internal/fitness/service"
)

// ptr helpers.
func fp(v float64) *float64 { return &v }
func sp(v string) *string   { return &v }

type mockBodySnapshotRepo struct {
	snapshot     *domain.BodySnapshot
	list         []domain.BodySnapshot
	latestBefore []domain.BodySnapshot
	err          error
}

func (m *mockBodySnapshotRepo) Create(_ context.Context, s domain.BodySnapshot) (*domain.BodySnapshot, error) {
	return &s, m.err
}
func (m *mockBodySnapshotRepo) GetByID(_ context.Context, _ string) (*domain.BodySnapshot, error) {
	return m.snapshot, m.err
}
func (m *mockBodySnapshotRepo) List(_ context.Context, _, _ *time.Time) ([]domain.BodySnapshot, error) {
	if m.list != nil {
		return m.list, m.err
	}
	if m.snapshot != nil {
		return []domain.BodySnapshot{*m.snapshot}, m.err
	}
	return nil, m.err
}
func (m *mockBodySnapshotRepo) LatestBefore(_ context.Context, _ time.Time, _ int) ([]domain.BodySnapshot, error) {
	return m.latestBefore, m.err
}
func (m *mockBodySnapshotRepo) Delete(_ context.Context, _ string) error { return m.err }

// --- Create tests ---

func TestBodySnapshotService_Create_AssignsID(t *testing.T) {
	svc := service.NewBodySnapshotService(&mockBodySnapshotRepo{})
	w := 75.0
	s, err := svc.Create(context.Background(), service.CreateBodySnapshotInput{
		Date: time.Now(), WeightKg: &w,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.ID == "" {
		t.Error("expected ID to be assigned")
	}
}

func TestBodySnapshotService_Create_RequiresDate(t *testing.T) {
	svc := service.NewBodySnapshotService(&mockBodySnapshotRepo{})
	_, err := svc.Create(context.Background(), service.CreateBodySnapshotInput{})
	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("got %v, want ErrValidation", err)
	}
}

func TestBodySnapshotService_Create_PropagatesDuplicateError(t *testing.T) {
	svc := service.NewBodySnapshotService(&mockBodySnapshotRepo{err: domain.ErrAlreadyExists})
	d := time.Now()
	_, err := svc.Create(context.Background(), service.CreateBodySnapshotInput{Date: d})
	if !errors.Is(err, domain.ErrAlreadyExists) {
		t.Errorf("got %v, want ErrAlreadyExists", err)
	}
}

// --- merge / forward-fill tests via GetByID ---

func TestBodySnapshotService_GetByID_ForwardFillsFromPrevious(t *testing.T) {
	target := domain.BodySnapshot{
		ID:   "target",
		Date: time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC),
		// only chest_cm recorded today
		ChestCm: fp(95.0),
	}
	previous := domain.BodySnapshot{
		ID:       "prev",
		Date:     time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		WeightKg: fp(81.5),
		WaistCm:  fp(88.0),
	}
	repo := &mockBodySnapshotRepo{
		snapshot:     &target,
		latestBefore: []domain.BodySnapshot{previous},
	}
	svc := service.NewBodySnapshotService(repo)

	got, err := svc.GetByID(context.Background(), "target")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ChestCm == nil || *got.ChestCm != 95.0 {
		t.Errorf("ChestCm: got %v, want 95.0", got.ChestCm)
	}
	if got.WeightKg == nil || *got.WeightKg != 81.5 {
		t.Errorf("WeightKg: got %v, want 81.5 (from previous)", got.WeightKg)
	}
	if got.WaistCm == nil || *got.WaistCm != 88.0 {
		t.Errorf("WaistCm: got %v, want 88.0 (from previous)", got.WaistCm)
	}
}

func TestBodySnapshotService_GetByID_OverrideWinsOverBase(t *testing.T) {
	target := domain.BodySnapshot{
		ID:       "target",
		Date:     time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC),
		WeightKg: fp(80.0), // updated weight
	}
	previous := domain.BodySnapshot{
		ID:       "prev",
		Date:     time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		WeightKg: fp(81.5), // older weight — should not win
	}
	repo := &mockBodySnapshotRepo{
		snapshot:     &target,
		latestBefore: []domain.BodySnapshot{previous},
	}
	svc := service.NewBodySnapshotService(repo)

	got, err := svc.GetByID(context.Background(), "target")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.WeightKg == nil || *got.WeightKg != 80.0 {
		t.Errorf("WeightKg: got %v, want 80.0 (target wins)", got.WeightKg)
	}
}

func TestBodySnapshotService_GetByID_NilFieldsStayNilWhenNoPrevious(t *testing.T) {
	target := domain.BodySnapshot{
		ID:       "target",
		Date:     time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC),
		WeightKg: fp(80.0),
	}
	repo := &mockBodySnapshotRepo{
		snapshot:     &target,
		latestBefore: nil, // no history
	}
	svc := service.NewBodySnapshotService(repo)

	got, err := svc.GetByID(context.Background(), "target")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.WaistCm != nil {
		t.Errorf("WaistCm: got %v, want nil", got.WaistCm)
	}
	if got.ChestSkinfoldMm != nil {
		t.Errorf("ChestSkinfoldMm: got %v, want nil", got.ChestSkinfoldMm)
	}
}

// --- List forward-fill tests ---

func TestBodySnapshotService_List_ForwardFillsAcrossSnapshots(t *testing.T) {
	day1 := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	day5 := time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC)
	day10 := time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)

	snapshots := []domain.BodySnapshot{
		{ID: "s1", Date: day1, WeightKg: fp(81.5)},
		{ID: "s2", Date: day5, ChestCm: fp(95.0)},         // no weight
		{ID: "s3", Date: day10, WaistCm: fp(88.0)},         // no weight, no chest
	}
	repo := &mockBodySnapshotRepo{
		list:         snapshots,
		latestBefore: nil, // nothing before the range
	}
	svc := service.NewBodySnapshotService(repo)

	from := day1
	got, err := svc.List(context.Background(), &from, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 snapshots, got %d", len(got))
	}

	// s1: only weight
	if got[0].WeightKg == nil || *got[0].WeightKg != 81.5 {
		t.Errorf("s1 WeightKg: got %v, want 81.5", got[0].WeightKg)
	}

	// s2: weight carried from s1, chest from itself
	if got[1].WeightKg == nil || *got[1].WeightKg != 81.5 {
		t.Errorf("s2 WeightKg: got %v, want 81.5 (carried)", got[1].WeightKg)
	}
	if got[1].ChestCm == nil || *got[1].ChestCm != 95.0 {
		t.Errorf("s2 ChestCm: got %v, want 95.0", got[1].ChestCm)
	}

	// s3: weight + chest carried, waist from itself
	if got[2].WeightKg == nil || *got[2].WeightKg != 81.5 {
		t.Errorf("s3 WeightKg: got %v, want 81.5 (carried)", got[2].WeightKg)
	}
	if got[2].ChestCm == nil || *got[2].ChestCm != 95.0 {
		t.Errorf("s3 ChestCm: got %v, want 95.0 (carried)", got[2].ChestCm)
	}
	if got[2].WaistCm == nil || *got[2].WaistCm != 88.0 {
		t.Errorf("s3 WaistCm: got %v, want 88.0", got[2].WaistCm)
	}
}

func TestBodySnapshotService_List_SeedsFromBeforeRange(t *testing.T) {
	day5 := time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC)

	snapshots := []domain.BodySnapshot{
		{ID: "s2", Date: day5, ChestCm: fp(95.0)}, // no weight
	}
	seed := domain.BodySnapshot{
		ID:       "s0",
		Date:     time.Date(2026, 3, 28, 0, 0, 0, 0, time.UTC),
		WeightKg: fp(81.5), // outside the requested range
	}
	repo := &mockBodySnapshotRepo{
		list:         snapshots,
		latestBefore: []domain.BodySnapshot{seed},
	}
	svc := service.NewBodySnapshotService(repo)

	from := day5
	got, err := svc.List(context.Background(), &from, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(got))
	}
	if got[0].WeightKg == nil || *got[0].WeightKg != 81.5 {
		t.Errorf("WeightKg: got %v, want 81.5 (seeded from before range)", got[0].WeightKg)
	}
}

func TestBodySnapshotService_List_PollockSkinfoldFieldsCarryForward(t *testing.T) {
	day1 := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	day5 := time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC)

	snapshots := []domain.BodySnapshot{
		{
			ID:   "s1",
			Date: day1,
			ChestSkinfoldMm:       fp(12.0),
			MidaxillarySkinfoldMm: fp(15.0),
			TricepsSkinfoldMm:     fp(10.0),
			SubscapularSkinfoldMm: fp(14.0),
			AbdominalSkinfoldMm:   fp(20.0),
			SuprailiacSkinfoldMm:  fp(18.0),
			ThighSkinfoldMm:       fp(16.0),
		},
		{ID: "s2", Date: day5, WeightKg: fp(81.5)}, // only weight, no skinfolds
	}
	repo := &mockBodySnapshotRepo{list: snapshots}
	svc := service.NewBodySnapshotService(repo)

	from := day1
	got, err := svc.List(context.Background(), &from, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s2 := got[1]
	checks := map[string]*float64{
		"ChestSkinfoldMm":       s2.ChestSkinfoldMm,
		"MidaxillarySkinfoldMm": s2.MidaxillarySkinfoldMm,
		"TricepsSkinfoldMm":     s2.TricepsSkinfoldMm,
		"SubscapularSkinfoldMm": s2.SubscapularSkinfoldMm,
		"AbdominalSkinfoldMm":   s2.AbdominalSkinfoldMm,
		"SuprailiacSkinfoldMm":  s2.SuprailiacSkinfoldMm,
		"ThighSkinfoldMm":       s2.ThighSkinfoldMm,
	}
	for name, v := range checks {
		if v == nil {
			t.Errorf("s2.%s: got nil, want non-nil (carried forward)", name)
		}
	}
}

// Ensure the _ = sp(...) helper compiles without unused-import warnings.
var _ = sp
