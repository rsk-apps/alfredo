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
		FirstName string  `json:"first_name" validate:"required,min=1,max=100"`
		LastName  string  `json:"last_name"  validate:"required,min=1,max=100"`
		BirthDate string  `json:"birth_date" validate:"required"`
		Gender    string  `json:"gender"     validate:"required,oneof=male female other"`
		HeightCm  float64 `json:"height_cm"  validate:"required,gt=0"`
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
	ID        string  `json:"id"`
	FirstName string  `json:"first_name"`
	LastName  string  `json:"last_name"`
	BirthDate string  `json:"birth_date"`
	Gender    string  `json:"gender"`
	HeightCm  float64 `json:"height_cm"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
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
