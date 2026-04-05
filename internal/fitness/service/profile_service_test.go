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
	negativeHeight := -1.0
	_, err := svc.Update(context.Background(), service.UpdateProfileInput{HeightCm: &negativeHeight})
	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("got %v, want ErrValidation", err)
	}
}
