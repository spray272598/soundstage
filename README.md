# SoundStage

A backend for audio live rooms: real-time interaction, gifting, mic-link & PK, AI room moderation, and observability.

## Features

- **Audio live rooms**: room lifecycle, anchor/audience management, online presence.
- **Real-time interaction**: WebSocket gateway, danmaku, likes, gifts with broadcast.
- **Mic-link & PK**: signaling and state machine for co-hosting and PK battles.
- **Task system**: delayed gift settlement, live reminders, leaderboard refresh via asynq.
- **AI room moderator**: LLM-based audit, smart replies, RAG knowledge base, tool-calling.
- **Observability**: Prometheus, Grafana, pprof, OpenTelemetry, structured logs.

## Tech Stack

- Go 1.22+
- Gin / gorilla/websocket
- Kafka (event fan-out), Redis (cache/state), MySQL (business data)
- asynq (task queue)
- OpenAI-compatible API for AI features
- Prometheus + Grafana + OpenTelemetry

## Quick Start

```bash
# Start infrastructure
make docker-up

# Run server
make run
```

See [docs/demo.md](docs/demo.md) for a step-by-step Phase 1 walkthrough
(room lifecycle + WebSocket + Kafka broadcast + metrics).

## Architecture

See [docs/architecture.md](docs/architecture.md).

## License

MIT
