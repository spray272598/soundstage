package application

import (
	"context"
	"encoding/json"

	"github.com/spray272598/soundstage/internal/pkg/event"
	"github.com/spray272598/soundstage/internal/pkg/logger"
	"go.uber.org/zap"
)

// MiclinkIngestConsumer is a Kafka handler for the ingest topic, scoped to the
// miclink context. It runs in its own consumer group so it reads the same
// events as the interaction consumer independently. It relays WebRTC signaling
// and feeds PK scores from gifts.
type MiclinkIngestConsumer struct {
	micSvc *MicLinkService
	pkSvc  *PKService
}

// NewMiclinkIngestConsumer creates a new MiclinkIngestConsumer.
func NewMiclinkIngestConsumer(micSvc *MicLinkService, pkSvc *PKService) *MiclinkIngestConsumer {
	return &MiclinkIngestConsumer{micSvc: micSvc, pkSvc: pkSvc}
}

// Handle implements pkgkafka.Handler.
func (c *MiclinkIngestConsumer) Handle(ctx context.Context, topic string, key string, payload []byte) error {
	var env event.InboundEnvelope
	if err := json.Unmarshal(payload, &env); err != nil {
		logger.L().Error("invalid miclink ingest envelope", zap.Error(err))
		return nil // drop malformed envelopes
	}

	switch env.Type {
	case "signal":
		var p signalInbound
		_ = json.Unmarshal(env.Payload, &p)
		if err := c.micSvc.RelaySignal(ctx, env.RoomID, env.UserID, p.To, p.SignalType, p.Data); err != nil {
			logger.L().Warn("relay signal failed", zap.Error(err), zap.String("room", env.RoomID))
		}
	case "gift":
		// Gifts sent during a live PK feed that room's score automatically.
		var p giftInbound
		_ = json.Unmarshal(env.Payload, &p)
		if _, err := c.pkSvc.Score(ctx, env.RoomID, p.TotalAmount); err != nil {
			logger.L().Warn("pk score from gift failed", zap.Error(err), zap.String("room", env.RoomID))
		}
	}

	// A processing failure is logged but never returned, so the raw envelope is
	// not redelivered in a loop. The async task layer owns its own retries.
	return nil
}

// signalInbound is the WS client payload for a WebRTC signal.
type signalInbound struct {
	To         string          `json:"to"`
	SignalType string          `json:"signal_type"`
	Data       json.RawMessage `json:"data"`
}

// giftInbound mirrors the gift broadcast shape from the interaction context.
type giftInbound struct {
	OrderID     string `json:"order_id"`
	SenderID    string `json:"sender_id"`
	GiftID      string `json:"gift_id"`
	GiftName    string `json:"gift_name"`
	Count       int    `json:"count"`
	TotalAmount int64  `json:"total_amount"`
}
