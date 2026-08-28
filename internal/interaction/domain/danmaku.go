package domain

import "time"

// DanmakuStatus represents the moderation state of a danmaku message.
type DanmakuStatus string

const (
	// DanmakuStatusApproved passed moderation and was broadcast.
	DanmakuStatusApproved DanmakuStatus = "approved"
	// DanmakuStatusRejected was blocked by the moderator (e.g. sensitive words).
	DanmakuStatusRejected DanmakuStatus = "rejected"
)

// Danmaku is a single chat/bullet-comment message in a room. Every message is
// persisted (sharded by day) for audit and replay; the synchronous path only
// does moderation + rate limiting before broadcasting.
type Danmaku struct {
	ID        string
	RoomID    string
	UserID    string
	Content   string
	Status    DanmakuStatus
	CreatedAt time.Time
}

// NewDanmaku builds an approved danmaku ready to broadcast and persist.
func NewDanmaku(id, roomID, userID, content string) *Danmaku {
	return &Danmaku{
		ID:        id,
		RoomID:    roomID,
		UserID:    userID,
		Content:   content,
		Status:    DanmakuStatusApproved,
		CreatedAt: time.Now().UTC(),
	}
}

// NewRejectedDanmaku builds a danmaku that was blocked by moderation. It is
// persisted for audit but never broadcast.
func NewRejectedDanmaku(id, roomID, userID, content string) *Danmaku {
	return &Danmaku{
		ID:        id,
		RoomID:    roomID,
		UserID:    userID,
		Content:   content,
		Status:    DanmakuStatusRejected,
		CreatedAt: time.Now().UTC(),
	}
}
