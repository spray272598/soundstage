package application

import (
	"context"
	"encoding/json"

	"github.com/spray272598/soundstage/internal/pkg/event"
	"github.com/spray272598/soundstage/internal/pkg/logger"
	"go.uber.org/zap"
)

// IngestConsumer is the Kafka handler for the ingest topic. The connection
// gateway publishes raw client messages here; we parse the envelope and
// dispatch to the single InterService processor.
type IngestConsumer struct {
	svc *InterService
}

// NewIngestConsumer creates a new IngestConsumer.
func NewIngestConsumer(svc *InterService) *IngestConsumer {
	return &IngestConsumer{svc: svc}
}

// Handle implements pkgkafka.Handler.
func (c *IngestConsumer) Handle(ctx context.Context, topic string, key string, payload []byte) error {
	var env event.InboundEnvelope
	if err := json.Unmarshal(payload, &env); err != nil {
		logger.L().Error("invalid ingest envelope", zap.Error(err))
		return nil // drop malformed envelopes
	}

	switch env.Type {
	case "chat":
		var p chatInbound
		_ = json.Unmarshal(env.Payload, &p)
		if _, err := c.svc.ProcessDanmaku(ctx, env.RoomID, env.UserID, p.Text); err != nil {
			logger.L().Warn("process danmaku failed", zap.Error(err), zap.String("room", env.RoomID))
		}
	case "gift":
		var p giftInbound
		_ = json.Unmarshal(env.Payload, &p)
		if _, err := c.svc.ProcessGift(ctx, env.RoomID, env.UserID, p.GiftID, p.Count, p.IdempotencyKey); err != nil {
			logger.L().Warn("process gift failed", zap.Error(err), zap.String("room", env.RoomID))
		}
	case "like":
		if err := c.svc.ProcessLike(ctx, env.RoomID, env.UserID); err != nil {
			logger.L().Warn("process like failed", zap.Error(err), zap.String("room", env.RoomID))
		}
	}

	// A processing failure is logged but never returned, so the raw envelope is
	// not redelivered in a loop. The async task layer owns its own retries.
	return nil
}

type chatInbound struct {
	Text string `json:"text"`
}

type giftInbound struct {
	GiftID         string `json:"gift_id"`
	Count          int    `json:"count"`
	IdempotencyKey string `json:"idempotency_key"`
}
