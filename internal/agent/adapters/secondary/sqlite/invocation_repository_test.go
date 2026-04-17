package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rafaelsoares/alfredo/internal/agent/domain"
	"github.com/rafaelsoares/alfredo/internal/database"
)

func TestInvocationRepositoryCreatePersistsAuditRow(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "alfredo.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close db: %v", err)
		}
	}()

	msg := "boom"
	inv := domain.Invocation{
		ID:           "inv-1",
		InputText:    "entrada",
		ToolCalls:    []domain.ToolCall{{ID: "call-1", Name: "list_pets", Arguments: map[string]any{"pet_id": "pet-1"}}},
		FinalReply:   "resposta",
		InputTokens:  10,
		OutputTokens: 3,
		DurationMS:   25,
		Outcome:      domain.OutcomeLLMError,
		ErrorMessage: &msg,
		CreatedAt:    time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC),
	}
	if err := NewInvocationRepository(db).Create(context.Background(), inv); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	var toolCalls, errorMessage sql.NullString
	var outcome string
	if err := db.QueryRow(`SELECT tool_calls_json, outcome, error_message FROM agent_invocations WHERE id = ?`, "inv-1").Scan(&toolCalls, &outcome, &errorMessage); err != nil {
		t.Fatalf("query invocation: %v", err)
	}
	if !strings.Contains(toolCalls.String, "list_pets") {
		t.Fatalf("tool calls json = %q", toolCalls.String)
	}
	if outcome != string(domain.OutcomeLLMError) {
		t.Fatalf("outcome = %q", outcome)
	}
	if !errorMessage.Valid || errorMessage.String != msg {
		t.Fatalf("error message = %#v", errorMessage)
	}
}

func TestInvocationRepositoryCreateAllowsNullErrorMessage(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "alfredo.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close db: %v", err)
		}
	}()

	inv := domain.Invocation{
		ID:        "inv-2",
		InputText: "entrada",
		Outcome:   domain.OutcomeSuccess,
		CreatedAt: time.Now().UTC(),
	}
	if err := NewInvocationRepository(db).Create(context.Background(), inv); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	var errorMessage sql.NullString
	if err := db.QueryRow(`SELECT error_message FROM agent_invocations WHERE id = ?`, "inv-2").Scan(&errorMessage); err != nil {
		t.Fatalf("query invocation: %v", err)
	}
	if errorMessage.Valid {
		t.Fatalf("expected null error_message, got %q", errorMessage.String)
	}
}
