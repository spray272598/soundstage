package domain

import (
	"context"
	"time"
)

// RoomStatus is a snapshot of a live room, surfaced to the AI moderator so it
// can answer "how many people are watching?" or report the current PK score.
type RoomStatus struct {
	RoomID      string
	Title       string
	AnchorID    string
	Status      string
	OnlineCount int
	MicLink     *MicLinkState
	PK          *PKState
}

// MicLinkState describes the current co-host connection.
type MicLinkState struct {
	Active  bool
	HostID  string
	GuestID string
}

// PKState describes the current cross-room PK battle.
type PKState struct {
	SessionID string
	Status    string
	RoomA     string
	RoomB     string
	ScoreA    int64
	ScoreB    int64
}

// RoomStatusProvider supplies room + miclink/PK state to the agent. It is
// implemented in app.go by composing the room/miclink services, so the ai
// context never imports those contexts directly.
type RoomStatusProvider interface {
	Status(ctx context.Context, roomID string) (*RoomStatus, error)
}

// LeaderboardEntry is one row of a gift leaderboard.
type LeaderboardEntry struct {
	UserID string
	Amount int64
	Rank   int
}

// LeaderboardProvider returns the gift leaderboard for a room and period.
type LeaderboardProvider interface {
	TopGifts(ctx context.Context, roomID string, period string, n int) ([]LeaderboardEntry, error)
}

// RoomModerator performs moderation actions on behalf of the agent. The
// implementation wraps interaction's Muter; the ai context stays decoupled.
type RoomModerator interface {
	Mute(ctx context.Context, roomID, userID string, duration time.Duration) error
	Unmute(ctx context.Context, roomID, userID string) error
}

// Broadcaster publishes an AI announcement to a room. Its shape matches
// interaction/domain.Broadcaster, so the concrete Kafka broadcaster satisfies
// it without an adapter.
type Broadcaster interface {
	Broadcast(ctx context.Context, roomID, msgType string, payload []byte) error
}
