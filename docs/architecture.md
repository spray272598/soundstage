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

- **connection**: WebSocket gateway, session registry, heartbeat, authentication.
- **room**: Room lifecycle, online presence, anchor/audience management.
- **interaction**: Danmaku, likes, gifts, broadcast.
- **miclink**: Mic-link and PK signaling, state machine, scoring.
- **task**: Delayed/periodic jobs via asynq (gift settlement, reminders, leaderboards).
- **ai**: LLM-based room moderator (audit, smart reply, RAG, agent tools).

## Communication Patterns

### Real-time broadcast

Real-time events flow through Kafka:

1. Client connects to the `connection` WebSocket gateway.
2. Gateway publishes events to Kafka topics such as `soundstage.room.<room_id>`.
3. All gateway nodes consume and fan out to local WebSocket sessions.
4. In-memory per-room buckets shard sessions to keep fan-out efficient.

### Asynchronous tasks

Background work is enqueued in Redis via asynq:

- Gift settlement and transaction records.
- Live start reminders.
- Leaderboard refresh.

## Data Storage

- **MySQL**: durable business data (rooms, users, gifts, transactions, PK records).
- **Redis**: online state, counters, leaderboards, distributed locks, idempotency.

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
