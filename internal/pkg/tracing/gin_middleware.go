package tracing

import (
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

// GinMiddleware returns a Gin middleware that adds tracing to HTTP requests.
func GinMiddleware(serviceName string) gin.HandlerFunc {
	tracer := otel.Tracer(serviceName)
	propagator := otel.GetTextMapPropagator()

	return func(c *gin.Context) {
		// Extract trace context from incoming headers
		ctx := propagator.Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))

		// Start span
		spanName := c.FullPath()
		if spanName == "" {
			spanName = c.Request.Method + " " + c.Request.URL.Path
		}

		ctx, span := tracer.Start(ctx, spanName,
			trace.WithAttributes(
				semconv.HTTPMethodKey.String(c.Request.Method),
				semconv.HTTPTargetKey.String(c.Request.URL.Path),
				semconv.HTTPSchemeKey.String(c.Request.URL.Scheme),
				attribute.String("http.host", c.Request.Host),
				attribute.String("net.host.port", c.Request.Host),
				semconv.NetPeerNameKey.String(c.ClientIP()),
			),
		)
		defer span.End()

		// Inject trace context into response headers
		propagator.Inject(ctx, propagation.HeaderCarrier(c.Writer.Header()))

		// Replace request context with traced context
		c.Request = c.Request.WithContext(ctx)

		// Process request
		c.Next()

		// Add response attributes
		span.SetAttributes(
			attribute.Int("http.status_code", c.Writer.Status()),
			attribute.Int64("http.response_content_length", int64(c.Writer.Size())),
		)

		// Record error if status >= 400
		if c.Writer.Status() >= 400 {
			span.SetAttributes(
				attribute.String("http.error", "true"),
				attribute.Int("http.status_code", c.Writer.Status()),
			)
		}
	}
}

// PropagationHeaderCarrier implements propagation.TextMapCarrier for Gin.
type PropagationHeaderCarrier map[string][]string

// Get returns the value associated with the passed key.
func (c PropagationHeaderCarrier) Get(key string) string {
	values := c[key]
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// Set stores the key-value pair.
func (c PropagationHeaderCarrier) Set(key, value string) {
	c[key] = []string{value}
}

// Keys lists all keys in the carrier.
func (c PropagationHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}