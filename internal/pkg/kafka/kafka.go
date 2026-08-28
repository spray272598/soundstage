package kafka

import "context"

// Producer is the outbound port for publishing events.
type Producer interface {
	Publish(ctx context.Context, topic string, key string, payload []byte) error
	Close() error
}

// Handler processes incoming Kafka messages.
type Handler interface {
	Handle(ctx context.Context, topic string, key string, payload []byte) error
}

// Consumer is the inbound port for subscribing to events.
type Consumer interface {
	Subscribe(ctx context.Context, topics []string, handler Handler) error
	Close() error
}
