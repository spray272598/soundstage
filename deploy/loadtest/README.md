# SoundStage Load Test

Two tools exercise the two hottest paths: the **SSE AI chat stream** and the
**danmaku ingest** endpoint. Both target a *running* server (MySQL + Redis +
Kafka + the `soundstage` binary on `:8080`, metrics on `:9091`).

## Option A — Go loadgen (no install)

Dependency-free; ships in the module.

```bash
# from repo root, with the server already running
go run ./cmd/loadtest -base http://localhost:8080 -vus 50 -duration 30s -danmaku-rate 200
```

Flags:

| flag           | default                  | meaning                                  |
| -------------- | ------------------------ | ---------------------------------------- |
| `-base`        | `http://localhost:8080`  | HTTP base URL                            |
| `-rooms`       | `1`                      | rooms created and shared across VUs      |
| `-vus`         | `50`                     | concurrent SSE chat virtual users        |
| `-duration`    | `30s`                    | test duration                            |
| `-danmaku-rate`| `200`                    | aggregate danmaku POSTs per second       |
| `-message`     | "房间现在多少人…"        | SSE chat prompt                          |
| `-danmaku-text`| "加油主播！"             | danmaku body                             |

It prints SSE latency (avg/p50/p95/p99), error rates, and throughput for both
paths. With the **mock LLM** (no `ai.api_key`), SSE turns complete in a few ms;
with a **real LLM** the p95 is dominated by provider latency (see the
`ai_llm_latency_seconds` histogram in Grafana).

## Option B — k6

For teams that already use k6 and want arrival-rate scenarios + thresholds.

```bash
k6 run -e BASE=http://localhost:8080 deploy/loadtest/k6.js
```

The script defines two scenarios: `sse_chat` (constant VUs) and `danmaku`
(constant arrival rate), with thresholds `sse_duration p95 < 5s` and
`danmaku_errors rate < 1%`.

## What good looks like

Single-node, mock LLM, modest hardware (values from the benchmark suite):

- RAG query (embed + cosine search + filter): **~38 µs**
- ReAct loop overhead per turn (2 model calls + 1 tool): **~2.9 µs**
- Brute-force index scan at 2k docs / topK=5: **~444 µs**

Under load the SSE p95 is essentially `agent_latency + streaming overhead`; the
bottlenecks are the LLM provider (real mode) and Kafka/Redis round-trips on the
ingest path, not the Go code.
