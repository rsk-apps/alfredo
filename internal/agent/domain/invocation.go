package domain

import "time"

type Outcome string

const (
	OutcomeSuccess          Outcome = "success"
	OutcomeAmbiguousRequest Outcome = "ambiguous_request"
	OutcomeToolExecFailed   Outcome = "tool_exec_failed"
	OutcomeIterationCapHit  Outcome = "iteration_cap_hit"
	OutcomeTimeout          Outcome = "timeout"
	OutcomeLLMError         Outcome = "llm_error"
)

type Invocation struct {
	ID           string
	InputText    string
	ToolCalls    []ToolCall
	FinalReply   string
	InputTokens  int
	OutputTokens int
	DurationMS   int
	Outcome      Outcome
	ErrorMessage *string
	CreatedAt    time.Time
}
