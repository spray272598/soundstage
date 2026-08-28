// Package domain holds the AI room-moderator bounded context's interfaces and
// value types. It depends on nothing from other contexts or from infrastructure,
// so the application/agent layers can be tested against fakes.
package domain

import "context"

// Gateway is the LLM provider port. Implemented in infrastructure/llm with an
// OpenAI-compatible client and an offline mock. Neither the agent loop nor the
// moderation service ever import a concrete provider.
type Gateway interface {
	// Generate returns a single completion.
	Generate(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
	// GenerateStream is like Generate but emits incremental text deltas as they
	// arrive. Implementations should call onDelta with the final answer text.
	GenerateStream(ctx context.Context, req *ChatRequest, onDelta func(StreamDelta)) (*ChatResponse, error)
}

// ChatMessage is one turn of the conversation sent to the model. Tool results
// use RoleTool with ToolCallID linking back to the originating assistant call.
type ChatMessage struct {
	Role    string // system | user | assistant | tool
	Content string
	Name    string
	// ToolCallID links a tool result message to its assistant tool call.
	ToolCallID string
	// ToolCalls carries assistant-side tool invocations for multi-turn history.
	ToolCalls []ToolCall
}

// ToolSpec is the OpenAI-style function description sent to the model.
type ToolSpec struct {
	Name        string
	Description string
	// Parameters is a JSON-schema object, e.g.
	// {"type":"object","properties":{"room_id":{"type":"string"}},"required":["room_id"]}.
	Parameters map[string]any
}

// ToolCall is a single tool invocation requested by the model.
type ToolCall struct {
	ID        string // provider-generated id (may be empty for mock/ReAct)
	Name      string
	Arguments string // raw JSON object string
}

// StreamDelta is an incremental piece of streamed output.
type StreamDelta struct {
	Type string // "text" | "thought"
	Text string
}

// ChatResponse is the model's completion.
type ChatResponse struct {
	Content      string
	ToolCalls    []ToolCall
	PromptTokens int
	OutputTokens int
	TotalTokens  int
}

// ChatRequest is the full request sent to a Gateway.
type ChatRequest struct {
	SystemPrompt string
	Messages     []ChatMessage
	Temperature  float64
	MaxTokens    int
	Tools        []ToolSpec
	// ToolChoice nudges the model; "auto" lets it decide, "none" forces text.
	ToolChoice string
}
