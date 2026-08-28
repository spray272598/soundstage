# API Reference

All endpoints are served by the gin router mounted in `internal/app`. The API
port is `:8080` (configurable via `http.addr`); Prometheus metrics are on
`:9091/metrics`.

Unless noted, request/response bodies are JSON. Timestamps use RFC 3339.

---

## Room (`room`)

Base: `/api/v1/rooms`

### `POST /api/v1/rooms`
Create a room.
- Body: `{ "anchor_id": "string (required)", "title": "string (required)" }`
- Response `201`: `{ "id", "anchor_id", "title", "status", "created_at", "started_at"?, "ended_at"? }`
- `status` ∈ `created | live | closed`.

### `POST /api/v1/rooms/:id/open`
Open (go live) a room. `404` if not found, `400` on invalid transition.

### `POST /api/v1/rooms/:id/close`
Close a room. `404` if not found.

### `GET /api/v1/rooms/:id`
Get room detail. `404` if not found.

### `GET /api/v1/rooms`
List recent rooms (currently limited to 20, offset 0).

---

## Connection (`connection`)

### `GET /ws/:room_id?user_id=...`
Upgrade to a WebSocket. After connect, the client receives broadcast frames
(`chat`, `gift`, `like`, `ai_announcement`, …) and can send interaction frames
(which are fanned out through Kafka to the interaction service). `user_id` is
auto-generated when omitted.

### `GET /api/v1/rooms/:room_id/online`
Current online user count.
- Response `200`: `{ "count": int }`

---

## Interaction (`interaction`)

Base: `/api/v1`

### `GET /api/v1/gifts`
Gift catalog. Response: `{ "items": [ { "id", "name", "price", "icon", "status" } ] }`.

### `POST /api/v1/rooms/:id/danmaku`
Send a danmaku. Runs rate-limit → mute check → moderation → broadcast.
- Body: `{ "text": "string (required)", "user_id": "string (optional; query param also accepted)" }`
- Response `200`: `{ "id", "room_id", "user_id", "content", "status", "created_at" }`
- Errors: `400` with `error: "user is muted"` / `"message rejected by moderator"` /
  `"rate limited"`. A blocked message is persisted for audit but never broadcast.

### `POST /api/v1/rooms/:id/gifts`
Send a gift (idempotent via `idempotency_key`).
- Body: `{ "gift_id": "string (required)", "count": int (required), "user_id"?, "idempotency_key"? }`
- Response `200`: `{ "order_id", "room_id", "gift_id", "gift_name", "count", "total_amount", "status" }`

### `POST /api/v1/rooms/:id/like`
Like the room. `user_id` via query or form.
- Response `200`: `{ "status": "ok" }`

### `GET /api/v1/rooms/:id/rank?period=day|week|month`
Gift leaderboard Top-20.
- Response `200`: `{ "period", "items": [ { "user_id", "amount" } ] }`

### `GET /api/v1/rooms/:id/likes`
Like tally. Response: `{ "count": int }`

---

## Mic-link & PK (`miclink`)

Base: `/api/v1/rooms/:id`

### Co-host (mic-link)
- `POST /api/v1/rooms/:id/miclink/request` — `{ "host_id", "guest_id" }`
- `POST /api/v1/rooms/:id/miclink/accept` — `{ "host_id", "guest_id" }`
- `POST /api/v1/rooms/:id/miclink/close`

Response (`miclink`): `{ "session_id", "room_id", "host_id", "guest_id", "status", "created_at", "closed_at" }`.

### Cross-room PK
- `POST /api/v1/rooms/:id/pk/challenge` — `{ "opponent_room_id", "anchor_id", "opponent_anchor_id" }`
- `POST /api/v1/rooms/:id/pk/:session_id/accept`
- `POST /api/v1/rooms/:id/pk/:session_id/start`
- `POST /api/v1/rooms/:id/pk/:session_id/score` — `{ "amount": int }`
- `POST /api/v1/rooms/:id/pk/:session_id/finish`
- `GET /api/v1/rooms/:id/pk/:session_id`

Response (`pk`): `{ "session_id", "room_a", "room_b", "status", "score_a", "score_b", "winner", "started_at"?, "ends_at"? }`.
`pk/score` returns `200 { "status": "ignored", "reason": "room not in an active pk" }` when the room is not in a PK.

---

## AI Room Moderator (`ai`)

### `GET /rooms/:id/ai/chat?message=...&user_id=...`
**Server-Sent Events.** Streams the AI moderator's reply. `message` is required;
`user_id` defaults to `host`.

Event frames (`event: <type>\ndata: <json>\n\n`):

| event         | data                                                       |
|---------------|------------------------------------------------------------|
| `text_delta`  | streamed answer fragment (string)                          |
| `tool_call`   | `{"tool": "<name>", "args": <json>}`                       |
| `tool_result` | `{"tool": "<name>", "result": "<string>"}`                 |
| `error`       | error message (string)                                     |
| `done`        | final answer (string)                                      |

The agent may call tools (`get_room_status`, `get_leaderboard`, `get_room_rules`,
`mute_user`, `send_announcement`) before emitting `done`. Offline (no API key)
the mock gateway emits a deterministic ReAct trace.

### `POST /rooms/:id/ai/auto-reply`
Non-streaming contextual reply to a danmaku.
- Body: `{ "user_id"?, "content": "string (required)" }`
- Response `200`: `{ "reply": "string" }`

### `POST /ai/knowledge`
Ingest a document into the RAG knowledge base.
- Body: `{ "title"?, "text": "string (required)" }`
- Response `200`: `{ "status": "indexed" }`

### `GET /ai/health`
Reports AI mode/model. Response: `{ "mode": "mock"|"llm", "model": "string" }`.

---

## Agent Tools (exposed to the model)

| Tool               | Purpose                                        | Args |
|--------------------|------------------------------------------------|------|
| `get_room_status`  | Online count, mic-link & PK state             | `room_id` |
| `get_leaderboard`  | Gift ranking by period                        | `room_id`, `period`, `n` |
| `get_room_rules`   | RAG retrieval of room rules / FAQ             | `query` |
| `mute_user`        | Mute a viewer (drives `interaction.Muter`)    | `room_id`, `user_id`, `duration_seconds` |
| `send_announcement`| Broadcast a room-wide announcement            | `room_id`, `text` |

The agent reaches these through the `ai` domain ports (`RoomStatusProvider`,
`LeaderboardProvider`, `RoomModerator`, `Broadcaster`, `KnowledgeBase`), all
implemented in `internal/app/ai_adapters.go` — the `ai` context never imports
`room`/`miclink`/`interaction`.
