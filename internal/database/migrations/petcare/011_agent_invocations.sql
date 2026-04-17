CREATE TABLE IF NOT EXISTS agent_invocations (
    id TEXT PRIMARY KEY,
    input_text TEXT NOT NULL,
    tool_calls_json TEXT NOT NULL,
    final_reply TEXT NOT NULL,
    input_tokens INTEGER NOT NULL,
    output_tokens INTEGER NOT NULL,
    duration_ms INTEGER NOT NULL,
    outcome TEXT NOT NULL,
    error_message TEXT,
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_agent_invocations_created_at ON agent_invocations(created_at);
CREATE INDEX IF NOT EXISTS idx_agent_invocations_outcome ON agent_invocations(outcome);
