package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"github.com/anthropics/anthropic-sdk-go/shared/constant"

	"github.com/rafaelsoares/alfredo/internal/agent/domain"
	"github.com/rafaelsoares/alfredo/internal/agent/port"
)

type Config struct {
	APIKey      string
	Model       string
	CallTimeout time.Duration
}

type Adapter struct {
	client anthropic.Client
	cfg    Config
}

func NewAdapter(cfg Config) (*Adapter, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("anthropic api key is required")
	}
	if cfg.Model == "" {
		return nil, fmt.Errorf("anthropic model is required")
	}
	return &Adapter{
		client: anthropic.NewClient(option.WithAPIKey(cfg.APIKey), option.WithMaxRetries(1)),
		cfg:    cfg,
	}, nil
}

func (a *Adapter) Complete(ctx context.Context, in port.LLMInput) (port.LLMOutput, error) {
	if a.cfg.CallTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, a.cfg.CallTimeout)
		defer cancel()
	}
	msg, err := a.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(a.cfg.Model),
		MaxTokens: int64(in.MaxOutputTokens),
		System:    []anthropic.TextBlockParam{{Text: in.SystemPrompt}},
		Messages:  toAnthropicMessages(in.Messages),
		Tools:     toAnthropicTools(in.Tools),
	})
	if err != nil {
		return port.LLMOutput{}, fmt.Errorf("claude message create: %w", err)
	}

	out := port.LLMOutput{
		InputTokens:  int(msg.Usage.InputTokens),
		OutputTokens: int(msg.Usage.OutputTokens),
	}
	for _, block := range msg.Content {
		switch block.Type {
		case "text":
			out.FinalText += block.AsText().Text
		case "tool_use":
			use := block.AsToolUse()
			args := map[string]any{}
			if len(use.Input) > 0 {
				if err := json.Unmarshal(use.Input, &args); err != nil {
					return port.LLMOutput{}, fmt.Errorf("decode claude tool input for %q: %w", use.Name, err)
				}
			}
			out.ToolCalls = append(out.ToolCalls, domain.ToolCall{
				ID:        use.ID,
				Name:      use.Name,
				Arguments: args,
			})
		}
	}
	return out, nil
}

func toAnthropicMessages(messages []port.LLMMessage) []anthropic.MessageParam {
	out := make([]anthropic.MessageParam, 0, len(messages))
	for _, msg := range messages {
		blocks := make([]anthropic.ContentBlockParamUnion, 0, 1+len(msg.ToolCalls)+len(msg.ToolResults))
		if msg.Text != "" {
			blocks = append(blocks, anthropic.NewTextBlock(msg.Text))
		}
		for _, call := range msg.ToolCalls {
			blocks = append(blocks, anthropic.NewToolUseBlock(call.ID, call.Arguments, call.Name))
		}
		for _, result := range msg.ToolResults {
			blocks = append(blocks, anthropic.NewToolResultBlock(result.CallID, result.Content, result.IsError))
		}
		role := anthropic.MessageParamRoleUser
		if msg.Role == "assistant" {
			role = anthropic.MessageParamRoleAssistant
		}
		out = append(out, anthropic.MessageParam{Role: role, Content: blocks})
	}
	return out
}

func toAnthropicTools(tools []domain.Tool) []anthropic.ToolUnionParam {
	out := make([]anthropic.ToolUnionParam, 0, len(tools))
	for _, tool := range tools {
		schema := anthropic.ToolInputSchemaParam{
			Type:       constant.Object(""),
			Properties: tool.InputSchema["properties"],
		}
		if req, ok := tool.InputSchema["required"].([]string); ok {
			schema.Required = req
		}
		paramTool := anthropic.ToolParam{
			Name:        tool.Name,
			Description: param.NewOpt(tool.Description),
			InputSchema: schema,
			Type:        anthropic.ToolTypeCustom,
		}
		out = append(out, anthropic.ToolUnionParam{OfTool: &paramTool})
	}
	return out
}
