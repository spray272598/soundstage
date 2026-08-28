package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var registry = prometheus.NewRegistry()

// Registry returns the application metrics registry.
func Registry() *prometheus.Registry {
	return registry
}

// HTTPHandler returns an HTTP handler that exposes metrics.
func HTTPHandler() http.Handler {
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
}

var (
	// WSConnections tracks active websocket connections per room.
	WSConnections = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "soundstage",
		Name:      "websocket_connections",
		Help:      "Number of active websocket connections",
	}, []string{"room_id"})

	// WSMessagesTotal counts websocket messages by type and direction.
	WSMessagesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "soundstage",
		Name:      "websocket_messages_total",
		Help:      "Total websocket messages processed",
	}, []string{"type", "direction"})
)

func init() {
	registry.MustRegister(WSConnections, WSMessagesTotal)
}
