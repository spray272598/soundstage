# SoundStage Local Demo

This demo verifies Phase 1: room lifecycle, WebSocket gateway, heartbeat, and
Kafka-backed broadcast fan-out.

## Prerequisites

```bash
make docker-up
```

This starts MySQL, Redis, Kafka, ZooKeeper, Prometheus and Grafana.

## Run the server

```bash
cd configs
cp config.local.yaml.example config.local.yaml
# optionally set ai.api_key in config.local.yaml
cd ..
make run
```

The server listens on:

- HTTP API: http://127.0.0.1:8080
- WebSocket: ws://127.0.0.1:8080/ws/:room_id
- Metrics: http://127.0.0.1:9091/metrics
- Prometheus: http://127.0.0.1:9090
- Grafana: http://127.0.0.1:3000 (admin/admin)

## Step 1: Create and open a room

```bash
curl -X POST http://127.0.0.1:8080/api/v1/rooms \
  -H "Content-Type: application/json" \
  -d '{"anchor_id":"anchor001","title":"Friday Night Audio Live"}'
```

Save the returned `id` as `ROOM_ID`.

```bash
curl -X POST http://127.0.0.1:8080/api/v1/rooms/$ROOM_ID/open
```

## Step 2: Connect via WebSocket

Open two browser tabs or use `wscat`:

```bash
# Tab 1
wscat -c "ws://127.0.0.1:8080/ws/$ROOM_ID?user_id=user001"

# Tab 2
wscat -c "ws://127.0.0.1:8080/ws/$ROOM_ID?user_id=user002"
```

## Step 3: Send a chat message

In Tab 1 type:

```json
{"type":"chat","payload":{"text":"hello from user001"}}
```

Tab 2 should receive the same message, demonstrating hub broadcast plus
Kafka fan-out consumed by the local gateway.

## Step 4: Check online count and metrics

```bash
curl http://127.0.0.1:8080/api/v1/rooms/$ROOM_ID/online
```

```bash
curl http://127.0.0.1:9091/metrics | grep soundstage
```

## Step 5: Gifts, danmaku and likes (Phase 2)

Gifts are a platform-wide catalog; rooms only decide whether gifting is on.
Send a gift over REST or WebSocket — both hit the same `InterService`.

```bash
# List the gift catalog (seeded by scripts/sql/schema.sql).
curl http://127.0.0.1:8080/api/v1/gifts

# Send a gift (returns a gift_order with status "created").
curl -X POST http://127.0.0.1:8080/api/v1/rooms/$ROOM_ID/gifts \
  -H "Content-Type: application/json" \
  -d '{"user_id":"user001","gift_id":"g_rose","count":10}'

# Resend with the same idempotency_key -> returns the same order (no double charge).
curl -X POST http://127.0.0.1:8080/api/v1/rooms/$ROOM_ID/gifts \
  -H "Content-Type: application/json" \
  -d '{"user_id":"user001","gift_id":"g_rose","count":10,"idempotency_key":"demo-1"}'
```

Over WebSocket (in Tab 1):

```json
{"type":"gift","payload":{"gift_id":"g_rocket","count":1}}
```

Danmaku goes through the same path: synchronous moderation + rate limit, then
broadcast; persistence is enqueued to asynq and written to a day-sharded table
asynchronously so the WS pump is never blocked.

```bash
curl -X POST http://127.0.0.1:8080/api/v1/rooms/$ROOM_ID/danmaku \
  -H "Content-Type: application/json" \
  -d '{"user_id":"user001","text":"love this song"}'

# A blocked keyword is dropped and never broadcast.
curl -X POST http://127.0.0.1:8080/api/v1/rooms/$ROOM_ID/danmaku \
  -H "Content-Type: application/json" \
  -d '{"user_id":"user001","text":"click my 广告 link"}'
```

Likes are counted in Redis and flushed to `room_stats` every 30s by the asynq
scheduler.

```bash
curl -X POST "http://127.0.0.1:8080/api/v1/rooms/$ROOM_ID/like?user_id=user002"
curl http://127.0.0.1:8080/api/v1/rooms/$ROOM_ID/likes
```

## Step 6: Gift leaderboard (day / week / month)

Settlement increments the per-room, per-period sorted set in Redis. The rank
is maintained incrementally at settlement time, never recomputed on read.

```bash
curl "http://127.0.0.1:8080/api/v1/rooms/$ROOM_ID/rank?period=day"
curl "http://127.0.0.1:8080/api/v1/rooms/$ROOM_ID/rank?period=week"
curl "http://127.0.0.1:8080/api/v1/rooms/$ROOM_ID/rank?period=month"
```

## Observability notes

- `soundstage_interaction_danmaku_total{result="accepted|rejected"}`
- `soundstage_interaction_gift_total{result="accepted|rejected"}`
- `soundstage_interaction_like_total`
- `soundstage_gift_order_status_total{status="created|settled|failed"}`
- `soundstage_asynq_task_enqueued_total{type="interaction:persist_danmaku|interaction:settle_gift"}`
- asynq queues are inspectable via `asynqmon` or the Redis `asynq_*` keys.

## Step 7: Co-host mic-link (连麦) and cross-room PK (Phase 3)

Both use cases live in the `miclink` bounded context. The state machine and
signaling are owned by the backend; the actual audio mix is external (WebRTC/SFU).

### 7.1 Co-host (mic-link) within one room

A guest (audience member) asks to join the host's mic; the host accepts.

```bash
# Guest user002 requests to co-host in ROOM_ID (host is anchor001).
curl -X POST http://127.0.0.1:8080/api/v1/rooms/$ROOM_ID/miclink/request \
  -H "Content-Type: application/json" \
  -d '{"host_id":"anchor001","guest_id":"user002"}'

# Host accepts the request.
curl -X POST http://127.0.0.1:8080/api/v1/rooms/$ROOM_ID/miclink/accept \
  -H "Content-Type: application/json" \
  -d '{"host_id":"anchor001","guest_id":"user002"}'
```

WebRTC signaling (offer/answer/ice) is relayed point-to-point over WebSocket:

```json
{"type":"signal","payload":{"to":"user002","signal_type":"offer","data":{"sdp":"..."}}}
```

The gateway routes `signal` messages through Kafka to the miclink consumer, which
relays them to the target user only (the broadcast envelope carries a `To`
recipient, delivered by the hub's `SendToUser`).

```bash
# Either side can end the co-host session.
curl -X POST http://127.0.0.1:8080/api/v1/rooms/$ROOM_ID/miclink/close
```

### 7.2 Cross-room PK battle

Create **two** rooms (`ROOM_A`, `ROOM_B`) and open both. Room A challenges room B.

```bash
# ROOM_A challenges ROOM_B.
curl -X POST http://127.0.0.1:8080/api/v1/rooms/$ROOM_A/pk/challenge \
  -H "Content-Type: application/json" \
  -d '{"anchor_id":"anchorA","opponent_room_id":"'$ROOM_B'","opponent_anchor_id":"anchorB"}'

# Save the returned session_id as PK_ID, then ROOM_B accepts.
curl -X POST http://127.0.0.1:8080/api/v1/rooms/$ROOM_B/pk/$PK_ID/accept

# ROOM_A starts the battle (arms the countdown timer).
curl -X POST http://127.0.0.1:8080/api/v1/rooms/$ROOM_A/pk/$PK_ID/start
```

A battle scores from gifts sent in either room. Send gifts via WebSocket in
either room during the battle — the miclink consumer feeds `gift` events into
that room's PK side automatically. You can also bump a side explicitly:

```bash
curl -X POST http://127.0.0.1:8080/api/v1/rooms/$ROOM_A/pk/$PK_ID/score \
  -H "Content-Type: application/json" \
  -d '{"amount":500}'
```

When the countdown expires, an asynq delayed task settles the battle and pushes
`pk_finish` (with the winner) to both rooms. Both rooms also receive live
`pk_score` updates. Inspect the state any time:

```bash
curl http://127.0.0.1:8080/api/v1/rooms/$ROOM_A/pk/$PK_ID
```

### Observability for mic-link / PK

- `soundstage_miclink_requests_total{result="requested|accepted"}`
- `soundstage_pk_sessions_total{status="pending|matched|ongoing|finished"}`
- `soundstage_pk_score_total{side="a|b"}`
- `soundstage_signaling_relayed_total`
- `soundstage_asynq_task_enqueued_total{type="miclink:pk_settle|miclink:pk_countdown"}`

## Step 8: AI room moderator (Phase 4)

The moderator is on by default. With no `ai.api_key` configured it runs in
**mock mode** (keyword fast-path + offline agent), so every command below works
without any external API. Set `ai.api_key` (or `SOUNDSTAGE_AI_API_KEY`) for real
LLM audit, RAG embeddings, and genuine tool reasoning.

### 8.1 Health

```bash
curl http://127.0.0.1:8080/ai/health
# {"mode":"mock","model":"mock"}   (or "llm" with a configured key)
```

### 8.2 Danmaku audit (replaces keyword moderation)

Send a danmaku with a blocked keyword — it is rejected and never broadcast:

```bash
curl -X POST http://127.0.0.1:8080/api/v1/rooms/$ROOM_ID/danmaku \
  -H "Content-Type: application/json" \
  -d '{"user_id":"user001","text":"buy my 广告 now"}'
# -> 409, ErrRejected (blocked by keyword fast-path)
```

In LLM mode the same endpoint runs a semantic audit; the moderation decision is
labeled by path in metrics (`soundstage_ai_moderation_total{decision,path}`).

### 8.3 SSE smart reply (conversational)

Stream a chat with the AI room moderator. It may call tools (`get_room_status`,
`get_leaderboard`, `get_room_rules`, `mute_user`, `send_announcement`) and stream
`text_delta` / `tool_call` / `tool_result` / `done` events:

```bash
curl -N "http://127.0.0.1:8080/rooms/$ROOM_ID/ai/chat?user_id=host&message=现在直播间多少人？"
```

In mock mode the moderator answers from its built-in rulebook and can simulate a
tool call (e.g. status/leaderboard/mute/announce) without a key.

### 8.4 Contextual auto-reply

```bash
curl -X POST http://127.0.0.1:8080/rooms/$ROOM_ID/ai/auto-reply \
  -H "Content-Type: application/json" \
  -d '{"user_id":"user002","content":"主播今晚唱什么歌？"}'
# {"reply":"..."}
```

### 8.5 RAG knowledge base

The default room rules/FAQ are seeded at startup. Extend them at runtime:

```bash
curl -X POST http://127.0.0.1:8080/ai/knowledge \
  -H "Content-Type: application/json" \
  -d '{"title":"新规则","text":"每晚 10 点后进行点歌环节，欢迎弹幕点歌。"}'
```

After ingestion, asking the moderator about the new rule will retrieve this chunk.

### 8.6 Agent tool-calling: mute a viewer

Ask the moderator to mute someone (mock or real mode) — it invokes `mute_user`
and the viewer's subsequent danmaku is rejected by `InterService`:

```bash
curl -N "http://127.0.0.1:8080/rooms/$ROOM_ID/ai/chat?user_id=host&message=把 user002 禁言 5 分钟"
```

```bash
# This danmaku from the muted user is now rejected.
curl -X POST http://127.0.0.1:8080/api/v1/rooms/$ROOM_ID/danmaku \
  -H "Content-Type: application/json" \
  -d '{"user_id":"user002","text":"hello?"}'
# -> 409, ErrMuted
```

### Observability for the AI context

- `soundstage_ai_moderation_total{decision="allowed|rejected",path="keyword|llm|llm_error"}`
- `soundstage_ai_agent_runs_total{outcome="ok|error|canceled"}`
- `soundstage_ai_tool_calls_total{tool="get_room_status|get_leaderboard|get_room_rules|mute_user|send_announcement",result="ok|error|unknown"}`
- `soundstage_ai_rag_queries_total{result="found|miss"}`
- `soundstage_ai_sse_connections` (gauge)
- `soundstage_ai_llm_latency_seconds{kind="agent"}` (histogram)

## Step 9: Observability & load test (Phase 5)

### 9.1 Metrics server

Metrics live on a dedicated server (default `:9091/metrics`), separate from the
API (`:8080`):

```bash
curl -s http://127.0.0.1:9091/metrics | grep soundstage_ai
```

### 9.2 Grafana dashboard

Bring up Prometheus + Grafana (the app must already be running on the host):

```bash
cd deploy/observability && docker compose up -d
```

Open http://localhost:3000 (admin/admin). The **SoundStage Overview** dashboard
is auto-provisioned and shows AI moderation rate, agent runs, tool calls, RAG
hits, live SSE connections, LLM p95 latency, danmaku throughput, WebSocket
stats, and PK/gift/miclink counters. Alert rules (target down, moderation error
rate, agent failures, SSE saturation) are in `prometheus/alert.rules.yml`.

### 9.3 Load test the SSE chat + danmaku ingest

Dependency-free Go generator (no k6 needed):

```bash
go run ./cmd/loadtest -base http://127.0.0.1:8080 -vus 50 -duration 30s -danmaku-rate 200
```

Or with k6:

```bash
k6 run -e BASE=http://127.0.0.1:8080 deploy/loadtest/k6.js
```

Both report SSE latency (avg/p50/p95/p99), error rates, and throughput. With the
mock LLM, SSE turns finish in a few ms; with a real LLM the p95 tracks provider
latency. Full reference: [`docs/observability.md`](observability.md).


