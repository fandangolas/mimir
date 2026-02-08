# Development Phases

This document defines the incremental delivery plan for the personal assistant. Each phase produces a working, usable system — not just infrastructure.

---

## Phase 1: Telegram ↔ LLM (Working Assistant) ✅

**Goal:** A real conversation via Telegram, powered by a local LLM. No calendar, no drive, no RAG yet — just chat.

### Deliverables

- [x] `go.mod` initialized, directory structure scaffolded
- [x] `docker-compose.yml` with PostgreSQL (+ pgvector)
- [x] `internal/config` — load config from env (Telegram token, Ollama URL, DB DSN, model name)
- [x] `internal/store` — DB connection + initial migrations (chat sessions, messages)
- [x] `internal/llm/ollama` — Ollama client: chat completion + model capability check at startup
- [x] `internal/telegram` — long polling loop, message ingestion, send message
- [x] `internal/orchestrator` — wire Telegram → LLM → Telegram response
- [x] Basic conversation context: persist messages, pass last N turns to LLM

### Definition of Done

> ✅ Send a message on Telegram, get a coherent LLM response back. Conversation history persists across restarts.

---

## Phase 2: Strong Foundation (Observability, Testing, Security) ✅

**Goal:** Make the codebase production-quality *for a personal project*. Harden what exists and add access control.

### Deliverables

#### Observability
- [x] `internal/observability` — structured JSON logging via `slog` with correlation IDs per message (`chat_id-message_id`)
- [x] Logs written to `./logs/assistant-YYYY-MM-DD.log` (tailed by Promtail) and stdout
- [x] Prometheus metrics endpoint (`:9090`) — message count, LLM latency histogram, error rates, unauthorized access
- [x] Loki log aggregation + Promtail shipper (Docker Compose)
- [x] Grafana dashboards auto-provisioned (Prometheus + Loki data sources)
- [x] OpenTelemetry tracing for the orchestrator request path (stdout exporter)

#### Resilience
- [x] Circuit breaker for Telegram API calls (opens after 5 consecutive failures)
- [x] Circuit breaker for Ollama API calls (opens after 3 consecutive failures)
- [x] Graceful shutdown — drains all in-flight LLM goroutines on SIGTERM via `sync.WaitGroup`
- [x] Context cancellation propagated throughout

#### Security
- [x] User allowlist via `ALLOWED_USER_IDS` env var (comma-separated Telegram user IDs)
- [x] Unauthorized messages silently dropped, logged as warning, and counted in Prometheus (`assistant_unauthorized_messages_total`)

#### Testing
- [x] Unit tests for config validation, Ollama model capability check, orchestrator message building
- [x] `go test ./...` passes cleanly

#### Code Quality
- [x] `golangci-lint` configured (`.golangci.yml`)
- [x] Makefile with targets: `build`, `test`, `lint`, `run`, `up`, `down`
- [x] All secrets loaded from env, never hardcoded

### Definition of Done

> ✅ The system is observable, recovers from transient failures, rejects unauthorized users, and has a test suite that prevents regressions.

### Observability Stack (Docker Compose)

| Service | Port | Purpose |
|---|---|---|
| Prometheus | 9091 | Scrapes metrics from app on `:9090` |
| Grafana | 3000 | Dashboards (admin/admin) |
| Loki | 3100 | Log storage |
| Promtail | — | Tails `./logs/*.log` → Loki |

### Prometheus Metrics

| Metric | Type | Description |
|---|---|---|
| `assistant_messages_received_total` | Counter | All messages received |
| `assistant_messages_processed_total{status}` | Counter | Processed by success/error |
| `assistant_llm_duration_seconds` | Histogram | LLM response latency |
| `assistant_telegram_send_errors_total` | Counter | Failed Telegram sends |
| `assistant_unauthorized_messages_total` | Counter | Rejected unauthorized messages |

---

## Phase 3: Google Integrations via MCP

**Goal:** Add calendar and drive awareness. The assistant can answer questions like "what do I have tomorrow?" or "find my project notes".

### MCP Strategy

| Integration | Approach | Rationale |
|---|---|---|
| **Google Calendar** | Reuse [`nspady/google-calendar-mcp`](https://github.com/nspady/google-calendar-mcp) | Feature-rich, multi-account, battle-tested (TypeScript) |
| **Google Drive** | Reuse [official `modelcontextprotocol/gdrive`](https://github.com/modelcontextprotocol/servers/tree/main/src/gdrive) | Official Anthropic implementation, well-maintained (TypeScript) |
| **Telegram (send/notify)** | **Build** internal tool | We own the bot and the polling loop; an external MCP adds no value here |

> **Note:** The existing Calendar and Drive MCP servers are TypeScript-based. They run as separate processes and communicate with our Go binary via the MCP stdio/HTTP transport. No Go reimplementation needed.

### Deliverables

#### Google OAuth + Daily Sync
- [ ] `internal/google/auth` — OAuth 2.0 flow for Calendar and Drive (tokens encrypted at rest)
- [ ] `internal/google/calendar/sync` — daily pull of events into PostgreSQL
- [ ] `internal/google/drive/sync` — daily pull + chunking of docs in "assistant vault" folder

#### RAG Pipeline
- [ ] `internal/rag/chunker` — split drive documents into 512–1024 token chunks
- [ ] `internal/rag/retriever` — hybrid search: pgvector (cosine > 0.7) + PostgreSQL FTS
- [ ] `internal/rag/reranker` — score and filter top-K results
- [ ] `internal/rag/context` — context budget manager (≤50% of model window)

#### MCP Client
- [ ] `internal/mcp/client` — spawn and communicate with external MCP servers
- [ ] Wire `nspady/google-calendar-mcp` as external process
- [ ] Wire `modelcontextprotocol/gdrive` as external process
- [ ] `internal/mcp/tools/telegram.go` — internal send-message tool (not external MCP)

#### Scheduler
- [ ] `internal/scheduler` — cron-like runner with idempotency keys
- [ ] Daily Google sync job
- [ ] Reminder job (fire-once, deduped on restart)
- [ ] Data lifecycle cleanup job (per retention policy)

### Definition of Done

> Ask "what's on my calendar tomorrow?" and get an accurate answer. Ask "find my notes on X" and get relevant Drive content. Reminders arrive on time.

---

## Phase 4: Cloud Deployment (Optional)

**Goal:** Move from macOS laptop to an always-on environment without changing the architecture.

### Target Environments (in order of preference)

1. **Home server / NAS** — same docker-compose, no code change
2. **VPS (Hetzner, etc.)** — cheap, private, always-on
3. **Cloud (AWS/GCP)** — private VPC, no public endpoints

### Deliverables

- [ ] `Dockerfile` for the Go binary
- [ ] `docker-compose.prod.yml` — production-tuned compose (resource limits, restart policies)
- [ ] Systemd service file (or equivalent) for auto-start
- [ ] Backup strategy for PostgreSQL (pg_dump to encrypted local/remote storage)
- [ ] Documented runbook: deploy, update, rollback, restore from backup
- [ ] Health check endpoint for uptime monitoring

### Definition of Done

> The assistant runs 24/7 without manual intervention. A machine restart recovers automatically. Backups are verified.

---

## Phase Summary

| Phase | Key Outcome | Status |
|---|---|---|
| 1 | Telegram ↔ LLM chat | ✅ Done |
| 2 | Observable, tested, resilient, secure | ✅ Done |
| 3 | Calendar + Drive awareness via RAG + MCP | Pending |
| 4 | Always-on deployment | Pending |
