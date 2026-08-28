# Observability & Load Testing

SoundStage exposes Prometheus metrics on a dedicated server and ships a
Grafana dashboard, alert rules, and two load-test tools. This document ties them
together.

## Metrics endpoint

The app starts a separate HTTP server for metrics (configured by `metrics.addr`
/ `metrics.path`, default `:9091/metrics`). All metrics live under the
`soundstage_` namespace and are registered in
[`internal/pkg/metrics/metrics.go`](../internal/pkg/metrics/metrics.go).

| Metric | Type | Labels | Meaning |
| --- | --- | --- | --- |
| `soundstage_websocket_connections` | gauge | `room_id` | Active WebSocket connections per room |
| `soundstage_websocket_messages_total` | counter | `type`, `direction` | WebSocket messages processed |
| `soundstage_interaction_danmaku_total` | counter | `result` (`accepted`/`rejected`) | Danmaku processed by the interaction context |
| `soundstage_interaction_gift_total` | counter | `result` | Gift messages processed |
| `soundstage_interaction_like_total` | counter | — | Like messages processed |
| `soundstage_gift_order_status_total` | counter | `status` | Gift order status transitions |
| `soundstage_asynq_task_enqueued_total` | counter | `type` | Background tasks enqueued |
| `soundstage_miclink_requests_total` | counter | `result` | Co-host mic-link requests |
| `soundstage_pk_sessions_total` | counter | `status` | PK sessions by terminal status |
| `soundstage_pk_score_total` | counter | `side` | PK score points by side |
| `soundstage_signaling_relayed_total` | counter | — | WebRTC signaling messages relayed |
| `soundstage_ai_moderation_total` | counter | `decision`, `path` | AI moderation calls (`allowed`/`rejected`/`error` × `keyword`/`llm`/`llm_error`) |
| `soundstage_ai_agent_runs_total` | counter | `outcome` | Agent (ReAct) runs (`ok`/`error`/`canceled`) |
| `soundstage_ai_tool_calls_total` | counter | `tool`, `result` | Agent tool invocations (`ok`/`error`/`unknown`) |
| `soundstage_ai_rag_queries_total` | counter | `result` | RAG queries (`found`/`miss`) |
| `soundstage_ai_sse_connections` | gauge | — | Active AI chat SSE connections |
| `soundstage_ai_llm_latency_seconds` | histogram | `kind` | LLM call latency (buckets 0.2/0.5/1/2/5/10/30s) |

Quick check while the server runs:

```bash
curl -s http://127.0.0.1:9091/metrics | grep soundstage_ai
```

## Prometheus + Grafana

Run the app on the host (it needs MySQL/Redis/Kafka), then bring up the
observability stack in Docker:

```bash
cd deploy/observability
docker compose up -d
```

- Prometheus UI: http://localhost:9090 (scrapes `host.docker.internal:9091`)
- Grafana: http://localhost:3000 (admin/admin) — the **SoundStage Overview**
  dashboard is auto-provisioned under the `SoundStage` folder.
- On Linux, `host.docker.internal` is mapped via `extra_hosts` in the compose
  file; if you run the app inside the same compose network instead, point the
  scrape target at the app service name.

The dashboard (`grafana/dashboards/soundstage-overview.json`) shows: AI
moderation rate, agent runs by outcome, tool calls, RAG hits, live SSE
connection count, LLM p95 latency, danmaku throughput, WebSocket connections and
message rates, and PK/gift/miclink counters.

## Alerts

`prometheus/alert.rules.yml` defines:

- `SoundStageTargetDown` — metrics target unreachable for 2m.
- `SoundStageAIModerationErrorsHigh` — moderation error rate > 20% over 5m.
- `SoundStageAgentFailuresHigh` — > 10 agent runs failed in 5m.
- `SoundStageSSEConnectionsSaturated` — > 500 concurrent SSE streams for 10m.

Wire Alertmanager (included in the compose) to your notification channel to
receive them.

## Load testing

Two tools target the SSE chat and danmaku ingest paths against a running server.
See [`deploy/loadtest/README.md`](../deploy/loadtest/README.md).

- `cmd/loadtest` — dependency-free Go generator:
  `go run ./cmd/loadtest -base http://localhost:8080 -vus 50 -duration 30s`
- `deploy/loadtest/k6.js` — k6 scenario with arrival-rate + thresholds.

## Benchmarks

The RAG index and the ReAct loop have Go benchmarks (`go test -bench=.`):

| Benchmark | Result (dev laptop, i7-12700) | Notes |
| --- | --- | --- |
| `BenchmarkMemIndexSearch` (2k docs, topK=5) | ~444 µs/op, 4 allocs | Brute-force cosine scan |
| `BenchmarkKnowledgeQuery` (200 docs) | ~38 µs/op, 8 allocs | Embed + search + filter (mock embedder) |
| `BenchmarkLoopToolCalling` (2 LLM calls + 1 tool) | ~2.9 µs/op loop overhead | Scripted gateway, no network |

Takeaways: retrieval and loop overhead are sub-millisecond; under real load the
SSE p95 is dominated by **LLM provider latency** (real mode) and **Kafka/Redis
round-trips** on the ingest path, not the Go code.

## Tuning knobs

| Config | Default | Effect |
| --- | --- | --- |
| `ai.agent_timeout` | `60s` | Caps one agent run; wraps the run context so a hung LLM can't pin SSE open |
| `ai.agent_max_rounds` | `8` | Max tool-calling iterations before forcing a summary |
| `ai.rag_top_k` | `3` | Chunks retrieved per RAG query |
| `http.write_timeout` | `120s` | Generous so SSE streams spanning a multi-round run aren't cut |
| `http.read_timeout` | `10s` | Guards slow header reads |
| `interaction.danmaku_rate_limit` / `danmaku_rate_window` | `10` / `1s` | Per-user danmaku rate limit |
| `Muter` min TTL | `1s` | Floor on mute duration (avoids zero-length mutes) |
