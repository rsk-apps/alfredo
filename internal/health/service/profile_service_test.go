package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rafaelsoares/alfredo/internal/health/domain"
)

var errSQLiteDown = errors.New("sqlite down")

type profileRepoStub struct {
	getFn    func(context.Context) (domain.HealthProfile, error)
	upsertFn func(context.Context, domain.HealthProfile) (domain.HealthProfile, error)
}

func (r *profileRepoStub) Get(ctx context.Context) (domain.HealthProfile, error) {
	if r.getFn != nil {
		return r.getFn(ctx)
	}
	return domain.HealthProfile{}, nil
}

func (r *profileRepoStub) Upsert(ctx context.Context, profile domain.HealthProfile) (domain.HealthProfile, error) {
	if r.upsertFn != nil {
		return r.upsertFn(ctx, profile)
	}
	return profile, nil
}

func TestProfileServiceGetPropagatesNotFound(t *testing.T) {
	svc := NewProfileService(&profileRepoStub{
		getFn: func(context.Context) (domain.HealthProfile, error) {
			return domain.HealthProfile{}, domain.ErrNotFound
		},
	})

	_, err := svc.Get(context.Background())
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Get error = %v, want ErrNotFound", err)
	}
}

func TestProfileServiceGetReturnsRepositoryValue(t *testing.T) {
	want := domain.HealthProfile{
		HeightCM:  178,
		BirthDate: "1993-06-15",
		Sex:       "male",
	}
	svc := NewProfileService(&profileRepoStub{
		getFn: func(context.Context) (domain.HealthProfile, error) {
			return want, nil
		},
	})

	got, err := svc.Get(context.Background())
	if err != nil {
		t.Fatalf("Get error = %v", err)
	}
	if got != want {
		t.Fatalf("Get profile = %#v, want %#v", got, want)
	}
}

func TestProfileServiceUpsertDelegatesAndWrapsRepositoryErrors(t *testing.T) {
	want := domain.HealthProfile{
		HeightCM:  178,
		BirthDate: "1993-06-15",
		Sex:       "male",
		CreatedAt: time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC),
	}
	svc := NewProfileService(&profileRepoStub{
		upsertFn: func(_ context.Context, profile domain.HealthProfile) (domain.HealthProfile, error) {
			if profile.HeightCM != want.HeightCM || profile.BirthDate != want.BirthDate || profile.Sex != want.Sex {
				t.Fatalf("Upsert profile = %#v, want %#v", profile, want)
			}
			return want, nil
		},
	})

	got, err := svc.Upsert(context.Background(), want)
	if err != nil {
		t.Fatalf("Upsert error = %v", err)
	}
	if got != want {
		t.Fatalf("Upsert profile = %#v, want %#v", got, want)
	}
}

func TestProfileServiceUpsertWrapsRepositoryErrors(t *testing.T) {
	want := domain.HealthProfile{HeightCM: 178, BirthDate: "1993-06-15", Sex: "male"}
	svc := NewProfileService(&profileRepoStub{
		upsertFn: func(_ context.Context, profile domain.HealthProfile) (domain.HealthProfile, error) {
			if profile.HeightCM != want.HeightCM || profile.BirthDate != want.BirthDate || profile.Sex != want.Sex {
				t.Fatalf("Upsert profile = %#v, want %#v", profile, want)
			}
			return domain.HealthProfile{}, errSQLiteDown
		},
	})

	_, err := svc.Upsert(context.Background(), want)
	if err == nil {
		t.Fatal("Upsert error = nil, want wrapped repository error")
	}
	if !errors.Is(err, errSQLiteDown) {
		t.Fatalf("Upsert error = %v, want wrapped repository error", err)
	}
}
