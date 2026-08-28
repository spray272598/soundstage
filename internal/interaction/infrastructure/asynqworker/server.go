// Package asynqworker wires the background task handlers (danmaku persistence,
// gift settlement, like flush) to an asynq server and scheduler.
package asynqworker

import (
	"github.com/hibiken/asynq"
	"github.com/spray272598/soundstage/internal/interaction/domain"
	"github.com/spray272598/soundstage/internal/interaction/infrastructure/cache"
	"github.com/spray272598/soundstage/internal/interaction/infrastructure/persistence"
	"github.com/spray272598/soundstage/internal/interaction/task"
	"github.com/spray272598/soundstage/internal/pkg/redis"
)

// NewServer builds an asynq server backed by the given Redis address.
func NewServer(redisAddr string) *asynq.Server {
	return asynq.NewServer(
		asynq.RedisClientOpt{Addr: redisAddr},
		asynq.Config{Concurrency: 10},
	)
}

// NewScheduler builds an asynq scheduler backed by the given Redis address.
func NewScheduler(redisAddr string) *asynq.Scheduler {
	return asynq.NewScheduler(asynq.RedisClientOpt{Addr: redisAddr}, &asynq.SchedulerOpts{})
}

// Deps bundles every port the task handlers need.
type Deps struct {
	Danmaku domain.DanmakuRepository
	Orders  domain.GiftOrderRepository
	Rank    domain.RankStore
	Likes   *cache.RedisLikeCounter
	Stats   *persistence.GormRoomStatsRepository
	RDB     *redis.Client
}

// Register installs all handlers on the given mux.
func (d Deps) Register(mux *asynq.ServeMux) {
	mux.Handle(task.TypePersistDanmaku, asynq.HandlerFunc(d.handlePersistDanmaku))
	mux.Handle(task.TypeSettleGift, asynq.HandlerFunc(d.handleSettleGift))
	mux.Handle(task.TypeFlushLikes, asynq.HandlerFunc(d.handleFlushLikes))
}
