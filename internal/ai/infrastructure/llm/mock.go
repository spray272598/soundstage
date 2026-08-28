package llm

import (
	"context"
	"encoding/json"
	"strings"

	aidomain "github.com/spray272598/soundstage/internal/ai/domain"
)

// MockGateway is a deterministic offline stand-in used when no API key is
// configured. It speaks the same ReAct protocol the real agent loop expects:
// it returns a JSON tool call when the user is asking the moderator to inspect
// or act on the room, and a plain answer otherwise. This lets the full AI
// pipeline (moderation, RAG, tool-calling) run end-to-end in demos and tests.
type MockGateway struct{}

// NewMock builds a MockGateway.
func NewMock() *MockGateway { return &MockGateway{} }

// Generate is delegated to the streaming path.
func (m *MockGateway) Generate(ctx context.Context, req *aidomain.ChatRequest) (*aidomain.ChatResponse, error) {
	return m.GenerateStream(ctx, req, nil)
}

// GenerateStream inspects the last message and either emits a tool call (as a
// JSON content string the loop parses) or a canned textual answer.
func (m *MockGateway) GenerateStream(ctx context.Context, req *aidomain.ChatRequest, onDelta func(aidomain.StreamDelta)) (*aidomain.ChatResponse, error) {
	_ = ctx
	last := lastUserMessage(req)

	// After a tool result, the mock produces a final natural-language answer.
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "tool" {
			answer := "收到，我已根据工具返回的信息处理完毕。" + truncate(req.Messages[i].Content, 200)
			if onDelta != nil {
				onDelta(aidomain.StreamDelta{Type: "text", Text: answer})
			}
			return &aidomain.ChatResponse{Content: answer, TotalTokens: 20}, nil
		}
	}

	lower := strings.ToLower(last)
	var call *aidomain.ToolCall
	switch {
	case strings.Contains(lower, "人数") || strings.Contains(lower, "在线") || strings.Contains(lower, "状态") ||
		strings.Contains(lower, "status") || strings.Contains(lower, "online"):
		call = &aidomain.ToolCall{Name: "get_room_status", Arguments: `{"room_id":"demo-room"}`}
	case strings.Contains(lower, "榜单") || strings.Contains(lower, "排行") || strings.Contains(lower, "榜") ||
		strings.Contains(lower, "leaderboard") || strings.Contains(lower, "rank"):
		call = &aidomain.ToolCall{Name: "get_leaderboard", Arguments: `{"room_id":"demo-room","period":"day","n":5}`}
	case strings.Contains(lower, "禁言") || strings.Contains(lower, "mute") || strings.Contains(lower, "闭麦"):
		call = &aidomain.ToolCall{Name: "mute_user", Arguments: `{"room_id":"demo-room","user_id":"target-user","duration_seconds":300}`}
	case strings.Contains(lower, "公告") || strings.Contains(lower, "广播") || strings.Contains(lower, "announce") ||
		strings.Contains(lower, "通知"):
		call = &aidomain.ToolCall{Name: "send_announcement", Arguments: `{"room_id":"demo-room","text":"房管提醒：请文明发言，禁止刷屏。"}`}
	case strings.Contains(lower, "规则") || strings.Contains(lower, "规定") || strings.Contains(lower, "rules") ||
		strings.Contains(lower, "违规"):
		call = &aidomain.ToolCall{Name: "get_room_rules", Arguments: `{"query":"直播间规则 违规处理"}`}
	}

	if call != nil {
		raw, _ := json.Marshal(call)
		// Place the JSON in Content; the loop parses it as a ReAct tool call.
		return &aidomain.ChatResponse{Content: string(raw), ToolCalls: nil, TotalTokens: 10}, nil
	}

	answer := "我是本直播间 AI 房管。我可以帮你查询房间状态、礼物榜单，对违规内容进行审核，必要时对观众禁言或发布房管公告。请问需要我做什么？"
	if onDelta != nil {
		onDelta(aidomain.StreamDelta{Type: "text", Text: answer})
	}
	return &aidomain.ChatResponse{Content: answer, TotalTokens: 30}, nil
}

func lastUserMessage(req *aidomain.ChatRequest) string {
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			return req.Messages[i].Content
		}
	}
	return ""
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}

// Compile-time check.
var _ aidomain.Gateway = (*MockGateway)(nil)
