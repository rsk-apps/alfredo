# Treatments Feature Design

**Date:** 2026-04-03  
**Status:** Approved  
**Author:** Rafael Soares + Claude

---

## Context

Alfredo already tracks pet vaccinations with recurrence logic. The next step is first-class **treatment tracking**: medication courses and ongoing prescriptions with automatic dose generation and webhook-driven calendar integration via n8n.

A treatment represents a medication schedule for a pet (e.g. "Amoxicillin 250mg orally every 12h for 10 days"). Doses are pre-generated and stored so n8n can create calendar events for each one. When a treatment is stopped early, n8n receives the exact dose IDs to remove from the calendar.

---

## Scope

**In scope:**
- Treatment CRUD (create, list, get, stop/delete)
- Automatic dose generation (finite and open-ended)
- Background job to extend open-ended treatments (rolling 90-day window)
- Webhook events: `treatment.doses_scheduled`, `treatment.stopped`

**Out of scope:**
- Dose administration tracking (mark dose as taken)
- Treatment editing/updating
- Notifications or reminders within Alfredo

---

## Domain Model

### `Treatment` (`internal/petcare/domain/treatment.go`)

```go
type Treatment struct {
    ID            string
    PetID         string
    Name          string
    DosageAmount  float64
    DosageUnit    string     // "mg", "ml", etc.
    Route         string     // "oral", "injection", "topical"
    IntervalHours int        // 24=daily, 12=BID, 8=TID
    StartedAt     time.Time
    EndedAt       *time.Time // nil = open-ended
    StoppedAt     *time.Time // set when stopped early via DELETE
    VetName       *string
    Notes         *string
    CreatedAt     time.Time
}
```

### `Dose` (`internal/petcare/domain/treatment.go`)

```go
type Dose struct {
    ID           string
    TreatmentID  string
    PetID        string
    ScheduledFor time.Time
}
```

`StoppedAt` is set on early stop; future doses are deleted but the treatment record is preserved for history. `EndedAt` is the natural end date for finite courses.

---

## Data Layer

### Migration `002_treatments.sql`

```sql
CREATE TABLE treatments (
    id             TEXT PRIMARY KEY,
    pet_id         TEXT NOT NULL REFERENCES pets(id) ON DELETE CASCADE,
    name           TEXT NOT NULL,
    dosage_amount  REAL NOT NULL,
    dosage_unit    TEXT NOT NULL,
    route          TEXT NOT NULL,
    interval_hours INTEGER NOT NULL,
    started_at     TEXT NOT NULL,  -- RFC3339
    ended_at       TEXT,           -- RFC3339, nil = open-ended
    stopped_at     TEXT,           -- RFC3339, set on early stop
    vet_name       TEXT,
    notes          TEXT,
    created_at     TEXT NOT NULL   -- RFC3339
);

CREATE TABLE doses (
    id            TEXT PRIMARY KEY,
    treatment_id  TEXT NOT NULL REFERENCES treatments(id) ON DELETE CASCADE,
    pet_id        TEXT NOT NULL REFERENCES pets(id) ON DELETE CASCADE,
    scheduled_for TEXT NOT NULL    -- RFC3339
);

CREATE INDEX idx_doses_treatment_id ON doses(treatment_id);
CREATE INDEX idx_doses_pet_id_scheduled ON doses(pet_id, scheduled_for);
```

### Repository Interfaces (added to `internal/petcare/port/ports.go`)

```go
type TreatmentRepository interface {
    Create(ctx context.Context, t Treatment) (*Treatment, error)
    GetByID(ctx context.Context, petID, treatmentID string) (*Treatment, error)
    List(ctx context.Context, petID string) ([]Treatment, error)
    Stop(ctx context.Context, treatmentID string, stoppedAt time.Time) error
}

type DoseRepository interface {
    CreateBatch(ctx context.Context, doses []Dose) error
    ListByTreatment(ctx context.Context, treatmentID string) ([]Dose, error)
    DeleteFutureDoses(ctx context.Context, treatmentID string, after time.Time) ([]string, error) // returns deleted IDs
    ListOpenEndedActiveTreatments(ctx context.Context) ([]Treatment, error)
    LatestDoseFor(ctx context.Context, treatmentID string) (*Dose, error)
}
```

`DeleteFutureDoses` returns the deleted dose IDs so the use case can include them in the webhook payload.

---

## Service Layer

### `TreatmentService` (`internal/petcare/service/treatment_service.go`)

Pure CRUD — no side effects, no dose generation.

```
Create(ctx, CreateTreatmentInput) → (*Treatment, error)
GetByID(ctx, petID, treatmentID) → (*Treatment, error)
List(ctx, petID) → ([]Treatment, error)
Stop(ctx, petID, treatmentID) → error
```

Validates: name required, dosage_amount > 0, dosage_unit required, route required, interval_hours >= 1, started_at required.

### `DoseService` (`internal/petcare/service/dose_service.go`)

Dose generation logic — no HTTP, no webhooks.

```
GenerateDoses(treatment Treatment, upTo time.Time) → []Dose
CreateBatch(ctx, []Dose) → error
DeleteFutureDoses(ctx, treatmentID string, after time.Time) → ([]string, error)
ExtendOpenEnded(ctx, treatment Treatment, windowEnd time.Time) → ([]Dose, error)
```

**Dose generation algorithm:**
- Iterate from `StartedAt` stepping by `IntervalHours` until `min(EndedAt, upTo)`
- For open-ended: `upTo = now + 90 days`
- `ExtendOpenEnded` fetches the latest existing dose, then generates from `latestDose.ScheduledFor + IntervalHours` to `windowEnd`, inserts via `CreateBatch`, returns new doses

---

## Use Case Layer

### `TreatmentUseCase` (`internal/app/treatment_usecase.go`)

Orchestrates service calls and webhook emission.

```
Create(ctx, input) → (*Treatment, []Dose, error)
  1. treatmentSvc.Create → *Treatment
  2. doseSvc.GenerateDoses(treatment, upTo)
     - finite: upTo = EndedAt
     - open-ended: upTo = now + 90 days
  3. doseSvc.CreateBatch(doses)
  4. emit "treatment.doses_scheduled" (all doses)

List(ctx, petID) → ([]TreatmentWithDoses, error)
  - treatmentSvc.List + doseSvc.ListByTreatment per treatment

Get(ctx, petID, treatmentID) → (*TreatmentWithDoses, error)
  - treatmentSvc.GetByID + doseSvc.ListByTreatment

Stop(ctx, petID, treatmentID) → error
  1. treatmentSvc.GetByID (fetch name for payload)
  2. doseSvc.DeleteFutureDoses(treatmentID, now) → deletedIDs
  3. treatmentSvc.Stop(treatmentID, now)
  4. emit "treatment.stopped" (with deletedIDs)
```

### `DoseExtender` (`internal/app/dose_extender.go`)

Background job (daily ticker) to extend open-ended treatments.

```
Run(ctx)
  - doseRepo.ListOpenEndedActiveTreatments()
  - for each: doseSvc.ExtendOpenEnded(treatment, now+90days)
  - if new doses generated: emit "treatment.doses_scheduled" (new doses only)
```

Uses the same `treatment.doses_scheduled` event shape — n8n reuses the same sub-workflow for initial creation and extensions.

---

## Webhook Events

All events follow the existing envelope: `{ event, occurred_at, domain: "petcare", payload }`.

### `treatment.doses_scheduled`

Emitted on treatment creation and by the background job for new doses.

```json
{
  "event": "treatment.doses_scheduled",
  "occurred_at": "2026-04-03T10:00:00Z",
  "domain": "petcare",
  "payload": {
    "pet_id": "uuid",
    "pet_name": "Luna",
    "treatment_id": "uuid",
    "treatment_name": "Amoxicillin",
    "dosage_amount": 250.0,
    "dosage_unit": "mg",
    "route": "oral",
    "interval_hours": 12,
    "doses": [
      { "dose_id": "uuid", "scheduled_for": "2026-04-03T08:00:00Z" },
      { "dose_id": "uuid", "scheduled_for": "2026-04-03T20:00:00Z" }
    ]
  }
}
```

### `treatment.stopped`

Emitted when a treatment is stopped early. Includes exactly the dose IDs that were deleted so n8n can remove the corresponding calendar events.

```json
{
  "event": "treatment.stopped",
  "occurred_at": "2026-04-10T14:00:00Z",
  "domain": "petcare",
  "payload": {
    "pet_id": "uuid",
    "pet_name": "Luna",
    "treatment_id": "uuid",
    "treatment_name": "Amoxicillin",
    "stopped_at": "2026-04-10T14:00:00Z",
    "deleted_dose_ids": ["uuid1", "uuid2", "uuid3"]
  }
}
```

---

## HTTP API

**Handler:** `internal/petcare/adapters/primary/http/treatment_handler.go`  
**Registered at:** `internal/app/` use case level (same pattern as vaccines)

| Method | Route | Description |
|--------|-------|-------------|
| `POST` | `/api/v1/pets/:id/treatments` | Start treatment, generate doses |
| `GET` | `/api/v1/pets/:id/treatments` | List treatments + doses for pet |
| `GET` | `/api/v1/pets/:id/treatments/:tid` | Get single treatment + doses |
| `DELETE` | `/api/v1/pets/:id/treatments/:tid` | Stop treatment, delete future doses |

**POST request body:**
```json
{
  "name": "Amoxicillin",
  "dosage_amount": 250.0,
  "dosage_unit": "mg",
  "route": "oral",
  "interval_hours": 12,
  "started_at": "2026-04-03T08:00:00Z",
  "ended_at": "2026-04-13T08:00:00Z",
  "vet_name": "Dr. Smith",
  "notes": "Give with food"
}
```

**POST/GET response shape:**
```json
{
  "id": "uuid",
  "pet_id": "uuid",
  "name": "Amoxicillin",
  "dosage_amount": 250.0,
  "dosage_unit": "mg",
  "route": "oral",
  "interval_hours": 12,
  "started_at": "2026-04-03T08:00:00Z",
  "ended_at": "2026-04-13T08:00:00Z",
  "stopped_at": null,
  "vet_name": "Dr. Smith",
  "notes": "Give with food",
  "created_at": "2026-04-03T10:00:00Z",
  "doses": [
    { "id": "uuid", "scheduled_for": "2026-04-03T08:00:00Z" },
    { "id": "uuid", "scheduled_for": "2026-04-03T20:00:00Z" }
  ]
}
```

**DELETE** returns `204 No Content`.

All handlers reuse existing helpers: `parseUUID`, `validateRequest`, `mapError`.

---

## New Files

| File | Purpose |
|------|---------|
| `internal/petcare/domain/treatment.go` | `Treatment`, `Dose` types |
| `internal/petcare/service/treatment_service.go` | Pure CRUD service |
| `internal/petcare/service/dose_service.go` | Dose generation logic |
| `internal/petcare/adapters/secondary/sqlite/treatment_repository.go` | SQLite impl for TreatmentRepository |
| `internal/petcare/adapters/secondary/sqlite/dose_repository.go` | SQLite impl for DoseRepository |
| `internal/petcare/adapters/secondary/sqlite/migrations/002_treatments.sql` | Schema migration |
| `internal/petcare/adapters/primary/http/treatment_handler.go` | HTTP handler |
| `internal/app/treatment_usecase.go` | Orchestration + webhooks |
| `internal/app/dose_extender.go` | Background job (daily ticker) |

## Modified Files

| File | Change |
|------|--------|
| `internal/petcare/port/ports.go` | Add `TreatmentRepository`, `DoseRepository` interfaces |
| `internal/app/ports.go` | Add `TreatmentServicer`, `DoseServicer` interfaces |
| `cmd/server/main.go` | Wire new repos, services, use cases, handler, background job |
| `bruno/` | Add 4 new Bruno requests for treatment routes |
| `CLAUDE.md` | Update routes table |

---

## Verification

1. **Unit tests:**
   - `dose_service_test.go` — test `GenerateDoses` for finite, open-ended, and edge cases (zero doses, exact boundary)
   - `treatment_usecase_test.go` — mock services, assert correct webhook events emitted with doses/deleted IDs
   - `dose_extender_test.go` — mock dose repo, assert extension only generates missing doses

2. **Integration smoke test (Bruno):**
   - `POST /pets/:id/treatments` (finite) → verify doses count = `(ended_at - started_at) / interval_hours`
   - `POST /pets/:id/treatments` (open-ended, no `ended_at`) → verify ~90 days of doses
   - `GET /pets/:id/treatments/:tid` → doses included in response
   - `DELETE /pets/:id/treatments/:tid` → 204; `GET` after shows `stopped_at` set, future doses gone

3. **Webhook smoke test:**
   - Set `APP_WEBHOOK_BASE_URL` to a local receiver (e.g. `ngrok` or n8n dev instance)
   - Confirm `treatment.doses_scheduled` payload includes all dose IDs and `scheduled_for` timestamps
   - Confirm `treatment.stopped` payload includes only the dose IDs that were in the future at stop time
