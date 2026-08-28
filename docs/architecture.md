# SoundStage Architecture

## Overview

SoundStage is a modular monolith backend for audio live rooms. It is organized
into bounded contexts, each with layered architecture:

```
transport -> application -> domain
infrastructure implements domain ports
```

Domain layers define ports (interfaces); infrastructure provides adapters.
This keeps core business logic independent of frameworks and external services.

## Bounded Contexts

- **connection**: WebSocket gateway, session registry (per-room in-memory buckets), heartbeat, authentication. Publishes inbound client messages to the ingest topic.
- **room**: Room lifecycle, online presence, anchor/audience management.
- **interaction**: The single processor for danmaku, likes and gifts. It owns moderation, rate limiting, broadcasting and background task enqueuing. Both the WebSocket ingest path (via Kafka) and the REST path call into it.
- **miclink**: Mic-link and PK signaling, state machine, scoring.
- **task**: Delayed/periodic jobs via asynq (gift settlement, reminders, leaderboards).
- **ai**: LLM-based room moderator (audit, smart reply, RAG, agent tools).

## Communication Patterns

### Real-time broadcast

Real-time events flow through two Kafka topics, keeping the gateway decoupled
from the interaction context:

1. Client connects to the `connection` WebSocket gateway.
2. Gateway publishes raw inbound messages (`chat`/`gift`/`like`) to `soundstage.ingest`.
3. The interaction context consumes `soundstage.ingest`, runs the synchronous
   path (rate limit + moderation) and broadcasts the result to `soundstage.broadcast`.
4. All gateway nodes consume `soundstage.broadcast` and fan out to local
   WebSocket sessions via per-room in-memory buckets.

The REST endpoints call the same `InterService` directly, so behavior is
identical regardless of entry point.

### Asynchronous tasks

The synchronous path only enqueues; the heavy lifting happens in the asynq
worker (Redis-backed), so a slow MySQL never blocks a WebSocket pump:

- `interaction:persist_danmaku` — write a danmaku to its day shard (retried on failure).
- `interaction:settle_gift` — mark a gift order settled and update leaderboards (idempotent).
- `interaction:flush_likes` — periodic (every 30s) snapshot of per-room like tallies to MySQL.

## Data Storage

- **MySQL**: durable business data.
  - `rooms` — room lifecycle.
  - `gifts` — platform-wide gift catalog (id, name, price, icon, status).
  - `gift_orders` — gift send records with a lifecycle (`created`/`settled`/`failed`),
    an `idempotency_key` unique index and reconciliation-friendly shape.
  - `danmaku_YYYYMMDD` — day-sharded danmaku tables, created lazily on first write.
  - `room_stats` — periodic snapshot of like/gift counters.
- **Redis**: online state, like counters, gift leaderboards (sorted sets per
  room and period), rate-limit windows, distributed locks, asynq queues.

## Observability

- Prometheus metrics exposed on `/metrics`.
- Grafana dashboards for connections, broadcast throughput, task latency.
- OpenTelemetry traces across HTTP/WebSocket and Kafka handlers.
- Structured JSON logs via zap.
- pprof endpoints for profiling.

## Dependency Inversion

Each bounded context keeps its `domain` package free of infrastructure imports.
Repositories, message producers, AI clients, and caches are defined as interfaces
in `domain` and implemented in `infrastructure`. The `app` package wires everything
together at startup.
