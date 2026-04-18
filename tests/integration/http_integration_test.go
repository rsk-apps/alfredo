package integration_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	agentdomain "github.com/rafaelsoares/alfredo/internal/agent/domain"
	agentport "github.com/rafaelsoares/alfredo/internal/agent/port"
	agentservice "github.com/rafaelsoares/alfredo/internal/agent/service"
	"github.com/rafaelsoares/alfredo/internal/database"
	"github.com/rafaelsoares/alfredo/internal/gcalendar"
	"github.com/rafaelsoares/alfredo/internal/httpserver"
	"github.com/rafaelsoares/alfredo/internal/telegram"
)

const testAPIKey = "integration-test-api-key-000000000000"

type fixture struct {
	t        *testing.T
	db       *sql.DB
	echo     *echo.Echo
	calendar *recordingCalendar
	telegram *recordingTelegram
}

func newFixture(t *testing.T) *fixture {
	return newFixtureWithAgent(t, nil, agentservice.RouterConfig{})
}

func newFixtureWithAgent(t *testing.T, llm agentport.LLMClient, routerCfg agentservice.RouterConfig) *fixture {
	t.Helper()

	db, err := database.Open(filepath.Join(t.TempDir(), "alfredo.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close test db: %v", err)
		}
	})

	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	calendar := &recordingCalendar{}
	telegramRecorder := &recordingTelegram{}
	e, err := httpserver.New(httpserver.Config{
		DB:                db,
		Calendar:          calendar,
		Telegram:          telegramRecorder,
		AgentLLM:          llm,
		AgentRouterConfig: routerCfg,
		APIKey:            testAPIKey,
		Location:          loc,
		Logger:            zap.NewNop(),
	})
	if err != nil {
		t.Fatalf("build server: %v", err)
	}
	return &fixture{t: t, db: db, echo: e, calendar: calendar, telegram: telegramRecorder}
}

type scriptedAgentLLM struct {
	mu      sync.Mutex
	outputs []agentport.LLMOutput
	err     error
	sleep   time.Duration
}

func (l *scriptedAgentLLM) Complete(ctx context.Context, _ agentport.LLMInput) (agentport.LLMOutput, error) {
	if l.sleep > 0 {
		select {
		case <-time.After(l.sleep):
		case <-ctx.Done():
			return agentport.LLMOutput{}, ctx.Err()
		}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.err != nil {
		return agentport.LLMOutput{}, l.err
	}
	if len(l.outputs) == 0 {
		return agentport.LLMOutput{}, nil
	}
	out := l.outputs[0]
	l.outputs = l.outputs[1:]
	return out, nil
}

type createdCalendar struct {
	Name string
	ID   string
}

type deletedCalendar struct {
	CalendarID string
}

type createdEvent struct {
	CalendarID string
	Event      gcalendar.Event
	Interval   int
	ID         string
	Recurring  bool
}

type stoppedRecurringEvent struct {
	CalendarID string
	EventID    string
	Until      time.Time
}

type deletedEvent struct {
	CalendarID string
	EventID    string
}

type recordingCalendar struct {
	mu sync.Mutex

	failCreateCalendar       error
	failDeleteCalendar       error
	failCreateEvent          error
	failCreateEventCall      int
	failCreateRecurringEvent error
	failStopRecurringEvent   error
	failDeleteEvent          error
	createdCalendars         []createdCalendar
	deletedCalendars         []deletedCalendar
	createdEvents            []createdEvent
	stoppedRecurringEvents   []stoppedRecurringEvent
	deletedEvents            []deletedEvent
	nextCalendarID           int
	nextEventID              int
	nextRecurringEventID     int
}

func (c *recordingCalendar) CreateCalendar(_ context.Context, name string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failCreateCalendar != nil {
		return "", c.failCreateCalendar
	}
	c.nextCalendarID++
	id := fmt.Sprintf("cal-%02d", c.nextCalendarID)
	c.createdCalendars = append(c.createdCalendars, createdCalendar{Name: name, ID: id})
	return id, nil
}

func (c *recordingCalendar) DeleteCalendar(_ context.Context, calendarID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failDeleteCalendar != nil {
		return c.failDeleteCalendar
	}
	c.deletedCalendars = append(c.deletedCalendars, deletedCalendar{CalendarID: calendarID})
	return nil
}

func (c *recordingCalendar) CreateEvent(_ context.Context, calendarID string, event gcalendar.Event) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	call := c.nextEventID + 1
	if c.failCreateEvent != nil && (c.failCreateEventCall == 0 || c.failCreateEventCall == call) {
		return "", c.failCreateEvent
	}
	c.nextEventID++
	id := fmt.Sprintf("evt-%02d", c.nextEventID)
	c.createdEvents = append(c.createdEvents, createdEvent{CalendarID: calendarID, Event: event, ID: id})
	return id, nil
}

func (c *recordingCalendar) UpdateEvent(_ context.Context, calendarID, eventID string, event gcalendar.Event) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.createdEvents = append(c.createdEvents, createdEvent{CalendarID: calendarID, Event: event, ID: eventID})
	return nil
}

func (c *recordingCalendar) CreateRecurringEvent(_ context.Context, calendarID string, event gcalendar.Event, intervalHours int) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failCreateRecurringEvent != nil {
		return "", c.failCreateRecurringEvent
	}
	c.nextRecurringEventID++
	id := fmt.Sprintf("series-%02d", c.nextRecurringEventID)
	c.createdEvents = append(c.createdEvents, createdEvent{CalendarID: calendarID, Event: event, Interval: intervalHours, ID: id, Recurring: true})
	return id, nil
}

func (c *recordingCalendar) StopRecurringEvent(_ context.Context, calendarID, eventID string, until time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failStopRecurringEvent != nil {
		return c.failStopRecurringEvent
	}
	c.stoppedRecurringEvents = append(c.stoppedRecurringEvents, stoppedRecurringEvent{CalendarID: calendarID, EventID: eventID, Until: until})
	return nil
}

func (c *recordingCalendar) DeleteEvent(_ context.Context, calendarID, eventID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failDeleteEvent != nil {
		return c.failDeleteEvent
	}
	c.deletedEvents = append(c.deletedEvents, deletedEvent{CalendarID: calendarID, EventID: eventID})
	return nil
}

type recordingTelegram struct {
	mu       sync.Mutex
	failSend error
	messages []telegram.Message
}

func (r *recordingTelegram) Send(_ context.Context, msg telegram.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.messages = append(r.messages, msg)
	if r.failSend != nil {
		return r.failSend
	}
	return nil
}

func (r *recordingTelegram) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.messages)
}

func (r *recordingTelegram) message(index int) telegram.Message {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.messages[index]
}

func TestHealthAuthAndInvalidPathValidation(t *testing.T) {
	fx := newFixture(t)

	rec := fx.doJSON(http.MethodGet, "/api/v1/health", nil, "")
	requireStatus(t, rec, http.StatusOK)
	var health struct {
		Status       string `json:"status"`
		Dependencies map[string]struct {
			Status string `json:"status"`
		} `json:"dependencies"`
	}
	decodeJSON(t, rec, &health)
	requireEqual(t, "healthy", health.Status, "health status")
	requireEqual(t, "up", health.Dependencies["sqlite"].Status, "sqlite health")

	rec = fx.doJSON(http.MethodGet, "/api/v1/pets", nil, "")
	requireStatus(t, rec, http.StatusUnauthorized)

	rec = fx.doJSON(http.MethodGet, "/api/v1/pets", nil, "bad")
	requireStatus(t, rec, http.StatusUnauthorized)

	rec = fx.doJSON(http.MethodGet, "/api/v1/pets", nil, "x-api-key")
	requireStatus(t, rec, http.StatusOK)

	rec = fx.doJSON(http.MethodGet, "/api/v1/pets/not-a-uuid/vaccines", nil, "bearer")
	requireStatus(t, rec, http.StatusBadRequest)
	requireEqual(t, 0, len(fx.calendar.createdEvents), "calendar event calls for invalid path")
}

func TestHealthProfileRouteIsProtectedAndPersists(t *testing.T) {
	fx := newFixture(t)

	rec := fx.doJSON(http.MethodGet, "/api/v1/health/profile", nil, "")
	requireStatus(t, rec, http.StatusUnauthorized)

	rec = fx.doJSON(http.MethodPut, "/api/v1/health/profile", map[string]any{
		"height_cm":  178.0,
		"birth_date": "1993-06-15",
		"sex":        "male",
	}, "bearer")
	requireStatus(t, rec, http.StatusOK)
	var created struct {
		ID        int     `json:"id"`
		HeightCM  float64 `json:"height_cm"`
		BirthDate string  `json:"birth_date"`
		Sex       string  `json:"sex"`
		CreatedAt string  `json:"created_at"`
		UpdatedAt string  `json:"updated_at"`
	}
	decodeJSON(t, rec, &created)
	requireEqual(t, 1, created.ID, "health profile id")
	requireEqual(t, 178.0, created.HeightCM, "health profile height")
	requireEqual(t, "1993-06-15", created.BirthDate, "health profile birth date")
	requireEqual(t, "male", created.Sex, "health profile sex")
	requireNonEmpty(t, created.CreatedAt, "health profile created_at")
	requireNonEmpty(t, created.UpdatedAt, "health profile updated_at")

	rec = fx.doJSON(http.MethodGet, "/api/v1/health/profile", nil, "bearer")
	requireStatus(t, rec, http.StatusOK)
	var fetched struct {
		ID        int     `json:"id"`
		HeightCM  float64 `json:"height_cm"`
		BirthDate string  `json:"birth_date"`
		Sex       string  `json:"sex"`
	}
	decodeJSON(t, rec, &fetched)
	requireEqual(t, 178.0, fetched.HeightCM, "fetched health profile height")
	requireEqual(t, "1993-06-15", fetched.BirthDate, "fetched health profile birth date")
	requireEqual(t, "male", fetched.Sex, "fetched health profile sex")
}

func TestAgentSiriEndpointAuthValidationAndNoop(t *testing.T) {
	fx := newFixture(t)

	rec := fx.doJSON(http.MethodPost, "/api/v1/agent/siri", map[string]any{"text": "teste"}, "")
	requireStatus(t, rec, http.StatusUnauthorized)

	rec = fx.doJSON(http.MethodPost, "/api/v1/agent/siri", map[string]any{"text": ""}, "bearer")
	requireStatus(t, rec, http.StatusBadRequest)
	requireEqual(t, `{"error":"text is required"}`+"\n", rec.Body.String(), "empty text response")

	rec = fx.doJSON(http.MethodPost, "/api/v1/agent/siri", map[string]any{"text": strings.Repeat("a", 2001)}, "bearer")
	requireStatus(t, rec, http.StatusBadRequest)

	rec = fx.doJSON(http.MethodPost, "/api/v1/agent/siri", map[string]any{"text": "registra uma observacao para a Luna"}, "bearer")
	requireStatus(t, rec, http.StatusOK)
	var body struct {
		Reply string `json:"reply"`
	}
	decodeJSON(t, rec, &body)
	requireNonEmpty(t, body.Reply, "agent noop reply")
	requireEqual(t, 1, countRows(t, fx.db, "agent_invocations"), "agent invocation rows")
}

func TestAgentSiriEndpointDispatchesToolLoopAndAudits(t *testing.T) {
	llm := &scriptedAgentLLM{}
	fx := newFixtureWithAgent(t, llm, agentservice.RouterConfig{})
	pet := fx.createPet(map[string]any{"name": "Luna", "species": "dog"})
	llm.outputs = []agentport.LLMOutput{
		{ToolCalls: []agentdomain.ToolCall{{ID: "call-1", Name: "list_pets", Arguments: map[string]any{}}}},
		{ToolCalls: []agentdomain.ToolCall{{ID: "call-2", Name: "log_observation", Arguments: map[string]any{
			"pet_id":      pet.ID,
			"observed_at": "2026-04-17T15:00:00",
			"description": "teve diarreia",
		}}}},
		{FinalText: "Registrei uma observacao para Luna."},
	}

	rec := fx.doJSON(http.MethodPost, "/api/v1/agent/siri", map[string]any{"text": "registra que a Luna teve diarreia hoje as 15h"}, "bearer")
	requireStatus(t, rec, http.StatusOK)
	var body struct {
		Reply string `json:"reply"`
	}
	decodeJSON(t, rec, &body)
	requireEqual(t, "Registrei uma observacao para Luna.", body.Reply, "agent reply")
	requireEqual(t, 1, countRows(t, fx.db, "pet_observations"), "observation rows")
	requireEqual(t, 1, countRowsWhere(t, fx.db, "agent_invocations", "outcome = ?", "success"), "success audit rows")

	var toolCalls string
	if err := fx.db.QueryRow(`SELECT tool_calls_json FROM agent_invocations LIMIT 1`).Scan(&toolCalls); err != nil {
		t.Fatalf("query tool calls: %v", err)
	}
	requireContains(t, toolCalls, "log_observation", "audit tool calls")
}

func TestAgentSiriEndpointRecordsIterationCapTimeoutAndLLMError(t *testing.T) {
	t.Run("iteration cap", func(t *testing.T) {
		llm := &scriptedAgentLLM{outputs: []agentport.LLMOutput{
			{ToolCalls: []agentdomain.ToolCall{{ID: "call-1", Name: "list_pets"}}},
		}}
		fx := newFixtureWithAgent(t, llm, agentservice.RouterConfig{MaxIterations: 1, TotalTimeout: time.Second, CallTimeout: time.Second})
		rec := fx.doJSON(http.MethodPost, "/api/v1/agent/siri", map[string]any{"text": "listar pets"}, "bearer")
		requireStatus(t, rec, http.StatusOK)
		requireEqual(t, 1, countRowsWhere(t, fx.db, "agent_invocations", "outcome = ?", "iteration_cap_hit"), "iteration cap audit rows")
		requireContains(t, rec.Body.String(), agentservice.IterationCapReply, "iteration cap reply")
	})

	t.Run("timeout", func(t *testing.T) {
		llm := &scriptedAgentLLM{sleep: 50 * time.Millisecond}
		fx := newFixtureWithAgent(t, llm, agentservice.RouterConfig{MaxIterations: 5, TotalTimeout: 10 * time.Millisecond, CallTimeout: 10 * time.Millisecond})
		rec := fx.doJSON(http.MethodPost, "/api/v1/agent/siri", map[string]any{"text": "listar pets"}, "bearer")
		requireStatus(t, rec, http.StatusOK)
		requireEqual(t, 1, countRowsWhere(t, fx.db, "agent_invocations", "outcome = ?", "timeout"), "timeout audit rows")
		requireContains(t, rec.Body.String(), agentservice.TimeoutReply, "timeout reply")
	})

	t.Run("llm error", func(t *testing.T) {
		llm := &scriptedAgentLLM{err: errors.New("llm unavailable")}
		fx := newFixtureWithAgent(t, llm, agentservice.RouterConfig{})
		rec := fx.doJSON(http.MethodPost, "/api/v1/agent/siri", map[string]any{"text": "listar pets"}, "bearer")
		requireStatus(t, rec, http.StatusOK)
		requireEqual(t, 1, countRowsWhere(t, fx.db, "agent_invocations", "outcome = ? AND error_message IS NOT NULL", "llm_error"), "llm error audit rows")
		requireContains(t, rec.Body.String(), agentservice.LLMErrorReply, "llm error reply")
	})
}

func TestPetLifecyclePersistsFieldsAndCalendarSideEffects(t *testing.T) {
	fx := newFixture(t)
	pet := fx.createPet(map[string]any{
		"name":             "Luna",
		"species":          "dog",
		"breed":            "Golden Retriever",
		"birth_date":       "2020-01-02",
		"weight_kg":        24.5,
		"daily_food_grams": 320,
		"photo_path":       "/pets/luna.jpg",
	})

	requireEqual(t, "cal-01", pet.GoogleCalendarID, "response calendar id")
	requireEqual(t, 1, len(fx.calendar.createdCalendars), "created calendar count")
	requireEqual(t, "Luna", fx.calendar.createdCalendars[0].Name, "created calendar name")
	requireEqual(t, 0, fx.telegram.count(), "telegram messages after pet create")

	row := queryPet(t, fx.db, pet.ID)
	requireEqual(t, "Luna", row.Name, "db pet name")
	requireEqual(t, "dog", row.Species, "db pet species")
	requireEqual(t, "Golden Retriever", row.Breed.String, "db pet breed")
	requireEqual(t, "2020-01-02", row.BirthDate.String, "db pet birth date")
	requireEqual(t, 24.5, row.WeightKg.Float64, "db pet weight")
	requireEqual(t, 320.0, row.DailyFoodGrams.Float64, "db pet daily food")
	requireEqual(t, "/pets/luna.jpg", row.PhotoPath.String, "db pet photo")
	requireEqual(t, "cal-01", row.GoogleCalendarID, "db pet calendar id")
	requireNonEmpty(t, row.CreatedAt, "db pet created_at")

	rec := fx.doJSON(http.MethodGet, "/api/v1/pets/"+pet.ID, nil, "bearer")
	requireStatus(t, rec, http.StatusOK)
	var fetched petResponse
	decodeJSON(t, rec, &fetched)
	requireEqual(t, pet.ID, fetched.ID, "fetched pet id")
	requireEqual(t, "cal-01", fetched.GoogleCalendarID, "fetched calendar id")

	rec = fx.doJSON(http.MethodGet, "/api/v1/pets", nil, "bearer")
	requireStatus(t, rec, http.StatusOK)
	var listed []petResponse
	decodeJSON(t, rec, &listed)
	requireEqual(t, 1, len(listed), "listed pet count")
	requireEqual(t, "cal-01", listed[0].GoogleCalendarID, "listed calendar id")

	rec = fx.doJSON(http.MethodPut, "/api/v1/pets/"+pet.ID, map[string]any{
		"name":             "Luna Updated",
		"species":          "dog",
		"breed":            "Border Collie",
		"birth_date":       "2021-03-04",
		"weight_kg":        20.2,
		"daily_food_grams": 280,
		"photo_path":       "/pets/luna-updated.jpg",
	}, "bearer")
	requireStatus(t, rec, http.StatusOK)
	var updated petResponse
	decodeJSON(t, rec, &updated)
	requireEqual(t, "Luna Updated", updated.Name, "updated pet name")
	requireEqual(t, "cal-01", updated.GoogleCalendarID, "updated pet calendar id")
	requireEqual(t, 1, len(fx.calendar.createdCalendars), "calendar creates after update")
	requireEqual(t, 0, len(fx.calendar.deletedCalendars), "calendar deletes after update")

	rec = fx.doJSON(http.MethodDelete, "/api/v1/pets/"+pet.ID, nil, "bearer")
	requireStatus(t, rec, http.StatusNoContent)
	requireEqual(t, 1, len(fx.calendar.deletedCalendars), "deleted calendar count")
	requireEqual(t, "cal-01", fx.calendar.deletedCalendars[0].CalendarID, "deleted calendar id")
	requireEqual(t, 0, countRows(t, fx.db, "pets"), "pet rows after delete")
	requireEqual(t, 0, fx.telegram.count(), "telegram messages after pet delete")
}

func TestPetCalendarFailuresPreserveTransactionalState(t *testing.T) {
	fx := newFixture(t)
	fx.calendar.failCreateCalendar = errors.New("calendar unavailable")
	rec := fx.doJSON(http.MethodPost, "/api/v1/pets", map[string]any{"name": "Luna", "species": "dog"}, "bearer")
	requireStatus(t, rec, http.StatusInternalServerError)
	requireEqual(t, 0, countRows(t, fx.db, "pets"), "pet rows after calendar create failure")

	fx = newFixture(t)
	pet := fx.createPet(map[string]any{"name": "Luna", "species": "dog"})
	fx.calendar.failDeleteCalendar = errors.New("calendar delete failed")
	rec = fx.doJSON(http.MethodDelete, "/api/v1/pets/"+pet.ID, nil, "bearer")
	requireStatus(t, rec, http.StatusInternalServerError)
	requireEqual(t, 1, countRows(t, fx.db, "pets"), "pet rows after calendar delete failure")
	requireEqual(t, "cal-01", queryPet(t, fx.db, pet.ID).GoogleCalendarID, "preserved pet calendar id")
}

func TestVaccineLifecyclePersistsFieldsAndCalendarSideEffects(t *testing.T) {
	fx := newFixture(t)
	pet := fx.createPet(map[string]any{"name": "Luna", "species": "dog"})

	rec := fx.doJSON(http.MethodPost, "/api/v1/pets/"+pet.ID+"/vaccines", map[string]any{
		"name":         "Rabies",
		"date":         "2026-05-10T09:30:00",
		"vet_name":     "Dr. Ana",
		"batch_number": "B-123",
		"notes":        "left shoulder",
	}, "bearer")
	requireStatus(t, rec, http.StatusCreated)
	var vaccine vaccineResponse
	decodeJSON(t, rec, &vaccine)
	requireEqual(t, "evt-01", vaccine.GoogleCalendarEventID, "vaccine event id")
	requireEqual(t, 1, len(fx.calendar.createdEvents), "created vaccine event count")
	requireEqual(t, "cal-01", fx.calendar.createdEvents[0].CalendarID, "vaccine event calendar id")
	requireEqual(t, "Rabies", fx.calendar.createdEvents[0].Event.Title, "vaccine event title")
	requireEqual(t, "Pet: Luna", fx.calendar.createdEvents[0].Event.Description, "vaccine event description")
	requireEqual(t, 0, fx.calendar.createdEvents[0].Event.ReminderMin, "vaccine event reminder")
	requireEqual(t, "America/Sao_Paulo", fx.calendar.createdEvents[0].Event.TimeZone, "vaccine event timezone")
	requireEqual(t, 1, fx.telegram.count(), "telegram message count after vaccine create")
	assertTelegramHTML(t, fx.telegram.message(0), []string{
		"<b>💉 Vacina registrada</b>",
		"<b>Pet:</b> Luna",
		"<b>Vacina:</b> Rabies",
		"<b>Aplicada em:</b> 10/05/2026 09:30",
		"<b>Próxima dose:</b> não configurada",
		"<b>Veterinário:</b> Dr. Ana",
		"<b>Lote:</b> B-123",
		"<b>Observações:</b> left shoulder",
	})

	row := queryVaccine(t, fx.db, vaccine.ID)
	requireEqual(t, pet.ID, row.PetID, "db vaccine pet id")
	requireEqual(t, "Rabies", row.Name, "db vaccine name")
	requireEqual(t, "Dr. Ana", row.VetName.String, "db vaccine vet")
	requireEqual(t, "B-123", row.BatchNumber.String, "db vaccine batch")
	requireEqual(t, "left shoulder", row.Notes.String, "db vaccine notes")
	requireEqual(t, "evt-01", row.GoogleCalendarEventID, "db vaccine event id")
	requireEqual(t, "", row.GoogleCalendarNextDueEventID, "db vaccine next due event id")

	rec = fx.doJSON(http.MethodPost, "/api/v1/pets/"+pet.ID+"/vaccines", map[string]any{
		"name":            "V10",
		"date":            "2026-06-01T08:00:00-03:00",
		"recurrence_days": 365,
	}, "bearer")
	requireStatus(t, rec, http.StatusCreated)
	var recurring vaccineResponse
	decodeJSON(t, rec, &recurring)
	requireEqual(t, "2027-06-01", *recurring.NextDueAt, "recurring vaccine next due")
	requireEqual(t, "evt-02", recurring.GoogleCalendarEventID, "recurring vaccine event id")
	requireEqual(t, "evt-03", recurring.GoogleCalendarNextDueEventID, "recurring vaccine next due event id")
	requireEqual(t, 3, len(fx.calendar.createdEvents), "vaccine event count after recurrence")
	requireEqual(t, 0, fx.calendar.createdEvents[1].Event.ReminderMin, "recurring vaccine administered reminder")
	requireEqual(t, "V10", fx.calendar.createdEvents[1].Event.Title, "recurring vaccine administered title")
	requireEqual(t, "2026-06-01T08:00:00-03:00", fx.calendar.createdEvents[1].Event.StartTime.Format(time.RFC3339), "recurring vaccine administered event time")
	requireEqual(t, 7*24*60, fx.calendar.createdEvents[2].Event.ReminderMin, "recurring vaccine next due reminder")
	requireEqual(t, "Next due: V10", fx.calendar.createdEvents[2].Event.Title, "recurring vaccine next due event title")
	requireEqual(t, "2027-06-01T08:00:00-03:00", fx.calendar.createdEvents[2].Event.StartTime.Format(time.RFC3339), "recurring vaccine next due event time")
	requireEqual(t, 2, fx.telegram.count(), "telegram message count after recurring vaccine create")
	assertTelegramHTML(t, fx.telegram.message(1), []string{
		"<b>💉 Vacina registrada</b>",
		"<b>Pet:</b> Luna",
		"<b>Vacina:</b> V10",
		"<b>Aplicada em:</b> 01/06/2026 08:00",
		"<b>Próxima dose:</b> 01/06/2027 08:00",
	})

	recurringRow := queryVaccine(t, fx.db, recurring.ID)
	requireEqual(t, "evt-02", recurringRow.GoogleCalendarEventID, "db recurring vaccine event id")
	requireEqual(t, "evt-03", recurringRow.GoogleCalendarNextDueEventID, "db recurring vaccine next due event id")

	rec = fx.doJSON(http.MethodGet, "/api/v1/pets/"+pet.ID+"/vaccines", nil, "bearer")
	requireStatus(t, rec, http.StatusOK)
	var listed []vaccineResponse
	decodeJSON(t, rec, &listed)
	requireEqual(t, 2, len(listed), "listed vaccine count")

	rec = fx.doJSON(http.MethodDelete, "/api/v1/pets/"+pet.ID+"/vaccines/"+vaccine.ID, nil, "bearer")
	requireStatus(t, rec, http.StatusNoContent)
	requireEqual(t, 1, len(fx.calendar.deletedEvents), "deleted vaccine event count")
	requireEqual(t, "cal-01", fx.calendar.deletedEvents[0].CalendarID, "deleted vaccine event calendar id")
	requireEqual(t, "evt-01", fx.calendar.deletedEvents[0].EventID, "deleted vaccine event id")
	requireEqual(t, 1, countRows(t, fx.db, "vaccines"), "vaccine rows after delete")
	requireEqual(t, 3, fx.telegram.count(), "telegram message count after vaccine delete")
	assertTelegramHTML(t, fx.telegram.message(2), []string{
		"<b>🗑️ Vacina removida</b>",
		"<b>Pet:</b> Luna",
		"<b>Vacina:</b> Rabies",
		"<b>Aplicada em:</b> 10/05/2026 09:30",
	})
	requireNotContains(t, fx.telegram.message(2).Text, "Próxima dose removida", "non-recurring vaccine delete message")

	rec = fx.doJSON(http.MethodDelete, "/api/v1/pets/"+pet.ID+"/vaccines/"+recurring.ID, nil, "bearer")
	requireStatus(t, rec, http.StatusNoContent)
	requireEqual(t, 3, len(fx.calendar.deletedEvents), "deleted vaccine events after recurring delete")
	requireEqual(t, "evt-02", fx.calendar.deletedEvents[1].EventID, "deleted recurring administered event id")
	requireEqual(t, "evt-03", fx.calendar.deletedEvents[2].EventID, "deleted recurring next due event id")
	requireEqual(t, 0, countRows(t, fx.db, "vaccines"), "vaccine rows after recurring delete")
	requireEqual(t, 4, fx.telegram.count(), "telegram message count after recurring vaccine delete")
	assertTelegramHTML(t, fx.telegram.message(3), []string{
		"<b>🗑️ Vacina removida</b>",
		"<b>Pet:</b> Luna",
		"<b>Vacina:</b> V10",
		"<b>Próxima dose removida:</b> 01/06/2027",
	})
}

func TestVaccineFailuresAndValidationPreserveState(t *testing.T) {
	fx := newFixture(t)
	pet := fx.createPet(map[string]any{"name": "Luna", "species": "dog"})

	beforeEvents := len(fx.calendar.createdEvents)
	rec := fx.doJSON(http.MethodPost, "/api/v1/pets/"+pet.ID+"/vaccines", map[string]any{
		"name": "Rabies",
		"date": "2026-04-12",
	}, "bearer")
	requireStatus(t, rec, http.StatusBadRequest)
	requireEqual(t, 0, countRows(t, fx.db, "vaccines"), "vaccine rows after invalid date")
	requireEqual(t, beforeEvents, len(fx.calendar.createdEvents), "calendar events after invalid date")
	requireEqual(t, 0, fx.telegram.count(), "telegram messages after invalid vaccine date")

	fx.calendar.failCreateEvent = errors.New("calendar event failed")
	rec = fx.doJSON(http.MethodPost, "/api/v1/pets/"+pet.ID+"/vaccines", map[string]any{
		"name": "Rabies",
		"date": "2026-05-10T09:30:00",
	}, "bearer")
	requireStatus(t, rec, http.StatusInternalServerError)
	requireEqual(t, 0, countRows(t, fx.db, "vaccines"), "vaccine rows after calendar create failure")
	requireEqual(t, 0, fx.telegram.count(), "telegram messages after vaccine calendar create failure")

	fx = newFixture(t)
	pet = fx.createPet(map[string]any{"name": "Luna", "species": "dog"})
	fx.calendar.failCreateEvent = errors.New("next due calendar event failed")
	fx.calendar.failCreateEventCall = 2
	rec = fx.doJSON(http.MethodPost, "/api/v1/pets/"+pet.ID+"/vaccines", map[string]any{
		"name":            "V10",
		"date":            "2026-06-01T08:00:00-03:00",
		"recurrence_days": 365,
	}, "bearer")
	requireStatus(t, rec, http.StatusInternalServerError)
	requireEqual(t, 0, countRows(t, fx.db, "vaccines"), "vaccine rows after next due calendar create failure")
	requireEqual(t, 1, len(fx.calendar.createdEvents), "created administered event before next due failure")
	requireEqual(t, 1, len(fx.calendar.deletedEvents), "compensated administered event after next due failure")
	requireEqual(t, "evt-01", fx.calendar.deletedEvents[0].EventID, "compensated administered event id")
	requireEqual(t, 0, fx.telegram.count(), "telegram messages after next due calendar failure")

	fx = newFixture(t)
	pet = fx.createPet(map[string]any{"name": "Luna", "species": "dog"})
	vaccine := fx.createVaccine(pet.ID, map[string]any{"name": "Rabies", "date": "2026-05-10T09:30:00"})
	fx.calendar.failDeleteEvent = errors.New("calendar delete failed")
	rec = fx.doJSON(http.MethodDelete, "/api/v1/pets/"+pet.ID+"/vaccines/"+vaccine.ID, nil, "bearer")
	requireStatus(t, rec, http.StatusInternalServerError)
	requireEqual(t, 1, countRows(t, fx.db, "vaccines"), "vaccine rows after calendar delete failure")
	requireEqual(t, "evt-01", queryVaccine(t, fx.db, vaccine.ID).GoogleCalendarEventID, "preserved vaccine event id")
	requireEqual(t, 1, fx.telegram.count(), "telegram messages only from vaccine setup after delete failure")
}

func TestFiniteTreatmentLifecyclePersistsDosesAndCalendarSideEffects(t *testing.T) {
	fx := newFixture(t)
	pet := fx.createPet(map[string]any{"name": "Luna", "species": "dog"})

	rec := fx.doJSON(http.MethodPost, "/api/v1/pets/"+pet.ID+"/treatments", map[string]any{
		"name":           "Amoxicillin",
		"dosage_amount":  1.5,
		"dosage_unit":    "ml",
		"route":          "oral",
		"interval_hours": 3,
		"started_at":     "2030-01-01T09:00:00",
		"ended_at":       "2030-01-01T18:00:00",
		"vet_name":       "Dr. Ana",
		"notes":          "with food",
	}, "bearer")
	requireStatus(t, rec, http.StatusCreated)
	var treatment treatmentResponse
	decodeJSON(t, rec, &treatment)
	requireEqual(t, 3, len(treatment.Doses), "finite treatment dose count")
	requireEqual(t, "", treatment.GoogleCalendarEventID, "finite treatment recurring event id")
	requireEqual(t, 3, len(fx.calendar.createdEvents), "finite treatment event count")
	for i, event := range fx.calendar.createdEvents {
		requireEqual(t, "cal-01", event.CalendarID, "finite dose event calendar id")
		requireEqual(t, fmt.Sprintf("%d/3 Amoxicillin", i+1), event.Event.Title, "finite dose event title")
		requireEqual(t, "Pet: Luna", event.Event.Description, "finite dose event description")
		requireEqual(t, "America/Sao_Paulo", event.Event.TimeZone, "finite dose event timezone")
		requireEqual(t, treatment.Doses[i].GoogleCalendarEventID, event.ID, "dose response event id")
	}
	requireEqual(t, 1, fx.telegram.count(), "telegram message count after finite treatment create")
	assertTelegramHTML(t, fx.telegram.message(0), []string{
		"<b>💊 Tratamento registrado</b>",
		"<b>Pet:</b> Luna",
		"<b>Tratamento:</b> Amoxicillin",
		"<b>Dose:</b> 1.5 ml",
		"<b>Via:</b> oral",
		"<b>Intervalo:</b> a cada 3 horas",
		"<b>Início:</b> 01/01/2030 09:00",
		"<b>Fim previsto:</b> 01/01/2030 18:00",
		"<b>Doses agendadas:</b> 3",
		"<b>Veterinário:</b> Dr. Ana",
		"<b>Observações:</b> with food",
	})

	row := queryTreatment(t, fx.db, treatment.ID)
	requireEqual(t, pet.ID, row.PetID, "db treatment pet id")
	requireEqual(t, "Amoxicillin", row.Name, "db treatment name")
	requireEqual(t, 1.5, row.DosageAmount, "db treatment dosage")
	requireEqual(t, "ml", row.DosageUnit, "db treatment unit")
	requireEqual(t, "oral", row.Route, "db treatment route")
	requireEqual(t, 3, row.IntervalHours, "db treatment interval")
	requireEqual(t, "Dr. Ana", row.VetName.String, "db treatment vet")
	requireEqual(t, "with food", row.Notes.String, "db treatment notes")
	requireEqual(t, "", row.GoogleCalendarEventID, "db finite treatment recurring id")
	requireEqual(t, 3, countRowsWhere(t, fx.db, "doses", "treatment_id = ?", treatment.ID), "db dose count")

	rec = fx.doJSON(http.MethodGet, "/api/v1/pets/"+pet.ID+"/treatments/"+treatment.ID, nil, "bearer")
	requireStatus(t, rec, http.StatusOK)
	var fetched treatmentResponse
	decodeJSON(t, rec, &fetched)
	requireEqual(t, treatment.ID, fetched.ID, "fetched treatment id")
	requireEqual(t, 3, len(fetched.Doses), "fetched dose count")

	rec = fx.doJSON(http.MethodGet, "/api/v1/pets/"+pet.ID+"/treatments", nil, "bearer")
	requireStatus(t, rec, http.StatusOK)
	var listed []treatmentResponse
	decodeJSON(t, rec, &listed)
	requireEqual(t, 1, len(listed), "listed treatment count")
	requireEqual(t, 3, len(listed[0].Doses), "listed treatment dose count")

	rec = fx.doJSON(http.MethodDelete, "/api/v1/pets/"+pet.ID+"/treatments/"+treatment.ID, nil, "bearer")
	requireStatus(t, rec, http.StatusNoContent)
	requireEqual(t, 3, len(fx.calendar.deletedEvents), "finite treatment deleted event count")
	requireEqual(t, 0, countRowsWhere(t, fx.db, "doses", "treatment_id = ?", treatment.ID), "future doses after stop")
	stopped := queryTreatment(t, fx.db, treatment.ID)
	requireEqual(t, true, stopped.StoppedAt.Valid, "finite treatment stopped_at set")
	requireEqual(t, 2, fx.telegram.count(), "telegram message count after finite treatment stop")
	assertTelegramHTML(t, fx.telegram.message(1), []string{
		"<b>⛔ Tratamento interrompido</b>",
		"<b>Pet:</b> Luna",
		"<b>Tratamento:</b> Amoxicillin",
		"<b>Interrompido em:</b>",
		"<b>Doses já ocorridas:</b> 0",
		"<b>Doses futuras removidas:</b> 3",
	})
}

func TestRecurringTreatmentLifecyclePersistsSeriesAndStopsCalendar(t *testing.T) {
	fx := newFixture(t)
	pet := fx.createPet(map[string]any{"name": "Luna", "species": "dog"})

	rec := fx.doJSON(http.MethodPost, "/api/v1/pets/"+pet.ID+"/treatments", map[string]any{
		"name":           "Daily med",
		"dosage_amount":  1,
		"dosage_unit":    "pill",
		"route":          "oral",
		"interval_hours": 24,
		"started_at":     "2030-02-01T08:00:00",
	}, "bearer")
	requireStatus(t, rec, http.StatusCreated)
	var treatment treatmentResponse
	decodeJSON(t, rec, &treatment)
	requireEqual(t, 0, len(treatment.Doses), "recurring treatment dose count")
	requireEqual(t, "series-01", treatment.GoogleCalendarEventID, "recurring treatment event id")
	requireEqual(t, 1, len(fx.calendar.createdEvents), "recurring event count")
	requireEqual(t, true, fx.calendar.createdEvents[0].Recurring, "recurring event flag")
	requireEqual(t, 24, fx.calendar.createdEvents[0].Interval, "recurring interval")
	requireEqual(t, "series-01", queryTreatment(t, fx.db, treatment.ID).GoogleCalendarEventID, "db recurring event id")
	requireEqual(t, 0, countRowsWhere(t, fx.db, "doses", "treatment_id = ?", treatment.ID), "recurring db dose count")
	requireEqual(t, 1, fx.telegram.count(), "telegram message count after recurring treatment create")
	assertTelegramHTML(t, fx.telegram.message(0), []string{
		"<b>💊 Tratamento contínuo registrado</b>",
		"<b>Pet:</b> Luna",
		"<b>Tratamento:</b> Daily med",
		"<b>Dose:</b> 1 pill",
		"<b>Via:</b> oral",
		"<b>Intervalo:</b> a cada 24 horas",
		"<b>Início:</b> 01/02/2030 08:00",
		"<b>Fim previsto:</b> sem data definida",
	})

	rec = fx.doJSON(http.MethodDelete, "/api/v1/pets/"+pet.ID+"/treatments/"+treatment.ID, nil, "bearer")
	requireStatus(t, rec, http.StatusNoContent)
	requireEqual(t, 1, len(fx.calendar.stoppedRecurringEvents), "stopped recurring count")
	requireEqual(t, "cal-01", fx.calendar.stoppedRecurringEvents[0].CalendarID, "stopped recurring calendar id")
	requireEqual(t, "series-01", fx.calendar.stoppedRecurringEvents[0].EventID, "stopped recurring event id")
	requireEqual(t, true, fx.calendar.stoppedRecurringEvents[0].Until.After(time.Time{}), "stopped recurring until set")
	requireEqual(t, true, queryTreatment(t, fx.db, treatment.ID).StoppedAt.Valid, "recurring treatment stopped_at set")
	requireEqual(t, 2, fx.telegram.count(), "telegram message count after recurring treatment stop")
	assertTelegramHTML(t, fx.telegram.message(1), []string{
		"<b>⛔ Tratamento contínuo interrompido</b>",
		"<b>Pet:</b> Luna",
		"<b>Tratamento:</b> Daily med",
		"<b>Interrompido em:</b>",
		"<b>Série recorrente:</b> encerrada no calendário",
	})
	requireNotContains(t, fx.telegram.message(1).Text, "Doses já ocorridas", "recurring treatment stop message")
	requireNotContains(t, fx.telegram.message(1).Text, "Doses futuras removidas", "recurring treatment stop message")
}

func TestTreatmentFailuresAndValidationPreserveState(t *testing.T) {
	fx := newFixture(t)
	pet := fx.createPet(map[string]any{"name": "Luna", "species": "dog"})

	rec := fx.doJSON(http.MethodPost, "/api/v1/pets/"+pet.ID+"/treatments", map[string]any{
		"name":           "Amoxicillin",
		"dosage_amount":  1,
		"dosage_unit":    "ml",
		"route":          "oral",
		"interval_hours": 8,
		"started_at":     "2026-04-12",
	}, "bearer")
	requireStatus(t, rec, http.StatusBadRequest)
	requireEqual(t, 0, countRows(t, fx.db, "treatments"), "treatment rows after invalid started_at")
	requireEqual(t, 0, len(fx.calendar.createdEvents), "calendar events after invalid started_at")
	requireEqual(t, 0, fx.telegram.count(), "telegram messages after invalid treatment started_at")

	fx.calendar.failCreateEvent = errors.New("dose calendar failed")
	rec = fx.doJSON(http.MethodPost, "/api/v1/pets/"+pet.ID+"/treatments", map[string]any{
		"name":           "Amoxicillin",
		"dosage_amount":  1,
		"dosage_unit":    "ml",
		"route":          "oral",
		"interval_hours": 8,
		"started_at":     "2030-03-01T08:00:00",
		"ended_at":       "2030-03-02T08:00:00",
	}, "bearer")
	requireStatus(t, rec, http.StatusInternalServerError)
	requireEqual(t, 0, countRows(t, fx.db, "treatments"), "treatment rows after finite calendar create failure")
	requireEqual(t, 0, countRows(t, fx.db, "doses"), "dose rows after finite calendar create failure")
	requireEqual(t, 0, fx.telegram.count(), "telegram messages after finite calendar create failure")

	fx = newFixture(t)
	pet = fx.createPet(map[string]any{"name": "Luna", "species": "dog"})
	fx.calendar.failCreateRecurringEvent = errors.New("recurring calendar failed")
	rec = fx.doJSON(http.MethodPost, "/api/v1/pets/"+pet.ID+"/treatments", map[string]any{
		"name":           "Daily med",
		"dosage_amount":  1,
		"dosage_unit":    "pill",
		"route":          "oral",
		"interval_hours": 24,
		"started_at":     "2030-04-01T08:00:00",
	}, "bearer")
	requireStatus(t, rec, http.StatusInternalServerError)
	requireEqual(t, 0, countRows(t, fx.db, "treatments"), "treatment rows after recurring calendar create failure")
	requireEqual(t, 0, fx.telegram.count(), "telegram messages after recurring calendar create failure")

	fx = newFixture(t)
	pet = fx.createPet(map[string]any{"name": "Luna", "species": "dog"})
	finite := fx.createTreatment(pet.ID, map[string]any{
		"name":           "Amoxicillin",
		"dosage_amount":  1,
		"dosage_unit":    "ml",
		"route":          "oral",
		"interval_hours": 12,
		"started_at":     "2030-05-01T08:00:00",
		"ended_at":       "2030-05-02T08:00:00",
	})
	fx.calendar.failDeleteEvent = errors.New("delete dose calendar failed")
	rec = fx.doJSON(http.MethodDelete, "/api/v1/pets/"+pet.ID+"/treatments/"+finite.ID, nil, "bearer")
	requireStatus(t, rec, http.StatusInternalServerError)
	requireEqual(t, 2, countRowsWhere(t, fx.db, "doses", "treatment_id = ?", finite.ID), "finite doses after delete event failure")
	requireEqual(t, false, queryTreatment(t, fx.db, finite.ID).StoppedAt.Valid, "finite treatment stopped after delete event failure")
	requireEqual(t, 1, fx.telegram.count(), "telegram messages only from finite treatment setup after stop failure")

	fx = newFixture(t)
	pet = fx.createPet(map[string]any{"name": "Luna", "species": "dog"})
	recurring := fx.createTreatment(pet.ID, map[string]any{
		"name":           "Daily med",
		"dosage_amount":  1,
		"dosage_unit":    "pill",
		"route":          "oral",
		"interval_hours": 24,
		"started_at":     "2030-06-01T08:00:00",
	})
	fx.calendar.failStopRecurringEvent = errors.New("stop recurring failed")
	rec = fx.doJSON(http.MethodDelete, "/api/v1/pets/"+pet.ID+"/treatments/"+recurring.ID, nil, "bearer")
	requireStatus(t, rec, http.StatusInternalServerError)
	requireEqual(t, false, queryTreatment(t, fx.db, recurring.ID).StoppedAt.Valid, "recurring treatment stopped after stop event failure")
	requireEqual(t, 1, fx.telegram.count(), "telegram messages only from recurring treatment setup after stop failure")

	rec = fx.doJSON(http.MethodPost, "/api/v1/pets/"+pet.ID+"/treatments", map[string]any{
		"name":           "Bad date",
		"dosage_amount":  1,
		"dosage_unit":    "ml",
		"route":          "oral",
		"interval_hours": 8,
		"started_at":     "2030-07-01T08:00:00",
		"ended_at":       "2030-07-02",
	}, "bearer")
	requireStatus(t, rec, http.StatusBadRequest)
}

func TestObservationLifecyclePersistsAndSendsBestEffortTelegram(t *testing.T) {
	fx := newFixture(t)
	pet := fx.createPet(map[string]any{"name": "Luna", "species": "dog"})

	rec := fx.doJSON(http.MethodPost, "/api/v1/pets/"+pet.ID+"/observations", map[string]any{
		"observed_at": "2026-04-15T09:30:00",
		"description": "Vomited after breakfast",
	}, "bearer")
	requireStatus(t, rec, http.StatusCreated)
	var observation observationResponse
	decodeJSON(t, rec, &observation)
	requireNonEmpty(t, observation.ID, "observation id")
	requireEqual(t, pet.ID, observation.PetID, "observation pet id")
	requireEqual(t, "Vomited after breakfast", observation.Description, "observation description")
	requireEqual(t, "2026-04-15T09:30:00-03:00", observation.ObservedAt, "observation observed_at")
	requireNonEmpty(t, observation.CreatedAt, "observation created_at")
	requireEqual(t, 0, len(fx.calendar.createdEvents), "calendar events after observation create")
	requireEqual(t, 1, fx.telegram.count(), "telegram messages after observation create")
	assertTelegramHTML(t, fx.telegram.message(0), []string{
		"<b>Observação registrada</b>",
		"<b>Pet:</b> Luna",
		"<b>Observado em:</b> 15/04/2026 09:30",
		"<b>Descrição:</b> Vomited after breakfast",
	})

	row := queryObservation(t, fx.db, observation.ID)
	requireEqual(t, pet.ID, row.PetID, "db observation pet id")
	requireEqual(t, "2026-04-15T09:30:00-03:00", row.ObservedAt, "db observation observed_at")
	requireEqual(t, "Vomited after breakfast", row.Description, "db observation description")
	requireNonEmpty(t, row.CreatedAt, "db observation created_at")

	older := fx.createObservation(pet.ID, map[string]any{
		"observed_at": "2026-04-14T09:30:00-03:00",
		"description": "Seemed tired",
	})
	rec = fx.doJSON(http.MethodGet, "/api/v1/pets/"+pet.ID+"/observations", nil, "bearer")
	requireStatus(t, rec, http.StatusOK)
	var listed []observationResponse
	decodeJSON(t, rec, &listed)
	requireEqual(t, 2, len(listed), "listed observation count")
	requireEqual(t, observation.ID, listed[0].ID, "first listed observation id")
	requireEqual(t, older.ID, listed[1].ID, "second listed observation id")

	rec = fx.doJSON(http.MethodGet, "/api/v1/pets/"+pet.ID+"/observations/"+observation.ID, nil, "bearer")
	requireStatus(t, rec, http.StatusOK)
	var fetched observationResponse
	decodeJSON(t, rec, &fetched)
	requireEqual(t, observation.ID, fetched.ID, "fetched observation id")
}

func TestObservationAuthValidationAndNotFound(t *testing.T) {
	fx := newFixture(t)
	pet := fx.createPet(map[string]any{"name": "Luna", "species": "dog"})

	rec := fx.doJSON(http.MethodGet, "/api/v1/pets/"+pet.ID+"/observations", nil, "")
	requireStatus(t, rec, http.StatusUnauthorized)

	rec = fx.doJSON(http.MethodPost, "/api/v1/pets/not-a-uuid/observations", map[string]any{
		"observed_at": "2026-04-15T09:30:00",
		"description": "Vomited",
	}, "bearer")
	requireStatus(t, rec, http.StatusBadRequest)

	rec = fx.doJSON(http.MethodPost, "/api/v1/pets/"+pet.ID+"/observations", map[string]any{
		"observed_at": "2026-04-15",
		"description": "Vomited",
	}, "bearer")
	requireStatus(t, rec, http.StatusBadRequest)
	requireEqual(t, 0, countRows(t, fx.db, "pet_observations"), "observation rows after invalid date")

	rec = fx.doJSON(http.MethodPost, "/api/v1/pets/"+pet.ID+"/observations", map[string]any{
		"observed_at": "2026-04-15T09:30:00",
		"description": "",
	}, "bearer")
	requireStatus(t, rec, http.StatusBadRequest)
	requireEqual(t, 0, countRows(t, fx.db, "pet_observations"), "observation rows after empty description")

	rec = fx.doJSON(http.MethodGet, "/api/v1/pets/"+pet.ID+"/observations/not-a-uuid", nil, "bearer")
	requireStatus(t, rec, http.StatusBadRequest)

	rec = fx.doJSON(http.MethodGet, "/api/v1/pets/"+pet.ID+"/observations/123e4567-e89b-12d3-a456-426614174999", nil, "bearer")
	requireStatus(t, rec, http.StatusNotFound)
}

func TestObservationTelegramFailureDoesNotRollback(t *testing.T) {
	fx := newFixture(t)
	pet := fx.createPet(map[string]any{"name": "Luna", "species": "dog"})
	fx.telegram.failSend = errors.New("telegram unavailable")

	rec := fx.doJSON(http.MethodPost, "/api/v1/pets/"+pet.ID+"/observations", map[string]any{
		"observed_at": "2026-04-15T09:30:00",
		"description": "Vomited after breakfast",
	}, "bearer")
	requireStatus(t, rec, http.StatusCreated)
	var observation observationResponse
	decodeJSON(t, rec, &observation)
	requireEqual(t, 1, countRows(t, fx.db, "pet_observations"), "observation rows after telegram failure")
	requireEqual(t, 1, fx.telegram.count(), "telegram send attempts after observation create failure")
	requireEqual(t, "Vomited after breakfast", queryObservation(t, fx.db, observation.ID).Description, "persisted observation description")
}

func TestSupplyLifecyclePersistsAndComputesReorderDate(t *testing.T) {
	fx := newFixture(t)
	pet := fx.createPet(map[string]any{"name": "Luna", "species": "dog"})

	rec := fx.doJSON(http.MethodPost, "/api/v1/pets/"+pet.ID+"/supplies", map[string]any{
		"name":                  "Royal Canin Medium Adult",
		"last_purchased_at":     "2026-04-16",
		"estimated_days_supply": 30,
		"notes":                 "Comprar no Petlove",
	}, "bearer")
	requireStatus(t, rec, http.StatusCreated)
	var supply supplyResponse
	decodeJSON(t, rec, &supply)
	requireNonEmpty(t, supply.ID, "supply id")
	requireEqual(t, pet.ID, supply.PetID, "supply pet id")
	requireEqual(t, "Royal Canin Medium Adult", supply.Name, "supply name")
	requireEqual(t, "2026-04-16", supply.LastPurchasedAt, "supply last purchased")
	requireEqual(t, 30, supply.EstimatedDaysSupply, "supply estimated days")
	requireEqual(t, "2026-05-16", supply.NextReorderAt, "supply next reorder")
	requireEqual(t, "Comprar no Petlove", *supply.Notes, "supply notes")
	requireNonEmpty(t, supply.CreatedAt, "supply created_at")
	requireNonEmpty(t, supply.UpdatedAt, "supply updated_at")
	requireEqual(t, 0, len(fx.calendar.createdEvents), "calendar events after supply create")
	requireEqual(t, 0, fx.telegram.count(), "telegram messages after supply create")

	row := querySupply(t, fx.db, supply.ID)
	requireEqual(t, pet.ID, row.PetID, "db supply pet id")
	requireEqual(t, "Royal Canin Medium Adult", row.Name, "db supply name")
	requireEqual(t, "2026-04-16", row.LastPurchasedAt, "db supply last purchased")
	requireEqual(t, 30, row.EstimatedDaysSupply, "db supply estimated days")
	requireEqual(t, "Comprar no Petlove", row.Notes.String, "db supply notes")

	earlier := fx.createSupply(pet.ID, map[string]any{
		"name":                  "A Snack",
		"last_purchased_at":     "2026-04-20",
		"estimated_days_supply": 10,
	})
	sameReorder := fx.createSupply(pet.ID, map[string]any{
		"name":                  "A Food",
		"last_purchased_at":     "2026-04-16",
		"estimated_days_supply": 30,
	})

	rec = fx.doJSON(http.MethodGet, "/api/v1/pets/"+pet.ID+"/supplies", nil, "bearer")
	requireStatus(t, rec, http.StatusOK)
	var listed []supplyResponse
	decodeJSON(t, rec, &listed)
	requireEqual(t, 3, len(listed), "listed supply count")
	requireEqual(t, earlier.ID, listed[0].ID, "first listed supply id")
	requireEqual(t, sameReorder.ID, listed[1].ID, "second listed supply id")
	requireEqual(t, supply.ID, listed[2].ID, "third listed supply id")

	rec = fx.doJSON(http.MethodGet, "/api/v1/pets/"+pet.ID+"/supplies/"+supply.ID, nil, "bearer")
	requireStatus(t, rec, http.StatusOK)
	var fetched supplyResponse
	decodeJSON(t, rec, &fetched)
	requireEqual(t, supply.ID, fetched.ID, "fetched supply id")

	rec = fx.doJSON(http.MethodPatch, "/api/v1/pets/"+pet.ID+"/supplies/"+supply.ID, map[string]any{
		"last_purchased_at":     "2026-05-16",
		"estimated_days_supply": 45,
		"notes":                 "Novo pacote aberto",
	}, "bearer")
	requireStatus(t, rec, http.StatusOK)
	var updated supplyResponse
	decodeJSON(t, rec, &updated)
	requireEqual(t, "2026-05-16", updated.LastPurchasedAt, "updated last purchased")
	requireEqual(t, 45, updated.EstimatedDaysSupply, "updated estimated days")
	requireEqual(t, "2026-06-30", updated.NextReorderAt, "updated next reorder")
	requireEqual(t, "Novo pacote aberto", *updated.Notes, "updated notes")

	rec = fx.doJSON(http.MethodDelete, "/api/v1/pets/"+pet.ID+"/supplies/"+supply.ID, nil, "bearer")
	requireStatus(t, rec, http.StatusNoContent)
	rec = fx.doJSON(http.MethodGet, "/api/v1/pets/"+pet.ID+"/supplies/"+supply.ID, nil, "bearer")
	requireStatus(t, rec, http.StatusNotFound)
}

func TestSupplyAuthValidationAndPetScopedLookup(t *testing.T) {
	fx := newFixture(t)
	pet := fx.createPet(map[string]any{"name": "Luna", "species": "dog"})
	otherPet := fx.createPet(map[string]any{"name": "Nina", "species": "cat"})
	supply := fx.createSupply(pet.ID, map[string]any{
		"name":                  "Food",
		"last_purchased_at":     "2026-04-16",
		"estimated_days_supply": 30,
	})

	rec := fx.doJSON(http.MethodGet, "/api/v1/pets/"+pet.ID+"/supplies", nil, "")
	requireStatus(t, rec, http.StatusUnauthorized)

	rec = fx.doJSON(http.MethodGet, "/api/v1/pets/"+otherPet.ID+"/supplies/"+supply.ID, nil, "bearer")
	requireStatus(t, rec, http.StatusNotFound)

	rec = fx.doJSON(http.MethodPost, "/api/v1/pets/"+pet.ID+"/supplies", map[string]any{
		"name":                  "Bad Date",
		"last_purchased_at":     "2026-04-16T12:00:00",
		"estimated_days_supply": 30,
	}, "bearer")
	requireStatus(t, rec, http.StatusBadRequest)

	rec = fx.doJSON(http.MethodPost, "/api/v1/pets/123e4567-e89b-12d3-a456-426614174999/supplies", map[string]any{
		"name":                  "Food",
		"last_purchased_at":     "2026-04-16",
		"estimated_days_supply": 30,
	}, "bearer")
	requireStatus(t, rec, http.StatusNotFound)

	rec = fx.doJSON(http.MethodPost, "/api/v1/pets/"+pet.ID+"/supplies", map[string]any{
		"name":                  " ",
		"last_purchased_at":     "2026-04-16",
		"estimated_days_supply": 30,
	}, "bearer")
	requireStatus(t, rec, http.StatusBadRequest)

	rec = fx.doJSON(http.MethodPatch, "/api/v1/pets/"+pet.ID+"/supplies/"+supply.ID, map[string]any{
		"estimated_days_supply": 0,
	}, "bearer")
	requireStatus(t, rec, http.StatusBadRequest)

	rec = fx.doJSON(http.MethodGet, "/api/v1/pets/"+pet.ID+"/supplies/not-a-uuid", nil, "bearer")
	requireStatus(t, rec, http.StatusBadRequest)
}

func TestTelegramFailuresDoNotRollbackPetcareState(t *testing.T) {
	fx := newFixture(t)
	pet := fx.createPet(map[string]any{"name": "Luna", "species": "dog"})
	fx.telegram.failSend = errors.New("telegram unavailable")

	rec := fx.doJSON(http.MethodPost, "/api/v1/pets/"+pet.ID+"/vaccines", map[string]any{
		"name": "Rabies",
		"date": "2026-05-10T09:30:00",
	}, "bearer")
	requireStatus(t, rec, http.StatusCreated)
	var vaccine vaccineResponse
	decodeJSON(t, rec, &vaccine)
	requireEqual(t, 1, countRows(t, fx.db, "vaccines"), "vaccine rows after telegram failure")
	requireEqual(t, 1, fx.telegram.count(), "telegram send attempts after vaccine create failure")

	rec = fx.doJSON(http.MethodDelete, "/api/v1/pets/"+pet.ID+"/vaccines/"+vaccine.ID, nil, "bearer")
	requireStatus(t, rec, http.StatusNoContent)
	requireEqual(t, 0, countRows(t, fx.db, "vaccines"), "vaccine rows after telegram delete failure")
	requireEqual(t, 2, fx.telegram.count(), "telegram send attempts after vaccine delete failure")

	rec = fx.doJSON(http.MethodPost, "/api/v1/pets/"+pet.ID+"/treatments", map[string]any{
		"name":           "Amoxicillin",
		"dosage_amount":  1,
		"dosage_unit":    "ml",
		"route":          "oral",
		"interval_hours": 24,
		"started_at":     "2030-01-01T09:00:00",
		"ended_at":       "2030-01-03T09:00:00",
	}, "bearer")
	requireStatus(t, rec, http.StatusCreated)
	var treatment treatmentResponse
	decodeJSON(t, rec, &treatment)
	requireEqual(t, 1, countRows(t, fx.db, "treatments"), "treatment rows after telegram failure")
	requireEqual(t, 3, fx.telegram.count(), "telegram send attempts after treatment create failure")

	rec = fx.doJSON(http.MethodDelete, "/api/v1/pets/"+pet.ID+"/treatments/"+treatment.ID, nil, "bearer")
	requireStatus(t, rec, http.StatusNoContent)
	requireEqual(t, true, queryTreatment(t, fx.db, treatment.ID).StoppedAt.Valid, "treatment stopped after telegram failure")
	requireEqual(t, 4, fx.telegram.count(), "telegram send attempts after treatment stop failure")
}

func (fx *fixture) createPet(body map[string]any) petResponse {
	rec := fx.doJSON(http.MethodPost, "/api/v1/pets", body, "bearer")
	requireStatus(fx.t, rec, http.StatusCreated)
	var pet petResponse
	decodeJSON(fx.t, rec, &pet)
	return pet
}

func (fx *fixture) createVaccine(petID string, body map[string]any) vaccineResponse {
	rec := fx.doJSON(http.MethodPost, "/api/v1/pets/"+petID+"/vaccines", body, "bearer")
	requireStatus(fx.t, rec, http.StatusCreated)
	var vaccine vaccineResponse
	decodeJSON(fx.t, rec, &vaccine)
	return vaccine
}

func (fx *fixture) createTreatment(petID string, body map[string]any) treatmentResponse {
	rec := fx.doJSON(http.MethodPost, "/api/v1/pets/"+petID+"/treatments", body, "bearer")
	requireStatus(fx.t, rec, http.StatusCreated)
	var treatment treatmentResponse
	decodeJSON(fx.t, rec, &treatment)
	return treatment
}

func (fx *fixture) createObservation(petID string, body map[string]any) observationResponse {
	rec := fx.doJSON(http.MethodPost, "/api/v1/pets/"+petID+"/observations", body, "bearer")
	requireStatus(fx.t, rec, http.StatusCreated)
	var observation observationResponse
	decodeJSON(fx.t, rec, &observation)
	return observation
}

func (fx *fixture) createSupply(petID string, body map[string]any) supplyResponse {
	rec := fx.doJSON(http.MethodPost, "/api/v1/pets/"+petID+"/supplies", body, "bearer")
	requireStatus(fx.t, rec, http.StatusCreated)
	var supply supplyResponse
	decodeJSON(fx.t, rec, &supply)
	return supply
}

func (fx *fixture) doJSON(method, path string, body any, auth string) *httptest.ResponseRecorder {
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			panic(err)
		}
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	switch auth {
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+testAPIKey)
	case "x-api-key":
		req.Header.Set("X-Api-Key", testAPIKey)
	case "bad":
		req.Header.Set("Authorization", "Bearer wrong")
	}
	rec := httptest.NewRecorder()
	fx.echo.ServeHTTP(rec, req)
	return rec
}

type petResponse struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Species          string   `json:"species"`
	Breed            *string  `json:"breed"`
	BirthDate        *string  `json:"birth_date"`
	WeightKg         *float64 `json:"weight_kg"`
	DailyFoodGrams   *float64 `json:"daily_food_grams"`
	PhotoPath        *string  `json:"photo_path"`
	GoogleCalendarID string   `json:"google_calendar_id"`
	CreatedAt        string   `json:"created_at"`
}

type vaccineResponse struct {
	ID                           string  `json:"id"`
	PetID                        string  `json:"pet_id"`
	Name                         string  `json:"name"`
	Date                         string  `json:"date"`
	NextDueAt                    *string `json:"next_due_at"`
	VetName                      *string `json:"vet_name"`
	BatchNumber                  *string `json:"batch_number"`
	Notes                        *string `json:"notes"`
	GoogleCalendarEventID        string  `json:"google_calendar_event_id"`
	GoogleCalendarNextDueEventID string  `json:"google_calendar_next_due_event_id"`
}

type doseResponse struct {
	ID                    string `json:"id"`
	ScheduledFor          string `json:"scheduled_for"`
	GoogleCalendarEventID string `json:"google_calendar_event_id"`
}

type treatmentResponse struct {
	ID                    string         `json:"id"`
	PetID                 string         `json:"pet_id"`
	Name                  string         `json:"name"`
	DosageAmount          float64        `json:"dosage_amount"`
	DosageUnit            string         `json:"dosage_unit"`
	Route                 string         `json:"route"`
	IntervalHours         int            `json:"interval_hours"`
	StartedAt             string         `json:"started_at"`
	EndedAt               *string        `json:"ended_at"`
	StoppedAt             *string        `json:"stopped_at"`
	VetName               *string        `json:"vet_name"`
	Notes                 *string        `json:"notes"`
	GoogleCalendarEventID string         `json:"google_calendar_event_id"`
	CreatedAt             string         `json:"created_at"`
	Doses                 []doseResponse `json:"doses"`
}

type observationResponse struct {
	ID          string `json:"id"`
	PetID       string `json:"pet_id"`
	ObservedAt  string `json:"observed_at"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
}

type supplyResponse struct {
	ID                  string  `json:"id"`
	PetID               string  `json:"pet_id"`
	Name                string  `json:"name"`
	LastPurchasedAt     string  `json:"last_purchased_at"`
	EstimatedDaysSupply int     `json:"estimated_days_supply"`
	NextReorderAt       string  `json:"next_reorder_at"`
	Notes               *string `json:"notes"`
	CreatedAt           string  `json:"created_at"`
	UpdatedAt           string  `json:"updated_at"`
}

type petRow struct {
	ID               string
	Name             string
	Species          string
	Breed            sql.NullString
	BirthDate        sql.NullString
	WeightKg         sql.NullFloat64
	DailyFoodGrams   sql.NullFloat64
	PhotoPath        sql.NullString
	GoogleCalendarID string
	CreatedAt        string
}

type vaccineRow struct {
	ID                           string
	PetID                        string
	Name                         string
	AdministeredAt               string
	NextDueAt                    sql.NullString
	VetName                      sql.NullString
	BatchNumber                  sql.NullString
	Notes                        sql.NullString
	GoogleCalendarEventID        string
	GoogleCalendarNextDueEventID string
}

type treatmentRow struct {
	ID                    string
	PetID                 string
	Name                  string
	DosageAmount          float64
	DosageUnit            string
	Route                 string
	IntervalHours         int
	StartedAt             string
	EndedAt               sql.NullString
	StoppedAt             sql.NullString
	VetName               sql.NullString
	Notes                 sql.NullString
	GoogleCalendarEventID string
	CreatedAt             string
}

type observationRow struct {
	ID          string
	PetID       string
	ObservedAt  string
	Description string
	CreatedAt   string
}

type supplyRow struct {
	ID                  string
	PetID               string
	Name                string
	LastPurchasedAt     string
	EstimatedDaysSupply int
	Notes               sql.NullString
	CreatedAt           string
	UpdatedAt           string
}

func queryPet(t *testing.T, db *sql.DB, id string) petRow {
	t.Helper()
	var row petRow
	err := db.QueryRow(`
		SELECT id, name, species, breed, birth_date, weight_kg, daily_food_grams, photo_path, google_calendar_id, created_at
		FROM pets WHERE id = ?`, id,
	).Scan(&row.ID, &row.Name, &row.Species, &row.Breed, &row.BirthDate, &row.WeightKg, &row.DailyFoodGrams, &row.PhotoPath, &row.GoogleCalendarID, &row.CreatedAt)
	if err != nil {
		t.Fatalf("query pet %q: %v", id, err)
	}
	return row
}

func queryVaccine(t *testing.T, db *sql.DB, id string) vaccineRow {
	t.Helper()
	var row vaccineRow
	err := db.QueryRow(`
		SELECT id, pet_id, name, administered_at, next_due_at, vet_name, batch_number, notes, google_calendar_event_id, google_calendar_next_due_event_id
		FROM vaccines WHERE id = ?`, id,
	).Scan(&row.ID, &row.PetID, &row.Name, &row.AdministeredAt, &row.NextDueAt, &row.VetName, &row.BatchNumber, &row.Notes, &row.GoogleCalendarEventID, &row.GoogleCalendarNextDueEventID)
	if err != nil {
		t.Fatalf("query vaccine %q: %v", id, err)
	}
	return row
}

func queryTreatment(t *testing.T, db *sql.DB, id string) treatmentRow {
	t.Helper()
	var row treatmentRow
	err := db.QueryRow(`
		SELECT id, pet_id, name, dosage_amount, dosage_unit, route, interval_hours, started_at, ended_at, stopped_at, vet_name, notes, google_calendar_event_id, created_at
		FROM treatments WHERE id = ?`, id,
	).Scan(&row.ID, &row.PetID, &row.Name, &row.DosageAmount, &row.DosageUnit, &row.Route, &row.IntervalHours, &row.StartedAt, &row.EndedAt, &row.StoppedAt, &row.VetName, &row.Notes, &row.GoogleCalendarEventID, &row.CreatedAt)
	if err != nil {
		t.Fatalf("query treatment %q: %v", id, err)
	}
	return row
}

func queryObservation(t *testing.T, db *sql.DB, id string) observationRow {
	t.Helper()
	var row observationRow
	err := db.QueryRow(`
		SELECT id, pet_id, observed_at, description, created_at
		FROM pet_observations WHERE id = ?`, id,
	).Scan(&row.ID, &row.PetID, &row.ObservedAt, &row.Description, &row.CreatedAt)
	if err != nil {
		t.Fatalf("query observation %q: %v", id, err)
	}
	return row
}

func querySupply(t *testing.T, db *sql.DB, id string) supplyRow {
	t.Helper()
	var row supplyRow
	err := db.QueryRow(`
		SELECT id, pet_id, name, last_purchased_at, estimated_days_supply, notes, created_at, updated_at
		FROM supplies WHERE id = ?`, id,
	).Scan(&row.ID, &row.PetID, &row.Name, &row.LastPurchasedAt, &row.EstimatedDaysSupply, &row.Notes, &row.CreatedAt, &row.UpdatedAt)
	if err != nil {
		t.Fatalf("query supply %q: %v", id, err)
	}
	return row
}

func countRows(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	return countRowsWhere(t, db, table, "1=1")
}

func countRowsWhere(t *testing.T, db *sql.DB, table, where string, args ...any) int {
	t.Helper()
	var count int
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", table, where)
	if err := db.QueryRow(query, args...).Scan(&count); err != nil {
		t.Fatalf("count rows %s: %v", table, err)
	}
	return count
}

func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder, out any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
		t.Fatalf("decode response %q: %v", rec.Body.String(), err)
	}
}

type requireT interface {
	Helper()
	Fatalf(format string, args ...any)
}

func requireStatus(t requireT, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, want, rec.Body.String())
	}
}

func requireEqual[T comparable](t requireT, want, got T, label string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %#v, want %#v", label, got, want)
	}
}

func requireNonEmpty(t requireT, got, label string) {
	t.Helper()
	if got == "" {
		t.Fatalf("%s is empty", label)
	}
}

func assertTelegramHTML(t requireT, msg telegram.Message, wantParts []string) {
	t.Helper()
	requireEqual(t, telegram.ParseModeHTML, msg.ParseMode, "telegram parse mode")
	for _, part := range wantParts {
		requireContains(t, msg.Text, part, "telegram message")
	}
}

func requireContains(t requireT, got, want, label string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("%s = %q, want substring %q", label, got, want)
	}
}

func requireNotContains(t requireT, got, unwanted, label string) {
	t.Helper()
	if strings.Contains(got, unwanted) {
		t.Fatalf("%s = %q, did not expect substring %q", label, got, unwanted)
	}
}
