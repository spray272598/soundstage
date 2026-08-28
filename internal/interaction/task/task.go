// Package task defines the asynq task types and payloads shared by the
// enqueuer (interaction context) and the worker (asynq server). Keeping them
// here avoids an infrastructure -> application import cycle.
package task

import "time"

// Asynq task type names.
const (
	// TypePersistDanmaku persists an approved/rejected danmaku to its day shard.
	TypePersistDanmaku = "interaction:persist_danmaku"
	// TypeSettleGift marks a gift order settled and updates the leaderboards.
	TypeSettleGift = "interaction:settle_gift"
	// TypeFlushLikes snapshots per-room like tallies to MySQL.
	TypeFlushLikes = "interaction:flush_likes"
)

// PersistDanmakuPayload is the payload for TypePersistDanmaku.
type PersistDanmakuPayload struct {
	ID        string
	RoomID    string
	UserID    string
	Content   string
	Status    string
	CreatedAt time.Time
}

// SettleGiftPayload is the payload for TypeSettleGift.
type SettleGiftPayload struct {
	OrderID string
}

// FlushLikesPayload is the payload for TypeFlushLikes (no fields yet).
type FlushLikesPayload struct{}
