package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/spray272598/soundstage/internal/interaction/domain"
	"github.com/spray272598/soundstage/internal/pkg/redis"
)

// RedisMuter stores per-room-per-user mutes as Redis keys with a TTL. The hot
// path (IsMuted) is a single EXISTS; muting is a SET with EXPIRE. A zero or
// negative duration is treated as the minimum one-second mute so a call can
// never create a key that lives forever.
type RedisMuter struct {
	rdb *redis.Client
	min time.Duration
}

// NewRedisMuter creates a new RedisMuter.
func NewRedisMuter(c *redis.Client) *RedisMuter {
	return &RedisMuter{rdb: c, min: time.Second}
}

func muteKey(roomID, userID string) string {
	return fmt.Sprintf("mute:%s:%s", roomID, userID)
}

// Mute silences userID in roomID for duration. Durations shorter than the
// configured minimum are clamped up to the minimum.
func (m *RedisMuter) Mute(ctx context.Context, roomID, userID string, duration time.Duration) error {
	if duration < m.min {
		duration = m.min
	}
	return m.rdb.RDB().Set(ctx, muteKey(roomID, userID), time.Now().Unix(), duration).Err()
}

// Unmute removes an active mute immediately.
func (m *RedisMuter) Unmute(ctx context.Context, roomID, userID string) error {
	return m.rdb.RDB().Del(ctx, muteKey(roomID, userID)).Err()
}

// IsMuted reports whether the user currently has an active mute key.
func (m *RedisMuter) IsMuted(ctx context.Context, roomID, userID string) (bool, error) {
	n, err := m.rdb.RDB().Exists(ctx, muteKey(roomID, userID)).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// Compile-time check.
var _ domain.Muter = (*RedisMuter)(nil)
