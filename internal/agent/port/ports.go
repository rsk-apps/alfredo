package port

import (
	"context"

	"github.com/rafaelsoares/alfredo/internal/agent/domain"
)

type LLMClient interface {
	Complete(ctx context.Context, in LLMInput) (LLMOutput, error)
}

type LLMInput struct {
	SystemPrompt    string
	Messages        []LLMMessage
	Tools           []domain.Tool
	MaxOutputTokens int
}

type LLMMessage struct {
	Role        string
	Text        string
	ToolCalls   []domain.ToolCall
	ToolResults []domain.ToolResult
}

type LLMOutput struct {
	FinalText    string
	ToolCalls    []domain.ToolCall
	InputTokens  int
	OutputTokens int
}

type InvocationRepository interface {
	Create(ctx context.Context, inv domain.Invocation) error
}
