package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	agentdomain "github.com/rafaelsoares/alfredo/internal/agent/domain"
	agentservice "github.com/rafaelsoares/alfredo/internal/agent/service"
	"github.com/rafaelsoares/alfredo/internal/petcare/domain"
	"github.com/rafaelsoares/alfredo/internal/petcare/service"
	"github.com/rafaelsoares/alfredo/internal/telegram"
	"github.com/rafaelsoares/alfredo/internal/timeutil"
)

const agentSystemPrompt = `Você é o Alfredo, assistente pessoal do Rafael para cuidados com os pets dele.
Esta é uma interação de uma única resposta: nunca faça perguntas, nunca peça esclarecimentos, nunca proponha próximos passos e nunca tente continuar a conversa.
Sempre responda em português brasileiro de forma curta, direta e assertiva.
Use as ferramentas disponíveis para registrar ou consultar informações sobre os pets.
Quando o Rafael pedir resumo diário, digest, pendências de hoje ou prioridades dos pets, chame get_pet_summary, escreva uma mensagem curta em português com os itens acionáveis e depois chame send_telegram com essa mensagem.
Para qualquer operação que envolva um pet específico, primeiro chame list_pets para resolver o identificador correto a partir do nome falado pelo Rafael.
Trate "banho", "banho e tosa", "tosa" e "grooming" como grooming/banho e tosa.
Se o Rafael perguntar quando foi o banho, quando foi a tosa ou quando foi a última consulta, consulte list_appointments.
Se o Rafael quiser marcar banho e tosa ou agendar grooming, use schedule_appointment com type=grooming.
Se o Rafael disser para registrar ou anotar uma observação, use log_observation.
Nunca invente identificadores.
Se o pedido do Rafael estiver ambíguo ou faltar informação essencial, responda apenas que não conseguiu concluir o pedido.`

type AgentRouter interface {
	Execute(
		ctx context.Context,
		systemPrompt string,
		tools []agentdomain.Tool,
		inputText string,
		dispatch func(ctx context.Context, call agentdomain.ToolCall) (agentdomain.ToolResult, error),
	) (reply string, inv agentdomain.Invocation, err error)
}

type AgentTreatmentUseCaser interface {
	Create(ctx context.Context, in service.CreateTreatmentInput) (*domain.Treatment, []domain.Dose, error)
	GetByID(ctx context.Context, petID, treatmentID string) (*domain.Treatment, []domain.Dose, error)
	List(ctx context.Context, petID string) ([]domain.Treatment, map[string][]domain.Dose, error)
	Stop(ctx context.Context, petID, treatmentID string) error
}

type AgentVaccineUseCaser interface {
	ListVaccines(ctx context.Context, petID string) ([]domain.Vaccine, error)
	RecordVaccine(ctx context.Context, in service.RecordVaccineInput) (*domain.Vaccine, error)
}

type AgentUseCase struct {
	router       AgentRouter
	pets         PetCareServicer
	vaccines     AgentVaccineUseCaser
	treatments   AgentTreatmentUseCaser
	observations ObservationServicer
	appointments AppointmentServicer
	supplies     SupplyServicer
	summary      SummaryUseCaser
	telegram     TelegramPort
	timezone     *time.Location
	logger       *zap.Logger
	tools        []agentdomain.Tool
}

func NewAgentUseCase(
	router AgentRouter,
	pets PetCareServicer,
	vaccines AgentVaccineUseCaser,
	treatments AgentTreatmentUseCaser,
	observations ObservationServicer,
	appointments AppointmentServicer,
	supplies SupplyServicer,
	summary SummaryUseCaser,
	telegramPort TelegramPort,
	timezone *time.Location,
	logger *zap.Logger,
) *AgentUseCase {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &AgentUseCase{
		router:       router,
		pets:         pets,
		vaccines:     vaccines,
		treatments:   treatments,
		observations: observations,
		appointments: appointments,
		supplies:     supplies,
		summary:      summary,
		telegram:     telegramPort,
		timezone:     timezone,
		logger:       logger,
		tools:        buildAgentTools(),
	}
}

func (uc *AgentUseCase) Handle(ctx context.Context, inputText string) (string, error) {
	reply, _, err := uc.router.Execute(ctx, agentSystemPrompt, uc.tools, inputText, uc.DispatchToolCall)
	if err != nil {
		uc.logger.Warn("agent handled request with fallback reply", zap.Error(err))
		return reply, nil
	}
	return reply, nil
}

func (uc *AgentUseCase) DispatchToolCall(ctx context.Context, call agentdomain.ToolCall) (agentdomain.ToolResult, error) {
	handlers := uc.toolHandlers()
	handler, ok := handlers[call.Name]
	if !ok {
		err := fmt.Errorf("unknown tool %q", call.Name)
		return errorToolResult(call, err), err
	}

	result, err := handler(ctx, call)
	if err != nil {
		return errorToolResult(call, err), err
	}

	content, err := json.Marshal(result)
	if err != nil {
		return errorToolResult(call, err), fmt.Errorf("marshal tool result for %q: %w", call.Name, err)
	}
	return agentdomain.ToolResult{CallID: call.ID, Content: string(content)}, nil
}

func (uc *AgentUseCase) toolHandlers() map[string]func(context.Context, agentdomain.ToolCall) (any, error) {
	return map[string]func(context.Context, agentdomain.ToolCall) (any, error){
		"list_pets":              uc.toolListPets,
		"get_pet":                uc.toolGetPet,
		"list_vaccines":          uc.toolListVaccines,
		"list_treatments":        uc.toolListTreatments,
		"list_appointments":      uc.toolListAppointments,
		"list_observations":      uc.toolListObservations,
		"list_supplies":          uc.toolListSupplies,
		"get_supply":             uc.toolGetSupply,
		"get_pet_summary":        uc.toolGetPetSummary,
		"send_telegram":          uc.toolSendTelegram,
		"log_observation":        uc.toolLogObservation,
		"record_vaccine":         uc.toolRecordVaccine,
		"schedule_appointment":   uc.toolScheduleAppointment,
		"start_treatment":        uc.toolStartTreatment,
		"reschedule_appointment": uc.toolRescheduleAppointment,
		"create_supply":          uc.toolCreateSupply,
		"update_supply":          uc.toolUpdateSupply,
	}
}

func (uc *AgentUseCase) toolListPets(ctx context.Context, call agentdomain.ToolCall) (any, error) {
	return uc.pets.List(ctx)
}

func (uc *AgentUseCase) toolGetPet(ctx context.Context, call agentdomain.ToolCall) (any, error) {
	petID, err := requireString(call.Arguments, "pet_id")
	if err != nil {
		return nil, err
	}
	return uc.pets.GetByID(ctx, petID)
}

func (uc *AgentUseCase) toolListVaccines(ctx context.Context, call agentdomain.ToolCall) (any, error) {
	petID, err := requireString(call.Arguments, "pet_id")
	if err != nil {
		return nil, err
	}
	return uc.vaccines.ListVaccines(ctx, petID)
}

func (uc *AgentUseCase) toolListTreatments(ctx context.Context, call agentdomain.ToolCall) (any, error) {
	petID, err := requireString(call.Arguments, "pet_id")
	if err != nil {
		return nil, err
	}
	treatments, doses, err := uc.treatments.List(ctx, petID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"treatments": treatments, "doses": doses}, nil
}

func (uc *AgentUseCase) toolListAppointments(ctx context.Context, call agentdomain.ToolCall) (any, error) {
	petID, err := requireString(call.Arguments, "pet_id")
	if err != nil {
		return nil, err
	}
	return uc.appointments.List(ctx, petID)
}

func (uc *AgentUseCase) toolListObservations(ctx context.Context, call agentdomain.ToolCall) (any, error) {
	petID, err := requireString(call.Arguments, "pet_id")
	if err != nil {
		return nil, err
	}
	return uc.observations.ListByPet(ctx, petID)
}

func (uc *AgentUseCase) toolListSupplies(ctx context.Context, call agentdomain.ToolCall) (any, error) {
	petID, err := requireString(call.Arguments, "pet_id")
	if err != nil {
		return nil, err
	}
	return uc.supplies.List(ctx, petID)
}

func (uc *AgentUseCase) toolGetSupply(ctx context.Context, call agentdomain.ToolCall) (any, error) {
	petID, supplyID, err := requireTwoStrings(call.Arguments, "pet_id", "supply_id")
	if err != nil {
		return nil, err
	}
	return uc.supplies.GetByID(ctx, petID, supplyID)
}

func (uc *AgentUseCase) toolGetPetSummary(ctx context.Context, call agentdomain.ToolCall) (any, error) {
	if uc.summary == nil {
		return nil, fmt.Errorf("summary use case is not configured")
	}
	return uc.summary.AllPets(ctx)
}

func (uc *AgentUseCase) toolSendTelegram(ctx context.Context, call agentdomain.ToolCall) (any, error) {
	message, err := requireString(call.Arguments, "message")
	if err != nil {
		return nil, err
	}
	return uc.sendTelegramBestEffort(ctx, message), nil
}

func (uc *AgentUseCase) toolLogObservation(ctx context.Context, call agentdomain.ToolCall) (any, error) {
	in, err := uc.decodeObservation(call.Arguments)
	if err != nil {
		return nil, err
	}
	return uc.observations.Create(ctx, in)
}

func (uc *AgentUseCase) toolRecordVaccine(ctx context.Context, call agentdomain.ToolCall) (any, error) {
	in, err := uc.decodeVaccine(call.Arguments)
	if err != nil {
		return nil, err
	}
	return uc.vaccines.RecordVaccine(ctx, in)
}

func (uc *AgentUseCase) toolScheduleAppointment(ctx context.Context, call agentdomain.ToolCall) (any, error) {
	in, err := uc.decodeAppointment(call.Arguments)
	if err != nil {
		return nil, err
	}
	return uc.appointments.Create(ctx, in)
}

func (uc *AgentUseCase) toolStartTreatment(ctx context.Context, call agentdomain.ToolCall) (any, error) {
	in, err := uc.decodeTreatment(call.Arguments)
	if err != nil {
		return nil, err
	}
	treatment, doses, err := uc.treatments.Create(ctx, in)
	if err != nil {
		return nil, err
	}
	return map[string]any{"treatment": treatment, "doses": doses}, nil
}

func (uc *AgentUseCase) toolRescheduleAppointment(ctx context.Context, call agentdomain.ToolCall) (any, error) {
	petID, appointmentID, err := requireTwoStrings(call.Arguments, "pet_id", "appointment_id")
	if err != nil {
		return nil, err
	}
	scheduledAt, err := uc.requireUserTime(call.Arguments, "scheduled_at")
	if err != nil {
		return nil, err
	}
	return uc.appointments.Update(ctx, petID, appointmentID, service.UpdateAppointmentInput{ScheduledAt: &scheduledAt})
}

func (uc *AgentUseCase) toolCreateSupply(ctx context.Context, call agentdomain.ToolCall) (any, error) {
	in, err := uc.decodeCreateSupply(call.Arguments)
	if err != nil {
		return nil, err
	}
	return uc.supplies.Create(ctx, in)
}

func (uc *AgentUseCase) toolUpdateSupply(ctx context.Context, call agentdomain.ToolCall) (any, error) {
	petID, supplyID, err := requireTwoStrings(call.Arguments, "pet_id", "supply_id")
	if err != nil {
		return nil, err
	}
	in, err := decodeUpdateSupply(call.Arguments)
	if err != nil {
		return nil, err
	}
	return uc.supplies.Update(ctx, petID, supplyID, in)
}

func (uc *AgentUseCase) sendTelegramBestEffort(ctx context.Context, message string) map[string]string {
	if uc.telegram == nil {
		uc.logger.Warn("telegram tool skipped because adapter is not configured")
		return map[string]string{"status": "erro", "message": "Não consegui enviar a mensagem no Telegram."}
	}
	if err := uc.telegram.Send(ctx, telegram.Message{Text: message}); err != nil {
		uc.logger.Warn("telegram tool send failed", zap.Error(err))
		return map[string]string{"status": "erro", "message": "Não consegui enviar a mensagem no Telegram."}
	}
	return map[string]string{"status": "enviado", "message": "Mensagem enviada no Telegram."}
}

func (uc *AgentUseCase) decodeObservation(args map[string]any) (service.CreateObservationInput, error) {
	petID, err := requireString(args, "pet_id")
	if err != nil {
		return service.CreateObservationInput{}, err
	}
	observedAt, err := uc.requireUserTime(args, "observed_at")
	if err != nil {
		return service.CreateObservationInput{}, err
	}
	description, err := requireString(args, "description")
	if err != nil {
		return service.CreateObservationInput{}, err
	}
	return service.CreateObservationInput{PetID: petID, ObservedAt: observedAt, Description: description}, nil
}

func (uc *AgentUseCase) decodeVaccine(args map[string]any) (service.RecordVaccineInput, error) {
	petID, err := requireString(args, "pet_id")
	if err != nil {
		return service.RecordVaccineInput{}, err
	}
	name, err := requireString(args, "name")
	if err != nil {
		return service.RecordVaccineInput{}, err
	}
	administeredAt, err := uc.requireUserTime(args, "date")
	if err != nil {
		return service.RecordVaccineInput{}, err
	}
	var recurrence *int
	if v, ok := args["recurrence_days"]; ok {
		n, err := numberToInt(v, "recurrence_days")
		if err != nil {
			return service.RecordVaccineInput{}, err
		}
		recurrence = &n
	}
	return service.RecordVaccineInput{
		PetID:          petID,
		Name:           name,
		AdministeredAt: administeredAt,
		RecurrenceDays: recurrence,
		VetName:        optionalString(args, "vet_name"),
		BatchNumber:    optionalString(args, "batch_number"),
		Notes:          optionalString(args, "notes"),
	}, nil
}

func (uc *AgentUseCase) decodeAppointment(args map[string]any) (service.CreateAppointmentInput, error) {
	petID, err := requireString(args, "pet_id")
	if err != nil {
		return service.CreateAppointmentInput{}, err
	}
	typeText, err := requireString(args, "type")
	if err != nil {
		return service.CreateAppointmentInput{}, err
	}
	appointmentType := domain.AppointmentType(typeText)
	switch appointmentType {
	case domain.AppointmentTypeVet, domain.AppointmentTypeGrooming, domain.AppointmentTypeOther:
	default:
		return service.CreateAppointmentInput{}, fmt.Errorf("type must be one of: vet, grooming, other")
	}
	scheduledAt, err := uc.requireUserTime(args, "scheduled_at")
	if err != nil {
		return service.CreateAppointmentInput{}, err
	}
	return service.CreateAppointmentInput{
		PetID:       petID,
		Type:        appointmentType,
		ScheduledAt: scheduledAt,
		Provider:    optionalString(args, "provider"),
		Location:    optionalString(args, "location"),
		Notes:       optionalString(args, "notes"),
	}, nil
}

func (uc *AgentUseCase) decodeTreatment(args map[string]any) (service.CreateTreatmentInput, error) {
	petID, err := requireString(args, "pet_id")
	if err != nil {
		return service.CreateTreatmentInput{}, err
	}
	name, err := requireString(args, "name")
	if err != nil {
		return service.CreateTreatmentInput{}, err
	}
	amount, err := requireFloat(args, "dosage_amount")
	if err != nil {
		return service.CreateTreatmentInput{}, err
	}
	unit, err := requireString(args, "dosage_unit")
	if err != nil {
		return service.CreateTreatmentInput{}, err
	}
	route, err := requireString(args, "route")
	if err != nil {
		return service.CreateTreatmentInput{}, err
	}
	interval, err := requireInt(args, "interval_hours")
	if err != nil {
		return service.CreateTreatmentInput{}, err
	}
	startedAt, err := uc.requireUserTime(args, "started_at")
	if err != nil {
		return service.CreateTreatmentInput{}, err
	}
	var endedAt *time.Time
	if v, ok := args["ended_at"]; ok && v != nil && strings.TrimSpace(fmt.Sprint(v)) != "" {
		t, err := uc.parseUserTime(fmt.Sprint(v), "ended_at")
		if err != nil {
			return service.CreateTreatmentInput{}, err
		}
		endedAt = &t
	}
	return service.CreateTreatmentInput{
		PetID:         petID,
		Name:          name,
		DosageAmount:  amount,
		DosageUnit:    unit,
		Route:         route,
		IntervalHours: interval,
		StartedAt:     startedAt,
		EndedAt:       endedAt,
		VetName:       optionalString(args, "vet_name"),
		Notes:         optionalString(args, "notes"),
	}, nil
}

func (uc *AgentUseCase) decodeCreateSupply(args map[string]any) (service.CreateSupplyInput, error) {
	petID, err := requireString(args, "pet_id")
	if err != nil {
		return service.CreateSupplyInput{}, err
	}
	name, err := requireString(args, "name")
	if err != nil {
		return service.CreateSupplyInput{}, err
	}
	lastPurchasedAt, err := requireDate(args, "last_purchased_at")
	if err != nil {
		return service.CreateSupplyInput{}, err
	}
	estimated, err := requireInt(args, "estimated_days_supply")
	if err != nil {
		return service.CreateSupplyInput{}, err
	}
	return service.CreateSupplyInput{
		PetID:               petID,
		Name:                name,
		LastPurchasedAt:     lastPurchasedAt,
		EstimatedDaysSupply: estimated,
		Notes:               optionalString(args, "notes"),
	}, nil
}

func decodeUpdateSupply(args map[string]any) (service.UpdateSupplyInput, error) {
	var in service.UpdateSupplyInput
	in.Name = optionalString(args, "name")
	if v, ok := args["last_purchased_at"]; ok && v != nil && strings.TrimSpace(fmt.Sprint(v)) != "" {
		t, err := parseDate(fmt.Sprint(v), "last_purchased_at")
		if err != nil {
			return service.UpdateSupplyInput{}, err
		}
		in.LastPurchasedAt = &t
	}
	if v, ok := args["estimated_days_supply"]; ok && v != nil {
		n, err := numberToInt(v, "estimated_days_supply")
		if err != nil {
			return service.UpdateSupplyInput{}, err
		}
		in.EstimatedDaysSupply = &n
	}
	in.Notes = optionalString(args, "notes")
	return in, nil
}

func (uc *AgentUseCase) requireUserTime(args map[string]any, key string) (time.Time, error) {
	text, err := requireString(args, key)
	if err != nil {
		return time.Time{}, err
	}
	return uc.parseUserTime(text, key)
}

func (uc *AgentUseCase) parseUserTime(text, key string) (time.Time, error) {
	t, err := timeutil.ParseUserTime(text, uc.timezone)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s: %w", key, err)
	}
	return t, nil
}

func requireTwoStrings(args map[string]any, a, b string) (first, second string, err error) {
	first, err = requireString(args, a)
	if err != nil {
		return "", "", err
	}
	second, err = requireString(args, b)
	if err != nil {
		return "", "", err
	}
	return first, second, nil
}

func requireString(args map[string]any, key string) (string, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return "", fmt.Errorf("%s is required", key)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", key)
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return s, nil
}

func optionalString(args map[string]any, key string) *string {
	v, ok := args[key]
	if !ok || v == nil {
		return nil
	}
	s, ok := v.(string)
	if !ok {
		s = fmt.Sprint(v)
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

func requireFloat(args map[string]any, key string) (float64, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return 0, fmt.Errorf("%s is required", key)
	}
	switch n := v.(type) {
	case float64:
		return n, nil
	case int:
		return float64(n), nil
	case json.Number:
		f, err := n.Float64()
		if err != nil {
			return 0, fmt.Errorf("%s must be a number: %w", key, err)
		}
		return f, nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
		if err != nil {
			return 0, fmt.Errorf("%s must be a number: %w", key, err)
		}
		return f, nil
	default:
		return 0, fmt.Errorf("%s must be a number", key)
	}
}

func requireInt(args map[string]any, key string) (int, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return 0, fmt.Errorf("%s is required", key)
	}
	return numberToInt(v, key)
}

func numberToInt(v any, key string) (int, error) {
	switch n := v.(type) {
	case float64:
		return int(n), nil
	case int:
		return n, nil
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return 0, fmt.Errorf("%s must be an integer: %w", key, err)
		}
		return int(i), nil
	case string:
		i, err := strconv.Atoi(strings.TrimSpace(n))
		if err != nil {
			return 0, fmt.Errorf("%s must be an integer: %w", key, err)
		}
		return i, nil
	default:
		return 0, fmt.Errorf("%s must be an integer", key)
	}
}

func requireDate(args map[string]any, key string) (time.Time, error) {
	text, err := requireString(args, key)
	if err != nil {
		return time.Time{}, err
	}
	return parseDate(text, key)
}

func parseDate(text, key string) (time.Time, error) {
	t, err := time.Parse("2006-01-02", text)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must be a date in YYYY-MM-DD format: %w", key, err)
	}
	return t, nil
}

func errorToolResult(call agentdomain.ToolCall, err error) agentdomain.ToolResult {
	return agentdomain.ToolResult{CallID: call.ID, Content: err.Error(), IsError: true}
}

func buildAgentTools() []agentdomain.Tool {
	return []agentdomain.Tool{
		tool("list_pets", "List every pet Rafael has registered.", objectSchema(nil, nil)),
		tool("get_pet", "Get one pet by pet_id.", objectSchema(properties("pet_id", "string"), []string{"pet_id"})),
		tool("list_vaccines", "List vaccine records for one pet.", objectSchema(properties("pet_id", "string"), []string{"pet_id"})),
		tool("list_treatments", "List treatments and dose events for one pet.", objectSchema(properties("pet_id", "string"), []string{"pet_id"})),
		tool("list_appointments", "List a pet's appointment history, including vet visits and grooming sessions such as banho, banho e tosa, tosa, or grooming. Use it for questions like quando foi o banho or quando foi a última consulta.", objectSchema(properties("pet_id", "string"), []string{"pet_id"})),
		tool("list_observations", "List observation history for one pet.", objectSchema(properties("pet_id", "string"), []string{"pet_id"})),
		tool("list_supplies", "List supply records for one pet.", objectSchema(properties("pet_id", "string"), []string{"pet_id"})),
		tool("get_supply", "Get one supply record by pet_id and supply_id.", objectSchema(properties("pet_id", "string", "supply_id", "string"), []string{"pet_id", "supply_id"})),
		tool("get_pet_summary", "Get the all-pets daily digest data with vaccines due soon, active treatments, upcoming appointments, recent observations, and supplies needing reorder. Use this for resumo diário, digest, pendências, or priorities across all pets.", objectSchema(nil, nil)),
		tool("send_telegram", "Send a plain-text Portuguese Telegram message to Rafael. Use after rendering the daily digest from get_pet_summary.", objectSchema(properties("message", "string"), []string{"message"})),
		tool("log_observation", "Create a new observation entry for one pet.", objectSchema(properties("pet_id", "string", "observed_at", "string", "description", "string"), []string{"pet_id", "observed_at", "description"})),
		tool("record_vaccine", "Record a vaccine administration for one pet.", objectSchema(properties("pet_id", "string", "name", "string", "date", "string", "recurrence_days", "integer", "vet_name", "string", "batch_number", "string", "notes", "string"), []string{"pet_id", "name", "date"})),
		tool("schedule_appointment", "Schedule a pet appointment. Use for vet visits and grooming sessions such as banho, banho e tosa, tosa, or grooming.", objectSchema(map[string]any{
			"pet_id": map[string]any{"type": "string"},
			"type": map[string]any{
				"type":        "string",
				"description": "Use vet for consulta veterinária, grooming for banho, banho e tosa, tosa, or grooming, and other for any other appointment.",
			},
			"scheduled_at": map[string]any{"type": "string"},
			"provider":     map[string]any{"type": "string"},
			"location":     map[string]any{"type": "string"},
			"notes":        map[string]any{"type": "string"},
		}, []string{"pet_id", "type", "scheduled_at"})),
		tool("start_treatment", "Start a pet treatment and create its dose schedule.", objectSchema(properties("pet_id", "string", "name", "string", "dosage_amount", "number", "dosage_unit", "string", "route", "string", "interval_hours", "integer", "started_at", "string", "ended_at", "string", "vet_name", "string", "notes", "string"), []string{"pet_id", "name", "dosage_amount", "dosage_unit", "route", "interval_hours", "started_at"})),
		tool("reschedule_appointment", "Move an existing appointment to a new time.", objectSchema(properties("pet_id", "string", "appointment_id", "string", "scheduled_at", "string"), []string{"pet_id", "appointment_id", "scheduled_at"})),
		tool("create_supply", "Create a supply record for one pet.", objectSchema(properties("pet_id", "string", "name", "string", "last_purchased_at", "string", "estimated_days_supply", "integer", "notes", "string"), []string{"pet_id", "name", "last_purchased_at", "estimated_days_supply"})),
		tool("update_supply", "Update a supply record for one pet.", objectSchema(properties("pet_id", "string", "supply_id", "string", "name", "string", "last_purchased_at", "string", "estimated_days_supply", "integer", "notes", "string"), []string{"pet_id", "supply_id"})),
	}
}

func tool(name, description string, schema map[string]any) agentdomain.Tool {
	return agentdomain.Tool{Name: name, Description: description, InputSchema: schema}
}

func objectSchema(props map[string]any, required []string) map[string]any {
	if props == nil {
		props = map[string]any{}
	}
	return map[string]any{"type": "object", "properties": props, "required": required}
}

func properties(kv ...string) map[string]any {
	out := make(map[string]any, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		out[kv[i]] = map[string]any{"type": kv[i+1]}
	}
	return out
}

var _ AgentRouter = (*agentservice.Router)(nil)
