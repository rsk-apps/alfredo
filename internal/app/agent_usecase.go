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
	"github.com/rafaelsoares/alfredo/internal/timeutil"
)

const agentSystemPrompt = `Você é o Alfredo, assistente pessoal do Rafael para cuidados com os pets dele.
Sempre responda em português brasileiro de forma curta, direta e natural.
Use as ferramentas disponíveis para registrar ou consultar informações sobre os pets.
Para qualquer operação que envolva um pet específico, primeiro chame list_pets para resolver o identificador correto a partir do nome falado pelo Rafael.
Nunca invente identificadores. Se o pedido do Rafael for ambíguo ou estiver faltando informação essencial, retorne uma resposta curta pedindo esclarecimento em vez de chamar uma ferramenta.
Ao concluir uma ação, confirme brevemente o que foi feito, incluindo o nome do pet e a data/hora relevantes quando aplicável.`

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
	var result any
	switch call.Name {
	case "list_pets":
		pets, err := uc.pets.List(ctx)
		if err != nil {
			return errorToolResult(call, err), err
		}
		result = pets
	case "get_pet":
		petID, err := requireString(call.Arguments, "pet_id")
		if err != nil {
			return errorToolResult(call, err), err
		}
		pet, err := uc.pets.GetByID(ctx, petID)
		if err != nil {
			return errorToolResult(call, err), err
		}
		result = pet
	case "list_vaccines":
		petID, err := requireString(call.Arguments, "pet_id")
		if err != nil {
			return errorToolResult(call, err), err
		}
		vaccines, err := uc.vaccines.ListVaccines(ctx, petID)
		if err != nil {
			return errorToolResult(call, err), err
		}
		result = vaccines
	case "list_treatments":
		petID, err := requireString(call.Arguments, "pet_id")
		if err != nil {
			return errorToolResult(call, err), err
		}
		treatments, doses, err := uc.treatments.List(ctx, petID)
		result = map[string]any{"treatments": treatments, "doses": doses}
		if err != nil {
			return errorToolResult(call, err), err
		}
	case "list_appointments":
		petID, err := requireString(call.Arguments, "pet_id")
		if err != nil {
			return errorToolResult(call, err), err
		}
		appointments, err := uc.appointments.List(ctx, petID)
		if err != nil {
			return errorToolResult(call, err), err
		}
		result = appointments
	case "list_observations":
		petID, err := requireString(call.Arguments, "pet_id")
		if err != nil {
			return errorToolResult(call, err), err
		}
		observations, err := uc.observations.ListByPet(ctx, petID)
		if err != nil {
			return errorToolResult(call, err), err
		}
		result = observations
	case "list_supplies":
		petID, err := requireString(call.Arguments, "pet_id")
		if err != nil {
			return errorToolResult(call, err), err
		}
		supplies, err := uc.supplies.List(ctx, petID)
		if err != nil {
			return errorToolResult(call, err), err
		}
		result = supplies
	case "get_supply":
		petID, supplyID, err := requireTwoStrings(call.Arguments, "pet_id", "supply_id")
		if err != nil {
			return errorToolResult(call, err), err
		}
		supply, err := uc.supplies.GetByID(ctx, petID, supplyID)
		if err != nil {
			return errorToolResult(call, err), err
		}
		result = supply
	case "log_observation":
		in, err := uc.decodeObservation(call.Arguments)
		if err != nil {
			return errorToolResult(call, err), err
		}
		observation, err := uc.observations.Create(ctx, in)
		if err != nil {
			return errorToolResult(call, err), err
		}
		result = observation
	case "record_vaccine":
		in, err := uc.decodeVaccine(call.Arguments)
		if err != nil {
			return errorToolResult(call, err), err
		}
		vaccine, err := uc.vaccines.RecordVaccine(ctx, in)
		if err != nil {
			return errorToolResult(call, err), err
		}
		result = vaccine
	case "schedule_appointment":
		in, err := uc.decodeAppointment(call.Arguments)
		if err != nil {
			return errorToolResult(call, err), err
		}
		appointment, err := uc.appointments.Create(ctx, in)
		if err != nil {
			return errorToolResult(call, err), err
		}
		result = appointment
	case "start_treatment":
		in, err := uc.decodeTreatment(call.Arguments)
		if err != nil {
			return errorToolResult(call, err), err
		}
		treatment, doses, err := uc.treatments.Create(ctx, in)
		result = map[string]any{"treatment": treatment, "doses": doses}
		if err != nil {
			return errorToolResult(call, err), err
		}
	case "reschedule_appointment":
		petID, appointmentID, err := requireTwoStrings(call.Arguments, "pet_id", "appointment_id")
		if err != nil {
			return errorToolResult(call, err), err
		}
		scheduledAt, err := uc.requireUserTime(call.Arguments, "scheduled_at")
		if err != nil {
			return errorToolResult(call, err), err
		}
		appointment, err := uc.appointments.Update(ctx, petID, appointmentID, service.UpdateAppointmentInput{ScheduledAt: &scheduledAt})
		if err != nil {
			return errorToolResult(call, err), err
		}
		result = appointment
	case "create_supply":
		in, err := uc.decodeCreateSupply(call.Arguments)
		if err != nil {
			return errorToolResult(call, err), err
		}
		supply, err := uc.supplies.Create(ctx, in)
		if err != nil {
			return errorToolResult(call, err), err
		}
		result = supply
	case "update_supply":
		petID, supplyID, err := requireTwoStrings(call.Arguments, "pet_id", "supply_id")
		if err != nil {
			return errorToolResult(call, err), err
		}
		in, err := decodeUpdateSupply(call.Arguments)
		if err != nil {
			return errorToolResult(call, err), err
		}
		supply, err := uc.supplies.Update(ctx, petID, supplyID, in)
		if err != nil {
			return errorToolResult(call, err), err
		}
		result = supply
	default:
		err := fmt.Errorf("unknown tool %q", call.Name)
		return errorToolResult(call, err), err
	}
	content, err := json.Marshal(result)
	if err != nil {
		return errorToolResult(call, err), fmt.Errorf("marshal tool result for %q: %w", call.Name, err)
	}
	return agentdomain.ToolResult{CallID: call.ID, Content: string(content)}, nil
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

func requireTwoStrings(args map[string]any, a, b string) (string, string, error) {
	first, err := requireString(args, a)
	if err != nil {
		return "", "", err
	}
	second, err := requireString(args, b)
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
		tool("list_pets", "List all pets Rafael has registered.", objectSchema(nil, nil)),
		tool("get_pet", "Get a pet by pet_id.", objectSchema(properties("pet_id", "string"), []string{"pet_id"})),
		tool("list_vaccines", "List vaccines for a pet.", objectSchema(properties("pet_id", "string"), []string{"pet_id"})),
		tool("list_treatments", "List treatments and doses for a pet.", objectSchema(properties("pet_id", "string"), []string{"pet_id"})),
		tool("list_appointments", "List appointments for a pet.", objectSchema(properties("pet_id", "string"), []string{"pet_id"})),
		tool("list_observations", "List observations for a pet.", objectSchema(properties("pet_id", "string"), []string{"pet_id"})),
		tool("list_supplies", "List supplies for a pet.", objectSchema(properties("pet_id", "string"), []string{"pet_id"})),
		tool("get_supply", "Get a supply by pet_id and supply_id.", objectSchema(properties("pet_id", "string", "supply_id", "string"), []string{"pet_id", "supply_id"})),
		tool("log_observation", "Create an observation for a pet.", objectSchema(properties("pet_id", "string", "observed_at", "string", "description", "string"), []string{"pet_id", "observed_at", "description"})),
		tool("record_vaccine", "Record a vaccine for a pet.", objectSchema(properties("pet_id", "string", "name", "string", "date", "string", "recurrence_days", "integer", "vet_name", "string", "batch_number", "string", "notes", "string"), []string{"pet_id", "name", "date"})),
		tool("schedule_appointment", "Schedule a pet appointment.", objectSchema(properties("pet_id", "string", "type", "string", "scheduled_at", "string", "provider", "string", "location", "string", "notes", "string"), []string{"pet_id", "type", "scheduled_at"})),
		tool("start_treatment", "Start a pet treatment.", objectSchema(properties("pet_id", "string", "name", "string", "dosage_amount", "number", "dosage_unit", "string", "route", "string", "interval_hours", "integer", "started_at", "string", "ended_at", "string", "vet_name", "string", "notes", "string"), []string{"pet_id", "name", "dosage_amount", "dosage_unit", "route", "interval_hours", "started_at"})),
		tool("reschedule_appointment", "Reschedule an existing appointment.", objectSchema(properties("pet_id", "string", "appointment_id", "string", "scheduled_at", "string"), []string{"pet_id", "appointment_id", "scheduled_at"})),
		tool("create_supply", "Create a per-pet supply record.", objectSchema(properties("pet_id", "string", "name", "string", "last_purchased_at", "string", "estimated_days_supply", "integer", "notes", "string"), []string{"pet_id", "name", "last_purchased_at", "estimated_days_supply"})),
		tool("update_supply", "Update a per-pet supply record.", objectSchema(properties("pet_id", "string", "supply_id", "string", "name", "string", "last_purchased_at", "string", "estimated_days_supply", "integer", "notes", "string"), []string{"pet_id", "supply_id"})),
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
