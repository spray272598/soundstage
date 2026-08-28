package messaging

import (
	"context"
	"encoding/json"

	"github.com/spray272598/soundstage/internal/miclink/domain"
	"github.com/spray272598/soundstage/internal/pkg/event"
	pkgkafka "github.com/spray272598/soundstage/internal/pkg/kafka"
)

// signalEnvelope is the client-facing payload for a relayed WebRTC signal.
type signalEnvelope struct {
	From       string          `json:"from"`
	SignalType string          `json:"signal_type"`
	Data       json.RawMessage `json:"data"`
}

// KafkaSignalingRelay forwards a WebRTC signaling message to a single target
// user in a room. It publishes a broadcast envelope with the To recipient set,
// so the gateway delivers it only to that user's sessions.
type KafkaSignalingRelay struct {
	producer pkgkafka.Producer
	topic    string
}

// NewKafkaSignalingRelay creates a new KafkaSignalingRelay.
func NewKafkaSignalingRelay(producer pkgkafka.Producer, topic string) *KafkaSignalingRelay {
	return &KafkaSignalingRelay{producer: producer, topic: topic}
}

// Relay sends the signaling payload to the target user. The raw WebRTC data is
// wrapped with the sender and signal type so the recipient can route it.
func (r *KafkaSignalingRelay) Relay(ctx context.Context, roomID, toUserID, fromUserID, signalType string, payload json.RawMessage) error {
	wrapped, err := json.Marshal(signalEnvelope{
		From:       fromUserID,
		SignalType: signalType,
		Data:       payload,
	})
	if err != nil {
		return err
	}
	env := event.BroadcastEnvelope{
		RoomID:  roomID,
		Type:    "signal",
		To:      toUserID,
		Payload: wrapped,
	}
	data, err := json.Marshal(env)
	if err != nil {
		return err
	}
	return r.producer.Publish(ctx, r.topic, roomID, data)
}

// Compile-time check.
var _ domain.SignalingRelay = (*KafkaSignalingRelay)(nil)
