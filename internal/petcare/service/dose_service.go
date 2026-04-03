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
