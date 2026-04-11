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
		ChestCm    *float64 `json:"chest_cm"     validate:"omitempty,gt=0"`
		BicepsCm   *float64 `json:"biceps_cm"    validate:"omitempty,gt=0"`
		TricepsCm  *float64 `json:"triceps_cm"   validate:"omitempty,gt=0"`
		BodyFatPct *float64 `json:"body_fat_pct" validate:"omitempty,gt=0,lte=100"`
		// Pollock 7-site skinfold measurements (mm)
		ChestSkinfoldMm       *float64 `json:"chest_skinfold_mm"       validate:"omitempty,gt=0"`
		MidaxillarySkinfoldMm *float64 `json:"midaxillary_skinfold_mm" validate:"omitempty,gt=0"`
		TricepsSkinfoldMm     *float64 `json:"triceps_skinfold_mm"     validate:"omitempty,gt=0"`
		SubscapularSkinfoldMm *float64 `json:"subscapular_skinfold_mm" validate:"omitempty,gt=0"`
		AbdominalSkinfoldMm   *float64 `json:"abdominal_skinfold_mm"   validate:"omitempty,gt=0"`
		SuprailiacSkinfoldMm  *float64 `json:"suprailiac_skinfold_mm"  validate:"omitempty,gt=0"`
		ThighSkinfoldMm       *float64 `json:"thigh_skinfold_mm"       validate:"omitempty,gt=0"`
		PhotoPath             *string  `json:"photo_path"`
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
		Date:                  date,
		WeightKg:              req.WeightKg,
		WaistCm:               req.WaistCm,
		HipCm:                 req.HipCm,
		NeckCm:                req.NeckCm,
		ChestCm:               req.ChestCm,
		BicepsCm:              req.BicepsCm,
		TricepsCm:             req.TricepsCm,
		BodyFatPct:            req.BodyFatPct,
		ChestSkinfoldMm:       req.ChestSkinfoldMm,
		MidaxillarySkinfoldMm: req.MidaxillarySkinfoldMm,
		TricepsSkinfoldMm:     req.TricepsSkinfoldMm,
		SubscapularSkinfoldMm: req.SubscapularSkinfoldMm,
		AbdominalSkinfoldMm:   req.AbdominalSkinfoldMm,
		SuprailiacSkinfoldMm:  req.SuprailiacSkinfoldMm,
		ThighSkinfoldMm:       req.ThighSkinfoldMm,
		PhotoPath:             req.PhotoPath,
	})
	if err != nil {
		return mapError(c, err)
	}
	logger.FromEcho(c).Info("body snapshot created", zap.String("snapshot_id", s.ID))
	return c.JSON(http.StatusCreated, toBodySnapshotResponse(*s))
}

func (h *BodySnapshotHandler) ListSnapshots(c echo.Context) error {
	from, to := parseDateOnlyParams(c)
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

// parseDateOnlyParams reads optional ?from= and ?to= query params as YYYY-MM-DD date strings.
func parseDateOnlyParams(c echo.Context) (*time.Time, *time.Time) {
	var from, to *time.Time
	if s := c.QueryParam("from"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			from = &t
		}
	}
	if s := c.QueryParam("to"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			to = &t
		}
	}
	return from, to
}

// --- response types ---

type bodySnapshotResponse struct {
	ID         string   `json:"id"`
	Date       string   `json:"date"`
	WeightKg   *float64 `json:"weight_kg,omitempty"`
	WaistCm    *float64 `json:"waist_cm,omitempty"`
	HipCm      *float64 `json:"hip_cm,omitempty"`
	NeckCm     *float64 `json:"neck_cm,omitempty"`
	ChestCm    *float64 `json:"chest_cm,omitempty"`
	BicepsCm   *float64 `json:"biceps_cm,omitempty"`
	TricepsCm  *float64 `json:"triceps_cm,omitempty"`
	BodyFatPct *float64 `json:"body_fat_pct,omitempty"`
	// Pollock 7-site skinfold measurements (mm)
	ChestSkinfoldMm       *float64 `json:"chest_skinfold_mm,omitempty"`
	MidaxillarySkinfoldMm *float64 `json:"midaxillary_skinfold_mm,omitempty"`
	TricepsSkinfoldMm     *float64 `json:"triceps_skinfold_mm,omitempty"`
	SubscapularSkinfoldMm *float64 `json:"subscapular_skinfold_mm,omitempty"`
	AbdominalSkinfoldMm   *float64 `json:"abdominal_skinfold_mm,omitempty"`
	SuprailiacSkinfoldMm  *float64 `json:"suprailiac_skinfold_mm,omitempty"`
	ThighSkinfoldMm       *float64 `json:"thigh_skinfold_mm,omitempty"`
	PhotoPath             *string  `json:"photo_path,omitempty"`
	CreatedAt             string   `json:"created_at"`
}

func toBodySnapshotResponse(s domain.BodySnapshot) bodySnapshotResponse {
	return bodySnapshotResponse{
		ID:                    s.ID,
		Date:                  s.Date.Format("2006-01-02"),
		WeightKg:              s.WeightKg,
		WaistCm:               s.WaistCm,
		HipCm:                 s.HipCm,
		NeckCm:                s.NeckCm,
		ChestCm:               s.ChestCm,
		BicepsCm:              s.BicepsCm,
		TricepsCm:             s.TricepsCm,
		BodyFatPct:            s.BodyFatPct,
		ChestSkinfoldMm:       s.ChestSkinfoldMm,
		MidaxillarySkinfoldMm: s.MidaxillarySkinfoldMm,
		TricepsSkinfoldMm:     s.TricepsSkinfoldMm,
		SubscapularSkinfoldMm: s.SubscapularSkinfoldMm,
		AbdominalSkinfoldMm:   s.AbdominalSkinfoldMm,
		SuprailiacSkinfoldMm:  s.SuprailiacSkinfoldMm,
		ThighSkinfoldMm:       s.ThighSkinfoldMm,
		PhotoPath:             s.PhotoPath,
		CreatedAt:             s.CreatedAt.Format(time.RFC3339),
	}
}
