# SoundStage

A backend for audio live rooms: real-time interaction, gifting, mic-link & PK,
an AI room moderator (LLM moderation, smart replies, RAG, tool-calling), and
first-class observability.

SoundStage is built as a **modular monolith** of DDD-lite bounded contexts
(`room`, `connection`, `miclink`, `interaction`, `ai`) wired by **dependency
inversion**: each context exposes its capabilities through ports, and the
composition root (`internal/app`) injects infrastructure adapters — so the `ai`
context never imports `room`/`miclink`/`interaction` directly.

The whole system runs **offline**: with no API key the AI moderator degrades to
a deterministic mock (ReAct JSON protocol + in-memory embeddings), so the demo,
tests, and load generator need zero external credentials.

![CI](https://github.com/spray272598/soundstage/actions/workflows/ci.yml/badge.svg)

## Features

- **Audio live rooms** — room lifecycle (create/open/close), anchor/audience, online presence.
- **Real-time interaction** — WebSocket gateway, danmaku, likes, gifts with fan-out broadcast.
- **Mic-link & PK** — co-host signaling and a cross-room PK state machine.
- **Task system** — delayed gift settlement, like-flush, leaderboard refresh via asynq.
- **AI room moderator** — LLM-based danmaku audit, SSE smart replies, a RAG knowledge base, and agent tool-calling (`get_room_status`, `get_leaderboard`, `get_room_rules`, `mute_user`, `send_announcement`).
- **Observability** — Prometheus + Grafana, structured logs, and load tests.

## Architecture

```
                ┌─────────────────────────────────────────────┐
   clients ───► │  gin router (internal/app)                   │
                │   ├─ room        (lifecycle)                 │
                │   ├─ connection   (WebSocket + presence)     │
                │   ├─ interaction  (danmaku/gift/like/rank)   │
                │   ├─ miclink      (co-host + PK)             │
                │   └─ ai           (moderator / SSE / RAG)    │
                └─────────────────────────────────────────────┘
                          │ ports (dependency inversion)
        ┌──────────┬───────┴────────┬──────────────┬─────────────┐
      Kafka      Redis            MySQL         asynq          LLM gateway
   (fan-out)  (mute/rank/  (business)       (tasks)        (OpenAI-compatible,
             like/presence)                                      mock fallback)
```

See [`docs/architecture.md`](docs/architecture.md) for the full bounded-context map,
and [`docs/api-reference.md`](docs/api-reference.md) for every endpoint.

## Tech Stack

- Go 1.26
- Gin / gorilla/websocket
- Kafka (event fan-out), Redis (cache/state), MySQL (business data)
- asynq (task queue)
- OpenAI-compatible API for AI features (mock gateway when no key is set)
- Prometheus + Grafana

## Quick Start

```bash
# 1. Bring up dependencies (Kafka, Redis, MySQL, Prometheus, Grafana).
docker compose -f deploy/observability/docker-compose.yml up -d

# 2. Run the server (config in configs/config.yaml).
go run ./cmd/server

# 3. (Optional) Emit some load against the SSE chat + danmaku paths.
go run ./cmd/loadtest -vus 50 -duration 30s
```

The server exposes:

- `:8080` — REST + WebSocket + SSE API
- `:9091/metrics` — Prometheus scrape endpoint

When `ai.api_key` is empty the AI moderator runs fully offline (mock LLM +
in-memory embeddings). Set `ai.api_key` (and optionally `ai.base_url`,
`ai.model`, `ai.embedding_*`) to switch to a real OpenAI-compatible provider.

## AI Room Moderator (Phase 4)

- **LLM danmaku moderation** — transparently swaps in via the `interaction.Moderator`
  port: keyword fast-path → LLM semantic audit → keyword-only fallback (fails open).
- **SSE smart replies** — `GET /rooms/:id/ai/chat` streams `text_delta` /
  `tool_call` / `tool_result` / `error` / `done` frames.
- **RAG knowledge base** — in-memory cosine index, seeded with room rules/FAQ.
- **Agent tool-calling** — a ReAct loop with native + content-JSON tool detection.

## Endpoints (summary)

| Context     | Method & Path                                            |
|-------------|----------------------------------------------------------|
| room        | `POST /api/v1/rooms`, `POST /api/v1/rooms/:id/open` …    |
| connection  | `GET /ws/:room_id`, `GET /api/v1/rooms/:room_id/online`  |
| interaction | `POST /api/v1/rooms/:id/danmaku`, `…/gifts`, `…/like` …  |
| miclink     | `POST /api/v1/rooms/:id/miclink/request` … `…/pk/*`      |
| ai          | `GET /rooms/:id/ai/chat`, `POST /rooms/:id/ai/auto-reply`, `POST /ai/knowledge`, `GET /ai/health` |

Full request/response schemas are in [`docs/api-reference.md`](docs/api-reference.md).

## Observability & Load Test

`deploy/observability` ships a Prometheus + Alertmanager + Grafana stack with a
provisioned **SoundStage Overview** dashboard and alert rules. `deploy/loadtest`
has a k6 script and a dependency-free Go generator. See
[`docs/observability.md`](docs/observability.md).

## Development

```bash
# Format, vet, build, test.
gofmt -w .
go vet ./...
go build ./...
go test ./...

# Benchmarks (RAG index, RAG query, agent loop).
go test -bench=. -benchmem ./internal/ai/...

# Integration tests (no Kafka/Redis/MySQL/LLM required).
go test ./internal/ai/transport/... ./internal/interaction/application/...
```

## Documentation

- [`docs/architecture.md`](docs/architecture.md) — bounded contexts, ports, data model.
- [`docs/api-reference.md`](docs/api-reference.md) — every REST/SSE endpoint.
- [`docs/demo.md`](docs/demo.md) — step-by-step walkthrough.
- [`docs/observability.md`](docs/observability.md) — metrics, dashboards, alerts, load test.

## License

MIT
