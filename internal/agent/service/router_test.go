package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/rafaelsoares/alfredo/internal/agent/domain"
	"github.com/rafaelsoares/alfredo/internal/agent/port"
)

type sequenceLLM struct {
	outputs []port.LLMOutput
	err     error
	sleep   time.Duration
	calls   int
}

func (l *sequenceLLM) Complete(ctx context.Context, _ port.LLMInput) (port.LLMOutput, error) {
	l.calls++
	if l.sleep > 0 {
		select {
		case <-time.After(l.sleep):
		case <-ctx.Done():
			return port.LLMOutput{}, ctx.Err()
		}
	}
	if l.err != nil {
		return port.LLMOutput{}, l.err
	}
	if l.calls-1 >= len(l.outputs) {
		return port.LLMOutput{}, nil
	}
	return l.outputs[l.calls-1], nil
}

type recordingRepo struct {
	invocations []domain.Invocation
	err         error
}

func (r *recordingRepo) Create(_ context.Context, inv domain.Invocation) error {
	r.invocations = append(r.invocations, inv)
	return r.err
}

func TestRouterExecuteSuccessWritesInvocation(t *testing.T) {
	repo := &recordingRepo{}
	router := NewRouter(&sequenceLLM{outputs: []port.LLMOutput{{FinalText: "Registrei.", InputTokens: 2, OutputTokens: 3}}}, repo, RouterConfig{}, zap.NewNop())

	reply, inv, err := router.Execute(context.Background(), "sys", nil, "entrada", func(context.Context, domain.ToolCall) (domain.ToolResult, error) {
		t.Fatal("dispatch should not be called")
		return domain.ToolResult{}, nil
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if reply != "Registrei." {
		t.Fatalf("reply = %q", reply)
	}
	if inv.Outcome != domain.OutcomeSuccess {
		t.Fatalf("outcome = %q", inv.Outcome)
	}
	if len(repo.invocations) != 1 {
		t.Fatalf("audit writes = %d", len(repo.invocations))
	}
	if repo.invocations[0].InputTokens != 2 || repo.invocations[0].OutputTokens != 3 {
		t.Fatalf("tokens = %d/%d", repo.invocations[0].InputTokens, repo.invocations[0].OutputTokens)
	}
}

func TestRouterExecuteToolLoopSuccess(t *testing.T) {
	repo := &recordingRepo{}
	llm := &sequenceLLM{outputs: []port.LLMOutput{
		{ToolCalls: []domain.ToolCall{{ID: "call-1", Name: "list_pets", Arguments: map[string]any{}}}},
		{FinalText: "Achei a Luna."},
	}}
	router := NewRouter(llm, repo, RouterConfig{}, zap.NewNop())

	reply, inv, err := router.Execute(context.Background(), "sys", nil, "entrada", func(_ context.Context, call domain.ToolCall) (domain.ToolResult, error) {
		return domain.ToolResult{CallID: call.ID, Content: "[]"}, nil
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if reply != "Achei a Luna." {
		t.Fatalf("reply = %q", reply)
	}
	if len(inv.ToolCalls) != 1 || inv.ToolCalls[0].Name != "list_pets" {
		t.Fatalf("tool calls = %#v", inv.ToolCalls)
	}
	if repo.invocations[0].Outcome != domain.OutcomeSuccess {
		t.Fatalf("audit outcome = %q", repo.invocations[0].Outcome)
	}
}

func TestRouterExecuteIterationCapWritesInvocation(t *testing.T) {
	repo := &recordingRepo{}
	llm := &sequenceLLM{outputs: []port.LLMOutput{
		{ToolCalls: []domain.ToolCall{{ID: "call-1", Name: "list_pets"}}},
		{ToolCalls: []domain.ToolCall{{ID: "call-2", Name: "list_pets"}}},
	}}
	router := NewRouter(llm, repo, RouterConfig{MaxIterations: 2, TotalTimeout: time.Second, CallTimeout: time.Second}, zap.NewNop())

	reply, inv, err := router.Execute(context.Background(), "sys", nil, "entrada", func(_ context.Context, call domain.ToolCall) (domain.ToolResult, error) {
		return domain.ToolResult{CallID: call.ID, Content: "[]"}, nil
	})
	if err == nil {
		t.Fatal("expected iteration cap error")
	}
	if reply != IterationCapReply {
		t.Fatalf("reply = %q", reply)
	}
	if inv.Outcome != domain.OutcomeIterationCapHit || repo.invocations[0].Outcome != domain.OutcomeIterationCapHit {
		t.Fatalf("outcome = %q audit=%q", inv.Outcome, repo.invocations[0].Outcome)
	}
}

func TestRouterExecuteTimeoutWritesInvocation(t *testing.T) {
	repo := &recordingRepo{}
	router := NewRouter(&sequenceLLM{sleep: 50 * time.Millisecond}, repo, RouterConfig{MaxIterations: 5, TotalTimeout: 10 * time.Millisecond, CallTimeout: 10 * time.Millisecond}, zap.NewNop())

	reply, inv, err := router.Execute(context.Background(), "sys", nil, "entrada", func(context.Context, domain.ToolCall) (domain.ToolResult, error) {
		t.Fatal("dispatch should not be called")
		return domain.ToolResult{}, nil
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if reply != TimeoutReply {
		t.Fatalf("reply = %q", reply)
	}
	if inv.Outcome != domain.OutcomeTimeout || repo.invocations[0].Outcome != domain.OutcomeTimeout {
		t.Fatalf("outcome = %q audit=%q", inv.Outcome, repo.invocations[0].Outcome)
	}
}

func TestRouterExecuteLLMErrorWritesInvocation(t *testing.T) {
	repo := &recordingRepo{}
	router := NewRouter(&sequenceLLM{err: errors.New("llm down")}, repo, RouterConfig{}, zap.NewNop())

	reply, inv, err := router.Execute(context.Background(), "sys", nil, "entrada", func(context.Context, domain.ToolCall) (domain.ToolResult, error) {
		t.Fatal("dispatch should not be called")
		return domain.ToolResult{}, nil
	})
	if err == nil {
		t.Fatal("expected llm error")
	}
	if reply != LLMErrorReply {
		t.Fatalf("reply = %q", reply)
	}
	if inv.Outcome != domain.OutcomeLLMError || repo.invocations[0].ErrorMessage == nil {
		t.Fatalf("invocation = %#v", inv)
	}
}

func TestRouterExecuteDispatchErrorWritesInvocation(t *testing.T) {
	repo := &recordingRepo{}
	llm := &sequenceLLM{outputs: []port.LLMOutput{{ToolCalls: []domain.ToolCall{{ID: "call-1", Name: "log_observation"}}}}}
	router := NewRouter(llm, repo, RouterConfig{}, zap.NewNop())

	reply, inv, err := router.Execute(context.Background(), "sys", nil, "entrada", func(context.Context, domain.ToolCall) (domain.ToolResult, error) {
		return domain.ToolResult{}, errors.New("bad args")
	})
	if err == nil {
		t.Fatal("expected dispatch error")
	}
	if reply != ToolErrorReply {
		t.Fatalf("reply = %q", reply)
	}
	if inv.Outcome != domain.OutcomeToolExecFailed || len(repo.invocations[0].ToolCalls) != 1 {
		t.Fatalf("invocation = %#v", repo.invocations[0])
	}
}

func TestRouterAuditFailureDoesNotFailResponse(t *testing.T) {
	repo := &recordingRepo{err: errors.New("disk full")}
	router := NewRouter(&sequenceLLM{outputs: []port.LLMOutput{{FinalText: "Pronto."}}}, repo, RouterConfig{}, zap.NewNop())

	reply, _, err := router.Execute(context.Background(), "sys", nil, "entrada", func(context.Context, domain.ToolCall) (domain.ToolResult, error) {
		return domain.ToolResult{}, nil
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if reply != "Pronto." {
		t.Fatalf("reply = %q", reply)
	}
}
