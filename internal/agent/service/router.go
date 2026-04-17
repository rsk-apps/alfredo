package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/rafaelsoares/alfredo/internal/agent/domain"
	"github.com/rafaelsoares/alfredo/internal/agent/port"
)

const (
	IterationCapReply = "Desculpe, não consegui entender. Pode reformular?"
	TimeoutReply      = "Demorei demais. Tente novamente."
	LLMErrorReply     = "Desculpe, não consegui processar agora. Tente novamente."
	ToolErrorReply    = "Desculpe, não consegui executar esse pedido. Pode reformular?"
)

type RouterConfig struct {
	MaxIterations          int
	MaxOutputTokensPerCall int
	TotalTimeout           time.Duration
	CallTimeout            time.Duration
}

type Router struct {
	llm         port.LLMClient
	invocations port.InvocationRepository
	cfg         RouterConfig
	logger      *zap.Logger
}

func NewRouter(llm port.LLMClient, invocations port.InvocationRepository, cfg RouterConfig, logger *zap.Logger) *Router {
	if logger == nil {
		logger = zap.NewNop()
	}
	if cfg.MaxIterations <= 0 {
		cfg.MaxIterations = 5
	}
	if cfg.MaxOutputTokensPerCall <= 0 {
		cfg.MaxOutputTokensPerCall = 512
	}
	if cfg.TotalTimeout <= 0 {
		cfg.TotalTimeout = 20 * time.Second
	}
	if cfg.CallTimeout <= 0 {
		cfg.CallTimeout = 8 * time.Second
	}
	return &Router{llm: llm, invocations: invocations, cfg: cfg, logger: logger}
}

func (r *Router) Execute(
	ctx context.Context,
	systemPrompt string,
	tools []domain.Tool,
	inputText string,
	dispatch func(ctx context.Context, call domain.ToolCall) (domain.ToolResult, error),
) (string, domain.Invocation, error) {
	started := time.Now()
	inv := domain.Invocation{
		ID:        uuid.New().String(),
		InputText: inputText,
		Outcome:   domain.OutcomeSuccess,
		CreatedAt: started.UTC(),
	}
	messages := []port.LLMMessage{{Role: "user", Text: inputText}}
	runCtx, cancel := context.WithTimeout(ctx, r.cfg.TotalTimeout)
	defer cancel()

	finish := func(reply string, outcome domain.Outcome, err error) (string, domain.Invocation, error) {
		inv.FinalReply = reply
		inv.Outcome = outcome
		inv.DurationMS = int(time.Since(started).Milliseconds())
		if err != nil {
			msg := err.Error()
			if len(inv.ToolCalls) > 0 && outcome != domain.OutcomeSuccess {
				msg += "; one or more tools may already have changed state"
			}
			inv.ErrorMessage = &msg
		}
		if r.invocations != nil {
			if auditErr := r.invocations.Create(context.WithoutCancel(ctx), inv); auditErr != nil {
				r.logger.Warn("agent audit write failed", zap.Error(auditErr), zap.String("invocation_id", inv.ID))
			}
		}
		return reply, inv, err
	}

	for i := 0; i < r.cfg.MaxIterations; i++ {
		select {
		case <-runCtx.Done():
			return finish(TimeoutReply, domain.OutcomeTimeout, runCtx.Err())
		default:
		}

		callCtx, callCancel := context.WithTimeout(runCtx, r.cfg.CallTimeout)
		out, err := completeWithContext(callCtx, r.llm, port.LLMInput{
			SystemPrompt:    systemPrompt,
			Messages:        messages,
			Tools:           tools,
			MaxOutputTokens: r.cfg.MaxOutputTokensPerCall,
		})
		callCancel()
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(runCtx.Err(), context.DeadlineExceeded) {
				return finish(TimeoutReply, domain.OutcomeTimeout, err)
			}
			return finish(LLMErrorReply, domain.OutcomeLLMError, fmt.Errorf("agent llm complete: %w", err))
		}
		inv.InputTokens += out.InputTokens
		inv.OutputTokens += out.OutputTokens

		if len(out.ToolCalls) == 0 {
			reply := strings.TrimSpace(out.FinalText)
			if reply == "" {
				return finish(LLMErrorReply, domain.OutcomeLLMError, fmt.Errorf("agent llm returned empty final reply"))
			}
			return finish(reply, domain.OutcomeSuccess, nil)
		}

		messages = append(messages, port.LLMMessage{Role: "assistant", ToolCalls: out.ToolCalls})
		results := make([]domain.ToolResult, 0, len(out.ToolCalls))
		for _, call := range out.ToolCalls {
			inv.ToolCalls = append(inv.ToolCalls, call)
			result, err := dispatch(runCtx, call)
			if err != nil {
				return finish(ToolErrorReply, domain.OutcomeToolExecFailed, fmt.Errorf("dispatch tool %q: %w", call.Name, err))
			}
			results = append(results, result)
		}
		messages = append(messages, port.LLMMessage{Role: "user", ToolResults: results})
	}

	return finish(IterationCapReply, domain.OutcomeIterationCapHit, fmt.Errorf("agent iteration cap hit after %d iterations", r.cfg.MaxIterations))
}

func completeWithContext(ctx context.Context, llm port.LLMClient, in port.LLMInput) (port.LLMOutput, error) {
	type result struct {
		out port.LLMOutput
		err error
	}
	ch := make(chan result, 1)
	go func() {
		out, err := llm.Complete(ctx, in)
		ch <- result{out: out, err: err}
	}()
	select {
	case <-ctx.Done():
		return port.LLMOutput{}, ctx.Err()
	case res := <-ch:
		return res.out, res.err
	}
}
