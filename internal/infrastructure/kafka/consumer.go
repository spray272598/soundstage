package kafka

import (
	"context"
	"sync"

	"github.com/segmentio/kafka-go"
	"github.com/spray272598/soundstage/internal/pkg/logger"
	pkgkafka "github.com/spray272598/soundstage/internal/pkg/kafka"
	"go.uber.org/zap"
)

// Consumer implements pkgkafka.Consumer using segmentio/kafka-go.
type Consumer struct {
	brokers []string
	groupID string
	readers []*kafka.Reader
	mu      sync.Mutex
}

// NewConsumer creates a new Consumer.
func NewConsumer(brokers []string, groupID string) *Consumer {
	return &Consumer{
		brokers: brokers,
		groupID: groupID,
	}
}

// Subscribe starts a reader for each topic and forwards messages to handler.
func (c *Consumer) Subscribe(ctx context.Context, topics []string, handler pkgkafka.Handler) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, topic := range topics {
		r := kafka.NewReader(kafka.ReaderConfig{
			Brokers: c.brokers,
			GroupID: c.groupID,
			Topic:   topic,
		})
		c.readers = append(c.readers, r)
		go c.run(ctx, r, handler, topic)
	}
	return nil
}

// Close closes all readers.
func (c *Consumer) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, r := range c.readers {
		_ = r.Close()
	}
	c.readers = nil
	return nil
}

func (c *Consumer) run(ctx context.Context, r *kafka.Reader, handler pkgkafka.Handler, topic string) {
	defer r.Close()
	for {
		m, err := r.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			logger.L().Error("kafka read error", zap.Error(err), zap.String("topic", topic))
			continue
		}
		if err := handler.Handle(ctx, topic, string(m.Key), m.Value); err != nil {
			logger.L().Error("kafka handler error", zap.Error(err), zap.String("topic", topic))
		}
	}
}

// Compile-time check.
var _ pkgkafka.Consumer = (*Consumer)(nil)
