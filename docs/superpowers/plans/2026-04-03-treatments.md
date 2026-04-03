# Treatments Feature Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add first-class treatment tracking to the petcare module — medication courses and ongoing prescriptions with automatic dose generation and webhook-driven calendar integration via n8n.

**Architecture:** Treatments and doses follow the exact same hexagonal layering as vaccines: domain types → port interfaces → SQLite repositories → pure CRUD service → use case (orchestration + webhooks) → HTTP handler. A `DoseExtender` background goroutine tops up open-ended treatments with a rolling 90-day window.

**Tech Stack:** Go, SQLite (`modernc.org/sqlite`), Echo v4, `go-playground/validator`, `google/uuid`, `go.uber.org/zap`; module `github.com/rafaelsoares/alfredo`

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Create | `internal/petcare/domain/treatment.go` | `Treatment`, `Dose` domain types |
| Modify | `internal/petcare/port/ports.go` | Add `TreatmentRepository`, `DoseRepository` interfaces |
| Create | `internal/petcare/adapters/secondary/sqlite/migrations/002_treatments.sql` | `treatments` + `doses` schema |
| Modify | `internal/petcare/adapters/secondary/sqlite/db.go` | Register migration 002 |
| Create | `internal/petcare/adapters/secondary/sqlite/treatment_repository.go` | `TreatmentRepository` SQLite impl |
| Create | `internal/petcare/adapters/secondary/sqlite/dose_repository.go` | `DoseRepository` SQLite impl |
| Create | `internal/petcare/service/treatment_service.go` | Pure CRUD service |
| Create | `internal/petcare/service/treatment_service_test.go` | Service unit tests |
| Create | `internal/petcare/service/dose_service.go` | Dose generation logic |
| Create | `internal/petcare/service/dose_service_test.go` | Dose generation unit tests |
| Modify | `internal/app/ports.go` | Add `TreatmentServicer`, `DoseServicer` interfaces |
| Create | `internal/app/treatment_usecase.go` | Orchestration + webhook emission |
| Create | `internal/app/treatment_usecase_test.go` | Use case unit tests |
| Create | `internal/app/dose_extender.go` | Background job (daily ticker) |
| Create | `internal/app/dose_extender_test.go` | Extender unit test |
| Create | `internal/petcare/adapters/primary/http/treatment_handler.go` | HTTP handler |
| Modify | `cmd/server/main.go` | Wire repos, services, use cases, handler, extender |
| Create | `bruno/treatments/Start Treatment.bru` | Bruno: POST |
| Create | `bruno/treatments/List Treatments.bru` | Bruno: GET list |
| Create | `bruno/treatments/Get Treatment.bru` | Bruno: GET single |
| Create | `bruno/treatments/Stop Treatment.bru` | Bruno: DELETE |
| Modify | `CLAUDE.md` | Update routes table |

---

## Task 1: Domain Types

**Files:**
- Create: `internal/petcare/domain/treatment.go`

- [ ] **Step 1: Create the domain types**

```go
// internal/petcare/domain/treatment.go
package domain

import "time"

type Treatment struct {
	ID            string
	PetID         string
	Name          string
	DosageAmount  float64
	DosageUnit    string    // e.g. "mg", "ml"
	Route         string    // e.g. "oral", "injection", "topical"
	IntervalHours int       // 24=daily, 12=BID, 8=TID
	StartedAt     time.Time
	EndedAt       *time.Time // nil = open-ended
	StoppedAt     *time.Time // set when stopped early via DELETE
	VetName       *string
	Notes         *string
	CreatedAt     time.Time
}

type Dose struct {
	ID           string
	TreatmentID  string
	PetID        string
	ScheduledFor time.Time
}
```

- [ ] **Step 2: Build to verify**

```bash
make build
```
Expected: compiles without errors.

- [ ] **Step 3: Commit**

```bash
git add internal/petcare/domain/treatment.go
git commit -m "feat(petcare): add Treatment and Dose domain types"
```

---

## Task 2: Port Interfaces

**Files:**
- Modify: `internal/petcare/port/ports.go`

- [ ] **Step 1: Add repository interfaces**

Append to the bottom of `internal/petcare/port/ports.go`:

```go
// TreatmentRepository persists treatment records.
type TreatmentRepository interface {
	Create(ctx context.Context, t domain.Treatment) (*domain.Treatment, error)
	GetByID(ctx context.Context, petID, treatmentID string) (*domain.Treatment, error)
	List(ctx context.Context, petID string) ([]domain.Treatment, error)
	Stop(ctx context.Context, treatmentID string, stoppedAt time.Time) error
}

// DoseRepository persists dose records and supports the rolling-window extension job.
type DoseRepository interface {
	CreateBatch(ctx context.Context, doses []domain.Dose) error
	ListByTreatment(ctx context.Context, treatmentID string) ([]domain.Dose, error)
	// DeleteFutureDoses deletes doses scheduled after `after` and returns their IDs.
	DeleteFutureDoses(ctx context.Context, treatmentID string, after time.Time) ([]string, error)
	// ListOpenEndedActiveTreatments returns treatments with ended_at IS NULL AND stopped_at IS NULL.
	ListOpenEndedActiveTreatments(ctx context.Context) ([]domain.Treatment, error)
	// LatestDoseFor returns the latest scheduled dose for a treatment, or nil if none exist.
	LatestDoseFor(ctx context.Context, treatmentID string) (*domain.Dose, error)
}
```

Add `"time"` to the imports in `ports.go` (it is not there yet).

- [ ] **Step 2: Build to verify**

```bash
make build
```
Expected: compiles without errors.

- [ ] **Step 3: Commit**

```bash
git add internal/petcare/port/ports.go
git commit -m "feat(petcare): add TreatmentRepository and DoseRepository port interfaces"
```

---

## Task 3: SQLite Migration + Registration

**Files:**
- Create: `internal/petcare/adapters/secondary/sqlite/migrations/002_treatments.sql`
- Modify: `internal/petcare/adapters/secondary/sqlite/db.go`

- [ ] **Step 1: Create the migration file**

```sql
-- internal/petcare/adapters/secondary/sqlite/migrations/002_treatments.sql
CREATE TABLE IF NOT EXISTS treatments (
    id             TEXT PRIMARY KEY,
    pet_id         TEXT NOT NULL REFERENCES pets(id) ON DELETE CASCADE,
    name           TEXT NOT NULL,
    dosage_amount  REAL NOT NULL,
    dosage_unit    TEXT NOT NULL,
    route          TEXT NOT NULL,
    interval_hours INTEGER NOT NULL,
    started_at     TEXT NOT NULL,
    ended_at       TEXT,
    stopped_at     TEXT,
    vet_name       TEXT,
    notes          TEXT,
    created_at     TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS doses (
    id            TEXT PRIMARY KEY,
    treatment_id  TEXT NOT NULL REFERENCES treatments(id) ON DELETE CASCADE,
    pet_id        TEXT NOT NULL REFERENCES pets(id) ON DELETE CASCADE,
    scheduled_for TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_doses_treatment_id ON doses(treatment_id);

CREATE INDEX IF NOT EXISTS idx_doses_pet_id_scheduled ON doses(pet_id, scheduled_for)
```

- [ ] **Step 2: Register the migration in `db.go`**

Add the embed directive and register the migration. In `internal/petcare/adapters/secondary/sqlite/db.go`:

```go
//go:embed migrations/001_initial.sql
var migration001 string

//go:embed migrations/002_treatments.sql
var migration002 string
```

In the `migrations` slice inside `migrate()`:

```go
migrations := []struct {
    version string
    sql     string
}{
    {"001_initial", migration001},
    {"002_treatments", migration002},
}
```

- [ ] **Step 3: Build to verify**

```bash
make build
```
Expected: compiles without errors.

- [ ] **Step 4: Commit**

```bash
git add internal/petcare/adapters/secondary/sqlite/migrations/002_treatments.sql
git add internal/petcare/adapters/secondary/sqlite/db.go
git commit -m "feat(petcare): add treatments and doses SQLite migration"
```

---

## Task 4: TreatmentRepository SQLite Implementation

**Files:**
- Create: `internal/petcare/adapters/secondary/sqlite/treatment_repository.go`

- [ ] **Step 1: Create the repository**

```go
// internal/petcare/adapters/secondary/sqlite/treatment_repository.go
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
)

type TreatmentRepository struct{ db *sql.DB }

func NewTreatmentRepository(db *sql.DB) *TreatmentRepository {
	return &TreatmentRepository{db: db}
}

func (r *TreatmentRepository) Create(ctx context.Context, t domain.Treatment) (*domain.Treatment, error) {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO treatments (id, pet_id, name, dosage_amount, dosage_unit, route, interval_hours, started_at, ended_at, stopped_at, vet_name, notes, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.PetID, t.Name, t.DosageAmount, t.DosageUnit, t.Route, t.IntervalHours,
		t.StartedAt.Format(time.RFC3339), formatOptionalRFC3339(t.EndedAt), formatOptionalRFC3339(t.StoppedAt),
		t.VetName, t.Notes, t.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *TreatmentRepository) GetByID(ctx context.Context, petID, treatmentID string) (*domain.Treatment, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, pet_id, name, dosage_amount, dosage_unit, route, interval_hours, started_at, ended_at, stopped_at, vet_name, notes, created_at
		 FROM treatments WHERE id = ? AND pet_id = ?`, treatmentID, petID)
	t, err := scanTreatment(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return t, err
}

func (r *TreatmentRepository) List(ctx context.Context, petID string) ([]domain.Treatment, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, pet_id, name, dosage_amount, dosage_unit, route, interval_hours, started_at, ended_at, stopped_at, vet_name, notes, created_at
		 FROM treatments WHERE pet_id = ? ORDER BY created_at DESC`, petID)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	var ts []domain.Treatment
	for rows.Next() {
		t, err := scanTreatment(rows)
		if err != nil {
			return nil, err
		}
		ts = append(ts, *t)
	}
	return ts, rows.Err()
}

func (r *TreatmentRepository) Stop(ctx context.Context, treatmentID string, stoppedAt time.Time) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE treatments SET stopped_at = ? WHERE id = ? AND stopped_at IS NULL`,
		stoppedAt.Format(time.RFC3339), treatmentID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func scanTreatment(s scanner) (*domain.Treatment, error) {
	var t domain.Treatment
	var startedAt, createdAt string
	var endedAt, stoppedAt sql.NullString
	err := s.Scan(
		&t.ID, &t.PetID, &t.Name, &t.DosageAmount, &t.DosageUnit, &t.Route, &t.IntervalHours,
		&startedAt, &endedAt, &stoppedAt, &t.VetName, &t.Notes, &createdAt,
	)
	if err != nil {
		return nil, err
	}
	t.StartedAt, err = time.Parse(time.RFC3339, startedAt)
	if err != nil {
		return nil, fmt.Errorf("parse started_at %q: %w", startedAt, err)
	}
	t.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at %q: %w", createdAt, err)
	}
	if endedAt.Valid && endedAt.String != "" {
		ts, err := time.Parse(time.RFC3339, endedAt.String)
		if err != nil {
			return nil, fmt.Errorf("parse ended_at %q: %w", endedAt.String, err)
		}
		t.EndedAt = &ts
	}
	if stoppedAt.Valid && stoppedAt.String != "" {
		ts, err := time.Parse(time.RFC3339, stoppedAt.String)
		if err != nil {
			return nil, fmt.Errorf("parse stopped_at %q: %w", stoppedAt.String, err)
		}
		t.StoppedAt = &ts
	}
	return &t, nil
}

// formatOptionalRFC3339 formats a *time.Time as RFC3339, returning nil if t is nil.
func formatOptionalRFC3339(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.RFC3339)
	return &s
}
```

- [ ] **Step 2: Build to verify**

```bash
make build
```
Expected: compiles without errors.

- [ ] **Step 3: Commit**

```bash
git add internal/petcare/adapters/secondary/sqlite/treatment_repository.go
git commit -m "feat(petcare): add TreatmentRepository SQLite implementation"
```

---

## Task 5: DoseRepository SQLite Implementation

**Files:**
- Create: `internal/petcare/adapters/secondary/sqlite/dose_repository.go`

- [ ] **Step 1: Create the repository**

```go
// internal/petcare/adapters/secondary/sqlite/dose_repository.go
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
)

type DoseRepository struct{ db *sql.DB }

func NewDoseRepository(db *sql.DB) *DoseRepository {
	return &DoseRepository{db: db}
}

func (r *DoseRepository) CreateBatch(ctx context.Context, doses []domain.Dose) error {
	if len(doses) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO doses (id, treatment_id, pet_id, scheduled_for) VALUES (?, ?, ?, ?)`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close() //nolint:errcheck
	for _, d := range doses {
		if _, err := stmt.ExecContext(ctx, d.ID, d.TreatmentID, d.PetID, d.ScheduledFor.Format(time.RFC3339)); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (r *DoseRepository) ListByTreatment(ctx context.Context, treatmentID string) ([]domain.Dose, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, treatment_id, pet_id, scheduled_for FROM doses WHERE treatment_id = ? ORDER BY scheduled_for ASC`,
		treatmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	var doses []domain.Dose
	for rows.Next() {
		d, err := scanDose(rows)
		if err != nil {
			return nil, err
		}
		doses = append(doses, *d)
	}
	return doses, rows.Err()
}

func (r *DoseRepository) DeleteFutureDoses(ctx context.Context, treatmentID string, after time.Time) ([]string, error) {
	afterStr := after.Format(time.RFC3339)
	rows, err := r.db.QueryContext(ctx,
		`SELECT id FROM doses WHERE treatment_id = ? AND scheduled_for > ?`,
		treatmentID, afterStr)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	if _, err := r.db.ExecContext(ctx,
		`DELETE FROM doses WHERE treatment_id = ? AND scheduled_for > ?`,
		treatmentID, afterStr); err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *DoseRepository) ListOpenEndedActiveTreatments(ctx context.Context) ([]domain.Treatment, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, pet_id, name, dosage_amount, dosage_unit, route, interval_hours, started_at, ended_at, stopped_at, vet_name, notes, created_at
		 FROM treatments WHERE ended_at IS NULL AND stopped_at IS NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	var ts []domain.Treatment
	for rows.Next() {
		t, err := scanTreatment(rows)
		if err != nil {
			return nil, err
		}
		ts = append(ts, *t)
	}
	return ts, rows.Err()
}

func (r *DoseRepository) LatestDoseFor(ctx context.Context, treatmentID string) (*domain.Dose, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, treatment_id, pet_id, scheduled_for FROM doses WHERE treatment_id = ? ORDER BY scheduled_for DESC LIMIT 1`,
		treatmentID)
	d, err := scanDose(row)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, err
	}
	return d, nil
}

func scanDose(s scanner) (*domain.Dose, error) {
	var d domain.Dose
	var scheduledFor string
	if err := s.Scan(&d.ID, &d.TreatmentID, &d.PetID, &scheduledFor); err != nil {
		return nil, err
	}
	var err error
	d.ScheduledFor, err = time.Parse(time.RFC3339, scheduledFor)
	if err != nil {
		return nil, fmt.Errorf("parse scheduled_for %q: %w", scheduledFor, err)
	}
	return &d, nil
}
```

- [ ] **Step 2: Build to verify**

```bash
make build
```
Expected: compiles without errors.

- [ ] **Step 3: Commit**

```bash
git add internal/petcare/adapters/secondary/sqlite/dose_repository.go
git commit -m "feat(petcare): add DoseRepository SQLite implementation"
```

---

## Task 6: TreatmentService (Pure CRUD)

**Files:**
- Create: `internal/petcare/service/treatment_service.go`
- Create: `internal/petcare/service/treatment_service_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/petcare/service/treatment_service_test.go
package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
	"github.com/rafaelsoares/alfredo/internal/petcare/service"
)

type mockTreatmentRepo struct {
	treatment *domain.Treatment
	err       error
}

func (m *mockTreatmentRepo) Create(_ context.Context, t domain.Treatment) (*domain.Treatment, error) {
	return &t, m.err
}
func (m *mockTreatmentRepo) GetByID(_ context.Context, _, _ string) (*domain.Treatment, error) {
	return m.treatment, m.err
}
func (m *mockTreatmentRepo) List(_ context.Context, _ string) ([]domain.Treatment, error) {
	if m.treatment != nil {
		return []domain.Treatment{*m.treatment}, m.err
	}
	return nil, m.err
}
func (m *mockTreatmentRepo) Stop(_ context.Context, _ string, _ time.Time) error { return m.err }

func TestTreatmentService_Create_AssignsID(t *testing.T) {
	svc := service.NewTreatmentService(&mockTreatmentRepo{})
	tr, err := svc.Create(context.Background(), service.CreateTreatmentInput{
		PetID: "p1", Name: "Amoxicillin", DosageAmount: 250, DosageUnit: "mg",
		Route: "oral", IntervalHours: 12, StartedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tr.ID == "" {
		t.Error("expected ID to be assigned")
	}
}

func TestTreatmentService_Create_ValidationErrors(t *testing.T) {
	svc := service.NewTreatmentService(&mockTreatmentRepo{})
	cases := []struct {
		name  string
		input service.CreateTreatmentInput
	}{
		{"missing name", service.CreateTreatmentInput{PetID: "p1", DosageAmount: 1, DosageUnit: "mg", Route: "oral", IntervalHours: 24, StartedAt: time.Now()}},
		{"zero dosage", service.CreateTreatmentInput{PetID: "p1", Name: "Drug", DosageUnit: "mg", Route: "oral", IntervalHours: 24, StartedAt: time.Now()}},
		{"missing unit", service.CreateTreatmentInput{PetID: "p1", Name: "Drug", DosageAmount: 1, Route: "oral", IntervalHours: 24, StartedAt: time.Now()}},
		{"missing route", service.CreateTreatmentInput{PetID: "p1", Name: "Drug", DosageAmount: 1, DosageUnit: "mg", IntervalHours: 24, StartedAt: time.Now()}},
		{"zero interval", service.CreateTreatmentInput{PetID: "p1", Name: "Drug", DosageAmount: 1, DosageUnit: "mg", Route: "oral", StartedAt: time.Now()}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Create(context.Background(), tc.input)
			if !errors.Is(err, domain.ErrValidation) {
				t.Errorf("got %v, want ErrValidation", err)
			}
		})
	}
}

func TestTreatmentService_Stop_NotFound(t *testing.T) {
	svc := service.NewTreatmentService(&mockTreatmentRepo{err: domain.ErrNotFound})
	err := svc.Stop(context.Background(), "p1", "t1")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/petcare/service/... -run TestTreatmentService -v
```
Expected: `FAIL` — `service.NewTreatmentService` and `service.CreateTreatmentInput` undefined.

- [ ] **Step 3: Implement TreatmentService**

```go
// internal/petcare/service/treatment_service.go
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
	"github.com/rafaelsoares/alfredo/internal/petcare/port"
)

type CreateTreatmentInput struct {
	PetID         string
	Name          string
	DosageAmount  float64
	DosageUnit    string
	Route         string
	IntervalHours int
	StartedAt     time.Time
	EndedAt       *time.Time
	VetName       *string
	Notes         *string
}

type TreatmentService struct {
	repo port.TreatmentRepository
}

func NewTreatmentService(repo port.TreatmentRepository) *TreatmentService {
	return &TreatmentService{repo: repo}
}

func (s *TreatmentService) Create(ctx context.Context, in CreateTreatmentInput) (*domain.Treatment, error) {
	if in.Name == "" {
		return nil, fmt.Errorf("%w: name is required", domain.ErrValidation)
	}
	if in.DosageAmount <= 0 {
		return nil, fmt.Errorf("%w: dosage_amount must be greater than zero", domain.ErrValidation)
	}
	if in.DosageUnit == "" {
		return nil, fmt.Errorf("%w: dosage_unit is required", domain.ErrValidation)
	}
	if in.Route == "" {
		return nil, fmt.Errorf("%w: route is required", domain.ErrValidation)
	}
	if in.IntervalHours <= 0 {
		return nil, fmt.Errorf("%w: interval_hours must be at least 1", domain.ErrValidation)
	}
	now := time.Now().UTC()
	return s.repo.Create(ctx, domain.Treatment{
		ID:            uuid.New().String(),
		PetID:         in.PetID,
		Name:          in.Name,
		DosageAmount:  in.DosageAmount,
		DosageUnit:    in.DosageUnit,
		Route:         in.Route,
		IntervalHours: in.IntervalHours,
		StartedAt:     in.StartedAt.UTC(),
		EndedAt:       in.EndedAt,
		VetName:       in.VetName,
		Notes:         in.Notes,
		CreatedAt:     now,
	})
}

func (s *TreatmentService) GetByID(ctx context.Context, petID, treatmentID string) (*domain.Treatment, error) {
	return s.repo.GetByID(ctx, petID, treatmentID)
}

func (s *TreatmentService) List(ctx context.Context, petID string) ([]domain.Treatment, error) {
	return s.repo.List(ctx, petID)
}

func (s *TreatmentService) Stop(ctx context.Context, petID, treatmentID string) error {
	if _, err := s.repo.GetByID(ctx, petID, treatmentID); err != nil {
		return err
	}
	return s.repo.Stop(ctx, treatmentID, time.Now().UTC())
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/petcare/service/... -run TestTreatmentService -v
```
Expected: all `PASS`.

- [ ] **Step 5: Commit**

```bash
git add internal/petcare/service/treatment_service.go internal/petcare/service/treatment_service_test.go
git commit -m "feat(petcare): add TreatmentService with validation"
```

---

## Task 7: DoseService (Dose Generation Logic)

**Files:**
- Create: `internal/petcare/service/dose_service.go`
- Create: `internal/petcare/service/dose_service_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/petcare/service/dose_service_test.go
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
	// Expect doses at: start, start+12h (start+24h == end, excluded since it equals EndedAt exactly)
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
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/petcare/service/... -run TestDoseService -v
```
Expected: `FAIL` — `service.NewDoseService` undefined.

- [ ] **Step 3: Implement DoseService**

```go
// internal/petcare/service/dose_service.go
package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
	"github.com/rafaelsoares/alfredo/internal/petcare/port"
)

type DoseService struct {
	repo port.DoseRepository
}

func NewDoseService(repo port.DoseRepository) *DoseService {
	return &DoseService{repo: repo}
}

// GenerateDoses produces doses from treatment.StartedAt stepping by IntervalHours up to (but not including) upTo.
// For finite treatments, call with upTo = *treatment.EndedAt.
// For open-ended treatments, call with upTo = now + 90 days.
func (s *DoseService) GenerateDoses(t domain.Treatment, upTo time.Time) []domain.Dose {
	var doses []domain.Dose
	for cur := t.StartedAt; cur.Before(upTo); cur = cur.Add(time.Duration(t.IntervalHours) * time.Hour) {
		doses = append(doses, domain.Dose{
			ID:           uuid.New().String(),
			TreatmentID:  t.ID,
			PetID:        t.PetID,
			ScheduledFor: cur,
		})
	}
	return doses
}

// CreateBatch persists a batch of doses.
func (s *DoseService) CreateBatch(ctx context.Context, doses []domain.Dose) error {
	return s.repo.CreateBatch(ctx, doses)
}

// ListByTreatment returns all doses for a treatment ordered by scheduled_for ASC.
func (s *DoseService) ListByTreatment(ctx context.Context, treatmentID string) ([]domain.Dose, error) {
	return s.repo.ListByTreatment(ctx, treatmentID)
}

// DeleteFutureDoses deletes doses scheduled after `after` and returns their IDs.
func (s *DoseService) DeleteFutureDoses(ctx context.Context, treatmentID string, after time.Time) ([]string, error) {
	return s.repo.DeleteFutureDoses(ctx, treatmentID, after)
}

// ListOpenEndedActiveTreatments returns open-ended treatments that have not been stopped.
func (s *DoseService) ListOpenEndedActiveTreatments(ctx context.Context) ([]domain.Treatment, error) {
	return s.repo.ListOpenEndedActiveTreatments(ctx)
}

// ExtendOpenEnded generates doses from (latestDose + IntervalHours) up to windowEnd and persists them.
// Returns the newly created doses (nil if nothing to add).
func (s *DoseService) ExtendOpenEnded(ctx context.Context, t domain.Treatment, windowEnd time.Time) ([]domain.Dose, error) {
	latest, err := s.repo.LatestDoseFor(ctx, t.ID)
	if err != nil {
		return nil, err
	}
	var from time.Time
	if latest == nil {
		from = t.StartedAt
	} else {
		from = latest.ScheduledFor.Add(time.Duration(t.IntervalHours) * time.Hour)
	}
	if !from.Before(windowEnd) {
		return nil, nil
	}
	// Build a synthetic treatment starting from `from` to generate only the missing doses.
	stub := t
	stub.StartedAt = from
	doses := s.GenerateDoses(stub, windowEnd)
	if len(doses) == 0 {
		return nil, nil
	}
	return doses, s.repo.CreateBatch(ctx, doses)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/petcare/service/... -run TestDoseService -v
```
Expected: all `PASS`.

- [ ] **Step 5: Commit**

```bash
git add internal/petcare/service/dose_service.go internal/petcare/service/dose_service_test.go
git commit -m "feat(petcare): add DoseService with automatic dose generation"
```

---

## Task 8: App Ports + TreatmentUseCase

**Files:**
- Modify: `internal/app/ports.go`
- Create: `internal/app/treatment_usecase.go`
- Create: `internal/app/treatment_usecase_test.go`

- [ ] **Step 1: Add app-level interfaces to `ports.go`**

Append to `internal/app/ports.go`:

```go
// TreatmentServicer is the narrow interface consumed by TreatmentUseCase.
type TreatmentServicer interface {
	Create(ctx context.Context, in service.CreateTreatmentInput) (*domain.Treatment, error)
	GetByID(ctx context.Context, petID, treatmentID string) (*domain.Treatment, error)
	List(ctx context.Context, petID string) ([]domain.Treatment, error)
	Stop(ctx context.Context, petID, treatmentID string) error
}

// DoseServicer is the narrow interface consumed by TreatmentUseCase and DoseExtender.
// Satisfied by *service.DoseService.
type DoseServicer interface {
	GenerateDoses(t domain.Treatment, upTo time.Time) []domain.Dose
	CreateBatch(ctx context.Context, doses []domain.Dose) error
	ListByTreatment(ctx context.Context, treatmentID string) ([]domain.Dose, error)
	DeleteFutureDoses(ctx context.Context, treatmentID string, after time.Time) ([]string, error)
	ListOpenEndedActiveTreatments(ctx context.Context) ([]domain.Treatment, error)
	ExtendOpenEnded(ctx context.Context, t domain.Treatment, windowEnd time.Time) ([]domain.Dose, error)
}
```

Add `"time"` to the imports in `ports.go` if not already present.

- [ ] **Step 2: Write the failing tests**

```go
// internal/app/treatment_usecase_test.go
package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/rafaelsoares/alfredo/internal/app"
	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
	"github.com/rafaelsoares/alfredo/internal/petcare/service"
	"go.uber.org/zap"
)

// --- stubs ---

type stubTreatmentService struct {
	treatment *domain.Treatment
}

func (s *stubTreatmentService) Create(_ context.Context, _ service.CreateTreatmentInput) (*domain.Treatment, error) {
	return s.treatment, nil
}
func (s *stubTreatmentService) GetByID(_ context.Context, _, _ string) (*domain.Treatment, error) {
	return s.treatment, nil
}
func (s *stubTreatmentService) List(_ context.Context, _ string) ([]domain.Treatment, error) {
	return []domain.Treatment{*s.treatment}, nil
}
func (s *stubTreatmentService) Stop(_ context.Context, _, _ string) error { return nil }

type stubDoseService struct {
	doses []domain.Dose
}

func (s *stubDoseService) GenerateDoses(_ domain.Treatment, _ time.Time) []domain.Dose {
	return s.doses
}
func (s *stubDoseService) CreateBatch(_ context.Context, _ []domain.Dose) error { return nil }
func (s *stubDoseService) DeleteFutureDoses(_ context.Context, _ string, _ time.Time) ([]string, error) {
	return []string{"dose-1", "dose-2"}, nil
}
func (s *stubDoseService) ExtendOpenEnded(_ context.Context, _ domain.Treatment, _ time.Time) ([]domain.Dose, error) {
	return s.doses, nil
}

// --- tests ---

func TestTreatmentUseCase_Create_EmitsDosesScheduled(t *testing.T) {
	spy := &spyEmitter{}
	tr := &domain.Treatment{ID: "t1", PetID: "p1", Name: "Amoxicillin", StartedAt: time.Now()}
	doses := []domain.Dose{{ID: "d1", TreatmentID: "t1", ScheduledFor: time.Now()}}
	uc := app.NewTreatmentUseCase(&stubTreatmentService{treatment: tr}, &stubDoseService{doses: doses}, &fakePetGetter{}, spy, zap.NewNop())

	_, _, err := uc.Create(context.Background(), service.CreateTreatmentInput{PetID: "p1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(spy.events) != 1 || spy.events[0] != "treatment.doses_scheduled" {
		t.Errorf("events = %v, want [treatment.doses_scheduled]", spy.events)
	}
}

func TestTreatmentUseCase_Stop_EmitsTreatmentStopped(t *testing.T) {
	spy := &spyEmitter{}
	tr := &domain.Treatment{ID: "t1", PetID: "p1", Name: "Amoxicillin", StartedAt: time.Now()}
	uc := app.NewTreatmentUseCase(&stubTreatmentService{treatment: tr}, &stubDoseService{}, &fakePetGetter{}, spy, zap.NewNop())

	if err := uc.Stop(context.Background(), "p1", "t1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(spy.events) != 1 || spy.events[0] != "treatment.stopped" {
		t.Errorf("events = %v, want [treatment.stopped]", spy.events)
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
go test ./internal/app/... -run TestTreatmentUseCase -v
```
Expected: `FAIL` — `app.NewTreatmentUseCase` undefined.

- [ ] **Step 4: Implement TreatmentUseCase**

```go
// internal/app/treatment_usecase.go
package app

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
	"github.com/rafaelsoares/alfredo/internal/petcare/service"
	"github.com/rafaelsoares/alfredo/internal/webhook"
)

const doseWindowDays = 90

// TreatmentUseCase orchestrates treatment creation, dose generation, and webhook emission.
type TreatmentUseCase struct {
	treatments TreatmentServicer
	doses      DoseServicer
	pets       PetNameGetter
	emitter    webhook.EventEmitter
	logger     *zap.Logger
}

func NewTreatmentUseCase(
	treatments TreatmentServicer,
	doses DoseServicer,
	pets PetNameGetter,
	emitter webhook.EventEmitter,
	logger *zap.Logger,
) *TreatmentUseCase {
	return &TreatmentUseCase{treatments: treatments, doses: doses, pets: pets, emitter: emitter, logger: logger}
}

func (uc *TreatmentUseCase) petName(ctx context.Context, petID string) string {
	pet, err := uc.pets.GetByID(ctx, petID)
	if err != nil || pet == nil {
		return petID
	}
	return pet.Name
}

// Create starts a treatment, generates doses, and emits treatment.doses_scheduled.
// Returns the treatment and the generated doses.
func (uc *TreatmentUseCase) Create(ctx context.Context, in service.CreateTreatmentInput) (*domain.Treatment, []domain.Dose, error) {
	tr, err := uc.treatments.Create(ctx, in)
	if err != nil {
		return nil, nil, err
	}
	var upTo time.Time
	if tr.EndedAt != nil {
		upTo = *tr.EndedAt
	} else {
		upTo = time.Now().UTC().AddDate(0, 0, doseWindowDays)
	}
	doses := uc.doses.GenerateDoses(*tr, upTo)
	if err := uc.doses.CreateBatch(ctx, doses); err != nil {
		return nil, nil, err
	}
	uc.emitter.Emit(ctx, "treatment.doses_scheduled", treatmentDosesScheduledPayload{
		PetID:         tr.PetID,
		PetName:       uc.petName(ctx, tr.PetID),
		TreatmentID:   tr.ID,
		TreatmentName: tr.Name,
		DosageAmount:  tr.DosageAmount,
		DosageUnit:    tr.DosageUnit,
		Route:         tr.Route,
		IntervalHours: tr.IntervalHours,
		Doses:         toDosePayloads(doses),
	})
	return tr, doses, nil
}

// GetByID returns a treatment and its doses.
func (uc *TreatmentUseCase) GetByID(ctx context.Context, petID, treatmentID string) (*domain.Treatment, []domain.Dose, error) {
	tr, err := uc.treatments.GetByID(ctx, petID, treatmentID)
	if err != nil {
		return nil, nil, err
	}
	doses, err := uc.doses.ListByTreatment(ctx, treatmentID)
	if err != nil {
		return nil, nil, err
	}
	return tr, doses, nil
}

// List returns all treatments for a pet with their doses.
func (uc *TreatmentUseCase) List(ctx context.Context, petID string) ([]domain.Treatment, map[string][]domain.Dose, error) {
	ts, err := uc.treatments.List(ctx, petID)
	if err != nil {
		return nil, nil, err
	}
	doseMap := make(map[string][]domain.Dose, len(ts))
	for _, t := range ts {
		doses, err := uc.doses.ListByTreatment(ctx, t.ID)
		if err != nil {
			return nil, nil, err
		}
		doseMap[t.ID] = doses
	}
	return ts, doseMap, nil
}

// Stop marks a treatment as stopped, deletes future doses, and emits treatment.stopped.
func (uc *TreatmentUseCase) Stop(ctx context.Context, petID, treatmentID string) error {
	tr, err := uc.treatments.GetByID(ctx, petID, treatmentID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	deletedIDs, err := uc.doses.DeleteFutureDoses(ctx, treatmentID, now)
	if err != nil {
		return err
	}
	if err := uc.treatments.Stop(ctx, petID, treatmentID); err != nil {
		return err
	}
	uc.emitter.Emit(ctx, "treatment.stopped", treatmentStoppedPayload{
		PetID:          tr.PetID,
		PetName:        uc.petName(ctx, tr.PetID),
		TreatmentID:    tr.ID,
		TreatmentName:  tr.Name,
		StoppedAt:      now,
		DeletedDoseIDs: deletedIDs,
	})
	return nil
}

// --- Payload types ---

type dosePayload struct {
	DoseID       string    `json:"dose_id"`
	ScheduledFor time.Time `json:"scheduled_for"`
}

type treatmentDosesScheduledPayload struct {
	PetID         string        `json:"pet_id"`
	PetName       string        `json:"pet_name"`
	TreatmentID   string        `json:"treatment_id"`
	TreatmentName string        `json:"treatment_name"`
	DosageAmount  float64       `json:"dosage_amount"`
	DosageUnit    string        `json:"dosage_unit"`
	Route         string        `json:"route"`
	IntervalHours int           `json:"interval_hours"`
	Doses         []dosePayload `json:"doses"`
}

type treatmentStoppedPayload struct {
	PetID          string    `json:"pet_id"`
	PetName        string    `json:"pet_name"`
	TreatmentID    string    `json:"treatment_id"`
	TreatmentName  string    `json:"treatment_name"`
	StoppedAt      time.Time `json:"stopped_at"`
	DeletedDoseIDs []string  `json:"deleted_dose_ids"`
}

func toDosePayloads(doses []domain.Dose) []dosePayload {
	p := make([]dosePayload, len(doses))
	for i, d := range doses {
		p[i] = dosePayload{DoseID: d.ID, ScheduledFor: d.ScheduledFor}
	}
	return p
}
```

Add `ListByTreatment` to `DoseServicer` in `ports.go`:

```go
type DoseServicer interface {
	GenerateDoses(t domain.Treatment, upTo time.Time) []domain.Dose
	CreateBatch(ctx context.Context, doses []domain.Dose) error
	DeleteFutureDoses(ctx context.Context, treatmentID string, after time.Time) ([]string, error)
	ExtendOpenEnded(ctx context.Context, t domain.Treatment, windowEnd time.Time) ([]domain.Dose, error)
	ListByTreatment(ctx context.Context, treatmentID string) ([]domain.Dose, error)
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/app/... -run TestTreatmentUseCase -v
```
Expected: all `PASS`.

- [ ] **Step 6: Run all tests**

```bash
make test
```
Expected: all tests pass.

- [ ] **Step 7: Commit**

```bash
git add internal/app/ports.go internal/app/treatment_usecase.go internal/app/treatment_usecase_test.go
git commit -m "feat(app): add TreatmentUseCase with dose generation and webhook events"
```

---

## Task 9: DoseExtender Background Job

**Files:**
- Create: `internal/app/dose_extender.go`
- Create: `internal/app/dose_extender_test.go`

`DoseExtender` accepts `DoseServicer` (from `ports.go`), which is already satisfied by `*service.DoseService`. No separate interface needed.

- [ ] **Step 1: Write the failing test**

```go
// internal/app/dose_extender_test.go
package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/rafaelsoares/alfredo/internal/app"
	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
	"go.uber.org/zap"
)

// stubDoseServiceForExtender implements DoseServicer for the extender tests.
type stubDoseServiceForExtender struct {
	treatments []domain.Treatment
	extended   []string // treatment IDs passed to ExtendOpenEnded
	newDoses   []domain.Dose
}

func (s *stubDoseServiceForExtender) GenerateDoses(_ domain.Treatment, _ time.Time) []domain.Dose {
	return nil
}
func (s *stubDoseServiceForExtender) CreateBatch(_ context.Context, _ []domain.Dose) error {
	return nil
}
func (s *stubDoseServiceForExtender) ListByTreatment(_ context.Context, _ string) ([]domain.Dose, error) {
	return nil, nil
}
func (s *stubDoseServiceForExtender) DeleteFutureDoses(_ context.Context, _ string, _ time.Time) ([]string, error) {
	return nil, nil
}
func (s *stubDoseServiceForExtender) ListOpenEndedActiveTreatments(_ context.Context) ([]domain.Treatment, error) {
	return s.treatments, nil
}
func (s *stubDoseServiceForExtender) ExtendOpenEnded(_ context.Context, t domain.Treatment, _ time.Time) ([]domain.Dose, error) {
	s.extended = append(s.extended, t.ID)
	return s.newDoses, nil
}

type stubExtenderEmitter struct {
	events []string
}

func (s *stubExtenderEmitter) Emit(_ context.Context, event string, _ any) {
	s.events = append(s.events, event)
}

func TestDoseExtender_ExtendOnce_EmitsForNewDoses(t *testing.T) {
	tr := domain.Treatment{ID: "t1", PetID: "p1", Name: "Drug", IntervalHours: 24, StartedAt: time.Now()}
	doseSvc := &stubDoseServiceForExtender{
		treatments: []domain.Treatment{tr},
		newDoses:   []domain.Dose{{ID: "d1", TreatmentID: "t1", ScheduledFor: time.Now().Add(24 * time.Hour)}},
	}
	spy := &stubExtenderEmitter{}
	ext := app.NewDoseExtender(doseSvc, &fakePetGetter{}, spy, zap.NewNop())

	ext.ExtendOnce(context.Background())

	if len(doseSvc.extended) != 1 || doseSvc.extended[0] != "t1" {
		t.Errorf("extended treatments = %v, want [t1]", doseSvc.extended)
	}
	if len(spy.events) != 1 || spy.events[0] != "treatment.doses_scheduled" {
		t.Errorf("events = %v, want [treatment.doses_scheduled]", spy.events)
	}
}

func TestDoseExtender_ExtendOnce_NoEmitWhenNoDoses(t *testing.T) {
	tr := domain.Treatment{ID: "t1", PetID: "p1", Name: "Drug", IntervalHours: 24, StartedAt: time.Now()}
	doseSvc := &stubDoseServiceForExtender{
		treatments: []domain.Treatment{tr},
		newDoses:   nil, // ExtendOpenEnded returns no new doses
	}
	spy := &stubExtenderEmitter{}
	ext := app.NewDoseExtender(doseSvc, &fakePetGetter{}, spy, zap.NewNop())

	ext.ExtendOnce(context.Background())

	if len(spy.events) != 0 {
		t.Errorf("expected no events when no new doses, got %v", spy.events)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/app/... -run TestDoseExtender -v
```
Expected: `FAIL` — `app.NewDoseExtender` undefined.

- [ ] **Step 3: Implement DoseExtender**

```go
// internal/app/dose_extender.go
package app

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/rafaelsoares/alfredo/internal/webhook"
)

// DoseExtender is a background job that tops up open-ended treatments with a rolling 90-day dose window.
type DoseExtender struct {
	doses   DoseServicer
	pets    PetNameGetter
	emitter webhook.EventEmitter
	logger  *zap.Logger
}

func NewDoseExtender(doses DoseServicer, pets PetNameGetter, emitter webhook.EventEmitter, logger *zap.Logger) *DoseExtender {
	return &DoseExtender{doses: doses, pets: pets, emitter: emitter, logger: logger}
}

// ExtendOnce runs one pass of the extension job. Call this from a goroutine on a ticker.
func (e *DoseExtender) ExtendOnce(ctx context.Context) {
	treatments, err := e.doses.ListOpenEndedActiveTreatments(ctx)
	if err != nil {
		e.logger.Error("dose extender: list open-ended treatments failed", zap.Error(err))
		return
	}
	windowEnd := time.Now().UTC().AddDate(0, 0, doseWindowDays)
	for _, t := range treatments {
		doses, err := e.doses.ExtendOpenEnded(ctx, t, windowEnd)
		if err != nil {
			e.logger.Error("dose extender: extend failed", zap.String("treatment_id", t.ID), zap.Error(err))
			continue
		}
		if len(doses) == 0 {
			continue
		}
		pet, _ := e.pets.GetByID(ctx, t.PetID)
		petName := t.PetID
		if pet != nil {
			petName = pet.Name
		}
		e.emitter.Emit(ctx, "treatment.doses_scheduled", treatmentDosesScheduledPayload{
			PetID:         t.PetID,
			PetName:       petName,
			TreatmentID:   t.ID,
			TreatmentName: t.Name,
			DosageAmount:  t.DosageAmount,
			DosageUnit:    t.DosageUnit,
			Route:         t.Route,
			IntervalHours: t.IntervalHours,
			Doses:         toDosePayloads(doses),
		})
	}
}

// Run starts the extension job on a daily ticker until ctx is cancelled.
func (e *DoseExtender) Run(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			e.ExtendOnce(ctx)
		case <-ctx.Done():
			return
		}
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/app/... -run TestDoseExtender -v
```
Expected: all `PASS`.

- [ ] **Step 5: Run all tests**

```bash
make test
```
Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/app/dose_extender.go internal/app/dose_extender_test.go
git commit -m "feat(app): add DoseExtender background job with rolling 90-day window"
```

---

## Task 10: HTTP Handler

**Files:**
- Create: `internal/petcare/adapters/primary/http/treatment_handler.go`

- [ ] **Step 1: Create the handler**

```go
// internal/petcare/adapters/primary/http/treatment_handler.go
package http

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/rafaelsoares/alfredo/internal/logger"
	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
	"github.com/rafaelsoares/alfredo/internal/petcare/service"
)

// TreatmentServicer is the consumer-defined interface consumed by TreatmentHandler.
type TreatmentServicer interface {
	Create(ctx context.Context, in service.CreateTreatmentInput) (*domain.Treatment, []domain.Dose, error)
	GetByID(ctx context.Context, petID, treatmentID string) (*domain.Treatment, []domain.Dose, error)
	List(ctx context.Context, petID string) ([]domain.Treatment, map[string][]domain.Dose, error)
	Stop(ctx context.Context, petID, treatmentID string) error
}

type TreatmentHandler struct {
	svc TreatmentServicer
}

func NewTreatmentHandler(svc TreatmentServicer) *TreatmentHandler {
	return &TreatmentHandler{svc: svc}
}

func (h *TreatmentHandler) Register(g *echo.Group) {
	g.POST("/pets/:id/treatments", h.StartTreatment)
	g.GET("/pets/:id/treatments", h.ListTreatments)
	g.GET("/pets/:id/treatments/:tid", h.GetTreatment)
	g.DELETE("/pets/:id/treatments/:tid", h.StopTreatment)
}

func (h *TreatmentHandler) StartTreatment(c echo.Context) error {
	petID, ok := parseUUID(c, "id")
	if !ok {
		return nil
	}
	var req struct {
		Name          string     `json:"name" validate:"required,min=1,max=100"`
		DosageAmount  float64    `json:"dosage_amount" validate:"required,gt=0"`
		DosageUnit    string     `json:"dosage_unit" validate:"required,min=1,max=20"`
		Route         string     `json:"route" validate:"required,min=1,max=50"`
		IntervalHours int        `json:"interval_hours" validate:"required,min=1"`
		StartedAt     string     `json:"started_at" validate:"required"`
		EndedAt       *string    `json:"ended_at"`
		VetName       *string    `json:"vet_name" validate:"omitempty,max=100"`
		Notes         *string    `json:"notes" validate:"omitempty,max=500"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, newErrorResponse("invalid_request_body", "Request body is invalid JSON", nil))
	}
	if !validateRequest(c, &req) {
		return nil
	}
	startedAt, err := time.Parse(time.RFC3339, req.StartedAt)
	if err != nil {
		return c.JSON(http.StatusBadRequest, newErrorResponse(
			"validation_failed", "Request validation failed",
			[]fieldError{{Field: "started_at", Issue: "must be RFC3339 format"}},
		))
	}
	in := service.CreateTreatmentInput{
		PetID:         petID,
		Name:          req.Name,
		DosageAmount:  req.DosageAmount,
		DosageUnit:    req.DosageUnit,
		Route:         req.Route,
		IntervalHours: req.IntervalHours,
		StartedAt:     startedAt,
		VetName:       req.VetName,
		Notes:         req.Notes,
	}
	if req.EndedAt != nil {
		endedAt, err := time.Parse(time.RFC3339, *req.EndedAt)
		if err != nil {
			return c.JSON(http.StatusBadRequest, newErrorResponse(
				"validation_failed", "Request validation failed",
				[]fieldError{{Field: "ended_at", Issue: "must be RFC3339 format"}},
			))
		}
		in.EndedAt = &endedAt
	}
	tr, doses, err := h.svc.Create(c.Request().Context(), in)
	if err != nil {
		return mapError(c, err)
	}
	logger.FromEcho(c).Info("treatment started",
		zap.String("pet_id", petID),
		zap.String("treatment_id", tr.ID),
		zap.Int("doses_generated", len(doses)),
	)
	return c.JSON(http.StatusCreated, toTreatmentResponse(*tr, doses))
}

func (h *TreatmentHandler) ListTreatments(c echo.Context) error {
	petID, ok := parseUUID(c, "id")
	if !ok {
		return nil
	}
	ts, doseMap, err := h.svc.List(c.Request().Context(), petID)
	if err != nil {
		return mapError(c, err)
	}
	resp := make([]treatmentResponse, 0, len(ts))
	for _, t := range ts {
		resp = append(resp, toTreatmentResponse(t, doseMap[t.ID]))
	}
	logger.FromEcho(c).Info("treatments listed", zap.String("pet_id", petID), zap.Int("count", len(ts)))
	return c.JSON(http.StatusOK, resp)
}

func (h *TreatmentHandler) GetTreatment(c echo.Context) error {
	petID, ok := parseUUID(c, "id")
	if !ok {
		return nil
	}
	treatmentID, ok := parseUUID(c, "tid")
	if !ok {
		return nil
	}
	tr, doses, err := h.svc.GetByID(c.Request().Context(), petID, treatmentID)
	if err != nil {
		return mapError(c, err)
	}
	return c.JSON(http.StatusOK, toTreatmentResponse(*tr, doses))
}

func (h *TreatmentHandler) StopTreatment(c echo.Context) error {
	petID, ok := parseUUID(c, "id")
	if !ok {
		return nil
	}
	treatmentID, ok := parseUUID(c, "tid")
	if !ok {
		return nil
	}
	if err := h.svc.Stop(c.Request().Context(), petID, treatmentID); err != nil {
		return mapError(c, err)
	}
	logger.FromEcho(c).Info("treatment stopped",
		zap.String("pet_id", petID),
		zap.String("treatment_id", treatmentID),
	)
	return c.NoContent(http.StatusNoContent)
}

// --- response types ---

type doseResponse struct {
	ID           string `json:"id"`
	ScheduledFor string `json:"scheduled_for"`
}

type treatmentResponse struct {
	ID            string         `json:"id"`
	PetID         string         `json:"pet_id"`
	Name          string         `json:"name"`
	DosageAmount  float64        `json:"dosage_amount"`
	DosageUnit    string         `json:"dosage_unit"`
	Route         string         `json:"route"`
	IntervalHours int            `json:"interval_hours"`
	StartedAt     string         `json:"started_at"`
	EndedAt       *string        `json:"ended_at,omitempty"`
	StoppedAt     *string        `json:"stopped_at,omitempty"`
	VetName       *string        `json:"vet_name,omitempty"`
	Notes         *string        `json:"notes,omitempty"`
	CreatedAt     string         `json:"created_at"`
	Doses         []doseResponse `json:"doses"`
}

func toTreatmentResponse(t domain.Treatment, doses []domain.Dose) treatmentResponse {
	r := treatmentResponse{
		ID:            t.ID,
		PetID:         t.PetID,
		Name:          t.Name,
		DosageAmount:  t.DosageAmount,
		DosageUnit:    t.DosageUnit,
		Route:         t.Route,
		IntervalHours: t.IntervalHours,
		StartedAt:     t.StartedAt.Format(time.RFC3339),
		VetName:       t.VetName,
		Notes:         t.Notes,
		CreatedAt:     t.CreatedAt.Format(time.RFC3339),
		Doses:         make([]doseResponse, 0, len(doses)),
	}
	if t.EndedAt != nil {
		s := t.EndedAt.Format(time.RFC3339)
		r.EndedAt = &s
	}
	if t.StoppedAt != nil {
		s := t.StoppedAt.Format(time.RFC3339)
		r.StoppedAt = &s
	}
	for _, d := range doses {
		r.Doses = append(r.Doses, doseResponse{
			ID:           d.ID,
			ScheduledFor: d.ScheduledFor.Format(time.RFC3339),
		})
	}
	return r
}
```

- [ ] **Step 2: Build to verify**

```bash
make build
```
Expected: compiles without errors.

- [ ] **Step 3: Run all tests**

```bash
make test
```
Expected: all pass.

- [ ] **Step 4: Commit**

```bash
git add internal/petcare/adapters/primary/http/treatment_handler.go
git commit -m "feat(petcare): add TreatmentHandler with all four routes"
```

---

## Task 11: Wire Everything in `main.go`

**Files:**
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Update `main.go`**

Add the following blocks to `main.go` in order, following the existing numbered comment pattern:

After step 6 (repositories), add:
```go
treatmentRepo := sqlite.NewTreatmentRepository(db)
doseRepo := sqlite.NewDoseRepository(db)
```

After step 7 (services), add:
```go
treatmentService := petsvc.NewTreatmentService(treatmentRepo)
doseService := petsvc.NewDoseService(doseRepo)
```

After step 8 (use cases), add:
```go
treatmentUC := app.NewTreatmentUseCase(treatmentService, doseService, petService, emitter, zapLogger)
```

After step 10 (HTTP handlers), add:
```go
treatmentHandler := pethttp.NewTreatmentHandler(treatmentUC)
```

After `vaccineHandler.Register(protected)`, add:
```go
treatmentHandler.Register(protected)
```

Add the `DoseExtender` goroutine with context cancellation. Add this after the server goroutine is started (before the signal wait):

```go
// Start dose extender background job
extenderCtx, cancelExtender := context.WithCancel(context.Background())
defer cancelExtender()
doseExtender := app.NewDoseExtender(doseService, petService, emitter, zapLogger)
go doseExtender.Run(extenderCtx)
```

Add required imports:
- `petsvc` already covers `petsvc.NewTreatmentService` and `petsvc.NewDoseService`
- `sqlite.NewTreatmentRepository`, `sqlite.NewDoseRepository` — covered by existing `sqlite` import

- [ ] **Step 2: Build to verify**

```bash
make build
```
Expected: compiles without errors.

- [ ] **Step 3: Run all tests**

```bash
make test
```
Expected: all pass.

- [ ] **Step 4: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat: wire treatments into server — repos, services, use case, handler, extender"
```

---

## Task 12: Bruno Collection + CLAUDE.md

**Files:**
- Create: `bruno/treatments/Start Treatment.bru`
- Create: `bruno/treatments/List Treatments.bru`
- Create: `bruno/treatments/Get Treatment.bru`
- Create: `bruno/treatments/Stop Treatment.bru`
- Modify: `CLAUDE.md`

- [ ] **Step 1: Create Bruno requests**

```
// bruno/treatments/Start Treatment.bru
meta {
  name: Start Treatment
  type: http
  seq: 1
}

post {
  url: {{baseUrl}}/api/v1/pets/:id/treatments
  body: json
  auth: none
}

headers {
  Authorization: Bearer {{apiKey}}
}

params:path {
  id: {{petId}}
}

body:json {
  {
    "name": "Amoxicillin",
    "dosage_amount": 250,
    "dosage_unit": "mg",
    "route": "oral",
    "interval_hours": 12,
    "started_at": "2026-04-03T08:00:00Z",
    "ended_at": "2026-04-13T08:00:00Z",
    "vet_name": "Dr. Costa",
    "notes": "Give with food"
  }
}
```

```
// bruno/treatments/List Treatments.bru
meta {
  name: List Treatments
  type: http
  seq: 2
}

get {
  url: {{baseUrl}}/api/v1/pets/:id/treatments
  body: none
  auth: none
}

headers {
  Authorization: Bearer {{apiKey}}
}

params:path {
  id: {{petId}}
}
```

```
// bruno/treatments/Get Treatment.bru
meta {
  name: Get Treatment
  type: http
  seq: 3
}

get {
  url: {{baseUrl}}/api/v1/pets/:id/treatments/:tid
  body: none
  auth: none
}

headers {
  Authorization: Bearer {{apiKey}}
}

params:path {
  id: {{petId}}
  tid: {{treatmentId}}
}
```

```
// bruno/treatments/Stop Treatment.bru
meta {
  name: Stop Treatment
  type: http
  seq: 4
}

delete {
  url: {{baseUrl}}/api/v1/pets/:id/treatments/:tid
  body: none
  auth: none
}

headers {
  Authorization: Bearer {{apiKey}}
}

params:path {
  id: {{petId}}
  tid: {{treatmentId}}
}
```

- [ ] **Step 2: Add `treatmentId` to the Bruno Local environment**

In `bruno/environments/Local.bru`, add:
```
treatmentId: <a-sample-uuid>
```

- [ ] **Step 3: Update CLAUDE.md routes table**

Add these rows to the Routes table in `CLAUDE.md`:

```
| `POST /api/v1/pets/:id/treatments` | TreatmentHandler |
| `GET /api/v1/pets/:id/treatments` | TreatmentHandler |
| `GET /api/v1/pets/:id/treatments/:tid` | TreatmentHandler |
| `DELETE /api/v1/pets/:id/treatments/:tid` | TreatmentHandler |
```

Also update the webhook Event Catalog table with the two new events:

```
| `treatment.doses_scheduled` | `POST /api/v1/pets/:id/treatments` and daily extender job | `pet_id`, `pet_name`, `treatment_id`, `treatment_name`, `dosage_amount`, `dosage_unit`, `route`, `interval_hours`, `doses` (array of `{dose_id, scheduled_for}`) |
| `treatment.stopped` | `DELETE /api/v1/pets/:id/treatments/:tid` | `pet_id`, `pet_name`, `treatment_id`, `treatment_name`, `stopped_at`, `deleted_dose_ids` |
```

- [ ] **Step 4: Run full test suite one final time**

```bash
make test
```
Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add bruno/treatments/ bruno/environments/Local.bru CLAUDE.md
git commit -m "docs: add Bruno collection for treatments and update CLAUDE.md routes"
```

---

## Verification Checklist

After all tasks are complete, run this end-to-end smoke test:

```bash
make run
# In another terminal:
TOKEN="<your-api-key>"
PET_ID=$(curl -s -X POST http://localhost:8080/api/v1/pets \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Luna","species":"dog"}' | jq -r .id)

# 1. Start a finite treatment (10 days BID = 20 doses expected)
TREAT=$(curl -s -X POST "http://localhost:8080/api/v1/pets/$PET_ID/treatments" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Amoxicillin","dosage_amount":250,"dosage_unit":"mg","route":"oral","interval_hours":12,"started_at":"2026-04-03T08:00:00Z","ended_at":"2026-04-13T08:00:00Z"}')
echo $TREAT | jq '.doses | length'   # expect 20

# 2. Start an open-ended treatment (24h interval, ~90 doses expected)
OPEN=$(curl -s -X POST "http://localhost:8080/api/v1/pets/$PET_ID/treatments" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Flea Prevention","dosage_amount":1,"dosage_unit":"tablet","route":"oral","interval_hours":720,"started_at":"2026-04-03T08:00:00Z"}')
echo $OPEN | jq '.doses | length'   # expect ~3 (monthly for 90 days)

TREAT_ID=$(echo $TREAT | jq -r .id)

# 3. Get treatment
curl -s "http://localhost:8080/api/v1/pets/$PET_ID/treatments/$TREAT_ID" \
  -H "Authorization: Bearer $TOKEN" | jq '.doses | length'

# 4. Stop treatment — future doses deleted
curl -s -X DELETE "http://localhost:8080/api/v1/pets/$PET_ID/treatments/$TREAT_ID" \
  -H "Authorization: Bearer $TOKEN" -o /dev/null -w "%{http_code}"  # expect 204

# 5. Confirm stopped_at is set and future doses gone
curl -s "http://localhost:8080/api/v1/pets/$PET_ID/treatments/$TREAT_ID" \
  -H "Authorization: Bearer $TOKEN" | jq '{stopped_at: .stopped_at, doses: (.doses | length)}'

make stop
```
