package domain

import "context"

// AgentEventType classifies a streamed agent event for the SSE transport.
type AgentEventType string

const (
	// EventTextDelta is a piece of the final streamed answer.
	EventTextDelta AgentEventType = "text_delta"
	// EventToolCall is emitted when the model decides to invoke a tool.
	EventToolCall AgentEventType = "tool_call"
	// EventToolResult is emitted after a tool returns.
	EventToolResult AgentEventType = "tool_result"
	// EventDone marks the end of the run.
	EventDone AgentEventType = "done"
	// EventError marks a failed run.
	EventError AgentEventType = "error"
)

// AgentEvent is one item on the agent's event stream.
type AgentEvent struct {
	Type       AgentEventType
	Text       string // answer text (EventTextDelta)
	ToolName   string // tool name (EventToolCall / EventToolResult)
	ToolArgs   string // raw args JSON (EventToolCall)
	ToolResult string // tool output (EventToolResult)
}

// Agent is the ReAct tool-calling runner. The application layer calls Run and
// streams AgentEvents to the client; the conversation history and tool
// executions are encapsulated inside the implementation.
type Agent interface {
	// Run executes one user turn. roomID scopes the moderator persona and lets
	// tools default to the current room; onEvent receives streamed events and
	// may be nil. It returns the final answer text.
	Run(ctx context.Context, roomID, userID, message string, onEvent func(AgentEvent)) (string, error)
}
