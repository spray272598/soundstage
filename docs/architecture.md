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
from the business contexts. Each context runs in its **own consumer group**, so
every context reads the same events independently:

1. Client connects to the `connection` WebSocket gateway.
2. Gateway publishes raw inbound messages to `soundstage.ingest`:
   - `chat`/`gift`/`like` → interaction context.
   - `signal` → miclink context (WebRTC co-host signaling).
   - `gift` → miclink context too, where it feeds a room's PK score when that
     room is in an active battle (event reuse across contexts).
3. Each business context consumes `soundstage.ingest` in its own group, runs the
   synchronous path (rate limit + moderation for interaction; state machine +
   scoring for miclink) and broadcasts results to `soundstage.broadcast`.
4. All gateway nodes consume `soundstage.broadcast` and fan out to local
   WebSocket sessions via per-room in-memory buckets. A `To` recipient on the
   envelope restricts delivery to a single user (used for point-to-point
   signaling).

The REST endpoints call the same services directly, so behavior is identical
regardless of entry point.

### Asynchronous tasks

The synchronous path only enqueues; the heavy lifting happens in the asynq
worker (Redis-backed), so a slow MySQL never blocks a WebSocket pump:

- `interaction:persist_danmaku` — write a danmaku to its day shard (retried on failure).
- `interaction:settle_gift` — mark a gift order settled and update leaderboards (idempotent).
- `interaction:flush_likes` — periodic (every 30s) snapshot of per-room like tallies to MySQL.

### Mic-link and PK (miclink context)

- **Co-host (连麦)**: an intra-room `MicLink` aggregate (requesting → connected →
  closed). WebRTC offer/answer/ice are relayed point-to-point between host and
  guest via the broadcast envelope's `To` recipient.
- **Cross-room PK (对战)**: a `PKSession` aggregate with a state machine
  (`pending` → `matched` → `ongoing` → `finished`). Each room's gifts during the
  battle feed that room's score. A countdown timer arms two asynq delayed tasks:
  - `miclink:pk_countdown` — warns clients shortly before the deadline.
  - `miclink:pk_settle` — finalizes the battle and decides the winner.
  - PK state transitions are guarded by a Redis distributed lock so concurrent
    triggers (from both rooms or a retry) cannot settle twice.

## AI Room Moderator (ai context)

Phase 4 turns the moderator into an LLM-powered agent. The context is layered
like the others (`domain` ports, `infrastructure` adapters, `application` use
cases, `transport` HTTP/SSE) and depends on no other context directly — it talks
to room/miclink/interaction through ports implemented in the `app` wiring.

### Responsibilities

- **Danmaku audit (replaces keyword moderation)**: `ai.Service` implements the
  `interaction.Moderator` port, so the interaction context swaps it in with no
  code change. A cheap keyword blocklist runs first (fast reject, no LLM call);
  when a real provider is configured a semantic LLM audit classifies each
  message. Without a key the pipeline degrades to the keyword fast-path, so the
  demo runs fully offline.
- **SSE smart reply**: `GET /rooms/:id/ai/chat` streams a conversational turn
  with the room moderator over Server-Sent Events (text deltas, tool calls,
  tool results, done).
- **Contextual auto-reply**: `POST /rooms/:id/ai/auto-reply` returns a short,
  RAG-grounded reply to a danmaku (used to auto-respond to the audience).
- **RAG knowledge base**: room rules / FAQ are embedded and stored in an
  in-process cosine index (`rag.Service`); the agent retrieves the top-K chunks
  to ground its answers. A mock embedder keeps it working without an embedding
  API.
- **Agent tool-calling**: a ReAct loop (`agent.Loop`) calls the model, executes
  requested tools, feeds results back, and repeats until the model answers
  without tools. Built-in tools:
  - `get_room_status` — online count, mic-link, PK score (via `RoomStatusProvider`).
  - `get_leaderboard` — gift rankings (via `LeaderboardProvider`).
  - `get_room_rules` — RAG retrieval from the knowledge base.
  - `mute_user` — bans a viewer for a duration (via `RoomModerator` → interaction `Muter`).
  - `send_announcement` — publishes an AI announcement to the room (via `Broadcaster`).

### Provider abstraction

- `domain.Gateway` is the LLM port. `llm.Gateway` is an OpenAI-compatible client
  with native tool calling + SSE tool-call parsing + retry/backoff;
  `llm.MockGateway` is a deterministic offline stand-in that speaks the same
  ReAct protocol (JSON tool calls) so the whole pipeline runs without an API key.
- `domain.Embedder` / `domain.KnowledgeBase` mirror the same real-vs-mock split.

### Mute enforcement

Muting is owned by the interaction context (`Muter` port + `RedisMuter`). The
agent's `mute_user` tool reaches it through the ai `RoomModerator` port, and
`InterService.ProcessDanmaku` rejects muted users before moderation/broadcast.

## Data Storage

- **MySQL**: durable business data.
  - `rooms` — room lifecycle.
  - `gifts` — platform-wide gift catalog (id, name, price, icon, status).
  - `gift_orders` — gift send records with a lifecycle (`created`/`settled`/`failed`),
    an `idempotency_key` unique index and reconciliation-friendly shape.
  - `danmaku_YYYYMMDD` — day-sharded danmaku tables, created lazily on first write.
  - `room_stats` — periodic snapshot of like/gift counters.
  - `mic_links` — co-host sessions (host, guest, status, timestamps).
  - `pk_sessions` — cross-room PK battles (both rooms, status, scores, deadline,
    winner).
- **Redis**: online state, like counters, gift leaderboards (sorted sets per
  room and period), rate-limit windows, distributed locks, asynq queues.

## Observability

- **Prometheus metrics** are exposed on a dedicated server (`config.metrics`,
  default `:9091/metrics`) using a custom registry. Every bounded context
  publishes counters/gauges/histograms — see [`docs/observability.md`](observability.md)
  for the full metric list and the Grafana dashboard JSON.
- **Grafana**: a provisioned `SoundStage Overview` dashboard ships under
  `deploy/observability/grafana/dashboards/`. Bring up Prometheus + Grafana with
  `deploy/observability/docker-compose.yml`.
- **Alerts**: `deploy/observability/prometheus/alert.rules.yml` covers target-down,
  moderation error rate, agent failure rate, and SSE connection saturation.
- **Structured JSON logs** via zap (level/format in `config.log`).
- **Load testing**: `cmd/loadtest` (dependency-free Go generator) and
  `deploy/loadtest/k6.js` exercise the SSE chat + danmaku ingest paths; see
  `deploy/loadtest/README.md`.

### Tuning knobs (Phase 5)

- `ai.agent_timeout` (default 60s) caps a single agent run; the ReAct loop wraps
  each run in a context timeout so a slow/hung LLM cannot pin an SSE connection
  open forever.
- `http.write_timeout` (default 120s) is deliberately generous so legitimate SSE
  streams that span a multi-round agent run are not cut off mid-stream;
  `http.read_timeout` guards slow header reads.
- `ai.agent_max_rounds`, `ai.rag_top_k`, `interaction.danmaku_rate_limit`, and the
  `Muter` TTL are the other primary dials.

## Dependency Inversion

Each bounded context keeps its `domain` package free of infrastructure imports.
Repositories, message producers, AI clients, and caches are defined as interfaces
in `domain` and implemented in `infrastructure`. The `app` package wires everything
together at startup.
