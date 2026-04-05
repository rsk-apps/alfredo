# Fitness Module Design

**Date:** 2026-04-04
**Branch:** feat/fitness-module
**Status:** Approved

## Overview

A new `fitness` domain added to the Alfredo modular monolith. Tracks the app owner's (single user) workouts ingested from Apple Fitness, daily body snapshots, and freeform fitness goals. Follows the same hexagonal architecture as the `petcare` domain, with use cases in `internal/app/` handling cross-domain side effects (webhook emission).

## Scope

- Single user — no multi-user support
- Apple Fitness is the primary workout data source, but the ingestion boundary is an interface (spike deferred)
- No activity ring tracking — Apple Watch handles that natively
- No ring-based goals — freeform goals only

---

## Domain Model

All types live in `internal/fitness/domain/`.

### Profile

One record per app instance. Fully updatable.

```go
type Profile struct {
    ID        string
    FirstName string
    LastName  string
    BirthDate time.Time
    Gender    string    // "male", "female", "other"
    HeightCm  float64
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

- Age is always derived from `BirthDate` at query time — never stored.
- BMI is calculated at query time from the latest `BodySnapshot.WeightKg` + `Profile.HeightCm`.

### BodySnapshot

Time-series weekly check-ins. One record per date.

```go
type BodySnapshot struct {
    ID          string
    Date        time.Time  // date only (no time component)
    WeightKg    *float64
    WaistCm     *float64
    HipCm       *float64
    NeckCm      *float64
    BodyFatPct  *float64
    PhotoPath   *string    // relative path under data/fitness/photos/
    CreatedAt   time.Time
}
```

- All measurement fields are nullable — record partial check-ins.
- BMI is not stored; calculated from `WeightKg` + `Profile.HeightCm` on read.
- Photo is saved to disk (`data/fitness/photos/<snapshot_id>.<ext>`); `PhotoPath` stores the relative path. Upload mechanism (multipart vs URL) decided during implementation.

### Workout

Aggregator model. Source-agnostic — Apple Fitness is the first source.

```go
type Workout struct {
    ID              string
    ExternalID      string     // source system ID (e.g. Apple Fitness UUID) — used for dedup
    Type            string     // "run", "cycle", "swim", "hiit", "walk", etc.
    StartedAt       time.Time
    DurationSeconds int
    ActiveCalories  float64
    TotalCalories   float64
    DistanceMeters  *float64
    AvgPaceSecPerKm *float64
    AvgHeartRate    *float64
    MaxHeartRate    *float64
    HRZone1Pct      *float64   // % of workout time in each HR zone
    HRZone2Pct      *float64
    HRZone3Pct      *float64
    HRZone4Pct      *float64
    HRZone5Pct      *float64
    Source          string     // "apple_fitness", etc.
    CreatedAt       time.Time
}
```

- `ExternalID` + `Source` must be unique — duplicates are rejected (not upserted).

### Goal

Freeform goals only.

```go
type Goal struct {
    ID          string
    Name        string
    Description *string
    TargetValue *float64
    TargetUnit  *string    // e.g. "kg", "workouts", "km"
    Deadline    *time.Time
    AchievedAt  *time.Time // nil = not yet achieved
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

---

## Package Layout

```
internal/
  fitness/
    domain/
      profile.go
      workout.go
      body_snapshot.go
      goal.go
      errors.go
    port/
      ports.go          — ProfileRepository, WorkoutRepository, BodySnapshotRepository, GoalRepository
      ingestion.go      — WorkoutIngester interface (single + batch)
    service/
      profile_service.go
      workout_service.go
      body_snapshot_service.go
      goal_service.go
    adapters/
      primary/http/
        profile_handler.go
        workout_handler.go
        body_snapshot_handler.go
        goal_handler.go
      secondary/sqlite/
        migrations/
          003_fitness.sql
        profile_repository.go
        workout_repository.go
        body_snapshot_repository.go
        goal_repository.go
  app/
    fitness_ingestion_usecase.go   — ingest workout → save + emit webhook
    fitness_body_usecase.go        — body snapshots CRUD + emit webhook
    fitness_goal_usecase.go        — goal CRUD + achieve + emit webhook
```

### Design Rules (same as petcare)

- Services are pure CRUD — no webhook calls, no cross-domain imports.
- All webhook emission happens in `internal/app/` use cases.
- `fitness/` domain must not import from `app/`.
- HTTP handlers define narrow interfaces; use cases satisfy them.

---

## Ingestion Port

Defined in `internal/fitness/port/ingestion.go`. This is the seam for the Apple Fitness spike.

```go
type WorkoutIngester interface {
    IngestWorkout(ctx context.Context, w domain.Workout) error
    IngestWorkoutBatch(ctx context.Context, ws []domain.Workout) error
}
```

Until the spike lands, the HTTP handler (`POST /api/v1/fitness/workouts`) acts as the entry point: n8n POSTs a workout payload, the handler maps it to `domain.Workout`, and calls the use case. The use case calls `WorkoutIngester.IngestWorkout` which delegates to `WorkoutService`.

---

## HTTP Routes

All routes require the existing API key middleware.

```
# Profile
GET    /api/v1/fitness/profile
POST   /api/v1/fitness/profile           — creates profile; 409 if one already exists
PUT    /api/v1/fitness/profile

# Workouts
POST   /api/v1/fitness/workouts          — ingest single workout
POST   /api/v1/fitness/workouts/batch    — ingest array of workouts
GET    /api/v1/fitness/workouts          — list (?from=&to= date filters)
GET    /api/v1/fitness/workouts/:id
DELETE /api/v1/fitness/workouts/:id

# Body Snapshots
POST   /api/v1/fitness/body-snapshots
GET    /api/v1/fitness/body-snapshots    — list (?from=&to= date filters)
GET    /api/v1/fitness/body-snapshots/:id
DELETE /api/v1/fitness/body-snapshots/:id

# Goals
POST   /api/v1/fitness/goals
GET    /api/v1/fitness/goals
GET    /api/v1/fitness/goals/:id
PUT    /api/v1/fitness/goals/:id
DELETE /api/v1/fitness/goals/:id
POST   /api/v1/fitness/goals/:id/achieve  — sets AchievedAt to now
```

Total: 20 routes (vs 13 in petcare today).

---

## Webhook Events

Follows the existing envelope: `{ event, occurred_at, domain, payload }`.
Posted to `POST {webhook.base_url}/events`.

| Event | Trigger | Key payload fields |
|---|---|---|
| `fitness.workout.saved` | Single or batch ingest — one event per workout | `workout_id`, `type`, `started_at`, `duration_seconds`, `active_calories`, `source` |
| `fitness.body_snapshot.saved` | `POST /api/v1/fitness/body-snapshots` | `snapshot_id`, `date`, `weight_kg`, `body_fat_pct` |
| `fitness.goal.achieved` | `POST /api/v1/fitness/goals/:id/achieve` | `goal_id`, `goal_name`, `achieved_at` |

Profile updates do not emit events.

---

## SQLite Migration

`internal/fitness/adapters/secondary/sqlite/migrations/003_fitness.sql`

Tables:
- `fitness_profiles`
- `fitness_workouts` — unique index on `(external_id, source)`
- `fitness_body_snapshots` — unique index on `date`
- `fitness_goals`

---

## Testing Strategy

Mirrors petcare conventions:

- **`fitness/service/*_test.go`** — mock repository interfaces; test CRUD logic in isolation. Notable cases: dedup by `ExternalID+Source` on workout save; one-snapshot-per-date enforcement on body snapshots.
- **`app/fitness_*_usecase_test.go`** — mock domain services; test webhook emission and orchestration.
- **Handler tests** — minimal wire-up verification only.
- **Photo file handling** — tested with a temp directory; tests must not touch `data/`.

---

## Open Questions

- **Photo upload mechanism**: multipart form upload vs. accepting a URL string. Decide during implementation sprint.
- **Apple Fitness ingestion spike**: separate investigation; will implement `WorkoutIngester` once the mechanism is known (n8n real-time webhook, daily batch sync, or export file).
