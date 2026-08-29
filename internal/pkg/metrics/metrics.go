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

	// AIModerationTotal counts danmaku moderation calls handled by the AI
	// moderator, labelled by decision (allowed/rejected/error) and path
	// (keyword/llm) so we can see how often the LLM audit fires.
	AIModerationTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "soundstage",
		Name:      "ai_moderation_total",
		Help:      "AI room-moderator moderation calls by decision and path",
	}, []string{"decision", "path"})

	// AIAgentRunsTotal counts agent (ReAct) runs by outcome.
	AIAgentRunsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "soundstage",
		Name:      "ai_agent_runs_total",
		Help:      "AI agent tool-calling runs by outcome",
	}, []string{"outcome"})

	// AIToolCallsTotal counts tool invocations by tool name and result.
	AIToolCallsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "soundstage",
		Name:      "ai_tool_calls_total",
		Help:      "AI agent tool invocations by tool and result",
	}, []string{"tool", "result"})

	// AIRagQueriesTotal counts RAG knowledge-base queries by hit (found/miss).
	AIRagQueriesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "soundstage",
		Name:      "ai_rag_queries_total",
		Help:      "RAG knowledge-base queries by retrieval result",
	}, []string{"result"})

	// AISSEConnections is the current number of open AI chat SSE streams.
	AISSEConnections = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "soundstage",
		Name:      "ai_sse_connections",
		Help:      "Active AI chat SSE connections",
	})

	// AILatencySeconds observes LLM call latency.
	AILatencySeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "soundstage",
		Name:      "ai_llm_latency_seconds",
		Help:      "LLM call latency in seconds",
		Buckets:   []float64{0.2, 0.5, 1, 2, 5, 10, 30},
	}, []string{"kind"})

	// MergerPendingTotal tracks pending messages in the merger queue.
	MergerPendingTotal = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "soundstage",
		Name:      "merger_pending_total",
		Help:      "Messages pending in merger queue by type",
	}, []string{"type"})

	// MergerChannelFullTotal counts messages dropped due to full merger channel.
	MergerChannelFullTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "soundstage",
		Name:      "merger_channel_full_total",
		Help:      "Messages dropped due to full merger channel by type",
	}, []string{"type"})

	// MergerRoomTotal counts messages batched for room delivery.
	MergerRoomTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "soundstage",
		Name:      "merger_room_total",
		Help:      "Messages batched for room delivery by result",
	}, []string{"result"})

	// MergerBroadcastTotal counts messages batched for broadcast delivery.
	MergerBroadcastTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "soundstage",
		Name:      "merger_broadcast_total",
		Help:      "Messages batched for broadcast delivery by result",
	}, []string{"result"})

	// Per-connection metrics
	// WSConnectionSendQueueDepth tracks the depth of the send queue per connection.
	WSConnectionSendQueueDepth = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "soundstage",
		Name:      "websocket_connection_send_queue_depth",
		Help:      "Depth of the WebSocket send queue per connection",
	}, []string{"room_id", "session_id"})

	// WSConnectionSendLatencySeconds observes the latency of sending messages per connection.
	WSConnectionSendLatencySeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "soundstage",
		Name:      "websocket_connection_send_latency_seconds",
		Help:      "Latency of sending WebSocket messages per connection",
		Buckets:   []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0},
	}, []string{"room_id"})

	// WSConnectionTotal counts total connections established.
	WSConnectionTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "soundstage",
		Name:      "websocket_connection_total",
		Help:      "Total WebSocket connections established",
	}, []string{"room_id", "result"})

	// WSConnectionDurationSeconds observes connection lifetime.
	WSConnectionDurationSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "soundstage",
		Name:      "websocket_connection_duration_seconds",
		Help:      "WebSocket connection lifetime in seconds",
		Buckets:   []float64{1, 5, 10, 30, 60, 300, 600, 1800, 3600},
	}, []string{"room_id", "close_reason"})
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
		AIModerationTotal,
		AIAgentRunsTotal,
		AIToolCallsTotal,
		AIRagQueriesTotal,
		AISSEConnections,
		AILatencySeconds,
		MergerPendingTotal,
		MergerChannelFullTotal,
		MergerRoomTotal,
		MergerBroadcastTotal,
		WSConnectionSendQueueDepth,
		WSConnectionSendLatencySeconds,
		WSConnectionTotal,
		WSConnectionDurationSeconds,
	)
}
