package app

import (
	"context"
	"fmt"
	"time"

	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
)

const (
	VaccineDueSoonDays      = 30
	AppointmentWindowDays   = 14
	ObservationWindowDays   = 7
	SupplyReorderBufferDays = 7
)

type SummaryUseCase struct {
	pets         PetCareServicer
	vaccines     VaccineServicer
	treatments   TreatmentServicer
	appointments AppointmentServicer
	observations ObservationServicer
	supplies     SupplyServicer
	timezone     *time.Location
}

func NewSummaryUseCase(
	pets PetCareServicer,
	vaccines VaccineServicer,
	treatments TreatmentServicer,
	appointments AppointmentServicer,
	observations ObservationServicer,
	supplies SupplyServicer,
	timezone *time.Location,
) *SummaryUseCase {
	if timezone == nil {
		timezone = time.UTC
	}
	return &SummaryUseCase{
		pets:         pets,
		vaccines:     vaccines,
		treatments:   treatments,
		appointments: appointments,
		observations: observations,
		supplies:     supplies,
		timezone:     timezone,
	}
}

func (uc *SummaryUseCase) AllPets(ctx context.Context) (domain.AllPetsSummary, error) {
	now := time.Now().In(uc.timezone)
	pets, err := uc.pets.List(ctx)
	if err != nil {
		return domain.AllPetsSummary{}, fmt.Errorf("list pets for summary: %w", err)
	}

	summary := domain.AllPetsSummary{
		GeneratedAt: now,
		Pets:        make([]domain.PetDigest, 0, len(pets)),
	}
	for _, pet := range pets {
		digest, err := uc.digestForPet(ctx, pet, now)
		if err != nil {
			return domain.AllPetsSummary{}, err
		}
		summary.Pets = append(summary.Pets, digest)
	}
	return summary, nil
}

func (uc *SummaryUseCase) digestForPet(ctx context.Context, pet domain.Pet, now time.Time) (domain.PetDigest, error) {
	vaccines, err := uc.vaccines.ListVaccines(ctx, pet.ID)
	if err != nil {
		return domain.PetDigest{}, fmt.Errorf("list vaccines for pet summary: %w", err)
	}
	treatments, err := uc.treatments.List(ctx, pet.ID)
	if err != nil {
		return domain.PetDigest{}, fmt.Errorf("list treatments for pet summary: %w", err)
	}
	appointments, err := uc.appointments.List(ctx, pet.ID)
	if err != nil {
		return domain.PetDigest{}, fmt.Errorf("list appointments for pet summary: %w", err)
	}
	observations, err := uc.observations.ListByPet(ctx, pet.ID)
	if err != nil {
		return domain.PetDigest{}, fmt.Errorf("list observations for pet summary: %w", err)
	}
	supplies, err := uc.supplies.List(ctx, pet.ID)
	if err != nil {
		return domain.PetDigest{}, fmt.Errorf("list supplies for pet summary: %w", err)
	}

	return domain.PetDigest{
		Pet:                    pet,
		VaccinesDueSoon:        uc.vaccinesDueSoon(vaccines, now),
		ActiveTreatments:       activeTreatments(treatments, now),
		UpcomingAppointments:   upcomingAppointments(appointments, now),
		RecentObservations:     uc.recentObservations(observations, now),
		SuppliesNeedingReorder: suppliesNeedingReorder(supplies, now, uc.timezone),
	}, nil
}

func (uc *SummaryUseCase) vaccinesDueSoon(vaccines []domain.Vaccine, now time.Time) []domain.VaccineSummary {
	today := localDate(now, uc.timezone)
	cutoff := today.AddDate(0, 0, VaccineDueSoonDays)
	out := make([]domain.VaccineSummary, 0)
	for _, vaccine := range vaccines {
		if vaccine.NextDueAt == nil {
			continue
		}
		dueDate := dateOnlyInLocation(*vaccine.NextDueAt, uc.timezone)
		if dueDate.After(cutoff) {
			continue
		}
		daysUntil := daysBetween(today, dueDate, uc.timezone)
		out = append(out, domain.VaccineSummary{
			Vaccine:      vaccine,
			DaysUntilDue: daysUntil,
			Overdue:      dueDate.Before(today),
		})
	}
	return out
}

func activeTreatments(treatments []domain.Treatment, now time.Time) []domain.Treatment {
	out := make([]domain.Treatment, 0)
	for _, treatment := range treatments {
		if treatment.StoppedAt != nil {
			continue
		}
		if treatment.StartedAt.After(now) {
			continue
		}
		if treatment.EndedAt != nil && treatment.EndedAt.Before(now) {
			continue
		}
		out = append(out, treatment)
	}
	return out
}

func upcomingAppointments(appointments []domain.Appointment, now time.Time) []domain.Appointment {
	cutoff := now.AddDate(0, 0, AppointmentWindowDays)
	out := make([]domain.Appointment, 0)
	for _, appointment := range appointments {
		if appointment.ScheduledAt.Before(now) || appointment.ScheduledAt.After(cutoff) {
			continue
		}
		out = append(out, appointment)
	}
	return out
}

func (uc *SummaryUseCase) recentObservations(observations []domain.Observation, now time.Time) []domain.Observation {
	out := make([]domain.Observation, 0)
	for _, observation := range observations {
		if observation.ObservedAt.After(now) || daysBetween(observation.ObservedAt, now, uc.timezone) > ObservationWindowDays {
			continue
		}
		out = append(out, observation)
	}
	return out
}

func suppliesNeedingReorder(supplies []domain.Supply, now time.Time, loc *time.Location) []domain.Supply {
	today := localDate(now, loc)
	cutoff := today.AddDate(0, 0, SupplyReorderBufferDays)
	out := make([]domain.Supply, 0)
	for _, supply := range supplies {
		reorderDate := dateOnlyInLocation(supply.NextReorderAt(), loc)
		if reorderDate.After(cutoff) {
			continue
		}
		out = append(out, supply)
	}
	return out
}

func daysBetween(from, to time.Time, loc *time.Location) int {
	fy, fm, fd := from.In(loc).Date()
	ty, tm, td := to.In(loc).Date()
	fromDate := time.Date(fy, fm, fd, 0, 0, 0, 0, loc)
	toDate := time.Date(ty, tm, td, 0, 0, 0, 0, loc)
	return int(toDate.Sub(fromDate).Hours() / 24)
}

func localDate(t time.Time, loc *time.Location) time.Time {
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}

func dateOnlyInLocation(t time.Time, loc *time.Location) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}
