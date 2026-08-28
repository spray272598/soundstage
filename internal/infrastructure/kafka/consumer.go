package kafka

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/segmentio/kafka-go"
	pkgkafka "github.com/spray272598/soundstage/internal/pkg/kafka"
	"github.com/spray272598/soundstage/internal/pkg/logger"
	"go.uber.org/zap"
)

// Consumer implements pkgkafka.Consumer using segmentio/kafka-go.
type Consumer struct {
	brokers       []string
	groupID       string
	consumerCount int
	readers       []*kafka.Reader
	mu            sync.Mutex
	closed        atomic.Bool
}

// NewConsumer creates a new Consumer.
func NewConsumer(brokers []string, groupID string) *Consumer {
	return &Consumer{
		brokers:       brokers,
		groupID:       groupID,
		consumerCount: 1, // default to 1 for backward compatibility
	}
}

// WithConsumerCount sets the number of parallel consumers per topic.
func (c *Consumer) WithConsumerCount(n int) *Consumer {
	if n > 0 {
		c.consumerCount = n
	}
	return c
}

// Subscribe starts multiple readers for each topic and forwards messages to handler.
func (c *Consumer) Subscribe(ctx context.Context, topics []string, handler pkgkafka.Handler) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed.Load() {
		return nil
	}

	for _, topic := range topics {
		// Create multiple readers for the same topic/group to parallelize processing
		for i := 0; i < c.consumerCount; i++ {
			r := kafka.NewReader(kafka.ReaderConfig{
				Brokers:  c.brokers,
				GroupID:  c.groupID,
				Topic:    topic,
				MinBytes: 1,
				MaxBytes: 10e6, // 10MB
			})
			c.readers = append(c.readers, r)
			go c.run(ctx, r, handler, topic, i)
		}
		logger.L().Info("kafka consumer started",
			zap.String("topic", topic),
			zap.Int("consumers", c.consumerCount),
			zap.String("group", c.groupID))
	}
	return nil
}

// Close closes all readers.
func (c *Consumer) Close() error {
	c.closed.Store(true)
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, r := range c.readers {
		_ = r.Close()
	}
	c.readers = nil
	return nil
}

func (c *Consumer) run(ctx context.Context, r *kafka.Reader, handler pkgkafka.Handler, topic string, consumerIdx int) {
	defer r.Close()
	for {
		if c.closed.Load() {
			return
		}
		m, err := r.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil || c.closed.Load() {
				return
			}
			logger.L().Error("kafka read error", zap.Error(err), zap.String("topic", topic), zap.Int("consumer", consumerIdx))
			continue
		}
		if err := handler.Handle(ctx, topic, string(m.Key), m.Value); err != nil {
			logger.L().Error("kafka handler error", zap.Error(err), zap.String("topic", topic), zap.Int("consumer", consumerIdx))
		}
	}
}

// Compile-time check.
var _ pkgkafka.Consumer = (*Consumer)(nil)
