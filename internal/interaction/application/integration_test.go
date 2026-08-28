package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	aiapplication "github.com/spray272598/soundstage/internal/ai/application"
	aidomain "github.com/spray272598/soundstage/internal/ai/domain"
	aiagent "github.com/spray272598/soundstage/internal/ai/infrastructure/agent"
	"github.com/spray272598/soundstage/internal/ai/infrastructure/llm"
	"github.com/spray272598/soundstage/internal/ai/infrastructure/rag"
	"github.com/spray272598/soundstage/internal/interaction/application"
	"github.com/spray272598/soundstage/internal/interaction/domain"
	"github.com/spray272598/soundstage/internal/pkg/id"
)

// inMemoryMuter implements BOTH interaction.Muter and ai.RoomModerator, so the
// same instance can be driven by the agent's mute_user tool and consulted by
// the interaction service — proving the two contexts stay decoupled yet wired.
type inMemoryMuter struct {
	mu    sync.Mutex
	muted map[string]time.Time // key "roomID:userID" -> expiry
}

func newInMemoryMuter() *inMemoryMuter { return &inMemoryMuter{muted: map[string]time.Time{}} }

func mk(roomID, userID string) string { return roomID + ":" + userID }

func (m *inMemoryMuter) Mute(_ context.Context, roomID, userID string, d time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.muted[mk(roomID, userID)] = time.Now().Add(d)
	return nil
}
func (m *inMemoryMuter) Unmute(_ context.Context, roomID, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.muted, mk(roomID, userID))
	return nil
}
func (m *inMemoryMuter) IsMuted(_ context.Context, roomID, userID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	exp, ok := m.muted[mk(roomID, userID)]
	if !ok {
		return false, nil
	}
	if time.Now().After(exp) {
		delete(m.muted, mk(roomID, userID))
		return false, nil
	}
	return true, nil
}

// broadcastRecorder captures published messages for assertions.
type broadcastRecorder struct {
	mu     sync.Mutex
	events []broadcastEvent
}

type broadcastEvent struct {
	roomID  string
	msgType string
	payload []byte
}

func (r *broadcastRecorder) add(roomID, msgType string, payload []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, broadcastEvent{roomID, msgType, append([]byte(nil), payload...)})
}

// interactionBroadcaster satisfies interaction/domain.Broadcaster (json.RawMessage).
type interactionBroadcaster struct{ rec *broadcastRecorder }

func (b interactionBroadcaster) Broadcast(_ context.Context, roomID, msgType string, payload json.RawMessage) error {
	b.rec.add(roomID, msgType, []byte(payload))
	return nil
}

// aiBroadcaster satisfies ai/domain.Broadcaster ([]byte).
type aiBroadcaster struct{ rec *broadcastRecorder }

func (b aiBroadcaster) Broadcast(_ context.Context, roomID, msgType string, payload []byte) error {
	b.rec.add(roomID, msgType, payload)
	return nil
}

type allowAllLimiter struct{}

func (allowAllLimiter) Allow(_ context.Context, _ string, _ int, _ time.Duration) (bool, error) {
	return true, nil
}

type noopTasks struct{}

func (noopTasks) EnqueuePersistDanmaku(_ context.Context, _ *domain.Danmaku) error { return nil }
func (noopTasks) EnqueueSettleGift(_ context.Context, _ string) error              { return nil }

type fakeStatus struct{ online int }

func (f fakeStatus) Status(_ context.Context, roomID string) (*aidomain.RoomStatus, error) {
	return &aidomain.RoomStatus{RoomID: roomID, Status: "live", OnlineCount: f.online}, nil
}

type fakeLeader struct{}

func (fakeLeader) TopGifts(_ context.Context, _, _ string, _ int) ([]aidomain.LeaderboardEntry, error) {
	return []aidomain.LeaderboardEntry{{UserID: "u1", Amount: 100, Rank: 1}}, nil
}

// buildInterService wires the AI moderator into the interaction service using
// in-memory fakes, so the test needs no Kafka/Redis/MySQL/LLM. It returns the
// interaction service, the AI moderator (for agent tool-calling), the shared
// muter, and the broadcast recorder.
func buildInterService(t *testing.T) (*application.InterService, *aiapplication.Service, *inMemoryMuter, *broadcastRecorder) {
	t.Helper()
	_ = id.Init(1) // snowflake node for the agent loop's per-call IDs
	kb := rag.NewService(llm.NewMockEmbedder(32))
	if err := rag.SeedDefaultKnowledge(context.Background(), kb); err != nil {
		t.Fatalf("seed knowledge base: %v", err)
	}
	gw := llm.NewMock()
	reg := aiagent.NewMapRegistry()
	rec := &broadcastRecorder{}
	muter := newInMemoryMuter()
	aiagent.RegisterBuiltinTools(reg, aiagent.Dependencies{
		Status:    fakeStatus{online: 42},
		Leader:    fakeLeader{},
		Muted:     muter,
		Broadcast: aiBroadcaster{rec},
		KB:        kb,
	})
	loop := aiagent.NewLoop(gw, reg, aiagent.Config{MaxRounds: 5, Timeout: 5 * time.Second})
	moderator := aiapplication.NewService(gw, kb, loop, []string{"广告", "违规词"}, false)

	svc := application.NewInterService(
		nil, // gifts repo (unused by danmaku path)
		nil, // orders repo (unused)
		nil, // danmaku repo (unused)
		moderator,
		allowAllLimiter{},
		muter,
		nil, // rank store (unused)
		nil, // like counter (unused)
		interactionBroadcaster{rec},
		noopTasks{},
		application.InterServiceConfig{DanmakuRateLimit: 5, DanmakuRateWindow: time.Second},
	)
	return svc, moderator, muter, rec
}

// TestDanmakuKeywordRejected verifies the AI moderator's keyword fast-path
// blocks an ad-bait danmaku before it is ever broadcast.
func TestDanmakuKeywordRejected(t *testing.T) {
	svc, _, _, _ := buildInterService(t)
	_, err := svc.ProcessDanmaku(context.Background(), "r1", "u1", "快加我微信看广告优惠")
	if err == nil {
		t.Fatal("expected rejection for ad keyword")
	}
	if !errors.Is(err, domain.ErrRejected) {
		t.Fatalf("want ErrRejected, got %v", err)
	}
}

// TestDanmakuMutedUserRejected verifies a muted user is dropped before
// moderation/broadcast (the AI moderator drives the same Muter).
func TestDanmakuMutedUserRejected(t *testing.T) {
	svc, _, muter, _ := buildInterService(t)
	if err := muter.Mute(context.Background(), "r1", "badguy", time.Minute); err != nil {
		t.Fatal(err)
	}
	_, err := svc.ProcessDanmaku(context.Background(), "r1", "badguy", "大家好呀")
	if err == nil {
		t.Fatal("expected muted user to be rejected")
	}
	if !errors.Is(err, domain.ErrMuted) {
		t.Fatalf("want ErrMuted, got %v", err)
	}
}

// TestDanmakuAcceptedAndBroadcast verifies a clean danmaku is accepted and
// broadcast to the room.
func TestDanmakuAcceptedAndBroadcast(t *testing.T) {
	svc, _, _, rec := buildInterService(t)
	d, err := svc.ProcessDanmaku(context.Background(), "r1", "u1", "主播好厉害")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d == nil || d.Content != "主播好厉害" {
		t.Fatalf("unexpected danmaku: %+v", d)
	}
	if len(rec.events) == 0 {
		t.Fatal("expected a broadcast event")
	}
	if rec.events[0].msgType != "chat" {
		t.Fatalf("want chat broadcast, got %s", rec.events[0].msgType)
	}
}

// TestAgentDrivenMutePropagatesToInteraction proves the agent's mute_user tool
// (driven through the ai RoomModerator port) and the interaction service's
// Muter are the same instance: after the agent mutes a user, that user's
// danmaku is rejected by interaction. This is the core Phase 4 decoupling test.
func TestAgentDrivenMutePropagatesToInteraction(t *testing.T) {
	svc, moderator, muter, _ := buildInterService(t)
	if _, err := moderator.Chat(context.Background(), "demo-room", "host", "把那个捣乱的用户禁言", nil); err != nil {
		t.Fatalf("agent run failed: %v", err)
	}
	muted, err := muter.IsMuted(context.Background(), "demo-room", "target-user")
	if err != nil {
		t.Fatal(err)
	}
	if !muted {
		t.Fatal("expected target-user to be muted in demo-room after agent tool call")
	}
	_, err = svc.ProcessDanmaku(context.Background(), "demo-room", "target-user", "hi")
	if !errors.Is(err, domain.ErrMuted) {
		t.Fatalf("want ErrMuted for muted user, got %v", err)
	}
}
