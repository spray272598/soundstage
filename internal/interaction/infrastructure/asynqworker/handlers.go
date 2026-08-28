package asynqworker

import (
	"context"
	"encoding/json"

	"github.com/hibiken/asynq"
	"github.com/spray272598/soundstage/internal/interaction/domain"
	"github.com/spray272598/soundstage/internal/interaction/task"
	"github.com/spray272598/soundstage/internal/pkg/logger"
	"github.com/spray272598/soundstage/internal/pkg/metrics"
	"go.uber.org/zap"
)

// handlePersistDanmaku writes a danmaku to its day shard. Failures are retried
// by asynq up to its max retry; a permanently bad payload is dropped after.
func (d Deps) handlePersistDanmaku(ctx context.Context, t *asynq.Task) error {
	var p task.PersistDanmakuPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return err
	}
	dm := &domain.Danmaku{
		ID:        p.ID,
		RoomID:    p.RoomID,
		UserID:    p.UserID,
		Content:   p.Content,
		Status:    domain.DanmakuStatus(p.Status),
		CreatedAt: p.CreatedAt,
	}
	if err := d.Danmaku.Create(ctx, dm); err != nil {
		logger.L().Error("persist danmaku failed", zap.Error(err), zap.String("id", p.ID))
		return err
	}
	return nil
}

// handleSettleGift marks an order settled (idempotent) and updates the
// leaderboards for every period. Settlement is the single source of truth for
// the rank, so a retried task is safe.
func (d Deps) handleSettleGift(ctx context.Context, t *asynq.Task) error {
	var p task.SettleGiftPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return err
	}
	order, err := d.Orders.GetByID(ctx, p.OrderID)
	if err != nil {
		return err
	}
	if order.Status == domain.GiftOrderStatusSettled {
		return nil
	}
	order.MarkSettled()
	if err := d.Orders.Update(ctx, order); err != nil {
		return err
	}
	for _, period := range domain.AllPeriods() {
		if err := d.Rank.IncrGift(ctx, order.RoomID, period, order.SenderID, order.TotalAmount); err != nil {
			return err
		}
	}
	metrics.GiftOrderStatusTotal.WithLabelValues(string(domain.GiftOrderStatusSettled)).Inc()
	return nil
}

// handleFlushLikes snapshots every room's like tally into MySQL.
func (d Deps) handleFlushLikes(ctx context.Context, t *asynq.Task) error {
	return d.Likes.ScanRooms(ctx, "like:*", func(roomID string) error {
		n, err := d.Likes.Get(ctx, roomID)
		if err != nil {
			return err
		}
		if err := d.Stats.UpsertLikes(ctx, roomID, n); err != nil {
			logger.L().Error("flush likes failed", zap.Error(err), zap.String("room", roomID))
			return err
		}
		return nil
	})
}
