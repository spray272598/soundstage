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
