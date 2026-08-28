// Package task defines the asynq task types and payloads shared by the
// enqueuer (interaction context) and the worker (asynq server). Keeping them
// here avoids an infrastructure -> application import cycle.
package task

import (
	"encoding/json"
	"time"
)

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

func (p PersistDanmakuPayload) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID        string    `json:"id"`
		RoomID    string    `json:"room_id"`
		UserID    string    `json:"user_id"`
		Content   string    `json:"content"`
		Status    string    `json:"status"`
		CreatedAt time.Time `json:"created_at"`
	}{
		ID:        p.ID,
		RoomID:    p.RoomID,
		UserID:    p.UserID,
		Content:   p.Content,
		Status:    p.Status,
		CreatedAt: p.CreatedAt,
	})
}

// SettleGiftPayload is the payload for TypeSettleGift.
type SettleGiftPayload struct {
	OrderID string
}

func (p SettleGiftPayload) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		OrderID string `json:"order_id"`
	}{
		OrderID: p.OrderID,
	})
}

// FlushLikesPayload is the payload for TypeFlushLikes (no fields yet).
type FlushLikesPayload struct{}

func (p FlushLikesPayload) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct{}{})
}
