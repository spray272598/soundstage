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

	// InteractionDanmakuTotal counts danmaku by sync result (accepted/rejected).
	InteractionDanmakuTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "soundstage",
		Name:      "interaction_danmaku_total",
		Help:      "Total danmaku processed by the interaction context",
	}, []string{"result"})

	// InteractionGiftTotal counts gift messages by sync result (accepted/rejected).
	InteractionGiftTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "soundstage",
		Name:      "interaction_gift_total",
		Help:      "Total gift messages processed by the interaction context",
	}, []string{"result"})

	// InteractionLikeTotal counts like messages processed by the interaction context.
	InteractionLikeTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "soundstage",
		Name:      "interaction_like_total",
		Help:      "Total like messages processed by the interaction context",
	})

	// GiftOrderStatusTotal counts gift order transitions by target status.
	GiftOrderStatusTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "soundstage",
		Name:      "gift_order_status_total",
		Help:      "Gift order status transitions",
	}, []string{"status"})

	// AsynqTaskEnqueuedTotal counts background tasks enqueued by type.
	AsynqTaskEnqueuedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "soundstage",
		Name:      "asynq_task_enqueued_total",
		Help:      "Background tasks enqueued by type",
	}, []string{"type"})

	// MicLinkRequestsTotal counts co-host (mic-link) requests by result.
	MicLinkRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "soundstage",
		Name:      "miclink_requests_total",
		Help:      "Co-host mic-link requests by result",
	}, []string{"result"})

	// PKSessionsTotal counts PK sessions by terminal status.
	PKSessionsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "soundstage",
		Name:      "pk_sessions_total",
		Help:      "PK sessions by status",
	}, []string{"status"})

	// PKScoreTotal counts points added to a PK side.
	PKScoreTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "soundstage",
		Name:      "pk_score_total",
		Help:      "PK score points by side",
	}, []string{"side"})

	// SignalingRelayedTotal counts WebRTC signaling messages relayed.
	SignalingRelayedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "soundstage",
		Name:      "signaling_relayed_total",
		Help:      "WebRTC signaling messages relayed by the miclink context",
	})
)

func init() {
	registry.MustRegister(
		WSConnections,
		WSMessagesTotal,
		InteractionDanmakuTotal,
		InteractionGiftTotal,
		InteractionLikeTotal,
		GiftOrderStatusTotal,
		AsynqTaskEnqueuedTotal,
		MicLinkRequestsTotal,
		PKSessionsTotal,
		PKScoreTotal,
		SignalingRelayedTotal,
	)
}
