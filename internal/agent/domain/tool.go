package domain

// Tool is the declarative description of an operation the LLM may invoke.
type Tool struct {
	Name        string
	Description string
	InputSchema map[string]any
}

// ToolCall is a request from the LLM to invoke a Tool.
type ToolCall struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// ToolResult is the outcome returned to the LLM after a dispatcher executes a ToolCall.
type ToolResult struct {
	CallID  string `json:"call_id"`
	Content string `json:"content"`
	IsError bool   `json:"is_error"`
}
