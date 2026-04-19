package app

import (
	"strings"
	"testing"
	"time"

	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
	"github.com/rafaelsoares/alfredo/internal/telegram"
)

func TestFormatAppointmentCreatedMessageEscapesUserFields(t *testing.T) {
	loc := "<b>Clínica</b>"
	prov := `Dr. <script>alert(1)</script>`
	appt := &domain.Appointment{
		Type:        domain.AppointmentTypeVet,
		ScheduledAt: time.Now(),
		Location:    &loc,
		Provider:    &prov,
	}
	pet := &domain.Pet{Name: "Rex & Co"}
	msg := formatAppointmentCreatedMessage(pet, appt, "America/Sao_Paulo")
	if msg.ParseMode != telegram.ParseModeHTML {
		t.Fatalf("parse mode = %q, want %q", msg.ParseMode, telegram.ParseModeHTML)
	}
	if strings.Contains(msg.Text, "<script>") {
		t.Fatal("unescaped HTML in message")
	}
	if !strings.Contains(msg.Text, "Rex &amp; Co") {
		t.Fatalf("pet name not escaped: %q", msg.Text)
	}
	if strings.Contains(msg.Text, "<b>Clínica</b>") {
		t.Fatalf("location not escaped: %q", msg.Text)
	}
}

func TestTelegramMessageFormattingEscapesUserControlledText(t *testing.T) {
	administeredAt := time.Date(2026, 5, 10, 9, 30, 0, 0, time.FixedZone("BRT", -3*60*60))
	vaccine := &domain.Vaccine{
		Name:           `Raiva <script>`,
		AdministeredAt: administeredAt,
	}
	pet := &domain.Pet{Name: `Luna & Bob`}

	msg := formatVaccineCreatedMessage(pet, vaccine, "America/Sao_Paulo")

	if msg.ParseMode != telegram.ParseModeHTML {
		t.Fatalf("parse mode = %q, want %q", msg.ParseMode, telegram.ParseModeHTML)
	}
	if !strings.Contains(msg.Text, "Luna &amp; Bob") {
		t.Fatalf("pet name was not escaped: %q", msg.Text)
	}
	if !strings.Contains(msg.Text, "Raiva &lt;script&gt;") {
		t.Fatalf("vaccine name was not escaped: %q", msg.Text)
	}
	if strings.Contains(msg.Text, "<script>") {
		t.Fatalf("message contains raw HTML input: %q", msg.Text)
	}
}

func TestFormatVaccineCreatedMessageDescribesMissingAndScheduledNextDue(t *testing.T) {
	pet := &domain.Pet{Name: "Luna"}
	administeredAt := time.Date(2026, 5, 10, 9, 30, 0, 0, time.FixedZone("BRT", -3*60*60))

	withoutRecurrence := formatVaccineCreatedMessage(pet, &domain.Vaccine{
		Name:           "V10",
		AdministeredAt: administeredAt,
	}, "America/Sao_Paulo")
	if !strings.Contains(withoutRecurrence.Text, "não configurada") {
		t.Fatalf("message without recurrence = %q, want next due fallback", withoutRecurrence.Text)
	}

	vet := "Dra Ana"
	batch := "L123"
	notes := "sem reação"
	nextDue := time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)
	withDetails := formatVaccineCreatedMessage(pet, &domain.Vaccine{
		Name:           "V10",
		AdministeredAt: administeredAt,
		NextDueAt:      &nextDue,
		VetName:        &vet,
		BatchNumber:    &batch,
		Notes:          &notes,
	}, "America/Sao_Paulo")
	for _, want := range []string{"Próxima dose", "20/05/2026", "Dra Ana", "L123", "sem reação"} {
		if !strings.Contains(withDetails.Text, want) {
			t.Fatalf("message with details missing %q: %q", want, withDetails.Text)
		}
	}
}

func TestFormatTreatmentMessagesDescribeOngoingAndFiniteCleanup(t *testing.T) {
	pet := &domain.Pet{Name: "Luna"}
	startedAt := time.Date(2026, 4, 17, 8, 0, 0, 0, time.FixedZone("BRT", -3*60*60))
	endedAt := time.Date(2026, 4, 19, 8, 0, 0, 0, time.FixedZone("BRT", -3*60*60))

	ongoing := &domain.Treatment{
		Name:          "Antibiotico",
		DosageAmount:  2.5,
		DosageUnit:    "ml",
		Route:         "oral",
		IntervalHours: 12,
		StartedAt:     startedAt,
	}
	createdOngoing := formatTreatmentCreatedMessage(pet, ongoing, nil, "America/Sao_Paulo")
	if !strings.Contains(createdOngoing.Text, "sem data definida") {
		t.Fatalf("ongoing treatment message = %q, want missing end fallback", createdOngoing.Text)
	}

	finite := &domain.Treatment{
		Name:          "Antibiotico",
		DosageAmount:  2.5,
		DosageUnit:    "ml",
		Route:         "oral",
		IntervalHours: 12,
		StartedAt:     startedAt,
		EndedAt:       &endedAt,
	}
	createdFinite := formatTreatmentCreatedMessage(pet, finite, []domain.Dose{{ID: "dose-1"}, {ID: "dose-2"}}, "America/Sao_Paulo")
	for _, want := range []string{"Doses agendadas", "2"} {
		if !strings.Contains(createdFinite.Text, want) {
			t.Fatalf("finite treatment message missing %q: %q", want, createdFinite.Text)
		}
	}

	stoppedRecurring := formatTreatmentStoppedMessage(pet, ongoing, nil, nil, startedAt, "America/Sao_Paulo")
	if !strings.Contains(stoppedRecurring.Text, "Série recorrente") {
		t.Fatalf("recurring stop message = %q, want recurring cleanup text", stoppedRecurring.Text)
	}

	stoppedFinite := formatTreatmentStoppedMessage(
		pet,
		finite,
		[]domain.Dose{{ID: "dose-1"}, {ID: "dose-2"}},
		[]domain.Dose{{ID: "dose-2"}},
		startedAt,
		"America/Sao_Paulo",
	)
	for _, want := range []string{"Doses já ocorridas", "1", "Doses futuras removidas"} {
		if !strings.Contains(stoppedFinite.Text, want) {
			t.Fatalf("finite stop message missing %q: %q", want, stoppedFinite.Text)
		}
	}
}
