package domain

import (
	"context"
	"encoding/json"
	"time"
)

// ModerationDecision is the result of running a content through the moderator.
type ModerationDecision struct {
	Allowed bool
	Reason  string
	Masked  string // content with sensitive parts masked, when Allowed
}

// Moderator screens user-generated content. The default implementation is a
// keyword blocklist; the AI context (Phase 4) can swap in an LLM-backed one
// without touching the application layer.
type Moderator interface {
	Moderate(ctx context.Context, content string) (ModerationDecision, error)
}

// RateLimiter guards a keyed resource with a fixed-window counter.
type RateLimiter interface {
	// Allow returns true if the key is still within its limit for the window.
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error)
}

// RankEntry is one row of a gift leaderboard.
type RankEntry struct {
	UserID string
	Amount int64
}

// RankStore maintains per-room, per-period gift leaderboards. It is backed by
// Redis sorted sets for O(log N) updates and fast TopN reads.
type RankStore interface {
	IncrGift(ctx context.Context, roomID string, period Period, userID string, amount int64) error
	TopN(ctx context.Context, roomID string, period Period, n int) ([]RankEntry, error)
}

// LikeCounter keeps a per-room like tally. Individual like events are never
// persisted; the count is periodically flushed to MySQL (see TaskEnqueuer).
type LikeCounter interface {
	Incr(ctx context.Context, roomID string) (int64, error)
	Get(ctx context.Context, roomID string) (int64, error)
}

// Broadcaster publishes a client-facing message to a room. It is implemented
// by the Kafka producer writing to the broadcast topic.
type Broadcaster interface {
	Broadcast(ctx context.Context, roomID, msgType string, payload json.RawMessage) error
}

// TaskEnqueuer schedules background work (async persistence, settlement, flush).
// Keeping it behind a port lets tests run with an in-memory fake.
type TaskEnqueuer interface {
	EnqueuePersistDanmaku(ctx context.Context, d *Danmaku) error
	EnqueueSettleGift(ctx context.Context, orderID string) error
}
