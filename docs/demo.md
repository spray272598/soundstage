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

