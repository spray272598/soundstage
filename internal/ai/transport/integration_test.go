package transport_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	aiapplication "github.com/spray272598/soundstage/internal/ai/application"
	aidomain "github.com/spray272598/soundstage/internal/ai/domain"
	aiagent "github.com/spray272598/soundstage/internal/ai/infrastructure/agent"
	"github.com/spray272598/soundstage/internal/ai/infrastructure/llm"
	"github.com/spray272598/soundstage/internal/ai/infrastructure/rag"
	aitransport "github.com/spray272598/soundstage/internal/ai/transport"
	"github.com/spray272598/soundstage/internal/pkg/id"
)

// --- in-memory fakes for the ai domain ports (no Kafka/Redis/LLM needed) ---

type fakeStatus struct{ online int }

func (f fakeStatus) Status(_ context.Context, roomID string) (*aidomain.RoomStatus, error) {
	return &aidomain.RoomStatus{RoomID: roomID, Status: "live", OnlineCount: f.online}, nil
}

type fakeLeader struct{}

func (fakeLeader) TopGifts(_ context.Context, _, _ string, _ int) ([]aidomain.LeaderboardEntry, error) {
	return []aidomain.LeaderboardEntry{{UserID: "u1", Amount: 100, Rank: 1}}, nil
}

type fakeMuter struct{}

func (fakeMuter) Mute(_ context.Context, _, _ string, _ time.Duration) error { return nil }
func (fakeMuter) Unmute(_ context.Context, _, _ string) error                { return nil }

type fakeBroadcaster struct{ last []byte }

func (f *fakeBroadcaster) Broadcast(_ context.Context, _, _ string, payload []byte) error {
	f.last = append([]byte(nil), payload...)
	return nil
}

// newTestAIService wires the full AI pipeline (mock LLM + RAG + ReAct loop) so
// the SSE handler can be exercised end-to-end without an API key or infra.
func newTestAIService(t *testing.T) *aiapplication.Service {
	t.Helper()
	_ = id.Init(1) // snowflake node for the agent loop's per-call IDs
	kb := rag.NewService(llm.NewMockEmbedder(32))
	if err := rag.SeedDefaultKnowledge(context.Background(), kb); err != nil {
		t.Fatalf("seed knowledge base: %v", err)
	}
	gw := llm.NewMock()
	reg := aiagent.NewMapRegistry()
	bc := &fakeBroadcaster{}
	aiagent.RegisterBuiltinTools(reg, aiagent.Dependencies{
		Status:    fakeStatus{online: 42},
		Leader:    fakeLeader{},
		Muted:     fakeMuter{},
		Broadcast: bc,
		KB:        kb,
	})
	loop := aiagent.NewLoop(gw, reg, aiagent.Config{MaxRounds: 5, Timeout: 5 * time.Second})
	return aiapplication.NewService(gw, kb, loop, []string{"广告", "违规词"}, false)
}

// TestChatSSEEndToEnd drives the SSE chat endpoint and asserts the full
// agent stream: a tool call, its result, and a done frame, with the canned
// room status (online=42) surfaced through the tool result.
func TestChatSSEEndToEnd(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newTestAIService(t)
	h := aitransport.NewHandler(svc, "mock", "mock")
	r := gin.New()
	h.Register(r)

	msg := url.QueryEscape("现在房间在线多少人")
	req := httptest.NewRequest(http.MethodGet, "/rooms/demo-room/ai/chat?message="+msg+"&user_id=host", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "event: tool_call") {
		t.Fatalf("expected a tool_call SSE frame, got:\n%s", body)
	}
	if !strings.Contains(body, "get_room_status") {
		t.Fatalf("expected get_room_status tool call, got:\n%s", body)
	}
	if !strings.Contains(body, "event: tool_result") {
		t.Fatalf("expected a tool_result SSE frame, got:\n%s", body)
	}
	if !strings.Contains(body, "event: done") {
		t.Fatalf("expected a done SSE frame, got:\n%s", body)
	}
	// The room-status tool returned OnlineCount=42; it must appear in the stream.
	if !strings.Contains(body, "42") {
		t.Fatalf("expected canned online count 42 in stream, got:\n%s", body)
	}
}
