package messaging

import (
	"context"
	"encoding/json"

	"github.com/spray272598/soundstage/internal/miclink/domain"
	"github.com/spray272598/soundstage/internal/pkg/event"
	pkgkafka "github.com/spray272598/soundstage/internal/pkg/kafka"
)

// KafkaBroadcaster publishes client-facing messages to the broadcast topic.
// The connection gateway consumes that topic and pushes to local sessions,
// which gives us cross-node fan-out for free.
type KafkaBroadcaster struct {
	producer pkgkafka.Producer
	topic    string
}

// NewKafkaBroadcaster creates a new KafkaBroadcaster.
func NewKafkaBroadcaster(producer pkgkafka.Producer, topic string) *KafkaBroadcaster {
	return &KafkaBroadcaster{producer: producer, topic: topic}
}

// Broadcast publishes a broadcast envelope keyed by room id.
func (b *KafkaBroadcaster) Broadcast(ctx context.Context, roomID, msgType string, payload json.RawMessage) error {
	env := event.BroadcastEnvelope{
		RoomID:  roomID,
		Type:    msgType,
		Payload: payload,
	}
	data, err := json.Marshal(env)
	if err != nil {
		return err
	}
	return b.producer.Publish(ctx, b.topic, roomID, data)
}

// Compile-time check.
var _ domain.Broadcaster = (*KafkaBroadcaster)(nil)
