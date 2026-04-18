package claude

import (
	"testing"

	"github.com/anthropics/anthropic-sdk-go"

	"github.com/rafaelsoares/alfredo/internal/agent/domain"
	"github.com/rafaelsoares/alfredo/internal/agent/port"
)

func TestToAnthropicToolsForwardsRequiredSchema(t *testing.T) {
	tools := []domain.Tool{
		{
			Name:        "record_vaccine",
			Description: "Record a vaccine.",
			InputSchema: map[string]any{
				"properties": map[string]any{
					"pet_id": map[string]any{"type": "string"},
					"name":   map[string]any{"type": "string"},
				},
				"required": []string{"pet_id", "name"},
			},
		},
	}

	got := toAnthropicTools(tools)
	if len(got) != 1 {
		t.Fatalf("tools len = %d, want 1", len(got))
	}
	tool := got[0].OfTool
	if tool == nil {
		t.Fatalf("converted tool is nil: %#v", got[0])
	}
	if tool.Name != "record_vaccine" {
		t.Fatalf("tool name = %q, want record_vaccine", tool.Name)
	}
	if tool.Description.Value != "Record a vaccine." {
		t.Fatalf("description = %q, want Record a vaccine.", tool.Description.Value)
	}
	if !sameStringSet(tool.InputSchema.Required, []string{"pet_id", "name"}) {
		t.Fatalf("required = %#v, want pet_id/name", tool.InputSchema.Required)
	}
	props, ok := tool.InputSchema.Properties.(map[string]any)
	if !ok {
		t.Fatalf("properties type = %T, want map[string]any", tool.InputSchema.Properties)
	}
	if _, ok := props["pet_id"]; !ok {
		t.Fatalf("properties missing pet_id: %#v", props)
	}
}

func TestToAnthropicMessagesMapsToolCallsAndResults(t *testing.T) {
	messages := []port.LLMMessage{
		{
			Role: "assistant",
			ToolCalls: []domain.ToolCall{
				{ID: "call-1", Name: "list_pets", Arguments: map[string]any{"pet_id": "pet-1"}},
			},
		},
		{
			Role: "user",
			ToolResults: []domain.ToolResult{
				{CallID: "call-1", Content: `{"ok":true}`, IsError: true},
			},
		},
	}

	got := toAnthropicMessages(messages)
	if len(got) != 2 {
		t.Fatalf("messages len = %d, want 2", len(got))
	}
	if got[0].Role != anthropic.MessageParamRoleAssistant {
		t.Fatalf("message 0 role = %q, want assistant", got[0].Role)
	}
	if len(got[0].Content) != 1 || got[0].Content[0].OfToolUse == nil {
		t.Fatalf("message 0 content = %#v, want one tool_use block", got[0].Content)
	}
	use := got[0].Content[0].OfToolUse
	if use.ID != "call-1" || use.Name != "list_pets" {
		t.Fatalf("tool use = %#v, want call-1/list_pets", use)
	}
	args, ok := use.Input.(map[string]any)
	if !ok || args["pet_id"] != "pet-1" {
		t.Fatalf("tool use input = %#v, want pet_id=pet-1", use.Input)
	}

	if got[1].Role != anthropic.MessageParamRoleUser {
		t.Fatalf("message 1 role = %q, want user", got[1].Role)
	}
	if len(got[1].Content) != 1 || got[1].Content[0].OfToolResult == nil {
		t.Fatalf("message 1 content = %#v, want one tool_result block", got[1].Content)
	}
	result := got[1].Content[0].OfToolResult
	if result.ToolUseID != "call-1" {
		t.Fatalf("tool result id = %q, want call-1", result.ToolUseID)
	}
	if !result.IsError.Valid() || !result.IsError.Value {
		t.Fatalf("tool result IsError = %#v, want true", result.IsError)
	}
	if len(result.Content) != 1 || result.Content[0].OfText == nil || result.Content[0].OfText.Text != `{"ok":true}` {
		t.Fatalf("tool result content = %#v, want JSON text", result.Content)
	}
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
