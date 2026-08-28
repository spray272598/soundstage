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

