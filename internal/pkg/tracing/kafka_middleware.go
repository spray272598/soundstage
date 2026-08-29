package tracing

import (
	"context"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

// KafkaProducerHeaders injects trace context into Kafka message headers.
func KafkaProducerHeaders(ctx context.Context, msg *kafka.Message) {
	propagator := otel.GetTextMapPropagator()
	carrier := propagation.MapCarrier{}
	for _, h := range msg.Headers {
		carrier.Set(h.Key, string(h.Value))
	}
	propagator.Inject(ctx, carrier)
	msg.Headers = make([]kafka.Header, 0, len(carrier))
	for _, k := range carrier.Keys() {
		msg.Headers = append(msg.Headers, kafka.Header{
			Key:   k,
			Value: []byte(carrier.Get(k)),
		})
	}
}

// ExtractTraceContext extracts trace context from Kafka message headers.
func ExtractTraceContext(ctx context.Context, msg kafka.Message) context.Context {
	propagator := otel.GetTextMapPropagator()
	carrier := propagation.MapCarrier{}
	for _, h := range msg.Headers {
		carrier.Set(h.Key, string(h.Value))
	}
	return propagator.Extract(ctx, carrier)
}

// StartProducerSpan starts a span for producing a Kafka message.
func StartProducerSpan(ctx context.Context, topic string) (context.Context, trace.Span) {
	return otel.Tracer("soundstage").Start(ctx, "kafka.publish",
		trace.WithAttributes(
			semconv.MessagingSystemKey.String("kafka"),
			semconv.MessagingDestinationNameKey.String(topic),
			semconv.MessagingOperationKey.String("publish"),
		),
	)
}

// StartConsumerSpan starts a span for processing a Kafka message.
func StartConsumerSpan(ctx context.Context, topic, groupID string) (context.Context, trace.Span) {
	return otel.Tracer("soundstage").Start(ctx, "kafka.consume",
		trace.WithAttributes(
			semconv.MessagingSystemKey.String("kafka"),
			semconv.MessagingDestinationNameKey.String(topic),
			semconv.MessagingOperationKey.String("receive"),
			semconv.MessagingKafkaConsumerGroupKey.String(groupID),
		),
	)
}