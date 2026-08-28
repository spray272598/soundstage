package application

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spray272598/soundstage/internal/interaction/domain"
	"github.com/spray272598/soundstage/internal/pkg/errors"
	"github.com/spray272598/soundstage/internal/pkg/id"
	"github.com/spray272598/soundstage/internal/pkg/logger"
	"github.com/spray272598/soundstage/internal/pkg/metrics"
	"go.uber.org/zap"
)

// InterServiceConfig holds tunables for the interaction service.
type InterServiceConfig struct {
	DanmakuRateLimit  int
	DanmakuRateWindow time.Duration
}

// InterService is the single processor for all interactive messages. Both the
// WebSocket ingest path (via Kafka) and the REST path call into it, so the
// moderation, rate-limiting and broadcasting logic lives in exactly one place.
type InterService struct {
	gifts      domain.GiftRepository
	orders     domain.GiftOrderRepository
	danmaku    domain.DanmakuRepository
	moderator  domain.Moderator
	limiter    domain.RateLimiter
	rank       domain.RankStore
	likes      domain.LikeCounter
	broadcaster domain.Broadcaster
	tasks      domain.TaskEnqueuer
	cfg        InterServiceConfig
}

// NewInterService constructs an InterService from its ports.
func NewInterService(
	gifts domain.GiftRepository,
	orders domain.GiftOrderRepository,
	danmaku domain.DanmakuRepository,
	moderator domain.Moderator,
	limiter domain.RateLimiter,
	rank domain.RankStore,
	likes domain.LikeCounter,
	broadcaster domain.Broadcaster,
	tasks domain.TaskEnqueuer,
	cfg InterServiceConfig,
) *InterService {
	return &InterService{
		gifts:      gifts,
		orders:     orders,
		danmaku:    danmaku,
		moderator:  moderator,
		limiter:    limiter,
		rank:       rank,
		likes:      likes,
		broadcaster: broadcaster,
		tasks:      tasks,
		cfg:        cfg,
	}
}

// ProcessDanmaku validates, moderates and broadcasts a danmaku. Persistence is
// enqueued asynchronously so the caller (and the WS pump) is never blocked.
func (s *InterService) ProcessDanmaku(ctx context.Context, roomID, userID, content string) (*domain.Danmaku, error) {
	key := fmt.Sprintf("danmaku:%s:%s", roomID, userID)
	allowed, err := s.limiter.Allow(ctx, key, s.cfg.DanmakuRateLimit, s.cfg.DanmakuRateWindow)
	if err != nil {
		return nil, err
	}
	if !allowed {
		metrics.InteractionDanmakuTotal.WithLabelValues("rejected").Inc()
		return nil, domain.ErrRateLimited
	}

	decision, err := s.moderator.Moderate(ctx, content)
	if err != nil {
		return nil, err
	}
	if !decision.Allowed {
		// Persist for audit but never broadcast a blocked message.
		rejected := domain.NewRejectedDanmaku(id.New(), roomID, userID, content)
		if qerr := s.tasks.EnqueuePersistDanmaku(ctx, rejected); qerr != nil {
			logger.L().Error("enqueue rejected danmaku failed", zap.Error(qerr))
		}
		metrics.InteractionDanmakuTotal.WithLabelValues("rejected").Inc()
		return nil, domain.ErrRejected
	}

	d := domain.NewDanmaku(id.New(), roomID, userID, decision.Masked)
	payload, err := json.Marshal(danmakuBroadcast{
		ID:        d.ID,
		UserID:    userID,
		Content:   d.Content,
		CreatedAt: d.CreatedAt,
	})
	if err == nil {
		if berr := s.broadcaster.Broadcast(ctx, roomID, "chat", payload); berr != nil {
			logger.L().Error("broadcast danmaku failed", zap.Error(berr))
		}
	}
	if qerr := s.tasks.EnqueuePersistDanmaku(ctx, d); qerr != nil {
		logger.L().Error("enqueue persist danmaku failed", zap.Error(qerr))
	}
	metrics.InteractionDanmakuTotal.WithLabelValues("accepted").Inc()
	return d, nil
}

// ProcessGift validates the gift, creates an idempotent order, broadcasts it
// and schedules async settlement. Settlement is what updates the leaderboards.
func (s *InterService) ProcessGift(ctx context.Context, roomID, senderID, giftID string, count int, idempotencyKey string) (*domain.GiftOrder, error) {
	if count <= 0 {
		return nil, errors.ErrInvalidInput
	}
	gift, err := s.gifts.GetByID(ctx, giftID)
	if err != nil {
		return nil, err
	}
	if gift.Status != domain.GiftStatusActive {
		return nil, domain.ErrGiftInactive
	}

	// Idempotency: a client-supplied key short-circuits a duplicate send.
	if idempotencyKey == "" {
		idempotencyKey = id.New()
	} else if existing, gerr := s.orders.GetByIDempotencyKey(ctx, idempotencyKey); gerr == nil {
		return existing, nil
	} else if gerr != domain.ErrOrderNotFound {
		return nil, gerr
	}

	order := domain.NewGiftOrder(id.New(), roomID, senderID, gift.ID, gift.Name, count, gift.Price, idempotencyKey)
	if err := s.orders.Create(ctx, order); err != nil {
		return nil, err
	}

	payload, err := json.Marshal(giftBroadcast{
		OrderID:    order.ID,
		SenderID:   senderID,
		GiftID:     gift.ID,
		GiftName:   gift.Name,
		Count:      count,
		TotalAmount: order.TotalAmount,
	})
	if err == nil {
		if berr := s.broadcaster.Broadcast(ctx, roomID, "gift", payload); berr != nil {
			logger.L().Error("broadcast gift failed", zap.Error(berr))
		}
	}
	if qerr := s.tasks.EnqueueSettleGift(ctx, order.ID); qerr != nil {
		logger.L().Error("enqueue settle gift failed", zap.Error(qerr))
	}
	metrics.InteractionGiftTotal.WithLabelValues("accepted").Inc()
	metrics.GiftOrderStatusTotal.WithLabelValues(string(domain.GiftOrderStatusCreated)).Inc()
	return order, nil
}

// ProcessLike increments the room like tally and broadcasts the new total.
// Like events are never written to MySQL individually; a periodic job flushes.
func (s *InterService) ProcessLike(ctx context.Context, roomID, userID string) error {
	n, err := s.likes.Incr(ctx, roomID)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(likeBroadcast{UserID: userID, Count: n})
	if err == nil {
		if berr := s.broadcaster.Broadcast(ctx, roomID, "like", payload); berr != nil {
			logger.L().Error("broadcast like failed", zap.Error(berr))
		}
	}
	metrics.InteractionLikeTotal.Inc()
	return nil
}

// danmakuBroadcast is the client-facing payload for an approved danmaku.
type danmakuBroadcast struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// giftBroadcast is the client-facing payload for a gift.
type giftBroadcast struct {
	OrderID     string `json:"order_id"`
	SenderID    string `json:"sender_id"`
	GiftID      string `json:"gift_id"`
	GiftName    string `json:"gift_name"`
	Count       int    `json:"count"`
	TotalAmount int64  `json:"total_amount"`
}

// likeBroadcast is the client-facing payload for a like.
type likeBroadcast struct {
	UserID string `json:"user_id"`
	Count  int64  `json:"count"`
}

// ListGifts returns the full gift catalog.
func (s *InterService) ListGifts(ctx context.Context) ([]*domain.Gift, error) {
	return s.gifts.ListCatalog(ctx)
}

// RankTopN returns the top n gift senders for a room and period.
func (s *InterService) RankTopN(ctx context.Context, roomID string, period domain.Period, n int) ([]domain.RankEntry, error) {
	return s.rank.TopN(ctx, roomID, period, n)
}

// LikeCount returns the current like tally for a room.
func (s *InterService) LikeCount(ctx context.Context, roomID string) (int64, error) {
	return s.likes.Get(ctx, roomID)
}
