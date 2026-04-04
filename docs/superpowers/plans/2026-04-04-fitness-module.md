# Fitness Module Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `fitness` domain to Alfredo that tracks the owner's workouts (ingested from Apple Fitness), body snapshots, and freeform goals, emitting webhook events to n8n on mutations.

**Architecture:** Hexagonal, mirroring `petcare`. Pure CRUD services in `internal/fitness/`, cross-domain orchestration and webhook emission in `internal/app/` use cases, HTTP handlers in `internal/fitness/adapters/primary/http/`. An explicit `WorkoutIngester` port in `internal/fitness/port/ingestion.go` provides the seam for the future Apple Fitness spike.

**Tech Stack:** Go 1.26, Echo v4, SQLite (modernc.org/sqlite), Zap logger, google/uuid, go-playground/validator.

---

## File Map

**Create:**
```
internal/fitness/domain/errors.go
internal/fitness/domain/profile.go
internal/fitness/domain/workout.go
internal/fitness/domain/body_snapshot.go
internal/fitness/domain/goal.go
internal/fitness/port/ports.go
internal/fitness/port/ingestion.go
internal/fitness/service/profile_service.go
internal/fitness/service/profile_service_test.go
internal/fitness/service/workout_service.go
internal/fitness/service/workout_service_test.go
internal/fitness/service/body_snapshot_service.go
internal/fitness/service/body_snapshot_service_test.go
internal/fitness/service/goal_service.go
internal/fitness/service/goal_service_test.go
internal/fitness/adapters/secondary/sqlite/migrations/003_fitness.sql
internal/fitness/adapters/secondary/sqlite/db.go
internal/fitness/adapters/secondary/sqlite/profile_repository.go
internal/fitness/adapters/secondary/sqlite/workout_repository.go
internal/fitness/adapters/secondary/sqlite/body_snapshot_repository.go
internal/fitness/adapters/secondary/sqlite/goal_repository.go
internal/fitness/adapters/primary/http/helpers.go
internal/fitness/adapters/primary/http/profile_handler.go
internal/fitness/adapters/primary/http/workout_handler.go
internal/fitness/adapters/primary/http/body_snapshot_handler.go
internal/fitness/adapters/primary/http/goal_handler.go
internal/app/fitness_ports.go
internal/app/fitness_profile_usecase.go
internal/app/fitness_ingestion_usecase.go
internal/app/fitness_ingestion_usecase_test.go
internal/app/fitness_body_usecase.go
internal/app/fitness_body_usecase_test.go
internal/app/fitness_goal_usecase.go
internal/app/fitness_goal_usecase_test.go
```

**Modify:**
```
cmd/server/main.go             — wire fitness repositories, services, use cases, handlers
CLAUDE.md                      — add 20 fitness routes to the routes table
```

---

## Task 1: Domain Types

**Files:**
- Create: `internal/fitness/domain/errors.go`
- Create: `internal/fitness/domain/profile.go`
- Create: `internal/fitness/domain/workout.go`
- Create: `internal/fitness/domain/body_snapshot.go`
- Create: `internal/fitness/domain/goal.go`

- [ ] **Step 1: Create errors.go**

```go
// internal/fitness/domain/errors.go
package domain

import "errors"

var (
	ErrNotFound      = errors.New("not found")
	ErrValidation    = errors.New("validation failed")
	ErrAlreadyExists = errors.New("already exists")
	ErrAlreadyAchieved = errors.New("already achieved")
)
```

- [ ] **Step 2: Create profile.go**

```go
// internal/fitness/domain/profile.go
package domain

import "time"

type Profile struct {
	ID        string
	FirstName string
	LastName  string
	BirthDate time.Time
	Gender    string // "male", "female", "other"
	HeightCm  float64
	CreatedAt time.Time
	UpdatedAt time.Time
}
```

- [ ] **Step 3: Create workout.go**

```go
// internal/fitness/domain/workout.go
package domain

import "time"

type Workout struct {
	ID              string
	ExternalID      string
	Type            string
	StartedAt       time.Time
	DurationSeconds int
	ActiveCalories  float64
	TotalCalories   float64
	DistanceMeters  *float64
	AvgPaceSecPerKm *float64
	AvgHeartRate    *float64
	MaxHeartRate    *float64
	HRZone1Pct      *float64
	HRZone2Pct      *float64
	HRZone3Pct      *float64
	HRZone4Pct      *float64
	HRZone5Pct      *float64
	Source          string
	CreatedAt       time.Time
}
```

- [ ] **Step 4: Create body_snapshot.go**

```go
// internal/fitness/domain/body_snapshot.go
package domain

import "time"

type BodySnapshot struct {
	ID         string
	Date       time.Time // date only (no time component)
	WeightKg   *float64
	WaistCm    *float64
	HipCm      *float64
	NeckCm     *float64
	BodyFatPct *float64
	PhotoPath  *string
	CreatedAt  time.Time
}
```

- [ ] **Step 5: Create goal.go**

```go
// internal/fitness/domain/goal.go
package domain

import "time"

type Goal struct {
	ID          string
	Name        string
	Description *string
	TargetValue *float64
	TargetUnit  *string
	Deadline    *time.Time
	AchievedAt  *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
```

- [ ] **Step 6: Verify it compiles**

```bash
go build ./internal/fitness/...
```

Expected: no output (success).

- [ ] **Step 7: Commit**

```bash
git add internal/fitness/domain/
git commit -m "feat(fitness): add domain types — Profile, Workout, BodySnapshot, Goal"
```

---

## Task 2: Port Interfaces

**Files:**
- Create: `internal/fitness/port/ports.go`
- Create: `internal/fitness/port/ingestion.go`

- [ ] **Step 1: Create ports.go**

```go
// internal/fitness/port/ports.go
package port

import (
	"context"
	"time"

	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
)

type ProfileRepository interface {
	Create(ctx context.Context, p domain.Profile) (*domain.Profile, error)
	Get(ctx context.Context) (*domain.Profile, error)
	Update(ctx context.Context, p domain.Profile) (*domain.Profile, error)
}

type WorkoutRepository interface {
	Create(ctx context.Context, w domain.Workout) (*domain.Workout, error)
	GetByID(ctx context.Context, id string) (*domain.Workout, error)
	List(ctx context.Context, from, to *time.Time) ([]domain.Workout, error)
	Delete(ctx context.Context, id string) error
}

type BodySnapshotRepository interface {
	Create(ctx context.Context, s domain.BodySnapshot) (*domain.BodySnapshot, error)
	GetByID(ctx context.Context, id string) (*domain.BodySnapshot, error)
	List(ctx context.Context, from, to *time.Time) ([]domain.BodySnapshot, error)
	Delete(ctx context.Context, id string) error
}

type GoalRepository interface {
	Create(ctx context.Context, g domain.Goal) (*domain.Goal, error)
	GetByID(ctx context.Context, id string) (*domain.Goal, error)
	List(ctx context.Context) ([]domain.Goal, error)
	Update(ctx context.Context, g domain.Goal) (*domain.Goal, error)
	Delete(ctx context.Context, id string) error
}
```

- [ ] **Step 2: Create ingestion.go**

```go
// internal/fitness/port/ingestion.go
package port

import (
	"context"

	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
)

// WorkoutIngester is the seam for Apple Fitness (and future) workout sources.
// Both single and batch ingestion are supported so the interface does not
// constrain the spike to a particular delivery mechanism.
type WorkoutIngester interface {
	IngestWorkout(ctx context.Context, w domain.Workout) (*domain.Workout, error)
	IngestWorkoutBatch(ctx context.Context, ws []domain.Workout) ([]domain.Workout, error)
}
```

- [ ] **Step 3: Verify it compiles**

```bash
go build ./internal/fitness/...
```

Expected: no output.

- [ ] **Step 4: Commit**

```bash
git add internal/fitness/port/
git commit -m "feat(fitness): add repository and ingestion port interfaces"
```

---

## Task 3: ProfileService

**Files:**
- Create: `internal/fitness/service/profile_service.go`
- Create: `internal/fitness/service/profile_service_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/fitness/service/profile_service_test.go
package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
	"github.com/rafaelsoares/alfredo/internal/fitness/service"
)

type mockProfileRepo struct {
	profile *domain.Profile
	err     error
}

func (m *mockProfileRepo) Create(_ context.Context, p domain.Profile) (*domain.Profile, error) {
	return &p, m.err
}
func (m *mockProfileRepo) Get(_ context.Context) (*domain.Profile, error) {
	return m.profile, m.err
}
func (m *mockProfileRepo) Update(_ context.Context, p domain.Profile) (*domain.Profile, error) {
	return &p, m.err
}

func TestProfileService_Create_AssignsID(t *testing.T) {
	svc := service.NewProfileService(&mockProfileRepo{})
	p, err := svc.Create(context.Background(), service.CreateProfileInput{
		FirstName: "Rafael", LastName: "Soares",
		BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC),
		Gender:    "male", HeightCm: 180,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.ID == "" {
		t.Error("expected ID to be assigned")
	}
}

func TestProfileService_Create_ValidationErrors(t *testing.T) {
	svc := service.NewProfileService(&mockProfileRepo{})
	cases := []struct {
		name  string
		input service.CreateProfileInput
	}{
		{"missing first name", service.CreateProfileInput{LastName: "S", BirthDate: time.Now(), Gender: "male", HeightCm: 180}},
		{"missing last name", service.CreateProfileInput{FirstName: "R", BirthDate: time.Now(), Gender: "male", HeightCm: 180}},
		{"zero height", service.CreateProfileInput{FirstName: "R", LastName: "S", BirthDate: time.Now(), Gender: "male"}},
		{"missing gender", service.CreateProfileInput{FirstName: "R", LastName: "S", BirthDate: time.Now(), HeightCm: 180}},
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

func TestProfileService_Create_ReturnsAlreadyExistsFromRepo(t *testing.T) {
	svc := service.NewProfileService(&mockProfileRepo{err: domain.ErrAlreadyExists})
	_, err := svc.Create(context.Background(), service.CreateProfileInput{
		FirstName: "R", LastName: "S",
		BirthDate: time.Now(), Gender: "male", HeightCm: 180,
	})
	if !errors.Is(err, domain.ErrAlreadyExists) {
		t.Errorf("got %v, want ErrAlreadyExists", err)
	}
}

func TestProfileService_Update_ValidationErrors(t *testing.T) {
	svc := service.NewProfileService(&mockProfileRepo{profile: &domain.Profile{ID: "p1"}})
	_, err := svc.Update(context.Background(), service.UpdateProfileInput{HeightCm: -1})
	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("got %v, want ErrValidation", err)
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./internal/fitness/service/... -run TestProfileService -v 2>&1 | head -5
```

Expected: compilation error — `service` package does not exist yet.

- [ ] **Step 3: Implement ProfileService**

```go
// internal/fitness/service/profile_service.go
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
	"github.com/rafaelsoares/alfredo/internal/fitness/port"
)

type CreateProfileInput struct {
	FirstName string
	LastName  string
	BirthDate time.Time
	Gender    string
	HeightCm  float64
}

type UpdateProfileInput struct {
	FirstName *string
	LastName  *string
	BirthDate *time.Time
	Gender    *string
	HeightCm  *float64
}

type ProfileService struct {
	repo port.ProfileRepository
}

func NewProfileService(repo port.ProfileRepository) *ProfileService {
	return &ProfileService{repo: repo}
}

func (s *ProfileService) Create(ctx context.Context, in CreateProfileInput) (*domain.Profile, error) {
	if in.FirstName == "" {
		return nil, fmt.Errorf("%w: first_name is required", domain.ErrValidation)
	}
	if in.LastName == "" {
		return nil, fmt.Errorf("%w: last_name is required", domain.ErrValidation)
	}
	if in.Gender == "" {
		return nil, fmt.Errorf("%w: gender is required", domain.ErrValidation)
	}
	if in.HeightCm <= 0 {
		return nil, fmt.Errorf("%w: height_cm must be greater than zero", domain.ErrValidation)
	}
	now := time.Now().UTC()
	return s.repo.Create(ctx, domain.Profile{
		ID:        uuid.New().String(),
		FirstName: in.FirstName,
		LastName:  in.LastName,
		BirthDate: in.BirthDate.UTC(),
		Gender:    in.Gender,
		HeightCm:  in.HeightCm,
		CreatedAt: now,
		UpdatedAt: now,
	})
}

func (s *ProfileService) Get(ctx context.Context) (*domain.Profile, error) {
	return s.repo.Get(ctx)
}

func (s *ProfileService) Update(ctx context.Context, in UpdateProfileInput) (*domain.Profile, error) {
	p, err := s.repo.Get(ctx)
	if err != nil {
		return nil, err
	}
	if in.HeightCm != nil && *in.HeightCm <= 0 {
		return nil, fmt.Errorf("%w: height_cm must be greater than zero", domain.ErrValidation)
	}
	if in.FirstName != nil {
		p.FirstName = *in.FirstName
	}
	if in.LastName != nil {
		p.LastName = *in.LastName
	}
	if in.BirthDate != nil {
		t := in.BirthDate.UTC()
		p.BirthDate = t
	}
	if in.Gender != nil {
		p.Gender = *in.Gender
	}
	if in.HeightCm != nil {
		p.HeightCm = *in.HeightCm
	}
	p.UpdatedAt = time.Now().UTC()
	return s.repo.Update(ctx, *p)
}
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
go test ./internal/fitness/service/... -run TestProfileService -v
```

Expected: all 4 test cases PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/fitness/service/
git commit -m "feat(fitness): add ProfileService with validation"
```

---

## Task 4: WorkoutService

**Files:**
- Create: `internal/fitness/service/workout_service.go`
- Create: `internal/fitness/service/workout_service_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/fitness/service/workout_service_test.go
package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
	"github.com/rafaelsoares/alfredo/internal/fitness/service"
)

type mockWorkoutRepo struct {
	workout *domain.Workout
	err     error
}

func (m *mockWorkoutRepo) Create(_ context.Context, w domain.Workout) (*domain.Workout, error) {
	return &w, m.err
}
func (m *mockWorkoutRepo) GetByID(_ context.Context, _ string) (*domain.Workout, error) {
	return m.workout, m.err
}
func (m *mockWorkoutRepo) List(_ context.Context, _, _ *time.Time) ([]domain.Workout, error) {
	if m.workout != nil {
		return []domain.Workout{*m.workout}, m.err
	}
	return nil, m.err
}
func (m *mockWorkoutRepo) Delete(_ context.Context, _ string) error { return m.err }

func TestWorkoutService_Create_AssignsID(t *testing.T) {
	svc := service.NewWorkoutService(&mockWorkoutRepo{})
	w, err := svc.Create(context.Background(), service.CreateWorkoutInput{
		ExternalID: "ext-1", Type: "run", Source: "apple_fitness",
		StartedAt: time.Now(), DurationSeconds: 3600, ActiveCalories: 400, TotalCalories: 500,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.ID == "" {
		t.Error("expected ID to be assigned")
	}
}

func TestWorkoutService_Create_ValidationErrors(t *testing.T) {
	svc := service.NewWorkoutService(&mockWorkoutRepo{})
	cases := []struct {
		name  string
		input service.CreateWorkoutInput
	}{
		{"missing external_id", service.CreateWorkoutInput{Type: "run", Source: "apple_fitness", StartedAt: time.Now(), DurationSeconds: 1, ActiveCalories: 1, TotalCalories: 1}},
		{"missing type", service.CreateWorkoutInput{ExternalID: "e1", Source: "apple_fitness", StartedAt: time.Now(), DurationSeconds: 1, ActiveCalories: 1, TotalCalories: 1}},
		{"missing source", service.CreateWorkoutInput{ExternalID: "e1", Type: "run", StartedAt: time.Now(), DurationSeconds: 1, ActiveCalories: 1, TotalCalories: 1}},
		{"zero duration", service.CreateWorkoutInput{ExternalID: "e1", Type: "run", Source: "apple_fitness", StartedAt: time.Now(), ActiveCalories: 1, TotalCalories: 1}},
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

func TestWorkoutService_Create_PropagatesDuplicateError(t *testing.T) {
	svc := service.NewWorkoutService(&mockWorkoutRepo{err: domain.ErrAlreadyExists})
	_, err := svc.Create(context.Background(), service.CreateWorkoutInput{
		ExternalID: "ext-1", Type: "run", Source: "apple_fitness",
		StartedAt: time.Now(), DurationSeconds: 60, ActiveCalories: 100, TotalCalories: 120,
	})
	if !errors.Is(err, domain.ErrAlreadyExists) {
		t.Errorf("got %v, want ErrAlreadyExists", err)
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./internal/fitness/service/... -run TestWorkoutService -v 2>&1 | head -5
```

Expected: compilation error — `NewWorkoutService` not defined.

- [ ] **Step 3: Implement WorkoutService**

```go
// internal/fitness/service/workout_service.go
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
	"github.com/rafaelsoares/alfredo/internal/fitness/port"
)

type CreateWorkoutInput struct {
	ExternalID      string
	Type            string
	StartedAt       time.Time
	DurationSeconds int
	ActiveCalories  float64
	TotalCalories   float64
	DistanceMeters  *float64
	AvgPaceSecPerKm *float64
	AvgHeartRate    *float64
	MaxHeartRate    *float64
	HRZone1Pct      *float64
	HRZone2Pct      *float64
	HRZone3Pct      *float64
	HRZone4Pct      *float64
	HRZone5Pct      *float64
	Source          string
}

type WorkoutService struct {
	repo port.WorkoutRepository
}

func NewWorkoutService(repo port.WorkoutRepository) *WorkoutService {
	return &WorkoutService{repo: repo}
}

func (s *WorkoutService) Create(ctx context.Context, in CreateWorkoutInput) (*domain.Workout, error) {
	if in.ExternalID == "" {
		return nil, fmt.Errorf("%w: external_id is required", domain.ErrValidation)
	}
	if in.Type == "" {
		return nil, fmt.Errorf("%w: type is required", domain.ErrValidation)
	}
	if in.Source == "" {
		return nil, fmt.Errorf("%w: source is required", domain.ErrValidation)
	}
	if in.DurationSeconds <= 0 {
		return nil, fmt.Errorf("%w: duration_seconds must be greater than zero", domain.ErrValidation)
	}
	return s.repo.Create(ctx, domain.Workout{
		ID:              uuid.New().String(),
		ExternalID:      in.ExternalID,
		Type:            in.Type,
		StartedAt:       in.StartedAt.UTC(),
		DurationSeconds: in.DurationSeconds,
		ActiveCalories:  in.ActiveCalories,
		TotalCalories:   in.TotalCalories,
		DistanceMeters:  in.DistanceMeters,
		AvgPaceSecPerKm: in.AvgPaceSecPerKm,
		AvgHeartRate:    in.AvgHeartRate,
		MaxHeartRate:    in.MaxHeartRate,
		HRZone1Pct:      in.HRZone1Pct,
		HRZone2Pct:      in.HRZone2Pct,
		HRZone3Pct:      in.HRZone3Pct,
		HRZone4Pct:      in.HRZone4Pct,
		HRZone5Pct:      in.HRZone5Pct,
		Source:          in.Source,
		CreatedAt:       time.Now().UTC(),
	})
}

func (s *WorkoutService) GetByID(ctx context.Context, id string) (*domain.Workout, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *WorkoutService) List(ctx context.Context, from, to *time.Time) ([]domain.Workout, error) {
	return s.repo.List(ctx, from, to)
}

func (s *WorkoutService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
go test ./internal/fitness/service/... -run TestWorkoutService -v
```

Expected: all 3 test cases PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/fitness/service/workout_service.go internal/fitness/service/workout_service_test.go
git commit -m "feat(fitness): add WorkoutService with dedup propagation"
```

---

## Task 5: BodySnapshotService

**Files:**
- Create: `internal/fitness/service/body_snapshot_service.go`
- Create: `internal/fitness/service/body_snapshot_service_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/fitness/service/body_snapshot_service_test.go
package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
	"github.com/rafaelsoares/alfredo/internal/fitness/service"
)

type mockBodySnapshotRepo struct {
	snapshot *domain.BodySnapshot
	err      error
}

func (m *mockBodySnapshotRepo) Create(_ context.Context, s domain.BodySnapshot) (*domain.BodySnapshot, error) {
	return &s, m.err
}
func (m *mockBodySnapshotRepo) GetByID(_ context.Context, _ string) (*domain.BodySnapshot, error) {
	return m.snapshot, m.err
}
func (m *mockBodySnapshotRepo) List(_ context.Context, _, _ *time.Time) ([]domain.BodySnapshot, error) {
	if m.snapshot != nil {
		return []domain.BodySnapshot{*m.snapshot}, m.err
	}
	return nil, m.err
}
func (m *mockBodySnapshotRepo) Delete(_ context.Context, _ string) error { return m.err }

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
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./internal/fitness/service/... -run TestBodySnapshotService -v 2>&1 | head -5
```

Expected: compilation error — `NewBodySnapshotService` not defined.

- [ ] **Step 3: Implement BodySnapshotService**

```go
// internal/fitness/service/body_snapshot_service.go
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
	"github.com/rafaelsoares/alfredo/internal/fitness/port"
)

type CreateBodySnapshotInput struct {
	Date       time.Time
	WeightKg   *float64
	WaistCm    *float64
	HipCm      *float64
	NeckCm     *float64
	BodyFatPct *float64
	PhotoPath  *string
}

type BodySnapshotService struct {
	repo port.BodySnapshotRepository
}

func NewBodySnapshotService(repo port.BodySnapshotRepository) *BodySnapshotService {
	return &BodySnapshotService{repo: repo}
}

func (s *BodySnapshotService) Create(ctx context.Context, in CreateBodySnapshotInput) (*domain.BodySnapshot, error) {
	if in.Date.IsZero() {
		return nil, fmt.Errorf("%w: date is required", domain.ErrValidation)
	}
	// Normalise to date-only (midnight UTC) so unique index on date works correctly.
	date := time.Date(in.Date.Year(), in.Date.Month(), in.Date.Day(), 0, 0, 0, 0, time.UTC)
	return s.repo.Create(ctx, domain.BodySnapshot{
		ID:         uuid.New().String(),
		Date:       date,
		WeightKg:   in.WeightKg,
		WaistCm:    in.WaistCm,
		HipCm:      in.HipCm,
		NeckCm:     in.NeckCm,
		BodyFatPct: in.BodyFatPct,
		PhotoPath:  in.PhotoPath,
		CreatedAt:  time.Now().UTC(),
	})
}

func (s *BodySnapshotService) GetByID(ctx context.Context, id string) (*domain.BodySnapshot, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *BodySnapshotService) List(ctx context.Context, from, to *time.Time) ([]domain.BodySnapshot, error) {
	return s.repo.List(ctx, from, to)
}

func (s *BodySnapshotService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
go test ./internal/fitness/service/... -run TestBodySnapshotService -v
```

Expected: all 3 test cases PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/fitness/service/body_snapshot_service.go internal/fitness/service/body_snapshot_service_test.go
git commit -m "feat(fitness): add BodySnapshotService with date normalisation"
```

---

## Task 6: GoalService

**Files:**
- Create: `internal/fitness/service/goal_service.go`
- Create: `internal/fitness/service/goal_service_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/fitness/service/goal_service_test.go
package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
	"github.com/rafaelsoares/alfredo/internal/fitness/service"
)

type mockGoalRepo struct {
	goal *domain.Goal
	err  error
}

func (m *mockGoalRepo) Create(_ context.Context, g domain.Goal) (*domain.Goal, error) {
	return &g, m.err
}
func (m *mockGoalRepo) GetByID(_ context.Context, _ string) (*domain.Goal, error) {
	return m.goal, m.err
}
func (m *mockGoalRepo) List(_ context.Context) ([]domain.Goal, error) {
	if m.goal != nil {
		return []domain.Goal{*m.goal}, m.err
	}
	return nil, m.err
}
func (m *mockGoalRepo) Update(_ context.Context, g domain.Goal) (*domain.Goal, error) {
	return &g, m.err
}
func (m *mockGoalRepo) Delete(_ context.Context, _ string) error { return m.err }

func TestGoalService_Create_AssignsID(t *testing.T) {
	svc := service.NewGoalService(&mockGoalRepo{})
	g, err := svc.Create(context.Background(), service.CreateGoalInput{Name: "Run a 5k"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.ID == "" {
		t.Error("expected ID to be assigned")
	}
}

func TestGoalService_Create_RequiresName(t *testing.T) {
	svc := service.NewGoalService(&mockGoalRepo{})
	_, err := svc.Create(context.Background(), service.CreateGoalInput{})
	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("got %v, want ErrValidation", err)
	}
}

func TestGoalService_Achieve_AlreadyAchieved(t *testing.T) {
	achieved := time.Now()
	svc := service.NewGoalService(&mockGoalRepo{
		goal: &domain.Goal{ID: "g1", Name: "Run a 5k", AchievedAt: &achieved},
	})
	_, err := svc.Achieve(context.Background(), "g1")
	if !errors.Is(err, domain.ErrAlreadyAchieved) {
		t.Errorf("got %v, want ErrAlreadyAchieved", err)
	}
}

func TestGoalService_Achieve_SetsAchievedAt(t *testing.T) {
	svc := service.NewGoalService(&mockGoalRepo{
		goal: &domain.Goal{ID: "g1", Name: "Run a 5k"},
	})
	g, err := svc.Achieve(context.Background(), "g1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.AchievedAt == nil {
		t.Error("expected AchievedAt to be set")
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./internal/fitness/service/... -run TestGoalService -v 2>&1 | head -5
```

Expected: compilation error — `NewGoalService` not defined.

- [ ] **Step 3: Implement GoalService**

```go
// internal/fitness/service/goal_service.go
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
	"github.com/rafaelsoares/alfredo/internal/fitness/port"
)

type CreateGoalInput struct {
	Name        string
	Description *string
	TargetValue *float64
	TargetUnit  *string
	Deadline    *time.Time
}

type UpdateGoalInput struct {
	Name        *string
	Description *string
	TargetValue *float64
	TargetUnit  *string
	Deadline    *time.Time
}

type GoalService struct {
	repo port.GoalRepository
}

func NewGoalService(repo port.GoalRepository) *GoalService {
	return &GoalService{repo: repo}
}

func (s *GoalService) Create(ctx context.Context, in CreateGoalInput) (*domain.Goal, error) {
	if in.Name == "" {
		return nil, fmt.Errorf("%w: name is required", domain.ErrValidation)
	}
	now := time.Now().UTC()
	return s.repo.Create(ctx, domain.Goal{
		ID:          uuid.New().String(),
		Name:        in.Name,
		Description: in.Description,
		TargetValue: in.TargetValue,
		TargetUnit:  in.TargetUnit,
		Deadline:    in.Deadline,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
}

func (s *GoalService) GetByID(ctx context.Context, id string) (*domain.Goal, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *GoalService) List(ctx context.Context) ([]domain.Goal, error) {
	return s.repo.List(ctx)
}

func (s *GoalService) Update(ctx context.Context, id string, in UpdateGoalInput) (*domain.Goal, error) {
	g, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if in.Name != nil {
		g.Name = *in.Name
	}
	if in.Description != nil {
		g.Description = in.Description
	}
	if in.TargetValue != nil {
		g.TargetValue = in.TargetValue
	}
	if in.TargetUnit != nil {
		g.TargetUnit = in.TargetUnit
	}
	if in.Deadline != nil {
		g.Deadline = in.Deadline
	}
	g.UpdatedAt = time.Now().UTC()
	return s.repo.Update(ctx, *g)
}

func (s *GoalService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *GoalService) Achieve(ctx context.Context, id string) (*domain.Goal, error) {
	g, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if g.AchievedAt != nil {
		return nil, fmt.Errorf("%w: goal %s has already been achieved", domain.ErrAlreadyAchieved, id)
	}
	now := time.Now().UTC()
	g.AchievedAt = &now
	g.UpdatedAt = now
	return s.repo.Update(ctx, *g)
}
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
go test ./internal/fitness/service/... -run TestGoalService -v
```

Expected: all 4 test cases PASS.

- [ ] **Step 5: Run all service tests**

```bash
go test ./internal/fitness/service/... -v
```

Expected: all tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/fitness/service/goal_service.go internal/fitness/service/goal_service_test.go
git commit -m "feat(fitness): add GoalService with achieve guard"
```

---

## Task 7: SQLite Migration + Migrate Function

**Files:**
- Create: `internal/fitness/adapters/secondary/sqlite/migrations/003_fitness.sql`
- Create: `internal/fitness/adapters/secondary/sqlite/db.go`

- [ ] **Step 1: Write the migration SQL**

```sql
-- internal/fitness/adapters/secondary/sqlite/migrations/003_fitness.sql
CREATE TABLE IF NOT EXISTS fitness_profiles (
    id         TEXT PRIMARY KEY,
    first_name TEXT NOT NULL,
    last_name  TEXT NOT NULL,
    birth_date TEXT NOT NULL,
    gender     TEXT NOT NULL,
    height_cm  REAL NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS fitness_workouts (
    id               TEXT PRIMARY KEY,
    external_id      TEXT NOT NULL,
    type             TEXT NOT NULL,
    started_at       TEXT NOT NULL,
    duration_seconds INTEGER NOT NULL,
    active_calories  REAL NOT NULL,
    total_calories   REAL NOT NULL,
    distance_meters  REAL,
    avg_pace_sec_per_km REAL,
    avg_heart_rate   REAL,
    max_heart_rate   REAL,
    hr_zone1_pct     REAL,
    hr_zone2_pct     REAL,
    hr_zone3_pct     REAL,
    hr_zone4_pct     REAL,
    hr_zone5_pct     REAL,
    source           TEXT NOT NULL,
    created_at       TEXT NOT NULL,
    UNIQUE(external_id, source)
);

CREATE INDEX IF NOT EXISTS idx_fitness_workouts_started_at ON fitness_workouts(started_at);

CREATE TABLE IF NOT EXISTS fitness_body_snapshots (
    id          TEXT PRIMARY KEY,
    date        TEXT NOT NULL UNIQUE,
    weight_kg   REAL,
    waist_cm    REAL,
    hip_cm      REAL,
    neck_cm     REAL,
    body_fat_pct REAL,
    photo_path  TEXT,
    created_at  TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_fitness_body_snapshots_date ON fitness_body_snapshots(date);

CREATE TABLE IF NOT EXISTS fitness_goals (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    description  TEXT,
    target_value REAL,
    target_unit  TEXT,
    deadline     TEXT,
    achieved_at  TEXT,
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL
)
```

- [ ] **Step 2: Create db.go**

```go
// internal/fitness/adapters/secondary/sqlite/db.go
package sqlite

import (
	"database/sql"
	_ "embed"
	"fmt"
	"strings"
)

//go:embed migrations/003_fitness.sql
var migration003 string

// Migrate applies the fitness module migration to an existing open *sql.DB.
// It uses the same schema_migrations table as the petcare migrations.
// Safe to call multiple times — skips already-applied versions.
func Migrate(db *sql.DB) error {
	migrations := []struct {
		version string
		sql     string
	}{
		{"003_fitness", migration003},
	}

	for _, m := range migrations {
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, m.version).Scan(&count); err != nil {
			return fmt.Errorf("check migration %s: %w", m.version, err)
		}
		if count > 0 {
			continue
		}
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin tx for %s: %w", m.version, err)
		}
		for _, stmt := range splitSQL(m.sql) {
			if _, err := tx.Exec(stmt); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("apply migration %s (stmt: %q): %w", m.version, stmt, err)
			}
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, m.version); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %s: %w", m.version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", m.version, err)
		}
	}
	return nil
}

func splitSQL(script string) []string {
	var out []string
	for _, s := range strings.Split(script, ";") {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
```

- [ ] **Step 3: Verify it compiles**

```bash
go build ./internal/fitness/...
```

Expected: no output.

- [ ] **Step 4: Commit**

```bash
git add internal/fitness/adapters/secondary/sqlite/
git commit -m "feat(fitness): add SQLite migration 003 and Migrate() function"
```

---

## Task 8: SQLite Repositories

**Files:**
- Create: `internal/fitness/adapters/secondary/sqlite/profile_repository.go`
- Create: `internal/fitness/adapters/secondary/sqlite/workout_repository.go`
- Create: `internal/fitness/adapters/secondary/sqlite/body_snapshot_repository.go`
- Create: `internal/fitness/adapters/secondary/sqlite/goal_repository.go`

- [ ] **Step 1: Create profile_repository.go**

```go
// internal/fitness/adapters/secondary/sqlite/profile_repository.go
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
)

type scanner interface {
	Scan(dest ...any) error
}

type ProfileRepository struct{ db *sql.DB }

func NewProfileRepository(db *sql.DB) *ProfileRepository { return &ProfileRepository{db: db} }

func (r *ProfileRepository) Create(ctx context.Context, p domain.Profile) (*domain.Profile, error) {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO fitness_profiles (id, first_name, last_name, birth_date, gender, height_cm, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.FirstName, p.LastName,
		p.BirthDate.Format("2006-01-02"), p.Gender, p.HeightCm,
		p.CreatedAt.Format(time.RFC3339), p.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, domain.ErrAlreadyExists
		}
		return nil, err
	}
	return &p, nil
}

func (r *ProfileRepository) Get(ctx context.Context) (*domain.Profile, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, first_name, last_name, birth_date, gender, height_cm, created_at, updated_at
		 FROM fitness_profiles LIMIT 1`)
	p, err := scanProfile(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return p, err
}

func (r *ProfileRepository) Update(ctx context.Context, p domain.Profile) (*domain.Profile, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE fitness_profiles SET first_name=?, last_name=?, birth_date=?, gender=?, height_cm=?, updated_at=?
		 WHERE id=?`,
		p.FirstName, p.LastName, p.BirthDate.Format("2006-01-02"),
		p.Gender, p.HeightCm, p.UpdatedAt.Format(time.RFC3339), p.ID,
	)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, domain.ErrNotFound
	}
	return &p, nil
}

func scanProfile(s scanner) (*domain.Profile, error) {
	var p domain.Profile
	var birthDate, createdAt, updatedAt string
	err := s.Scan(&p.ID, &p.FirstName, &p.LastName, &birthDate, &p.Gender, &p.HeightCm, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	p.BirthDate, err = time.Parse("2006-01-02", birthDate)
	if err != nil {
		return nil, fmt.Errorf("parse birth_date %q: %w", birthDate, err)
	}
	p.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at %q: %w", createdAt, err)
	}
	p.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at %q: %w", updatedAt, err)
	}
	return &p, nil
}

// isUniqueViolation reports whether err is a SQLite UNIQUE constraint violation.
func isUniqueViolation(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "UNIQUE constraint failed") ||
		strings.Contains(err.Error(), "unique constraint"))
}
```

> **Note:** add `"strings"` to the import block in the file above.

- [ ] **Step 2: Create workout_repository.go**

```go
// internal/fitness/adapters/secondary/sqlite/workout_repository.go
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
)

type WorkoutRepository struct{ db *sql.DB }

func NewWorkoutRepository(db *sql.DB) *WorkoutRepository { return &WorkoutRepository{db: db} }

func (r *WorkoutRepository) Create(ctx context.Context, w domain.Workout) (*domain.Workout, error) {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO fitness_workouts
		 (id, external_id, type, started_at, duration_seconds, active_calories, total_calories,
		  distance_meters, avg_pace_sec_per_km, avg_heart_rate, max_heart_rate,
		  hr_zone1_pct, hr_zone2_pct, hr_zone3_pct, hr_zone4_pct, hr_zone5_pct, source, created_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		w.ID, w.ExternalID, w.Type, w.StartedAt.Format(time.RFC3339),
		w.DurationSeconds, w.ActiveCalories, w.TotalCalories,
		w.DistanceMeters, w.AvgPaceSecPerKm, w.AvgHeartRate, w.MaxHeartRate,
		w.HRZone1Pct, w.HRZone2Pct, w.HRZone3Pct, w.HRZone4Pct, w.HRZone5Pct,
		w.Source, w.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, domain.ErrAlreadyExists
		}
		return nil, err
	}
	return &w, nil
}

func (r *WorkoutRepository) GetByID(ctx context.Context, id string) (*domain.Workout, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, external_id, type, started_at, duration_seconds, active_calories, total_calories,
		        distance_meters, avg_pace_sec_per_km, avg_heart_rate, max_heart_rate,
		        hr_zone1_pct, hr_zone2_pct, hr_zone3_pct, hr_zone4_pct, hr_zone5_pct, source, created_at
		 FROM fitness_workouts WHERE id = ?`, id)
	w, err := scanWorkout(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return w, err
}

func (r *WorkoutRepository) List(ctx context.Context, from, to *time.Time) ([]domain.Workout, error) {
	query := `SELECT id, external_id, type, started_at, duration_seconds, active_calories, total_calories,
	                 distance_meters, avg_pace_sec_per_km, avg_heart_rate, max_heart_rate,
	                 hr_zone1_pct, hr_zone2_pct, hr_zone3_pct, hr_zone4_pct, hr_zone5_pct, source, created_at
	          FROM fitness_workouts`
	args := []any{}
	clauses := []string{}
	if from != nil {
		clauses = append(clauses, "started_at >= ?")
		args = append(args, from.Format(time.RFC3339))
	}
	if to != nil {
		clauses = append(clauses, "started_at <= ?")
		args = append(args, to.Format(time.RFC3339))
	}
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY started_at DESC"
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	var ws []domain.Workout
	for rows.Next() {
		w, err := scanWorkout(rows)
		if err != nil {
			return nil, err
		}
		ws = append(ws, *w)
	}
	return ws, rows.Err()
}

func (r *WorkoutRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM fitness_workouts WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func scanWorkout(s scanner) (*domain.Workout, error) {
	var w domain.Workout
	var startedAt, createdAt string
	err := s.Scan(
		&w.ID, &w.ExternalID, &w.Type, &startedAt, &w.DurationSeconds,
		&w.ActiveCalories, &w.TotalCalories,
		&w.DistanceMeters, &w.AvgPaceSecPerKm, &w.AvgHeartRate, &w.MaxHeartRate,
		&w.HRZone1Pct, &w.HRZone2Pct, &w.HRZone3Pct, &w.HRZone4Pct, &w.HRZone5Pct,
		&w.Source, &createdAt,
	)
	if err != nil {
		return nil, err
	}
	w.StartedAt, err = time.Parse(time.RFC3339, startedAt)
	if err != nil {
		return nil, fmt.Errorf("parse started_at %q: %w", startedAt, err)
	}
	w.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at %q: %w", createdAt, err)
	}
	return &w, nil
}
```

- [ ] **Step 3: Create body_snapshot_repository.go**

```go
// internal/fitness/adapters/secondary/sqlite/body_snapshot_repository.go
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
)

type BodySnapshotRepository struct{ db *sql.DB }

func NewBodySnapshotRepository(db *sql.DB) *BodySnapshotRepository {
	return &BodySnapshotRepository{db: db}
}

func (r *BodySnapshotRepository) Create(ctx context.Context, s domain.BodySnapshot) (*domain.BodySnapshot, error) {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO fitness_body_snapshots
		 (id, date, weight_kg, waist_cm, hip_cm, neck_cm, body_fat_pct, photo_path, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.Date.Format("2006-01-02"),
		s.WeightKg, s.WaistCm, s.HipCm, s.NeckCm, s.BodyFatPct, s.PhotoPath,
		s.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, domain.ErrAlreadyExists
		}
		return nil, err
	}
	return &s, nil
}

func (r *BodySnapshotRepository) GetByID(ctx context.Context, id string) (*domain.BodySnapshot, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, date, weight_kg, waist_cm, hip_cm, neck_cm, body_fat_pct, photo_path, created_at
		 FROM fitness_body_snapshots WHERE id = ?`, id)
	s, err := scanBodySnapshot(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return s, err
}

func (r *BodySnapshotRepository) List(ctx context.Context, from, to *time.Time) ([]domain.BodySnapshot, error) {
	query := `SELECT id, date, weight_kg, waist_cm, hip_cm, neck_cm, body_fat_pct, photo_path, created_at
	          FROM fitness_body_snapshots`
	args := []any{}
	clauses := []string{}
	if from != nil {
		clauses = append(clauses, "date >= ?")
		args = append(args, from.Format("2006-01-02"))
	}
	if to != nil {
		clauses = append(clauses, "date <= ?")
		args = append(args, to.Format("2006-01-02"))
	}
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY date DESC"
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	var snapshots []domain.BodySnapshot
	for rows.Next() {
		s, err := scanBodySnapshot(rows)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, *s)
	}
	return snapshots, rows.Err()
}

func (r *BodySnapshotRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM fitness_body_snapshots WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func scanBodySnapshot(s scanner) (*domain.BodySnapshot, error) {
	var snap domain.BodySnapshot
	var date, createdAt string
	err := s.Scan(&snap.ID, &date, &snap.WeightKg, &snap.WaistCm, &snap.HipCm,
		&snap.NeckCm, &snap.BodyFatPct, &snap.PhotoPath, &createdAt)
	if err != nil {
		return nil, err
	}
	snap.Date, err = time.Parse("2006-01-02", date)
	if err != nil {
		return nil, fmt.Errorf("parse date %q: %w", date, err)
	}
	snap.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at %q: %w", createdAt, err)
	}
	return &snap, nil
}
```

- [ ] **Step 4: Create goal_repository.go**

```go
// internal/fitness/adapters/secondary/sqlite/goal_repository.go
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
)

type GoalRepository struct{ db *sql.DB }

func NewGoalRepository(db *sql.DB) *GoalRepository { return &GoalRepository{db: db} }

func (r *GoalRepository) Create(ctx context.Context, g domain.Goal) (*domain.Goal, error) {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO fitness_goals
		 (id, name, description, target_value, target_unit, deadline, achieved_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		g.ID, g.Name, g.Description, g.TargetValue, g.TargetUnit,
		formatOptionalRFC3339(g.Deadline), formatOptionalRFC3339(g.AchievedAt),
		g.CreatedAt.Format(time.RFC3339), g.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, err
	}
	return &g, nil
}

func (r *GoalRepository) GetByID(ctx context.Context, id string) (*domain.Goal, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, name, description, target_value, target_unit, deadline, achieved_at, created_at, updated_at
		 FROM fitness_goals WHERE id = ?`, id)
	g, err := scanGoal(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return g, err
}

func (r *GoalRepository) List(ctx context.Context) ([]domain.Goal, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, description, target_value, target_unit, deadline, achieved_at, created_at, updated_at
		 FROM fitness_goals ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	var goals []domain.Goal
	for rows.Next() {
		g, err := scanGoal(rows)
		if err != nil {
			return nil, err
		}
		goals = append(goals, *g)
	}
	return goals, rows.Err()
}

func (r *GoalRepository) Update(ctx context.Context, g domain.Goal) (*domain.Goal, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE fitness_goals
		 SET name=?, description=?, target_value=?, target_unit=?, deadline=?, achieved_at=?, updated_at=?
		 WHERE id=?`,
		g.Name, g.Description, g.TargetValue, g.TargetUnit,
		formatOptionalRFC3339(g.Deadline), formatOptionalRFC3339(g.AchievedAt),
		g.UpdatedAt.Format(time.RFC3339), g.ID,
	)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, domain.ErrNotFound
	}
	return &g, nil
}

func (r *GoalRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM fitness_goals WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func scanGoal(s scanner) (*domain.Goal, error) {
	var g domain.Goal
	var createdAt, updatedAt string
	var deadline, achievedAt sql.NullString
	err := s.Scan(&g.ID, &g.Name, &g.Description, &g.TargetValue, &g.TargetUnit,
		&deadline, &achievedAt, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	g.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at %q: %w", createdAt, err)
	}
	g.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at %q: %w", updatedAt, err)
	}
	if deadline.Valid && deadline.String != "" {
		t, err := time.Parse(time.RFC3339, deadline.String)
		if err != nil {
			return nil, fmt.Errorf("parse deadline %q: %w", deadline.String, err)
		}
		g.Deadline = &t
	}
	if achievedAt.Valid && achievedAt.String != "" {
		t, err := time.Parse(time.RFC3339, achievedAt.String)
		if err != nil {
			return nil, fmt.Errorf("parse achieved_at %q: %w", achievedAt.String, err)
		}
		g.AchievedAt = &t
	}
	return &g, nil
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

- [ ] **Step 5: Verify it all compiles**

```bash
go build ./internal/fitness/...
```

Expected: no output.

- [ ] **Step 7: Commit**

```bash
git add internal/fitness/adapters/secondary/sqlite/
git commit -m "feat(fitness): add SQLite repositories for Profile, Workout, BodySnapshot, Goal"
```

---

## Task 9: App-Level Fitness Ports

**Files:**
- Create: `internal/app/fitness_ports.go`

- [ ] **Step 1: Create fitness_ports.go**

```go
// internal/app/fitness_ports.go
package app

import (
	"context"
	"time"

	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
	fitnesssvc "github.com/rafaelsoares/alfredo/internal/fitness/service"
)

// FitnessProfileServicer is the narrow interface consumed by FitnessProfileUseCase.
type FitnessProfileServicer interface {
	Create(ctx context.Context, in fitnesssvc.CreateProfileInput) (*domain.Profile, error)
	Get(ctx context.Context) (*domain.Profile, error)
	Update(ctx context.Context, in fitnesssvc.UpdateProfileInput) (*domain.Profile, error)
}

// FitnessWorkoutServicer is the narrow interface consumed by FitnessIngestionUseCase.
type FitnessWorkoutServicer interface {
	Create(ctx context.Context, in fitnesssvc.CreateWorkoutInput) (*domain.Workout, error)
	GetByID(ctx context.Context, id string) (*domain.Workout, error)
	List(ctx context.Context, from, to *time.Time) ([]domain.Workout, error)
	Delete(ctx context.Context, id string) error
}

// FitnessBodySnapshotServicer is the narrow interface consumed by FitnessBodyUseCase.
type FitnessBodySnapshotServicer interface {
	Create(ctx context.Context, in fitnesssvc.CreateBodySnapshotInput) (*domain.BodySnapshot, error)
	GetByID(ctx context.Context, id string) (*domain.BodySnapshot, error)
	List(ctx context.Context, from, to *time.Time) ([]domain.BodySnapshot, error)
	Delete(ctx context.Context, id string) error
}

// FitnessGoalServicer is the narrow interface consumed by FitnessGoalUseCase.
type FitnessGoalServicer interface {
	Create(ctx context.Context, in fitnesssvc.CreateGoalInput) (*domain.Goal, error)
	GetByID(ctx context.Context, id string) (*domain.Goal, error)
	List(ctx context.Context) ([]domain.Goal, error)
	Update(ctx context.Context, id string, in fitnesssvc.UpdateGoalInput) (*domain.Goal, error)
	Delete(ctx context.Context, id string) error
	Achieve(ctx context.Context, id string) (*domain.Goal, error)
}
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./internal/app/...
```

Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add internal/app/fitness_ports.go
git commit -m "feat(fitness): add app-level fitness port interfaces"
```

---

## Task 10: FitnessIngestionUseCase

**Files:**
- Create: `internal/app/fitness_ingestion_usecase.go`
- Create: `internal/app/fitness_ingestion_usecase_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/app/fitness_ingestion_usecase_test.go
package app_test

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/rafaelsoares/alfredo/internal/app"
	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
	fitnesssvc "github.com/rafaelsoares/alfredo/internal/fitness/service"
)

type stubFitnessWorkoutService struct {
	workout *domain.Workout
}

func (s *stubFitnessWorkoutService) Create(_ context.Context, in fitnesssvc.CreateWorkoutInput) (*domain.Workout, error) {
	if s.workout != nil {
		return s.workout, nil
	}
	return &domain.Workout{ID: "w1", ExternalID: in.ExternalID, Type: in.Type, Source: in.Source}, nil
}
func (s *stubFitnessWorkoutService) GetByID(_ context.Context, _ string) (*domain.Workout, error) {
	return s.workout, nil
}
func (s *stubFitnessWorkoutService) List(_ context.Context, _, _ *time.Time) ([]domain.Workout, error) {
	return nil, nil
}
func (s *stubFitnessWorkoutService) Delete(_ context.Context, _ string) error { return nil }

func TestFitnessIngestionUseCase_IngestWorkout_EmitsEvent(t *testing.T) {
	spy := &spyEmitter{}
	uc := app.NewFitnessIngestionUseCase(&stubFitnessWorkoutService{}, spy, zap.NewNop())

	_, err := uc.IngestWorkout(context.Background(), domain.Workout{
		ExternalID: "ext-1", Type: "run", Source: "apple_fitness",
		StartedAt: time.Now(), DurationSeconds: 3600, ActiveCalories: 400, TotalCalories: 500,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(spy.events) != 1 || spy.events[0] != "fitness.workout.saved" {
		t.Errorf("events = %v, want [fitness.workout.saved]", spy.events)
	}
}

func TestFitnessIngestionUseCase_IngestWorkoutBatch_EmitsOneEventPerWorkout(t *testing.T) {
	spy := &spyEmitter{}
	uc := app.NewFitnessIngestionUseCase(&stubFitnessWorkoutService{}, spy, zap.NewNop())

	workouts := []domain.Workout{
		{ExternalID: "ext-1", Type: "run", Source: "apple_fitness", StartedAt: time.Now(), DurationSeconds: 1, ActiveCalories: 1, TotalCalories: 1},
		{ExternalID: "ext-2", Type: "cycle", Source: "apple_fitness", StartedAt: time.Now(), DurationSeconds: 1, ActiveCalories: 1, TotalCalories: 1},
	}
	_, err := uc.IngestWorkoutBatch(context.Background(), workouts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(spy.events) != 2 {
		t.Errorf("got %d events, want 2", len(spy.events))
	}
	for _, e := range spy.events {
		if e != "fitness.workout.saved" {
			t.Errorf("unexpected event %q", e)
		}
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./internal/app/... -run TestFitnessIngestion -v 2>&1 | head -5
```

Expected: compilation error — `NewFitnessIngestionUseCase` not defined.

- [ ] **Step 3: Implement FitnessIngestionUseCase**

```go
// internal/app/fitness_ingestion_usecase.go
package app

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
	fitnesssvc "github.com/rafaelsoares/alfredo/internal/fitness/service"
	"github.com/rafaelsoares/alfredo/internal/webhook"
)

// FitnessIngestionUseCase saves workouts and emits fitness.workout.saved.
// It also satisfies fitness/port.WorkoutIngester so external adapters (future Apple Fitness
// spike) can call IngestWorkout directly without going through HTTP.
type FitnessIngestionUseCase struct {
	workouts FitnessWorkoutServicer
	emitter  webhook.EventEmitter
	logger   *zap.Logger
}

func NewFitnessIngestionUseCase(
	workouts FitnessWorkoutServicer,
	emitter webhook.EventEmitter,
	logger *zap.Logger,
) *FitnessIngestionUseCase {
	return &FitnessIngestionUseCase{workouts: workouts, emitter: emitter, logger: logger}
}

func (uc *FitnessIngestionUseCase) IngestWorkout(ctx context.Context, w domain.Workout) (*domain.Workout, error) {
	saved, err := uc.workouts.Create(ctx, fitnesssvc.CreateWorkoutInput{
		ExternalID:      w.ExternalID,
		Type:            w.Type,
		StartedAt:       w.StartedAt,
		DurationSeconds: w.DurationSeconds,
		ActiveCalories:  w.ActiveCalories,
		TotalCalories:   w.TotalCalories,
		DistanceMeters:  w.DistanceMeters,
		AvgPaceSecPerKm: w.AvgPaceSecPerKm,
		AvgHeartRate:    w.AvgHeartRate,
		MaxHeartRate:    w.MaxHeartRate,
		HRZone1Pct:      w.HRZone1Pct,
		HRZone2Pct:      w.HRZone2Pct,
		HRZone3Pct:      w.HRZone3Pct,
		HRZone4Pct:      w.HRZone4Pct,
		HRZone5Pct:      w.HRZone5Pct,
		Source:          w.Source,
	})
	if err != nil {
		return nil, err
	}
	uc.emitter.Emit(ctx, "fitness.workout.saved", fitnessWorkoutSavedPayload{
		WorkoutID:       saved.ID,
		Type:            saved.Type,
		StartedAt:       saved.StartedAt,
		DurationSeconds: saved.DurationSeconds,
		ActiveCalories:  saved.ActiveCalories,
		Source:          saved.Source,
	})
	return saved, nil
}

func (uc *FitnessIngestionUseCase) IngestWorkoutBatch(ctx context.Context, ws []domain.Workout) ([]domain.Workout, error) {
	var saved []domain.Workout
	for _, w := range ws {
		s, err := uc.IngestWorkout(ctx, w)
		if err != nil {
			return nil, err
		}
		saved = append(saved, *s)
	}
	return saved, nil
}

func (uc *FitnessIngestionUseCase) GetByID(ctx context.Context, id string) (*domain.Workout, error) {
	return uc.workouts.GetByID(ctx, id)
}

func (uc *FitnessIngestionUseCase) List(ctx context.Context, from, to *time.Time) ([]domain.Workout, error) {
	return uc.workouts.List(ctx, from, to)
}

func (uc *FitnessIngestionUseCase) Delete(ctx context.Context, id string) error {
	return uc.workouts.Delete(ctx, id)
}

// --- payload types ---

type fitnessWorkoutSavedPayload struct {
	WorkoutID       string    `json:"workout_id"`
	Type            string    `json:"type"`
	StartedAt       time.Time `json:"started_at"`
	DurationSeconds int       `json:"duration_seconds"`
	ActiveCalories  float64   `json:"active_calories"`
	Source          string    `json:"source"`
}
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
go test ./internal/app/... -run TestFitnessIngestion -v
```

Expected: both test cases PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/fitness_ingestion_usecase.go internal/app/fitness_ingestion_usecase_test.go
git commit -m "feat(fitness): add FitnessIngestionUseCase — saves workouts and emits fitness.workout.saved"
```

---

## Task 11: FitnessBodyUseCase

**Files:**
- Create: `internal/app/fitness_body_usecase.go`
- Create: `internal/app/fitness_body_usecase_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/app/fitness_body_usecase_test.go
package app_test

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/rafaelsoares/alfredo/internal/app"
	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
	fitnesssvc "github.com/rafaelsoares/alfredo/internal/fitness/service"
)

type stubFitnessBodySnapshotService struct {
	snapshot *domain.BodySnapshot
}

func (s *stubFitnessBodySnapshotService) Create(_ context.Context, in fitnesssvc.CreateBodySnapshotInput) (*domain.BodySnapshot, error) {
	return &domain.BodySnapshot{ID: "bs1", Date: in.Date, WeightKg: in.WeightKg}, nil
}
func (s *stubFitnessBodySnapshotService) GetByID(_ context.Context, _ string) (*domain.BodySnapshot, error) {
	return s.snapshot, nil
}
func (s *stubFitnessBodySnapshotService) List(_ context.Context, _, _ *time.Time) ([]domain.BodySnapshot, error) {
	return nil, nil
}
func (s *stubFitnessBodySnapshotService) Delete(_ context.Context, _ string) error { return nil }

func TestFitnessBodyUseCase_CreateSnapshot_EmitsEvent(t *testing.T) {
	spy := &spyEmitter{}
	uc := app.NewFitnessBodyUseCase(&stubFitnessBodySnapshotService{}, spy, zap.NewNop())

	w := 75.0
	_, err := uc.CreateSnapshot(context.Background(), fitnesssvc.CreateBodySnapshotInput{
		Date: time.Now(), WeightKg: &w,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(spy.events) != 1 || spy.events[0] != "fitness.body_snapshot.saved" {
		t.Errorf("events = %v, want [fitness.body_snapshot.saved]", spy.events)
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./internal/app/... -run TestFitnessBody -v 2>&1 | head -5
```

Expected: compilation error — `NewFitnessBodyUseCase` not defined.

- [ ] **Step 3: Implement FitnessBodyUseCase**

```go
// internal/app/fitness_body_usecase.go
package app

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
	fitnesssvc "github.com/rafaelsoares/alfredo/internal/fitness/service"
	"github.com/rafaelsoares/alfredo/internal/webhook"
)

type FitnessBodyUseCase struct {
	snapshots FitnessBodySnapshotServicer
	emitter   webhook.EventEmitter
	logger    *zap.Logger
}

func NewFitnessBodyUseCase(
	snapshots FitnessBodySnapshotServicer,
	emitter webhook.EventEmitter,
	logger *zap.Logger,
) *FitnessBodyUseCase {
	return &FitnessBodyUseCase{snapshots: snapshots, emitter: emitter, logger: logger}
}

func (uc *FitnessBodyUseCase) CreateSnapshot(ctx context.Context, in fitnesssvc.CreateBodySnapshotInput) (*domain.BodySnapshot, error) {
	s, err := uc.snapshots.Create(ctx, in)
	if err != nil {
		return nil, err
	}
	uc.emitter.Emit(ctx, "fitness.body_snapshot.saved", fitnessBodySnapshotSavedPayload{
		SnapshotID: s.ID,
		Date:       s.Date.Format("2006-01-02"),
		WeightKg:   s.WeightKg,
		BodyFatPct: s.BodyFatPct,
	})
	return s, nil
}

func (uc *FitnessBodyUseCase) GetSnapshot(ctx context.Context, id string) (*domain.BodySnapshot, error) {
	return uc.snapshots.GetByID(ctx, id)
}

func (uc *FitnessBodyUseCase) ListSnapshots(ctx context.Context, from, to *time.Time) ([]domain.BodySnapshot, error) {
	return uc.snapshots.List(ctx, from, to)
}

func (uc *FitnessBodyUseCase) DeleteSnapshot(ctx context.Context, id string) error {
	return uc.snapshots.Delete(ctx, id)
}

// --- payload types ---

type fitnessBodySnapshotSavedPayload struct {
	SnapshotID string   `json:"snapshot_id"`
	Date       string   `json:"date"`
	WeightKg   *float64 `json:"weight_kg,omitempty"`
	BodyFatPct *float64 `json:"body_fat_pct,omitempty"`
}
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
go test ./internal/app/... -run TestFitnessBody -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/fitness_body_usecase.go internal/app/fitness_body_usecase_test.go
git commit -m "feat(fitness): add FitnessBodyUseCase — creates snapshots and emits fitness.body_snapshot.saved"
```

---

## Task 12: FitnessGoalUseCase

**Files:**
- Create: `internal/app/fitness_goal_usecase.go`
- Create: `internal/app/fitness_goal_usecase_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/app/fitness_goal_usecase_test.go
package app_test

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/rafaelsoares/alfredo/internal/app"
	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
	fitnesssvc "github.com/rafaelsoares/alfredo/internal/fitness/service"
)

type stubFitnessGoalService struct {
	goal *domain.Goal
}

func (s *stubFitnessGoalService) Create(_ context.Context, in fitnesssvc.CreateGoalInput) (*domain.Goal, error) {
	return &domain.Goal{ID: "g1", Name: in.Name}, nil
}
func (s *stubFitnessGoalService) GetByID(_ context.Context, _ string) (*domain.Goal, error) {
	return s.goal, nil
}
func (s *stubFitnessGoalService) List(_ context.Context) ([]domain.Goal, error) { return nil, nil }
func (s *stubFitnessGoalService) Update(_ context.Context, _ string, _ fitnesssvc.UpdateGoalInput) (*domain.Goal, error) {
	return s.goal, nil
}
func (s *stubFitnessGoalService) Delete(_ context.Context, _ string) error { return nil }
func (s *stubFitnessGoalService) Achieve(_ context.Context, _ string) (*domain.Goal, error) {
	now := time.Now()
	g := *s.goal
	g.AchievedAt = &now
	return &g, nil
}

func TestFitnessGoalUseCase_Achieve_EmitsEvent(t *testing.T) {
	spy := &spyEmitter{}
	uc := app.NewFitnessGoalUseCase(&stubFitnessGoalService{
		goal: &domain.Goal{ID: "g1", Name: "Run a 5k"},
	}, spy, zap.NewNop())

	_, err := uc.AchieveGoal(context.Background(), "g1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(spy.events) != 1 || spy.events[0] != "fitness.goal.achieved" {
		t.Errorf("events = %v, want [fitness.goal.achieved]", spy.events)
	}
}

func TestFitnessGoalUseCase_Create_DoesNotEmit(t *testing.T) {
	spy := &spyEmitter{}
	uc := app.NewFitnessGoalUseCase(&stubFitnessGoalService{}, spy, zap.NewNop())

	_, err := uc.CreateGoal(context.Background(), fitnesssvc.CreateGoalInput{Name: "Run a 5k"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(spy.events) != 0 {
		t.Errorf("expected no events, got %v", spy.events)
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./internal/app/... -run TestFitnessGoal -v 2>&1 | head -5
```

Expected: compilation error — `NewFitnessGoalUseCase` not defined.

- [ ] **Step 3: Implement FitnessGoalUseCase**

```go
// internal/app/fitness_goal_usecase.go
package app

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
	fitnesssvc "github.com/rafaelsoares/alfredo/internal/fitness/service"
	"github.com/rafaelsoares/alfredo/internal/webhook"
)

type FitnessGoalUseCase struct {
	goals   FitnessGoalServicer
	emitter webhook.EventEmitter
	logger  *zap.Logger
}

func NewFitnessGoalUseCase(
	goals FitnessGoalServicer,
	emitter webhook.EventEmitter,
	logger *zap.Logger,
) *FitnessGoalUseCase {
	return &FitnessGoalUseCase{goals: goals, emitter: emitter, logger: logger}
}

func (uc *FitnessGoalUseCase) CreateGoal(ctx context.Context, in fitnesssvc.CreateGoalInput) (*domain.Goal, error) {
	return uc.goals.Create(ctx, in)
}

func (uc *FitnessGoalUseCase) GetGoal(ctx context.Context, id string) (*domain.Goal, error) {
	return uc.goals.GetByID(ctx, id)
}

func (uc *FitnessGoalUseCase) ListGoals(ctx context.Context) ([]domain.Goal, error) {
	return uc.goals.List(ctx)
}

func (uc *FitnessGoalUseCase) UpdateGoal(ctx context.Context, id string, in fitnesssvc.UpdateGoalInput) (*domain.Goal, error) {
	return uc.goals.Update(ctx, id, in)
}

func (uc *FitnessGoalUseCase) DeleteGoal(ctx context.Context, id string) error {
	return uc.goals.Delete(ctx, id)
}

func (uc *FitnessGoalUseCase) AchieveGoal(ctx context.Context, id string) (*domain.Goal, error) {
	g, err := uc.goals.Achieve(ctx, id)
	if err != nil {
		return nil, err
	}
	uc.emitter.Emit(ctx, "fitness.goal.achieved", fitnessGoalAchievedPayload{
		GoalID:     g.ID,
		GoalName:   g.Name,
		AchievedAt: *g.AchievedAt,
	})
	return g, nil
}

// --- payload types ---

type fitnessGoalAchievedPayload struct {
	GoalID     string    `json:"goal_id"`
	GoalName   string    `json:"goal_name"`
	AchievedAt time.Time `json:"achieved_at"`
}
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
go test ./internal/app/... -run TestFitnessGoal -v
```

Expected: both test cases PASS.

- [ ] **Step 5: Run all app tests**

```bash
go test ./internal/app/... -v
```

Expected: all tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/app/fitness_goal_usecase.go internal/app/fitness_goal_usecase_test.go
git commit -m "feat(fitness): add FitnessGoalUseCase — goal CRUD and fitness.goal.achieved event"
```

---

## Task 13: FitnessProfileUseCase

**Files:**
- Create: `internal/app/fitness_profile_usecase.go`

This is a pass-through — no webhook events, no test needed.

- [ ] **Step 1: Create fitness_profile_usecase.go**

```go
// internal/app/fitness_profile_usecase.go
package app

import (
	"context"

	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
	fitnesssvc "github.com/rafaelsoares/alfredo/internal/fitness/service"
)

// FitnessProfileUseCase is a pass-through to ProfileService.
// No webhook events are emitted for profile changes.
type FitnessProfileUseCase struct {
	profiles FitnessProfileServicer
}

func NewFitnessProfileUseCase(profiles FitnessProfileServicer) *FitnessProfileUseCase {
	return &FitnessProfileUseCase{profiles: profiles}
}

func (uc *FitnessProfileUseCase) Create(ctx context.Context, in fitnesssvc.CreateProfileInput) (*domain.Profile, error) {
	return uc.profiles.Create(ctx, in)
}

func (uc *FitnessProfileUseCase) Get(ctx context.Context) (*domain.Profile, error) {
	return uc.profiles.Get(ctx)
}

func (uc *FitnessProfileUseCase) Update(ctx context.Context, in fitnesssvc.UpdateProfileInput) (*domain.Profile, error) {
	return uc.profiles.Update(ctx, in)
}
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./internal/app/...
```

Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add internal/app/fitness_profile_usecase.go
git commit -m "feat(fitness): add FitnessProfileUseCase (pass-through, no events)"
```

---

## Task 14: HTTP Handlers

**Files:**
- Create: `internal/fitness/adapters/primary/http/helpers.go`
- Create: `internal/fitness/adapters/primary/http/profile_handler.go`
- Create: `internal/fitness/adapters/primary/http/workout_handler.go`
- Create: `internal/fitness/adapters/primary/http/body_snapshot_handler.go`
- Create: `internal/fitness/adapters/primary/http/goal_handler.go`

- [ ] **Step 1: Create helpers.go**

```go
// internal/fitness/adapters/primary/http/helpers.go
package http

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
	"github.com/rafaelsoares/alfredo/internal/logger"
)

var validate = validator.New()

type errorResponse struct {
	Error   string       `json:"error"`
	Message string       `json:"message"`
	Fields  []fieldError `json:"fields,omitempty"`
}

type fieldError struct {
	Field string `json:"field"`
	Issue string `json:"issue"`
}

func newErrorResponse(code, message string, fields []fieldError) errorResponse {
	return errorResponse{Error: code, Message: message, Fields: fields}
}

func parseUUID(c echo.Context, param string) (string, bool) {
	val := c.Param(param)
	if _, err := uuid.Parse(val); err != nil {
		_ = c.JSON(http.StatusBadRequest, newErrorResponse(
			"invalid_param", param+" must be a valid UUID", nil,
		))
		return "", false
	}
	return val, true
}

func validateRequest(c echo.Context, req any) bool {
	if err := validate.Struct(req); err != nil {
		var ve validator.ValidationErrors
		if errors.As(err, &ve) {
			fields := make([]fieldError, len(ve))
			for i, fe := range ve {
				fields[i] = fieldError{Field: strings.ToLower(fe.Field()), Issue: fe.Tag()}
			}
			_ = c.JSON(http.StatusBadRequest, newErrorResponse("validation_failed", "Request validation failed", fields))
			return false
		}
		_ = c.JSON(http.StatusBadRequest, newErrorResponse("validation_failed", err.Error(), nil))
		return false
	}
	return true
}

func mapError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		logger.SetError(c, "not_found")
		return c.JSON(http.StatusNotFound, newErrorResponse("not_found", "Resource not found", nil))
	case errors.Is(err, domain.ErrValidation):
		logger.SetError(c, "validation_failed")
		msg := strings.TrimPrefix(err.Error(), "validation failed: ")
		return c.JSON(http.StatusBadRequest, newErrorResponse("validation_failed", msg, nil))
	case errors.Is(err, domain.ErrAlreadyExists):
		logger.SetError(c, "already_exists")
		return c.JSON(http.StatusConflict, newErrorResponse("already_exists", "Resource already exists", nil))
	case errors.Is(err, domain.ErrAlreadyAchieved):
		logger.SetError(c, "already_achieved")
		return c.JSON(http.StatusConflict, newErrorResponse("already_achieved", "Goal has already been achieved", nil))
	default:
		logger.SetError(c, "internal_error")
		return c.JSON(http.StatusInternalServerError, newErrorResponse("internal_error", "An unexpected error occurred", nil))
	}
}
```

- [ ] **Step 2: Create profile_handler.go**

```go
// internal/fitness/adapters/primary/http/profile_handler.go
package http

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
	fitnesssvc "github.com/rafaelsoares/alfredo/internal/fitness/service"
	"github.com/rafaelsoares/alfredo/internal/logger"
)

type ProfileServicer interface {
	Create(ctx context.Context, in fitnesssvc.CreateProfileInput) (*domain.Profile, error)
	Get(ctx context.Context) (*domain.Profile, error)
	Update(ctx context.Context, in fitnesssvc.UpdateProfileInput) (*domain.Profile, error)
}

type ProfileHandler struct{ svc ProfileServicer }

func NewProfileHandler(svc ProfileServicer) *ProfileHandler { return &ProfileHandler{svc: svc} }

func (h *ProfileHandler) Register(g *echo.Group) {
	g.POST("/fitness/profile", h.CreateProfile)
	g.GET("/fitness/profile", h.GetProfile)
	g.PUT("/fitness/profile", h.UpdateProfile)
}

func (h *ProfileHandler) CreateProfile(c echo.Context) error {
	var req struct {
		FirstName string `json:"first_name" validate:"required,min=1,max=100"`
		LastName  string `json:"last_name"  validate:"required,min=1,max=100"`
		BirthDate string `json:"birth_date" validate:"required"`
		Gender    string `json:"gender"     validate:"required,oneof=male female other"`
		HeightCm  float64 `json:"height_cm" validate:"required,gt=0"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, newErrorResponse("invalid_request_body", "Request body is invalid JSON", nil))
	}
	if !validateRequest(c, &req) {
		return nil
	}
	birthDate, err := time.Parse("2006-01-02", req.BirthDate)
	if err != nil {
		return c.JSON(http.StatusBadRequest, newErrorResponse("validation_failed", "Request validation failed",
			[]fieldError{{Field: "birth_date", Issue: "must be YYYY-MM-DD format"}}))
	}
	p, err := h.svc.Create(c.Request().Context(), fitnesssvc.CreateProfileInput{
		FirstName: req.FirstName, LastName: req.LastName,
		BirthDate: birthDate, Gender: req.Gender, HeightCm: req.HeightCm,
	})
	if err != nil {
		return mapError(c, err)
	}
	logger.FromEcho(c).Info("fitness profile created", zap.String("profile_id", p.ID))
	return c.JSON(http.StatusCreated, toProfileResponse(*p))
}

func (h *ProfileHandler) GetProfile(c echo.Context) error {
	p, err := h.svc.Get(c.Request().Context())
	if err != nil {
		return mapError(c, err)
	}
	return c.JSON(http.StatusOK, toProfileResponse(*p))
}

func (h *ProfileHandler) UpdateProfile(c echo.Context) error {
	var req struct {
		FirstName *string  `json:"first_name" validate:"omitempty,min=1,max=100"`
		LastName  *string  `json:"last_name"  validate:"omitempty,min=1,max=100"`
		BirthDate *string  `json:"birth_date"`
		Gender    *string  `json:"gender"     validate:"omitempty,oneof=male female other"`
		HeightCm  *float64 `json:"height_cm"  validate:"omitempty,gt=0"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, newErrorResponse("invalid_request_body", "Request body is invalid JSON", nil))
	}
	if !validateRequest(c, &req) {
		return nil
	}
	in := fitnesssvc.UpdateProfileInput{
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Gender:    req.Gender,
		HeightCm:  req.HeightCm,
	}
	if req.BirthDate != nil {
		t, err := time.Parse("2006-01-02", *req.BirthDate)
		if err != nil {
			return c.JSON(http.StatusBadRequest, newErrorResponse("validation_failed", "Request validation failed",
				[]fieldError{{Field: "birth_date", Issue: "must be YYYY-MM-DD format"}}))
		}
		in.BirthDate = &t
	}
	p, err := h.svc.Update(c.Request().Context(), in)
	if err != nil {
		return mapError(c, err)
	}
	return c.JSON(http.StatusOK, toProfileResponse(*p))
}

// --- response types ---

type profileResponse struct {
	ID        string `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	BirthDate string `json:"birth_date"`
	Gender    string `json:"gender"`
	HeightCm  float64 `json:"height_cm"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func toProfileResponse(p domain.Profile) profileResponse {
	return profileResponse{
		ID:        p.ID,
		FirstName: p.FirstName,
		LastName:  p.LastName,
		BirthDate: p.BirthDate.Format("2006-01-02"),
		Gender:    p.Gender,
		HeightCm:  p.HeightCm,
		CreatedAt: p.CreatedAt.Format(time.RFC3339),
		UpdatedAt: p.UpdatedAt.Format(time.RFC3339),
	}
}
```

- [ ] **Step 3: Create workout_handler.go**

```go
// internal/fitness/adapters/primary/http/workout_handler.go
package http

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
	"github.com/rafaelsoares/alfredo/internal/logger"
)

type WorkoutServicer interface {
	IngestWorkout(ctx context.Context, w domain.Workout) (*domain.Workout, error)
	IngestWorkoutBatch(ctx context.Context, ws []domain.Workout) ([]domain.Workout, error)
	GetByID(ctx context.Context, id string) (*domain.Workout, error)
	List(ctx context.Context, from, to *time.Time) ([]domain.Workout, error)
	Delete(ctx context.Context, id string) error
}

type WorkoutHandler struct{ svc WorkoutServicer }

func NewWorkoutHandler(svc WorkoutServicer) *WorkoutHandler { return &WorkoutHandler{svc: svc} }

func (h *WorkoutHandler) Register(g *echo.Group) {
	g.POST("/fitness/workouts", h.IngestWorkout)
	g.POST("/fitness/workouts/batch", h.IngestWorkoutBatch)
	g.GET("/fitness/workouts", h.ListWorkouts)
	g.GET("/fitness/workouts/:id", h.GetWorkout)
	g.DELETE("/fitness/workouts/:id", h.DeleteWorkout)
}

type workoutRequest struct {
	ExternalID      string   `json:"external_id"       validate:"required"`
	Type            string   `json:"type"              validate:"required,min=1,max=50"`
	StartedAt       string   `json:"started_at"        validate:"required"`
	DurationSeconds int      `json:"duration_seconds"  validate:"required,gt=0"`
	ActiveCalories  float64  `json:"active_calories"   validate:"gte=0"`
	TotalCalories   float64  `json:"total_calories"    validate:"gte=0"`
	DistanceMeters  *float64 `json:"distance_meters"`
	AvgPaceSecPerKm *float64 `json:"avg_pace_sec_per_km"`
	AvgHeartRate    *float64 `json:"avg_heart_rate"`
	MaxHeartRate    *float64 `json:"max_heart_rate"`
	HRZone1Pct      *float64 `json:"hr_zone1_pct"`
	HRZone2Pct      *float64 `json:"hr_zone2_pct"`
	HRZone3Pct      *float64 `json:"hr_zone3_pct"`
	HRZone4Pct      *float64 `json:"hr_zone4_pct"`
	HRZone5Pct      *float64 `json:"hr_zone5_pct"`
	Source          string   `json:"source"            validate:"required,min=1,max=50"`
}

func parseWorkoutRequest(req workoutRequest) (domain.Workout, error) {
	startedAt, err := time.Parse(time.RFC3339, req.StartedAt)
	if err != nil {
		return domain.Workout{}, err
	}
	return domain.Workout{
		ExternalID:      req.ExternalID,
		Type:            req.Type,
		StartedAt:       startedAt,
		DurationSeconds: req.DurationSeconds,
		ActiveCalories:  req.ActiveCalories,
		TotalCalories:   req.TotalCalories,
		DistanceMeters:  req.DistanceMeters,
		AvgPaceSecPerKm: req.AvgPaceSecPerKm,
		AvgHeartRate:    req.AvgHeartRate,
		MaxHeartRate:    req.MaxHeartRate,
		HRZone1Pct:      req.HRZone1Pct,
		HRZone2Pct:      req.HRZone2Pct,
		HRZone3Pct:      req.HRZone3Pct,
		HRZone4Pct:      req.HRZone4Pct,
		HRZone5Pct:      req.HRZone5Pct,
		Source:          req.Source,
	}, nil
}

func (h *WorkoutHandler) IngestWorkout(c echo.Context) error {
	var req workoutRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, newErrorResponse("invalid_request_body", "Request body is invalid JSON", nil))
	}
	if !validateRequest(c, &req) {
		return nil
	}
	w, err := parseWorkoutRequest(req)
	if err != nil {
		return c.JSON(http.StatusBadRequest, newErrorResponse("validation_failed", "Request validation failed",
			[]fieldError{{Field: "started_at", Issue: "must be RFC3339 format"}}))
	}
	saved, err := h.svc.IngestWorkout(c.Request().Context(), w)
	if err != nil {
		return mapError(c, err)
	}
	logger.FromEcho(c).Info("workout ingested", zap.String("workout_id", saved.ID))
	return c.JSON(http.StatusCreated, toWorkoutResponse(*saved))
}

func (h *WorkoutHandler) IngestWorkoutBatch(c echo.Context) error {
	var reqs []workoutRequest
	if err := c.Bind(&reqs); err != nil {
		return c.JSON(http.StatusBadRequest, newErrorResponse("invalid_request_body", "Request body is invalid JSON", nil))
	}
	var workouts []domain.Workout
	for _, req := range reqs {
		if err := validate.Struct(req); err != nil {
			return c.JSON(http.StatusBadRequest, newErrorResponse("validation_failed", "One or more workouts failed validation", nil))
		}
		w, err := parseWorkoutRequest(req)
		if err != nil {
			return c.JSON(http.StatusBadRequest, newErrorResponse("validation_failed", "Request validation failed",
				[]fieldError{{Field: "started_at", Issue: "must be RFC3339 format"}}))
		}
		workouts = append(workouts, w)
	}
	saved, err := h.svc.IngestWorkoutBatch(c.Request().Context(), workouts)
	if err != nil {
		return mapError(c, err)
	}
	logger.FromEcho(c).Info("workout batch ingested", zap.Int("count", len(saved)))
	resp := make([]workoutResponse, 0, len(saved))
	for _, w := range saved {
		resp = append(resp, toWorkoutResponse(w))
	}
	return c.JSON(http.StatusCreated, resp)
}

func (h *WorkoutHandler) ListWorkouts(c echo.Context) error {
	from, to := parseDateRangeParams(c)
	ws, err := h.svc.List(c.Request().Context(), from, to)
	if err != nil {
		return mapError(c, err)
	}
	resp := make([]workoutResponse, 0, len(ws))
	for _, w := range ws {
		resp = append(resp, toWorkoutResponse(w))
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *WorkoutHandler) GetWorkout(c echo.Context) error {
	id, ok := parseUUID(c, "id")
	if !ok {
		return nil
	}
	w, err := h.svc.GetByID(c.Request().Context(), id)
	if err != nil {
		return mapError(c, err)
	}
	return c.JSON(http.StatusOK, toWorkoutResponse(*w))
}

func (h *WorkoutHandler) DeleteWorkout(c echo.Context) error {
	id, ok := parseUUID(c, "id")
	if !ok {
		return nil
	}
	if err := h.svc.Delete(c.Request().Context(), id); err != nil {
		return mapError(c, err)
	}
	logger.FromEcho(c).Info("workout deleted", zap.String("workout_id", id))
	return c.NoContent(http.StatusNoContent)
}

// parseDateRangeParams reads optional ?from= and ?to= query params as RFC3339 strings.
func parseDateRangeParams(c echo.Context) (*time.Time, *time.Time) {
	var from, to *time.Time
	if s := c.QueryParam("from"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			from = &t
		}
	}
	if s := c.QueryParam("to"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			to = &t
		}
	}
	return from, to
}

// --- response types ---

type workoutResponse struct {
	ID              string   `json:"id"`
	ExternalID      string   `json:"external_id"`
	Type            string   `json:"type"`
	StartedAt       string   `json:"started_at"`
	DurationSeconds int      `json:"duration_seconds"`
	ActiveCalories  float64  `json:"active_calories"`
	TotalCalories   float64  `json:"total_calories"`
	DistanceMeters  *float64 `json:"distance_meters,omitempty"`
	AvgPaceSecPerKm *float64 `json:"avg_pace_sec_per_km,omitempty"`
	AvgHeartRate    *float64 `json:"avg_heart_rate,omitempty"`
	MaxHeartRate    *float64 `json:"max_heart_rate,omitempty"`
	HRZone1Pct      *float64 `json:"hr_zone1_pct,omitempty"`
	HRZone2Pct      *float64 `json:"hr_zone2_pct,omitempty"`
	HRZone3Pct      *float64 `json:"hr_zone3_pct,omitempty"`
	HRZone4Pct      *float64 `json:"hr_zone4_pct,omitempty"`
	HRZone5Pct      *float64 `json:"hr_zone5_pct,omitempty"`
	Source          string   `json:"source"`
	CreatedAt       string   `json:"created_at"`
}

func toWorkoutResponse(w domain.Workout) workoutResponse {
	return workoutResponse{
		ID:              w.ID,
		ExternalID:      w.ExternalID,
		Type:            w.Type,
		StartedAt:       w.StartedAt.Format(time.RFC3339),
		DurationSeconds: w.DurationSeconds,
		ActiveCalories:  w.ActiveCalories,
		TotalCalories:   w.TotalCalories,
		DistanceMeters:  w.DistanceMeters,
		AvgPaceSecPerKm: w.AvgPaceSecPerKm,
		AvgHeartRate:    w.AvgHeartRate,
		MaxHeartRate:    w.MaxHeartRate,
		HRZone1Pct:      w.HRZone1Pct,
		HRZone2Pct:      w.HRZone2Pct,
		HRZone3Pct:      w.HRZone3Pct,
		HRZone4Pct:      w.HRZone4Pct,
		HRZone5Pct:      w.HRZone5Pct,
		Source:          w.Source,
		CreatedAt:       w.CreatedAt.Format(time.RFC3339),
	}
}
```

- [ ] **Step 4: Create body_snapshot_handler.go**

```go
// internal/fitness/adapters/primary/http/body_snapshot_handler.go
package http

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
	fitnesssvc "github.com/rafaelsoares/alfredo/internal/fitness/service"
	"github.com/rafaelsoares/alfredo/internal/logger"
)

type BodySnapshotServicer interface {
	CreateSnapshot(ctx context.Context, in fitnesssvc.CreateBodySnapshotInput) (*domain.BodySnapshot, error)
	GetSnapshot(ctx context.Context, id string) (*domain.BodySnapshot, error)
	ListSnapshots(ctx context.Context, from, to *time.Time) ([]domain.BodySnapshot, error)
	DeleteSnapshot(ctx context.Context, id string) error
}

type BodySnapshotHandler struct{ svc BodySnapshotServicer }

func NewBodySnapshotHandler(svc BodySnapshotServicer) *BodySnapshotHandler {
	return &BodySnapshotHandler{svc: svc}
}

func (h *BodySnapshotHandler) Register(g *echo.Group) {
	g.POST("/fitness/body-snapshots", h.CreateSnapshot)
	g.GET("/fitness/body-snapshots", h.ListSnapshots)
	g.GET("/fitness/body-snapshots/:id", h.GetSnapshot)
	g.DELETE("/fitness/body-snapshots/:id", h.DeleteSnapshot)
}

func (h *BodySnapshotHandler) CreateSnapshot(c echo.Context) error {
	var req struct {
		Date       string   `json:"date"         validate:"required"`
		WeightKg   *float64 `json:"weight_kg"    validate:"omitempty,gt=0"`
		WaistCm    *float64 `json:"waist_cm"     validate:"omitempty,gt=0"`
		HipCm      *float64 `json:"hip_cm"       validate:"omitempty,gt=0"`
		NeckCm     *float64 `json:"neck_cm"      validate:"omitempty,gt=0"`
		BodyFatPct *float64 `json:"body_fat_pct" validate:"omitempty,gt=0,lte=100"`
		PhotoPath  *string  `json:"photo_path"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, newErrorResponse("invalid_request_body", "Request body is invalid JSON", nil))
	}
	if !validateRequest(c, &req) {
		return nil
	}
	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		return c.JSON(http.StatusBadRequest, newErrorResponse("validation_failed", "Request validation failed",
			[]fieldError{{Field: "date", Issue: "must be YYYY-MM-DD format"}}))
	}
	s, err := h.svc.CreateSnapshot(c.Request().Context(), fitnesssvc.CreateBodySnapshotInput{
		Date: date, WeightKg: req.WeightKg, WaistCm: req.WaistCm,
		HipCm: req.HipCm, NeckCm: req.NeckCm, BodyFatPct: req.BodyFatPct, PhotoPath: req.PhotoPath,
	})
	if err != nil {
		return mapError(c, err)
	}
	logger.FromEcho(c).Info("body snapshot created", zap.String("snapshot_id", s.ID))
	return c.JSON(http.StatusCreated, toBodySnapshotResponse(*s))
}

func (h *BodySnapshotHandler) ListSnapshots(c echo.Context) error {
	from, to := parseDateRangeParams(c)
	snapshots, err := h.svc.ListSnapshots(c.Request().Context(), from, to)
	if err != nil {
		return mapError(c, err)
	}
	resp := make([]bodySnapshotResponse, 0, len(snapshots))
	for _, s := range snapshots {
		resp = append(resp, toBodySnapshotResponse(s))
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *BodySnapshotHandler) GetSnapshot(c echo.Context) error {
	id, ok := parseUUID(c, "id")
	if !ok {
		return nil
	}
	s, err := h.svc.GetSnapshot(c.Request().Context(), id)
	if err != nil {
		return mapError(c, err)
	}
	return c.JSON(http.StatusOK, toBodySnapshotResponse(*s))
}

func (h *BodySnapshotHandler) DeleteSnapshot(c echo.Context) error {
	id, ok := parseUUID(c, "id")
	if !ok {
		return nil
	}
	if err := h.svc.DeleteSnapshot(c.Request().Context(), id); err != nil {
		return mapError(c, err)
	}
	logger.FromEcho(c).Info("body snapshot deleted", zap.String("snapshot_id", id))
	return c.NoContent(http.StatusNoContent)
}

// --- response types ---

type bodySnapshotResponse struct {
	ID         string   `json:"id"`
	Date       string   `json:"date"`
	WeightKg   *float64 `json:"weight_kg,omitempty"`
	WaistCm    *float64 `json:"waist_cm,omitempty"`
	HipCm      *float64 `json:"hip_cm,omitempty"`
	NeckCm     *float64 `json:"neck_cm,omitempty"`
	BodyFatPct *float64 `json:"body_fat_pct,omitempty"`
	PhotoPath  *string  `json:"photo_path,omitempty"`
	CreatedAt  string   `json:"created_at"`
}

func toBodySnapshotResponse(s domain.BodySnapshot) bodySnapshotResponse {
	return bodySnapshotResponse{
		ID:         s.ID,
		Date:       s.Date.Format("2006-01-02"),
		WeightKg:   s.WeightKg,
		WaistCm:    s.WaistCm,
		HipCm:      s.HipCm,
		NeckCm:     s.NeckCm,
		BodyFatPct: s.BodyFatPct,
		PhotoPath:  s.PhotoPath,
		CreatedAt:  s.CreatedAt.Format(time.RFC3339),
	}
}
```

- [ ] **Step 5: Create goal_handler.go**

```go
// internal/fitness/adapters/primary/http/goal_handler.go
package http

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/rafaelsoares/alfredo/internal/fitness/domain"
	fitnesssvc "github.com/rafaelsoares/alfredo/internal/fitness/service"
	"github.com/rafaelsoares/alfredo/internal/logger"
)

type GoalServicer interface {
	CreateGoal(ctx context.Context, in fitnesssvc.CreateGoalInput) (*domain.Goal, error)
	GetGoal(ctx context.Context, id string) (*domain.Goal, error)
	ListGoals(ctx context.Context) ([]domain.Goal, error)
	UpdateGoal(ctx context.Context, id string, in fitnesssvc.UpdateGoalInput) (*domain.Goal, error)
	DeleteGoal(ctx context.Context, id string) error
	AchieveGoal(ctx context.Context, id string) (*domain.Goal, error)
}

type GoalHandler struct{ svc GoalServicer }

func NewGoalHandler(svc GoalServicer) *GoalHandler { return &GoalHandler{svc: svc} }

func (h *GoalHandler) Register(g *echo.Group) {
	g.POST("/fitness/goals", h.CreateGoal)
	g.GET("/fitness/goals", h.ListGoals)
	g.GET("/fitness/goals/:id", h.GetGoal)
	g.PUT("/fitness/goals/:id", h.UpdateGoal)
	g.DELETE("/fitness/goals/:id", h.DeleteGoal)
	g.POST("/fitness/goals/:id/achieve", h.AchieveGoal)
}

func (h *GoalHandler) CreateGoal(c echo.Context) error {
	var req struct {
		Name        string   `json:"name"         validate:"required,min=1,max=200"`
		Description *string  `json:"description"  validate:"omitempty,max=1000"`
		TargetValue *float64 `json:"target_value"`
		TargetUnit  *string  `json:"target_unit"  validate:"omitempty,max=50"`
		Deadline    *string  `json:"deadline"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, newErrorResponse("invalid_request_body", "Request body is invalid JSON", nil))
	}
	if !validateRequest(c, &req) {
		return nil
	}
	in := fitnesssvc.CreateGoalInput{
		Name:        req.Name,
		Description: req.Description,
		TargetValue: req.TargetValue,
		TargetUnit:  req.TargetUnit,
	}
	if req.Deadline != nil {
		t, err := time.Parse("2006-01-02", *req.Deadline)
		if err != nil {
			return c.JSON(http.StatusBadRequest, newErrorResponse("validation_failed", "Request validation failed",
				[]fieldError{{Field: "deadline", Issue: "must be YYYY-MM-DD format"}}))
		}
		in.Deadline = &t
	}
	g, err := h.svc.CreateGoal(c.Request().Context(), in)
	if err != nil {
		return mapError(c, err)
	}
	logger.FromEcho(c).Info("fitness goal created", zap.String("goal_id", g.ID))
	return c.JSON(http.StatusCreated, toGoalResponse(*g))
}

func (h *GoalHandler) ListGoals(c echo.Context) error {
	goals, err := h.svc.ListGoals(c.Request().Context())
	if err != nil {
		return mapError(c, err)
	}
	resp := make([]goalResponse, 0, len(goals))
	for _, g := range goals {
		resp = append(resp, toGoalResponse(g))
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *GoalHandler) GetGoal(c echo.Context) error {
	id, ok := parseUUID(c, "id")
	if !ok {
		return nil
	}
	g, err := h.svc.GetGoal(c.Request().Context(), id)
	if err != nil {
		return mapError(c, err)
	}
	return c.JSON(http.StatusOK, toGoalResponse(*g))
}

func (h *GoalHandler) UpdateGoal(c echo.Context) error {
	id, ok := parseUUID(c, "id")
	if !ok {
		return nil
	}
	var req struct {
		Name        *string  `json:"name"         validate:"omitempty,min=1,max=200"`
		Description *string  `json:"description"  validate:"omitempty,max=1000"`
		TargetValue *float64 `json:"target_value"`
		TargetUnit  *string  `json:"target_unit"  validate:"omitempty,max=50"`
		Deadline    *string  `json:"deadline"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, newErrorResponse("invalid_request_body", "Request body is invalid JSON", nil))
	}
	if !validateRequest(c, &req) {
		return nil
	}
	in := fitnesssvc.UpdateGoalInput{
		Name:        req.Name,
		Description: req.Description,
		TargetValue: req.TargetValue,
		TargetUnit:  req.TargetUnit,
	}
	if req.Deadline != nil {
		t, err := time.Parse("2006-01-02", *req.Deadline)
		if err != nil {
			return c.JSON(http.StatusBadRequest, newErrorResponse("validation_failed", "Request validation failed",
				[]fieldError{{Field: "deadline", Issue: "must be YYYY-MM-DD format"}}))
		}
		in.Deadline = &t
	}
	g, err := h.svc.UpdateGoal(c.Request().Context(), id, in)
	if err != nil {
		return mapError(c, err)
	}
	return c.JSON(http.StatusOK, toGoalResponse(*g))
}

func (h *GoalHandler) DeleteGoal(c echo.Context) error {
	id, ok := parseUUID(c, "id")
	if !ok {
		return nil
	}
	if err := h.svc.DeleteGoal(c.Request().Context(), id); err != nil {
		return mapError(c, err)
	}
	logger.FromEcho(c).Info("fitness goal deleted", zap.String("goal_id", id))
	return c.NoContent(http.StatusNoContent)
}

func (h *GoalHandler) AchieveGoal(c echo.Context) error {
	id, ok := parseUUID(c, "id")
	if !ok {
		return nil
	}
	g, err := h.svc.AchieveGoal(c.Request().Context(), id)
	if err != nil {
		return mapError(c, err)
	}
	logger.FromEcho(c).Info("fitness goal achieved", zap.String("goal_id", id))
	return c.JSON(http.StatusOK, toGoalResponse(*g))
}

// --- response types ---

type goalResponse struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description *string  `json:"description,omitempty"`
	TargetValue *float64 `json:"target_value,omitempty"`
	TargetUnit  *string  `json:"target_unit,omitempty"`
	Deadline    *string  `json:"deadline,omitempty"`
	AchievedAt  *string  `json:"achieved_at,omitempty"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

func toGoalResponse(g domain.Goal) goalResponse {
	r := goalResponse{
		ID:          g.ID,
		Name:        g.Name,
		Description: g.Description,
		TargetValue: g.TargetValue,
		TargetUnit:  g.TargetUnit,
		CreatedAt:   g.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   g.UpdatedAt.Format(time.RFC3339),
	}
	if g.Deadline != nil {
		s := g.Deadline.Format("2006-01-02")
		r.Deadline = &s
	}
	if g.AchievedAt != nil {
		s := g.AchievedAt.Format(time.RFC3339)
		r.AchievedAt = &s
	}
	return r
}
```

- [ ] **Step 6: Verify it all compiles**

```bash
go build ./internal/fitness/...
```

Expected: no output.

- [ ] **Step 7: Commit**

```bash
git add internal/fitness/adapters/primary/http/
git commit -m "feat(fitness): add HTTP handlers for Profile, Workout, BodySnapshot, Goal"
```

---

## Task 15: Wire main.go

**Files:**
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Add fitness imports and wiring to main.go**

In `cmd/server/main.go`, make the following additions. After the existing imports add:

```go
fitnesshttp "github.com/rafaelsoares/alfredo/internal/fitness/adapters/primary/http"
fitnesssqlite "github.com/rafaelsoares/alfredo/internal/fitness/adapters/secondary/sqlite"
fitnesssvc "github.com/rafaelsoares/alfredo/internal/fitness/service"
```

After `// 4. Open SQLite` and before `// 5. Webhook emitter`, add:

```go
// 4a. Run fitness migrations
if err := fitnesssqlite.Migrate(db); err != nil {
    zapLogger.Fatal("fitness migrate failed", zap.Error(err))
}
```

After `// 5. Webhook emitter`, add a second emitter for fitness (the domain field on the envelope differs):

```go
fitnessEmitter := webhook.New(cfg.Webhook.BaseURL, cfg.Webhook.APIKey, "fitness", zapLogger)
```

After `// 6. Pet-care repositories`, add:

```go
// 6a. Fitness repositories
fitnessProfileRepo := fitnesssqlite.NewProfileRepository(db)
fitnessWorkoutRepo := fitnesssqlite.NewWorkoutRepository(db)
fitnessBodySnapshotRepo := fitnesssqlite.NewBodySnapshotRepository(db)
fitnessGoalRepo := fitnesssqlite.NewGoalRepository(db)
```

After `// 7. Pet-care services`, add:

```go
// 7a. Fitness services (pure CRUD — no side-effects)
fitnessProfileSvc := fitnesssvc.NewProfileService(fitnessProfileRepo)
fitnessWorkoutSvc := fitnesssvc.NewWorkoutService(fitnessWorkoutRepo)
fitnessBodySnapshotSvc := fitnesssvc.NewBodySnapshotService(fitnessBodySnapshotRepo)
fitnessGoalSvc := fitnesssvc.NewGoalService(fitnessGoalRepo)
```

After `// 8. Use Cases`, add:

```go
// 8a. Fitness use cases
fitnessProfileUC := app.NewFitnessProfileUseCase(fitnessProfileSvc)
fitnessIngestionUC := app.NewFitnessIngestionUseCase(fitnessWorkoutSvc, fitnessEmitter, zapLogger)
fitnessBodyUC := app.NewFitnessBodyUseCase(fitnessBodySnapshotSvc, fitnessEmitter, zapLogger)
fitnessGoalUC := app.NewFitnessGoalUseCase(fitnessGoalSvc, fitnessEmitter, zapLogger)
```

After `// 10. HTTP handlers`, add:

```go
// 10a. Fitness HTTP handlers
fitnessProfileHandler := fitnesshttp.NewProfileHandler(fitnessProfileUC)
fitnessWorkoutHandler := fitnesshttp.NewWorkoutHandler(fitnessIngestionUC)
fitnessBodySnapshotHandler := fitnesshttp.NewBodySnapshotHandler(fitnessBodyUC)
fitnessGoalHandler := fitnesshttp.NewGoalHandler(fitnessGoalUC)
```

After `treatmentHandler.Register(protected)`, add:

```go
fitnessProfileHandler.Register(protected)
fitnessWorkoutHandler.Register(protected)
fitnessBodySnapshotHandler.Register(protected)
fitnessGoalHandler.Register(protected)
```

Also update `emitter.Wait()` at shutdown to drain the fitness emitter too:

```go
emitter.Wait()      // drain petcare webhooks
fitnessEmitter.Wait() // drain fitness webhooks
```

- [ ] **Step 2: Build to confirm wiring compiles**

```bash
go build ./cmd/server/...
```

Expected: no output.

- [ ] **Step 3: Run all tests**

```bash
make test
```

Expected: all packages PASS, no failures.

- [ ] **Step 4: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat(fitness): wire fitness module into main.go"
```

---

## Task 16: Update CLAUDE.md Routes Table

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Add fitness routes to the Routes table in CLAUDE.md**

Find the Routes table and append these rows:

```markdown
| `GET /api/v1/fitness/profile` | FitnessProfileHandler |
| `POST /api/v1/fitness/profile` | FitnessProfileHandler |
| `PUT /api/v1/fitness/profile` | FitnessProfileHandler |
| `POST /api/v1/fitness/workouts` | FitnessWorkoutHandler |
| `POST /api/v1/fitness/workouts/batch` | FitnessWorkoutHandler |
| `GET /api/v1/fitness/workouts` | FitnessWorkoutHandler |
| `GET /api/v1/fitness/workouts/:id` | FitnessWorkoutHandler |
| `DELETE /api/v1/fitness/workouts/:id` | FitnessWorkoutHandler |
| `POST /api/v1/fitness/body-snapshots` | FitnessBodySnapshotHandler |
| `GET /api/v1/fitness/body-snapshots` | FitnessBodySnapshotHandler |
| `GET /api/v1/fitness/body-snapshots/:id` | FitnessBodySnapshotHandler |
| `DELETE /api/v1/fitness/body-snapshots/:id` | FitnessBodySnapshotHandler |
| `POST /api/v1/fitness/goals` | FitnessGoalHandler |
| `GET /api/v1/fitness/goals` | FitnessGoalHandler |
| `GET /api/v1/fitness/goals/:id` | FitnessGoalHandler |
| `PUT /api/v1/fitness/goals/:id` | FitnessGoalHandler |
| `DELETE /api/v1/fitness/goals/:id` | FitnessGoalHandler |
| `POST /api/v1/fitness/goals/:id/achieve` | FitnessGoalHandler |
```

Also update the architecture diagram in CLAUDE.md to include `fitness/` alongside `petcare/`.

- [ ] **Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: add fitness module routes to CLAUDE.md"
```

---

## Final Verification

- [ ] **Run full test suite**

```bash
make test
```

Expected: all packages PASS.

- [ ] **Build the binary**

```bash
make build
```

Expected: `./alfredo` binary produced with no errors.

- [ ] **Smoke test** (requires APP_AUTH_API_KEY in .env)

```bash
make run
curl -s -X POST http://localhost:8080/api/v1/fitness/profile \
  -H "X-Api-Key: $APP_AUTH_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"first_name":"Rafael","last_name":"Soares","birth_date":"1990-01-01","gender":"male","height_cm":180}' | jq .
make stop
```

Expected: JSON response with `"id"` field set.
