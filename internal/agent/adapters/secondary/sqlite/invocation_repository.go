package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rafaelsoares/alfredo/internal/agent/domain"
)

type InvocationRepository struct {
	db DBTX
}

func NewInvocationRepository(db DBTX) *InvocationRepository {
	return &InvocationRepository{db: db}
}

func (r *InvocationRepository) Create(ctx context.Context, inv domain.Invocation) error {
	toolCalls, err := json.Marshal(inv.ToolCalls)
	if err != nil {
		return fmt.Errorf("marshal agent tool calls: %w", err)
	}
	createdAt := inv.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	_, err = r.db.ExecContext(ctx, `
INSERT INTO agent_invocations (
	id, input_text, tool_calls_json, final_reply, input_tokens, output_tokens,
	duration_ms, outcome, error_message, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		inv.ID,
		inv.InputText,
		string(toolCalls),
		inv.FinalReply,
		inv.InputTokens,
		inv.OutputTokens,
		inv.DurationMS,
		string(inv.Outcome),
		inv.ErrorMessage,
		createdAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert agent invocation %q: %w", inv.ID, err)
	}
	return nil
}
