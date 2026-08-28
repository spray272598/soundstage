package domain

import "context"

// Tool is a single capability the agent can invoke. Tools are read-only
// (room status, leaderboard, RAG lookup) or mutating (mute, announce); the
// agent loop logs every invocation so mutating tools stay auditable.
type Tool interface {
	Name() string
	Description() string
	// InputSchema returns a JSON-schema object describing the tool arguments.
	InputSchema() map[string]any
	// Execute runs the tool. The returned string is fed back to the model as a
	// tool result; a non-nil error is also surfaced but the loop continues.
	Execute(ctx context.Context, args map[string]any) (string, error)
}

// Registry is a thread-safe collection of tools keyed by name.
type Registry interface {
	Register(t Tool)
	Get(name string) (Tool, bool)
	List() []Tool
	// Specs returns the OpenAI-style schemas for every registered tool.
	Specs() []ToolSpec
}
