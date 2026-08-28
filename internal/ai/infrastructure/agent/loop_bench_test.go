package agent

import (
	"context"
	"testing"

	aidomain "github.com/spray272598/soundstage/internal/ai/domain"
)

// BenchmarkLoopToolCalling measures the per-turn overhead of the ReAct loop:
// two model calls (request tool, then answer) plus one tool execution, using a
// scripted gateway so the number isolates loop/dispatch cost, not network.
func BenchmarkLoopToolCalling(b *testing.B) {
	reg := NewMapRegistry()
	reg.Register(&fakeTool{})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Fresh gateway each iteration: the scripted gateway advances an index
		// that must not overflow across benchmark iterations.
		gw := &scriptedGateway{responses: []aidomain.ChatResponse{
			{Content: "", ToolCalls: []aidomain.ToolCall{{ID: "c1", Name: "get_room_status", Arguments: `{"room_id":"r1"}`}}},
			{Content: "当前在线 42 人。"},
		}}
		loop := NewLoop(gw, reg, Config{MaxRounds: 5})
		_, _ = loop.Run(context.Background(), "r1", "host", "多少人？", nil)
	}
}
