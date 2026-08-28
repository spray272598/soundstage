package kafka

import (
	"context"
	"sync"

	"github.com/segmentio/kafka-go"
	pkgkafka "github.com/spray272598/soundstage/internal/pkg/kafka"
)

// Producer implements pkgkafka.Producer using segmentio/kafka-go.
type Producer struct {
	brokers []string
	writers map[string]*kafka.Writer
	mu      sync.RWMutex
}

// NewProducer creates a new Producer.
func NewProducer(brokers []string) *Producer {
	return &Producer{
		brokers: brokers,
		writers: make(map[string]*kafka.Writer),
	}
}

// Publish writes a message to the given topic.
func (p *Producer) Publish(ctx context.Context, topic, key string, payload []byte) error {
	w := p.writer(topic)
	return w.WriteMessages(ctx, kafka.Message{
		Key:   []byte(key),
		Value: payload,
	})
}

// Close closes all cached writers.
func (p *Producer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, w := range p.writers {
		_ = w.Close()
	}
	p.writers = make(map[string]*kafka.Writer)
	return nil
}

func (p *Producer) writer(topic string) *kafka.Writer {
	p.mu.RLock()
	w, ok := p.writers[topic]
	p.mu.RUnlock()
	if ok {
		return w
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if w, ok := p.writers[topic]; ok {
		return w
	}
	w = &kafka.Writer{
		Addr:     kafka.TCP(p.brokers...),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}
	p.writers[topic] = w
	return w
}

// Compile-time check.
var _ pkgkafka.Producer = (*Producer)(nil)
