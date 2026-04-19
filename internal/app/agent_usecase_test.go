package app

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	agentdomain "github.com/rafaelsoares/alfredo/internal/agent/domain"
	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
	"github.com/rafaelsoares/alfredo/internal/petcare/service"
	"github.com/rafaelsoares/alfredo/internal/telegram"
)

type recordingAgentRouter struct {
	systemPrompt string
	tools        []agentdomain.Tool
	inputText    string
	calls        int
}

func (r *recordingAgentRouter) Execute(
	_ context.Context,
	systemPrompt string,
	tools []agentdomain.Tool,
	inputText string,
	_ func(context.Context, agentdomain.ToolCall) (agentdomain.ToolResult, error),
) (string, agentdomain.Invocation, error) {
	r.calls++
	r.systemPrompt = systemPrompt
	r.tools = append([]agentdomain.Tool(nil), tools...)
	r.inputText = inputText
	return "resposta", agentdomain.Invocation{}, nil
}

type failingAgentRouter struct {
	reply string
	err   error
}

func (r *failingAgentRouter) Execute(
	context.Context,
	string,
	[]agentdomain.Tool,
	string,
	func(context.Context, agentdomain.ToolCall) (agentdomain.ToolResult, error),
) (string, agentdomain.Invocation, error) {
	return r.reply, agentdomain.Invocation{}, r.err
}

func TestAgentUseCaseHandleUsesOneShotPrompt(t *testing.T) {
	router := &recordingAgentRouter{}
	uc := NewAgentUseCase(router, nil, nil, nil, nil, nil, nil, nil, nil, time.UTC, zap.NewNop())

	reply, err := uc.Handle(context.Background(), "Nutella tomou banho quando?")
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if reply != "resposta" {
		t.Fatalf("reply = %q", reply)
	}
	if router.calls != 1 {
		t.Fatalf("router calls = %d", router.calls)
	}
	if router.inputText != "Nutella tomou banho quando?" {
		t.Fatalf("input text = %q", router.inputText)
	}

	required := []string{
		"interação de uma única resposta",
		"nunca faça perguntas",
		"nunca peça esclarecimentos",
		"nunca proponha próximos passos",
		"nunca tente continuar a conversa",
		"quando foi o banho",
		"quando foi a última consulta",
		"marcar banho e tosa",
		"type=grooming",
		"get_pet_summary",
		"send_telegram",
	}
	for _, want := range required {
		if !strings.Contains(router.systemPrompt, want) {
			t.Fatalf("system prompt missing %q:\n%s", want, router.systemPrompt)
		}
	}
	if strings.Contains(router.systemPrompt, "esclarecimento em vez de chamar uma ferramenta") {
		t.Fatalf("system prompt still allows clarification questions:\n%s", router.systemPrompt)
	}
}

func TestAgentUseCaseHandleReturnsRouterFallbackReply(t *testing.T) {
	router := &failingAgentRouter{reply: "Não consegui concluir.", err: errors.New("router down")}
	uc := NewAgentUseCase(router, nil, nil, nil, nil, nil, nil, nil, nil, time.UTC, zap.NewNop())

	reply, err := uc.Handle(context.Background(), "resumo dos pets")
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if reply != "Não consegui concluir." {
		t.Fatalf("reply = %q, want fallback reply", reply)
	}
}

func TestBuildAgentToolsAppointmentMetadata(t *testing.T) {
	tools := buildAgentTools()

	listAppointments := toolByName(t, tools, "list_appointments")
	for _, want := range []string{"banho", "banho e tosa", "tosa", "grooming", "quando foi a última consulta"} {
		if !strings.Contains(strings.ToLower(listAppointments.Description), strings.ToLower(want)) {
			t.Fatalf("list_appointments description missing %q: %q", want, listAppointments.Description)
		}
	}

	scheduleAppointment := toolByName(t, tools, "schedule_appointment")
	props, ok := scheduleAppointment.InputSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schedule_appointment properties has unexpected type: %#v", scheduleAppointment.InputSchema["properties"])
	}
	typeProp, ok := props["type"].(map[string]any)
	if !ok {
		t.Fatalf("schedule_appointment type schema has unexpected type: %#v", props["type"])
	}
	desc, ok := typeProp["description"].(string)
	if !ok {
		t.Fatalf("schedule_appointment type description missing: %#v", typeProp)
	}
	for _, want := range []string{"banho", "banho e tosa", "tosa", "grooming"} {
		if !strings.Contains(strings.ToLower(desc), strings.ToLower(want)) {
			t.Fatalf("schedule_appointment type description missing %q: %q", want, desc)
		}
	}
}

func TestBuildAgentToolsDailyDigestMetadata(t *testing.T) {
	tools := buildAgentTools()

	summary := toolByName(t, tools, "get_pet_summary")
	required, ok := summary.InputSchema["required"].([]string)
	if !ok {
		t.Fatalf("get_pet_summary required has unexpected type: %#v", summary.InputSchema["required"])
	}
	if len(required) != 0 {
		t.Fatalf("get_pet_summary should not require input: %#v", summary.InputSchema)
	}
	for _, want := range []string{"all-pets", "daily digest", "resumo diário", "supplies needing reorder"} {
		if !strings.Contains(strings.ToLower(summary.Description), strings.ToLower(want)) {
			t.Fatalf("get_pet_summary description missing %q: %q", want, summary.Description)
		}
	}

	send := toolByName(t, tools, "send_telegram")
	sendRequired, ok := send.InputSchema["required"].([]string)
	if !ok || !sameStringSet(sendRequired, []string{"message"}) {
		t.Fatalf("send_telegram required = %#v", send.InputSchema["required"])
	}
}

func TestAgentUseCaseSendTelegramIsBestEffort(t *testing.T) {
	uc := NewAgentUseCase(nil, nil, nil, nil, nil, nil, nil, nil, failingTelegram{err: errors.New("telegram down")}, time.UTC, zap.NewNop())

	result, err := uc.DispatchToolCall(context.Background(), agentdomain.ToolCall{
		ID:        "call-1",
		Name:      "send_telegram",
		Arguments: map[string]any{"message": "Resumo dos pets"},
	})
	if err != nil {
		t.Fatalf("DispatchToolCall returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("send_telegram returned error tool result: %#v", result)
	}
	if !strings.Contains(result.Content, "Não consegui enviar") {
		t.Fatalf("send_telegram result content = %q", result.Content)
	}
}

func TestAgentUseCaseSendTelegramReportsSuccessAndMissingAdapter(t *testing.T) {
	success := &recordingTelegram{}
	uc := NewAgentUseCase(nil, nil, nil, nil, nil, nil, nil, nil, success, time.UTC, nil)

	result, err := uc.DispatchToolCall(context.Background(), agentdomain.ToolCall{
		ID:        "call-1",
		Name:      "send_telegram",
		Arguments: map[string]any{"message": "Resumo dos pets"},
	})
	if err != nil {
		t.Fatalf("DispatchToolCall returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("send_telegram returned error tool result: %#v", result)
	}
	assertJSONContains(t, result.Content, "enviado")
	if len(success.messages) != 1 || success.messages[0].Text != "Resumo dos pets" {
		t.Fatalf("telegram messages = %#v, want one sent message", success.messages)
	}

	withoutTelegram := NewAgentUseCase(nil, nil, nil, nil, nil, nil, nil, nil, nil, time.UTC, zap.NewNop())
	result, err = withoutTelegram.DispatchToolCall(context.Background(), agentdomain.ToolCall{
		ID:        "call-2",
		Name:      "send_telegram",
		Arguments: map[string]any{"message": "Resumo dos pets"},
	})
	if err != nil {
		t.Fatalf("DispatchToolCall returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("send_telegram returned error tool result for missing adapter: %#v", result)
	}
	assertJSONContains(t, result.Content, "Não consegui enviar")
}

func TestAgentPromptAndToolsMetadataSentinelsForRepresentativeUtterances(t *testing.T) {
	tools := buildAgentTools()
	prompt := agentSystemPrompt

	tests := []struct {
		name string
		want []string
	}{
		{
			name: "bath history",
			want: []string{"quando foi o banho", "list_appointments"},
		},
		{
			name: "bath booking",
			want: []string{"marcar banho e tosa", "schedule_appointment", "type=grooming"},
		},
		{
			name: "consult history",
			want: []string{"quando foi a última consulta", "list_appointments"},
		},
		{
			name: "observation logging",
			want: []string{"observação", "log_observation"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			text := prompt + "\n" + allToolText(tools)
			for _, want := range tc.want {
				if !strings.Contains(strings.ToLower(text), strings.ToLower(want)) {
					t.Fatalf("combined prompt/tool text missing %q:\n%s", want, text)
				}
			}
		})
	}
}

func TestAgentUseCaseDispatchReadTools(t *testing.T) {
	loc := mustLocation(t, "America/Sao_Paulo")
	generatedAt := time.Date(2026, 4, 17, 9, 0, 0, 0, time.UTC)

	tests := []struct {
		name   string
		call   agentdomain.ToolCall
		config func(*agentTestDeps)
		assert func(*testing.T, *agentTestDeps, agentdomain.ToolResult)
	}{
		{
			name: "list_pets",
			call: toolCall("list_pets", nil),
			config: func(d *agentTestDeps) {
				d.pets.list = []domain.Pet{{ID: "pet-1", Name: "Nutella"}}
			},
			assert: func(t *testing.T, d *agentTestDeps, result agentdomain.ToolResult) {
				t.Helper()
				if d.pets.listCalls != 1 {
					t.Fatalf("list calls = %d, want 1", d.pets.listCalls)
				}
				assertJSONContains(t, result.Content, "Nutella")
			},
		},
		{
			name: "get_pet",
			call: toolCall("get_pet", map[string]any{"pet_id": "pet-1"}),
			config: func(d *agentTestDeps) {
				d.pets.get = &domain.Pet{ID: "pet-1", Name: "Nutella"}
			},
			assert: func(t *testing.T, d *agentTestDeps, result agentdomain.ToolResult) {
				t.Helper()
				if d.pets.getID != "pet-1" {
					t.Fatalf("get pet id = %q, want pet-1", d.pets.getID)
				}
				assertJSONContains(t, result.Content, "Nutella")
			},
		},
		{
			name: "list_vaccines",
			call: toolCall("list_vaccines", map[string]any{"pet_id": "pet-1"}),
			config: func(d *agentTestDeps) {
				d.vaccines.list = []domain.Vaccine{{ID: "vac-1", PetID: "pet-1", Name: "V10"}}
			},
			assert: func(t *testing.T, d *agentTestDeps, result agentdomain.ToolResult) {
				t.Helper()
				if d.vaccines.listPetID != "pet-1" {
					t.Fatalf("list vaccines pet id = %q, want pet-1", d.vaccines.listPetID)
				}
				assertJSONContains(t, result.Content, "V10")
			},
		},
		{
			name: "list_treatments",
			call: toolCall("list_treatments", map[string]any{"pet_id": "pet-1"}),
			config: func(d *agentTestDeps) {
				d.treatments.list = []domain.Treatment{{ID: "tr-1", PetID: "pet-1", Name: "Antibiotico"}}
				d.treatments.doses = map[string][]domain.Dose{"tr-1": {{ID: "dose-1", TreatmentID: "tr-1", PetID: "pet-1"}}}
			},
			assert: func(t *testing.T, d *agentTestDeps, result agentdomain.ToolResult) {
				t.Helper()
				if d.treatments.listPetID != "pet-1" {
					t.Fatalf("list treatments pet id = %q, want pet-1", d.treatments.listPetID)
				}
				assertJSONContains(t, result.Content, "Antibiotico")
				assertJSONContains(t, result.Content, "dose-1")
			},
		},
		{
			name: "list_appointments",
			call: toolCall("list_appointments", map[string]any{"pet_id": "pet-1"}),
			config: func(d *agentTestDeps) {
				d.appointments.list = []domain.Appointment{{ID: "appt-1", PetID: "pet-1", Type: domain.AppointmentTypeGrooming}}
			},
			assert: func(t *testing.T, d *agentTestDeps, result agentdomain.ToolResult) {
				t.Helper()
				if d.appointments.listPetID != "pet-1" {
					t.Fatalf("list appointments pet id = %q, want pet-1", d.appointments.listPetID)
				}
				assertJSONContains(t, result.Content, "appt-1")
			},
		},
		{
			name: "list_observations",
			call: toolCall("list_observations", map[string]any{"pet_id": "pet-1"}),
			config: func(d *agentTestDeps) {
				d.observations.list = []domain.Observation{{ID: "obs-1", PetID: "pet-1", Description: "Vomitou"}}
			},
			assert: func(t *testing.T, d *agentTestDeps, result agentdomain.ToolResult) {
				t.Helper()
				if d.observations.listPetID != "pet-1" {
					t.Fatalf("list observations pet id = %q, want pet-1", d.observations.listPetID)
				}
				assertJSONContains(t, result.Content, "Vomitou")
			},
		},
		{
			name: "list_supplies",
			call: toolCall("list_supplies", map[string]any{"pet_id": "pet-1"}),
			config: func(d *agentTestDeps) {
				d.supplies.list = []domain.Supply{{ID: "supply-1", PetID: "pet-1", Name: "Racao"}}
			},
			assert: func(t *testing.T, d *agentTestDeps, result agentdomain.ToolResult) {
				t.Helper()
				if d.supplies.listPetID != "pet-1" {
					t.Fatalf("list supplies pet id = %q, want pet-1", d.supplies.listPetID)
				}
				assertJSONContains(t, result.Content, "Racao")
			},
		},
		{
			name: "get_supply",
			call: toolCall("get_supply", map[string]any{"pet_id": "pet-1", "supply_id": "supply-1"}),
			config: func(d *agentTestDeps) {
				d.supplies.get = &domain.Supply{ID: "supply-1", PetID: "pet-1", Name: "Racao"}
			},
			assert: func(t *testing.T, d *agentTestDeps, result agentdomain.ToolResult) {
				t.Helper()
				if d.supplies.getPetID != "pet-1" || d.supplies.getSupplyID != "supply-1" {
					t.Fatalf("get supply ids = %q/%q, want pet-1/supply-1", d.supplies.getPetID, d.supplies.getSupplyID)
				}
				assertJSONContains(t, result.Content, "Racao")
			},
		},
		{
			name: "get_pet_summary",
			call: toolCall("get_pet_summary", nil),
			config: func(d *agentTestDeps) {
				d.summary.summary = domain.AllPetsSummary{GeneratedAt: generatedAt, Pets: []domain.PetDigest{{Pet: domain.Pet{ID: "pet-1", Name: "Nutella"}}}}
			},
			assert: func(t *testing.T, d *agentTestDeps, result agentdomain.ToolResult) {
				t.Helper()
				if d.summary.calls != 1 {
					t.Fatalf("summary calls = %d, want 1", d.summary.calls)
				}
				assertJSONContains(t, result.Content, "Nutella")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deps := newAgentTestDeps()
			tc.config(deps)
			uc := deps.useCase(loc)

			result, err := uc.DispatchToolCall(context.Background(), tc.call)
			if err != nil {
				t.Fatalf("DispatchToolCall returned error: %v", err)
			}
			if result.IsError {
				t.Fatalf("DispatchToolCall returned error result: %#v", result)
			}
			if result.CallID != tc.call.ID {
				t.Fatalf("call id = %q, want %q", result.CallID, tc.call.ID)
			}
			assertValidJSON(t, result.Content)
			tc.assert(t, deps, result)
		})
	}
}

func TestAgentUseCaseDispatchMutationToolsDecodeInputs(t *testing.T) {
	loc := mustLocation(t, "America/Sao_Paulo")
	observedAt := time.Date(2026, 4, 17, 9, 30, 0, 0, loc)
	administeredAt := time.Date(2026, 4, 17, 10, 0, 0, 0, loc)
	scheduledAt := time.Date(2026, 4, 18, 11, 0, 0, 0, loc)
	startedAt := time.Date(2026, 4, 17, 8, 0, 0, 0, loc)
	endedAt := time.Date(2026, 4, 19, 8, 0, 0, 0, loc)
	purchasedAt := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name   string
		call   agentdomain.ToolCall
		assert func(*testing.T, *agentTestDeps, agentdomain.ToolResult)
	}{
		{
			name: "log_observation",
			call: toolCall("log_observation", map[string]any{
				"pet_id":      "pet-1",
				"observed_at": "2026-04-17T09:30:00",
				"description": "Vomitou",
			}),
			assert: func(t *testing.T, d *agentTestDeps, result agentdomain.ToolResult) {
				t.Helper()
				in := d.observations.createInput
				if in.PetID != "pet-1" || in.Description != "Vomitou" || !in.ObservedAt.Equal(observedAt) {
					t.Fatalf("observation input = %#v, want pet-1/Vomitou/%s", in, observedAt)
				}
				assertJSONContains(t, result.Content, "obs-created")
			},
		},
		{
			name: "record_vaccine",
			call: toolCall("record_vaccine", map[string]any{
				"pet_id":          "pet-1",
				"name":            "V10",
				"date":            "2026-04-17T10:00:00",
				"recurrence_days": float64(365),
				"vet_name":        "Dra Ana",
				"batch_number":    "L123",
				"notes":           "sem reacao",
			}),
			assert: func(t *testing.T, d *agentTestDeps, result agentdomain.ToolResult) {
				t.Helper()
				in := d.vaccines.recordInput
				if in.PetID != "pet-1" || in.Name != "V10" || !in.AdministeredAt.Equal(administeredAt) {
					t.Fatalf("vaccine input = %#v", in)
				}
				if in.RecurrenceDays == nil || *in.RecurrenceDays != 365 {
					t.Fatalf("recurrence days = %#v, want 365", in.RecurrenceDays)
				}
				assertStringPtr(t, in.VetName, "Dra Ana")
				assertStringPtr(t, in.BatchNumber, "L123")
				assertStringPtr(t, in.Notes, "sem reacao")
				assertJSONContains(t, result.Content, "vac-created")
			},
		},
		{
			name: "schedule_appointment",
			call: toolCall("schedule_appointment", map[string]any{
				"pet_id":       "pet-1",
				"type":         "grooming",
				"scheduled_at": "2026-04-18T11:00:00",
				"provider":     "Pet Shop",
				"location":     "Centro",
				"notes":        "banho",
			}),
			assert: func(t *testing.T, d *agentTestDeps, result agentdomain.ToolResult) {
				t.Helper()
				in := d.appointments.createInput
				if in.PetID != "pet-1" || in.Type != domain.AppointmentTypeGrooming || !in.ScheduledAt.Equal(scheduledAt) {
					t.Fatalf("appointment input = %#v", in)
				}
				assertStringPtr(t, in.Provider, "Pet Shop")
				assertStringPtr(t, in.Location, "Centro")
				assertStringPtr(t, in.Notes, "banho")
				assertJSONContains(t, result.Content, "appt-created")
			},
		},
		{
			name: "start_treatment",
			call: toolCall("start_treatment", map[string]any{
				"pet_id":         "pet-1",
				"name":           "Antibiotico",
				"dosage_amount":  "2.5",
				"dosage_unit":    "ml",
				"route":          "oral",
				"interval_hours": "12",
				"started_at":     "2026-04-17T08:00:00",
				"ended_at":       "2026-04-19T08:00:00",
				"vet_name":       "Dra Ana",
				"notes":          "com comida",
			}),
			assert: func(t *testing.T, d *agentTestDeps, result agentdomain.ToolResult) {
				t.Helper()
				in := d.treatments.createInput
				if in.PetID != "pet-1" || in.Name != "Antibiotico" || in.DosageAmount != 2.5 || in.DosageUnit != "ml" || in.Route != "oral" || in.IntervalHours != 12 || !in.StartedAt.Equal(startedAt) {
					t.Fatalf("treatment input = %#v", in)
				}
				if in.EndedAt == nil || !in.EndedAt.Equal(endedAt) {
					t.Fatalf("ended at = %#v, want %s", in.EndedAt, endedAt)
				}
				assertStringPtr(t, in.VetName, "Dra Ana")
				assertStringPtr(t, in.Notes, "com comida")
				assertJSONContains(t, result.Content, "tr-created")
				assertJSONContains(t, result.Content, "dose-created")
			},
		},
		{
			name: "reschedule_appointment",
			call: toolCall("reschedule_appointment", map[string]any{
				"pet_id":         "pet-1",
				"appointment_id": "appt-1",
				"scheduled_at":   "2026-04-18T11:00:00",
			}),
			assert: func(t *testing.T, d *agentTestDeps, result agentdomain.ToolResult) {
				t.Helper()
				if d.appointments.updatePetID != "pet-1" || d.appointments.updateAppointmentID != "appt-1" {
					t.Fatalf("update ids = %q/%q, want pet-1/appt-1", d.appointments.updatePetID, d.appointments.updateAppointmentID)
				}
				if d.appointments.updateInput.ScheduledAt == nil || !d.appointments.updateInput.ScheduledAt.Equal(scheduledAt) {
					t.Fatalf("scheduled at = %#v, want %s", d.appointments.updateInput.ScheduledAt, scheduledAt)
				}
				assertJSONContains(t, result.Content, "appt-updated")
			},
		},
		{
			name: "create_supply",
			call: toolCall("create_supply", map[string]any{
				"pet_id":                "pet-1",
				"name":                  "Racao",
				"last_purchased_at":     "2026-04-01",
				"estimated_days_supply": float64(30),
				"notes":                 "pacote 10kg",
			}),
			assert: func(t *testing.T, d *agentTestDeps, result agentdomain.ToolResult) {
				t.Helper()
				in := d.supplies.createInput
				if in.PetID != "pet-1" || in.Name != "Racao" || in.EstimatedDaysSupply != 30 || !in.LastPurchasedAt.Equal(purchasedAt) {
					t.Fatalf("supply input = %#v", in)
				}
				assertStringPtr(t, in.Notes, "pacote 10kg")
				assertJSONContains(t, result.Content, "supply-created")
			},
		},
		{
			name: "update_supply",
			call: toolCall("update_supply", map[string]any{
				"pet_id":                "pet-1",
				"supply_id":             "supply-1",
				"name":                  "Racao senior",
				"last_purchased_at":     "2026-04-01",
				"estimated_days_supply": "45",
				"notes":                 "novo pacote",
			}),
			assert: func(t *testing.T, d *agentTestDeps, result agentdomain.ToolResult) {
				t.Helper()
				if d.supplies.updatePetID != "pet-1" || d.supplies.updateSupplyID != "supply-1" {
					t.Fatalf("update supply ids = %q/%q, want pet-1/supply-1", d.supplies.updatePetID, d.supplies.updateSupplyID)
				}
				in := d.supplies.updateInput
				assertStringPtr(t, in.Name, "Racao senior")
				if in.LastPurchasedAt == nil || !in.LastPurchasedAt.Equal(purchasedAt) {
					t.Fatalf("last purchased at = %#v, want %s", in.LastPurchasedAt, purchasedAt)
				}
				if in.EstimatedDaysSupply == nil || *in.EstimatedDaysSupply != 45 {
					t.Fatalf("estimated days = %#v, want 45", in.EstimatedDaysSupply)
				}
				assertStringPtr(t, in.Notes, "novo pacote")
				assertJSONContains(t, result.Content, "supply-updated")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deps := newAgentTestDeps()
			uc := deps.useCase(loc)

			result, err := uc.DispatchToolCall(context.Background(), tc.call)
			if err != nil {
				t.Fatalf("DispatchToolCall returned error: %v", err)
			}
			if result.IsError {
				t.Fatalf("DispatchToolCall returned error result: %#v", result)
			}
			assertValidJSON(t, result.Content)
			tc.assert(t, deps, result)
		})
	}
}

func TestAgentUseCaseDispatchToolErrors(t *testing.T) {
	loc := mustLocation(t, "America/Sao_Paulo")
	downstreamErr := errors.New("database down")

	tests := []struct {
		name        string
		call        agentdomain.ToolCall
		config      func(*agentTestDeps)
		useCase     func(*agentTestDeps) *AgentUseCase
		wantMessage string
	}{
		{
			name:        "unknown tool",
			call:        toolCall("missing_tool", nil),
			wantMessage: `unknown tool "missing_tool"`,
		},
		{
			name:        "missing required argument",
			call:        toolCall("get_pet", map[string]any{}),
			wantMessage: "pet_id is required",
		},
		{
			name:        "invalid appointment type",
			call:        toolCall("schedule_appointment", map[string]any{"pet_id": "pet-1", "type": "bath", "scheduled_at": "2026-04-18T11:00:00"}),
			wantMessage: "type must be one of",
		},
		{
			name:        "invalid datetime",
			call:        toolCall("log_observation", map[string]any{"pet_id": "pet-1", "observed_at": "2026-04-17", "description": "Vomitou"}),
			wantMessage: "observed_at",
		},
		{
			name:        "invalid date",
			call:        toolCall("create_supply", map[string]any{"pet_id": "pet-1", "name": "Racao", "last_purchased_at": "2026-04-01T10:00:00", "estimated_days_supply": 30}),
			wantMessage: "last_purchased_at must be a date",
		},
		{
			name: "summary not configured",
			call: toolCall("get_pet_summary", nil),
			useCase: func(d *agentTestDeps) *AgentUseCase {
				return NewAgentUseCase(nil, d.pets, d.vaccines, d.treatments, d.observations, d.appointments, d.supplies, nil, nil, loc, zap.NewNop())
			},
			wantMessage: "summary use case is not configured",
		},
		{
			name: "downstream service error",
			call: toolCall("list_pets", nil),
			config: func(d *agentTestDeps) {
				d.pets.listErr = downstreamErr
			},
			wantMessage: downstreamErr.Error(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deps := newAgentTestDeps()
			if tc.config != nil {
				tc.config(deps)
			}
			uc := deps.useCase(loc)
			if tc.useCase != nil {
				uc = tc.useCase(deps)
			}

			result, err := uc.DispatchToolCall(context.Background(), tc.call)
			if err == nil {
				t.Fatal("DispatchToolCall returned nil error")
			}
			if !result.IsError {
				t.Fatalf("tool result IsError = false, result = %#v", result)
			}
			if !strings.Contains(result.Content, tc.wantMessage) {
				t.Fatalf("error content = %q, want substring %q", result.Content, tc.wantMessage)
			}
		})
	}
}

func TestAgentUseCaseDispatchRejectsInvalidToolArguments(t *testing.T) {
	loc := mustLocation(t, "America/Sao_Paulo")

	tests := []struct {
		name        string
		call        agentdomain.ToolCall
		wantMessage string
	}{
		{
			name:        "blank required string",
			call:        toolCall("get_pet", map[string]any{"pet_id": "  "}),
			wantMessage: "pet_id is required",
		},
		{
			name:        "non string required argument",
			call:        toolCall("get_pet", map[string]any{"pet_id": 123}),
			wantMessage: "pet_id must be a string",
		},
		{
			name: "invalid treatment amount",
			call: toolCall("start_treatment", map[string]any{
				"pet_id":         "pet-1",
				"name":           "Antibiotico",
				"dosage_amount":  true,
				"dosage_unit":    "ml",
				"route":          "oral",
				"interval_hours": 12,
				"started_at":     "2026-04-17T08:00:00",
			}),
			wantMessage: "dosage_amount must be a number",
		},
		{
			name: "invalid treatment interval",
			call: toolCall("start_treatment", map[string]any{
				"pet_id":         "pet-1",
				"name":           "Antibiotico",
				"dosage_amount":  2.5,
				"dosage_unit":    "ml",
				"route":          "oral",
				"interval_hours": "doze",
				"started_at":     "2026-04-17T08:00:00",
			}),
			wantMessage: "interval_hours must be an integer",
		},
		{
			name: "invalid treatment end time",
			call: toolCall("start_treatment", map[string]any{
				"pet_id":         "pet-1",
				"name":           "Antibiotico",
				"dosage_amount":  2.5,
				"dosage_unit":    "ml",
				"route":          "oral",
				"interval_hours": 12,
				"started_at":     "2026-04-17T08:00:00",
				"ended_at":       "amanha",
			}),
			wantMessage: "ended_at",
		},
		{
			name: "invalid vaccine recurrence",
			call: toolCall("record_vaccine", map[string]any{
				"pet_id":          "pet-1",
				"name":            "V10",
				"date":            "2026-04-17T10:00:00",
				"recurrence_days": true,
			}),
			wantMessage: "recurrence_days must be an integer",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deps := newAgentTestDeps()
			uc := deps.useCase(loc)

			result, err := uc.DispatchToolCall(context.Background(), tc.call)
			if err == nil {
				t.Fatal("DispatchToolCall returned nil error")
			}
			if !result.IsError {
				t.Fatalf("tool result IsError = false, result = %#v", result)
			}
			if !strings.Contains(result.Content, tc.wantMessage) {
				t.Fatalf("error content = %q, want substring %q", result.Content, tc.wantMessage)
			}
		})
	}
}

func toolByName(t *testing.T, tools []agentdomain.Tool, name string) agentdomain.Tool {
	t.Helper()
	for _, tool := range tools {
		if tool.Name == name {
			return tool
		}
	}
	t.Fatalf("tool %q not found", name)
	return agentdomain.Tool{}
}

func allToolText(tools []agentdomain.Tool) string {
	var b strings.Builder
	for _, tool := range tools {
		b.WriteString(tool.Name)
		b.WriteString("\n")
		b.WriteString(tool.Description)
		b.WriteString("\n")
	}
	return b.String()
}

func toolCall(name string, args map[string]any) agentdomain.ToolCall {
	if args == nil {
		args = map[string]any{}
	}
	return agentdomain.ToolCall{ID: "call-" + name, Name: name, Arguments: args}
}

func assertValidJSON(t *testing.T, content string) {
	t.Helper()
	var decoded any
	if err := json.Unmarshal([]byte(content), &decoded); err != nil {
		t.Fatalf("content is not valid JSON: %v\n%s", err, content)
	}
}

func assertJSONContains(t *testing.T, content, want string) {
	t.Helper()
	assertValidJSON(t, content)
	if !strings.Contains(content, want) {
		t.Fatalf("content = %s, want substring %q", content, want)
	}
}

func assertStringPtr(t *testing.T, got *string, want string) {
	t.Helper()
	if got == nil || *got != want {
		t.Fatalf("string ptr = %#v, want %q", got, want)
	}
}

func mustLocation(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("load location %q: %v", name, err)
	}
	return loc
}

type agentTestDeps struct {
	pets         *agentPetFake
	vaccines     *agentVaccineFake
	treatments   *agentTreatmentFake
	observations *agentObservationFake
	appointments *agentAppointmentFake
	supplies     *agentSupplyFake
	summary      *agentSummaryFake
}

func newAgentTestDeps() *agentTestDeps {
	return &agentTestDeps{
		pets:         &agentPetFake{},
		vaccines:     &agentVaccineFake{},
		treatments:   &agentTreatmentFake{},
		observations: &agentObservationFake{},
		appointments: &agentAppointmentFake{},
		supplies:     &agentSupplyFake{},
		summary:      &agentSummaryFake{},
	}
}

func (d *agentTestDeps) useCase(loc *time.Location) *AgentUseCase {
	return NewAgentUseCase(nil, d.pets, d.vaccines, d.treatments, d.observations, d.appointments, d.supplies, d.summary, nil, loc, zap.NewNop())
}

type failingTelegram struct {
	err error
}

func (t failingTelegram) Send(context.Context, telegram.Message) error {
	return t.err
}

type recordingTelegram struct {
	messages []telegram.Message
}

func (t *recordingTelegram) Send(_ context.Context, msg telegram.Message) error {
	t.messages = append(t.messages, msg)
	return nil
}

type agentPetFake struct {
	list      []domain.Pet
	get       *domain.Pet
	listErr   error
	getErr    error
	listCalls int
	getID     string
}

func (f *agentPetFake) List(context.Context) ([]domain.Pet, error) {
	f.listCalls++
	return f.list, f.listErr
}

func (f *agentPetFake) Create(context.Context, service.CreatePetInput) (*domain.Pet, error) {
	panic("not used")
}

func (f *agentPetFake) GetByID(_ context.Context, id string) (*domain.Pet, error) {
	f.getID = id
	return f.get, f.getErr
}

func (f *agentPetFake) Update(context.Context, string, service.UpdatePetInput) (*domain.Pet, error) {
	panic("not used")
}

func (f *agentPetFake) Delete(context.Context, string) error {
	panic("not used")
}

type agentVaccineFake struct {
	list        []domain.Vaccine
	listErr     error
	recordErr   error
	listPetID   string
	recordInput service.RecordVaccineInput
}

func (f *agentVaccineFake) ListVaccines(_ context.Context, petID string) ([]domain.Vaccine, error) {
	f.listPetID = petID
	return f.list, f.listErr
}

func (f *agentVaccineFake) RecordVaccine(_ context.Context, in service.RecordVaccineInput) (*domain.Vaccine, error) {
	f.recordInput = in
	if f.recordErr != nil {
		return nil, f.recordErr
	}
	return &domain.Vaccine{ID: "vac-created", PetID: in.PetID, Name: in.Name, AdministeredAt: in.AdministeredAt, VetName: in.VetName, BatchNumber: in.BatchNumber, Notes: in.Notes}, nil
}

type agentTreatmentFake struct {
	list        []domain.Treatment
	doses       map[string][]domain.Dose
	listErr     error
	createErr   error
	listPetID   string
	createInput service.CreateTreatmentInput
}

func (f *agentTreatmentFake) Create(_ context.Context, in service.CreateTreatmentInput) (*domain.Treatment, []domain.Dose, error) {
	f.createInput = in
	if f.createErr != nil {
		return nil, nil, f.createErr
	}
	return &domain.Treatment{ID: "tr-created", PetID: in.PetID, Name: in.Name, DosageAmount: in.DosageAmount, DosageUnit: in.DosageUnit, Route: in.Route, IntervalHours: in.IntervalHours, StartedAt: in.StartedAt, EndedAt: in.EndedAt, VetName: in.VetName, Notes: in.Notes}, []domain.Dose{{ID: "dose-created", PetID: in.PetID, TreatmentID: "tr-created"}}, nil
}

func (f *agentTreatmentFake) GetByID(context.Context, string, string) (*domain.Treatment, []domain.Dose, error) {
	panic("not used")
}

func (f *agentTreatmentFake) List(_ context.Context, petID string) ([]domain.Treatment, map[string][]domain.Dose, error) {
	f.listPetID = petID
	return f.list, f.doses, f.listErr
}

func (f *agentTreatmentFake) Stop(context.Context, string, string) error {
	panic("not used")
}

type agentObservationFake struct {
	list        []domain.Observation
	listErr     error
	createErr   error
	listPetID   string
	createInput service.CreateObservationInput
}

func (f *agentObservationFake) Create(_ context.Context, in service.CreateObservationInput) (*domain.Observation, error) {
	f.createInput = in
	if f.createErr != nil {
		return nil, f.createErr
	}
	return &domain.Observation{ID: "obs-created", PetID: in.PetID, ObservedAt: in.ObservedAt, Description: in.Description}, nil
}

func (f *agentObservationFake) ListByPet(_ context.Context, petID string) ([]domain.Observation, error) {
	f.listPetID = petID
	return f.list, f.listErr
}

func (f *agentObservationFake) GetByID(context.Context, string, string) (*domain.Observation, error) {
	panic("not used")
}

type agentAppointmentFake struct {
	list                []domain.Appointment
	listErr             error
	createErr           error
	updateErr           error
	listPetID           string
	createInput         service.CreateAppointmentInput
	updatePetID         string
	updateAppointmentID string
	updateInput         service.UpdateAppointmentInput
}

func (f *agentAppointmentFake) Create(_ context.Context, in service.CreateAppointmentInput) (*domain.Appointment, error) {
	f.createInput = in
	if f.createErr != nil {
		return nil, f.createErr
	}
	return &domain.Appointment{ID: "appt-created", PetID: in.PetID, Type: in.Type, ScheduledAt: in.ScheduledAt, Provider: in.Provider, Location: in.Location, Notes: in.Notes}, nil
}

func (f *agentAppointmentFake) GetByID(context.Context, string, string) (*domain.Appointment, error) {
	panic("not used")
}

func (f *agentAppointmentFake) List(_ context.Context, petID string) ([]domain.Appointment, error) {
	f.listPetID = petID
	return f.list, f.listErr
}

func (f *agentAppointmentFake) Update(_ context.Context, petID, appointmentID string, in service.UpdateAppointmentInput) (*domain.Appointment, error) {
	f.updatePetID = petID
	f.updateAppointmentID = appointmentID
	f.updateInput = in
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	return &domain.Appointment{ID: "appt-updated", PetID: petID, ScheduledAt: *in.ScheduledAt}, nil
}

func (f *agentAppointmentFake) Delete(context.Context, string, string) error {
	panic("not used")
}

type agentSupplyFake struct {
	list           []domain.Supply
	get            *domain.Supply
	listErr        error
	getErr         error
	createErr      error
	updateErr      error
	listPetID      string
	getPetID       string
	getSupplyID    string
	createInput    service.CreateSupplyInput
	updatePetID    string
	updateSupplyID string
	updateInput    service.UpdateSupplyInput
}

func (f *agentSupplyFake) Create(_ context.Context, in service.CreateSupplyInput) (*domain.Supply, error) {
	f.createInput = in
	if f.createErr != nil {
		return nil, f.createErr
	}
	return &domain.Supply{ID: "supply-created", PetID: in.PetID, Name: in.Name, LastPurchasedAt: in.LastPurchasedAt, EstimatedDaysSupply: in.EstimatedDaysSupply, Notes: in.Notes}, nil
}

func (f *agentSupplyFake) GetByID(_ context.Context, petID, supplyID string) (*domain.Supply, error) {
	f.getPetID = petID
	f.getSupplyID = supplyID
	return f.get, f.getErr
}

func (f *agentSupplyFake) List(_ context.Context, petID string) ([]domain.Supply, error) {
	f.listPetID = petID
	return f.list, f.listErr
}

func (f *agentSupplyFake) Update(_ context.Context, petID, supplyID string, in service.UpdateSupplyInput) (*domain.Supply, error) {
	f.updatePetID = petID
	f.updateSupplyID = supplyID
	f.updateInput = in
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	return &domain.Supply{ID: "supply-updated", PetID: petID, Name: *in.Name, LastPurchasedAt: *in.LastPurchasedAt, EstimatedDaysSupply: *in.EstimatedDaysSupply, Notes: in.Notes}, nil
}

func (f *agentSupplyFake) Delete(context.Context, string, string) error {
	panic("not used")
}

type agentSummaryFake struct {
	summary domain.AllPetsSummary
	err     error
	calls   int
}

func (f *agentSummaryFake) AllPets(context.Context) (domain.AllPetsSummary, error) {
	f.calls++
	return f.summary, f.err
}

func sameStringSet(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	seen := make(map[string]int, len(got))
	for _, value := range got {
		seen[value]++
	}
	for _, value := range want {
		seen[value]--
		if seen[value] < 0 {
			return false
		}
	}
	return true
}
