package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	aidomain "github.com/spray272598/soundstage/internal/ai/domain"
)

// scriptedGateway returns queued responses in order, simulating a model that
// first asks for a tool, then answers.
type scriptedGateway struct {
	calls     int
	responses []aidomain.ChatResponse
}

func (g *scriptedGateway) Generate(ctx context.Context, req *aidomain.ChatRequest) (*aidomain.ChatResponse, error) {
	return g.GenerateStream(ctx, req, nil)
}

func (g *scriptedGateway) GenerateStream(_ context.Context, _ *aidomain.ChatRequest, onDelta func(aidomain.StreamDelta)) (*aidomain.ChatResponse, error) {
	resp := g.responses[g.calls]
	g.calls++
	if resp.Content != "" && onDelta != nil {
		onDelta(aidomain.StreamDelta{Type: "text", Text: resp.Content})
	}
	return &resp, nil
}

// fakeTool records invocations and returns a fixed string.
type fakeTool struct {
	calls int
}

func (f *fakeTool) Name() string        { return "get_room_status" }
func (f *fakeTool) Description() string { return "fake" }
func (f *fakeTool) InputSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{"room_id": map[string]any{"type": "string"}}}
}
func (f *fakeTool) Execute(_ context.Context, _ map[string]any) (string, error) {
	f.calls++
	return "online: 42", nil
}

func TestLoopToolCalling(t *testing.T) {
	tool := &fakeTool{}
	reg := NewMapRegistry()
	reg.Register(tool)

	gw := &scriptedGateway{responses: []aidomain.ChatResponse{
		{Content: "", ToolCalls: []aidomain.ToolCall{{ID: "c1", Name: "get_room_status", Arguments: `{"room_id":"r1"}`}}},
		{Content: "当前在线 42 人。"},
	}}

	loop := NewLoop(gw, reg, Config{MaxRounds: 5})
	var gotText string
	final, err := loop.Run(context.Background(), "r1", "host", "多少人？", func(ev aidomain.AgentEvent) {
		if ev.Type == aidomain.EventTextDelta {
			gotText += ev.Text
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if tool.calls != 1 {
		t.Fatalf("expected tool invoked once, got %d", tool.calls)
	}
	if !strings.Contains(final, "42") {
		t.Fatalf("final answer missing tool result: %q", final)
	}
	if !strings.Contains(gotText, "42") {
		t.Fatalf("streamed text missing tool result: %q", gotText)
	}
}

func TestLoopParseToolCallFromContent(t *testing.T) {
	calls := parseToolCallsFromContent(`{"name":"mute_user","args":{"user_id":"u1"}}`)
	if len(calls) != 1 || calls[0].Name != "mute_user" {
		t.Fatalf("expected mute_user parsed, got %+v", calls)
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(calls[0].Arguments), &args); err != nil {
		t.Fatal(err)
	}
	if args["user_id"] != "u1" {
		t.Fatalf("unexpected args: %+v", args)
	}
}
