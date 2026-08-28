package cache

import (
	"context"
	"fmt"

	"github.com/spray272598/soundstage/internal/interaction/domain"
	"github.com/spray272598/soundstage/internal/pkg/redis"
)

// RedisRankStore maintains per-room, per-period gift leaderboards in Redis
// sorted sets. All periods (day/week/month) plus a global board are updated
// on every settlement so TopN reads are O(log N).
type RedisRankStore struct {
	rdb *redis.Client
}

// NewRedisRankStore creates a new RedisRankStore.
func NewRedisRankStore(c *redis.Client) *RedisRankStore {
	return &RedisRankStore{rdb: c}
}

func rankKey(roomID string, p domain.Period) string {
	return fmt.Sprintf("rank:gift:%s:%s:%s", roomID, p, p.Value())
}

func globalRankKey(p domain.Period) string {
	return fmt.Sprintf("rank:gift:global:%s:%s", p, p.Value())
}

// IncrGift adds amount to the user's score in the room board and the global board.
func (s *RedisRankStore) IncrGift(ctx context.Context, roomID string, p domain.Period, userID string, amount int64) error {
	pipe := s.rdb.RDB().TxPipeline()
	pipe.ZIncrBy(ctx, rankKey(roomID, p), float64(amount), userID)
	pipe.ZIncrBy(ctx, globalRankKey(p), float64(amount), userID)
	_, err := pipe.Exec(ctx)
	return err
}

// TopN returns the top n senders for a room and period, highest score first.
func (s *RedisRankStore) TopN(ctx context.Context, roomID string, p domain.Period, n int) ([]domain.RankEntry, error) {
	res, err := s.rdb.RDB().ZRevRangeWithScores(ctx, rankKey(roomID, p), 0, int64(n-1)).Result()
	if err != nil {
		return nil, err
	}
	entries := make([]domain.RankEntry, 0, len(res))
	for _, z := range res {
		uid, ok := z.Member.(string)
		if !ok {
			continue
		}
		entries = append(entries, domain.RankEntry{UserID: uid, Amount: int64(z.Score)})
	}
	return entries, nil
}

// Compile-time check.
var _ domain.RankStore = (*RedisRankStore)(nil)
