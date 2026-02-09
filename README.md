# Mimir

<p align="center">
  <em>Your personal Mimir — knowledge, wisdom, and conversation. All local, all private.</em>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.24.3-00ADD8?logo=go" alt="Go Version">
  <img src="https://img.shields.io/badge/PostgreSQL-17-336791?logo=postgresql" alt="PostgreSQL">
  <img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="MIT License">
</p>

---

## Overview

Mimir is a **privacy-first conversational AI** that runs entirely on your machine. Chat with it via Telegram, powered by local LLMs (Ollama), with full conversation history, observability, and zero external data sharing.

**Key Features:**
- 🔒 **100% Local** — No data sent to external LLM providers
- 💬 **Telegram Interface** — Natural chat experience with long polling (no webhooks)
- 🧠 **Ollama-Powered** — Swap models freely (llama3.1, qwen2.5, DeepSeek, etc.)
- 📊 **Full Observability** — Prometheus metrics, Grafana dashboards, structured logs via Loki
- 🛡️ **User Allowlist** — Only authorized Telegram users can interact
- 🔄 **Conversation Memory** — PostgreSQL persistence with configurable history window
- ⚡ **Circuit Breakers** — Graceful degradation on Telegram or Ollama failures

---

## Architecture

```
┌─────────────────┐
│  Telegram App   │ (You)
└────────┬────────┘
         │ Long Polling (HTTPS)
         ↓
┌────────────────────────────────┐
│   Mimir (Go)                   │
│  ┌──────────────────────────┐  │
│  │ Telegram Poller          │  │
│  │ Orchestrator             │  │
│  │ LLM Client (Ollama)      │  │
│  │ Observability            │  │
│  └──────────────────────────┘  │
└───┬────────────────────────┬───┘
    │                        │
    ↓                        ↓
┌─────────────┐      ┌──────────────┐
│ PostgreSQL  │      │    Ollama    │
│ + pgvector  │      │  (Local LLM) │
└─────────────┘      └──────────────┘

┌──────────────────────────────────┐
│  Observability Stack (Docker)    │
│  Prometheus │ Grafana │ Loki     │
└──────────────────────────────────┘
```

**Core Principles:**
- No inbound network connections (polling only)
- LLM has no internet access
- All tool calls are deterministic and auditable
- Circuit breakers prevent cascading failures

---

## Prerequisites

| Requirement | Version | Purpose |
|-------------|---------|---------|
| [Go](https://go.dev/dl/) | 1.24.3+ | Build and run the assistant |
| [Docker](https://www.docker.com/get-started) | Latest | PostgreSQL + observability stack |
| [Ollama](https://ollama.com/) | Latest | Local LLM runtime |
| Telegram Bot Token | — | Create via [@BotFather](https://t.me/BotFather) |
| Your Telegram User ID | — | Get from [@userinfobot](https://t.me/userinfobot) |

**Supported Platforms:** macOS (Apple Silicon tested), Linux

---

## Quick Start

### 1. Clone and Configure

```bash
git clone https://github.com/fandangolas/mimir.git
cd mimir

cp .env.example .env
# Edit .env with your Telegram bot token and user ID
```

### 2. Pull an Ollama Model

```bash
ollama pull llama3.1
# Or: ollama pull qwen2.5
```

### 3. Start Infrastructure

```bash
make up
# Starts: PostgreSQL, Prometheus, Grafana, Loki, Promtail
```

### 4. Run the Assistant

```bash
make run
```

### 5. Chat on Telegram

Open your bot on Telegram, send `/start`, and start chatting!

---

## RAG: Long-Term Memory (Optional)

By default, Mimir remembers only the last 20 messages. **RAG (Retrieval-Augmented Generation)** gives Mimir true long-term memory, allowing it to remember conversations from weeks, months, or even years ago.

### How RAG Works

RAG uses **embeddings** (numerical representations of text) to search your entire conversation history semantically:

```
You (3 months ago): "I'm allergic to peanuts"
You (today): "Recommend a restaurant"
Mimir (with RAG): "I'll suggest places with allergy-friendly menus since you have a peanut allergy" ✅
```

Without RAG, Mimir would have forgotten the allergy mention.

### Enabling RAG

**Step 1: Pull the embedding model**

```bash
ollama pull nomic-embed-text
```

**Step 2: Backfill existing messages** (if you have chat history)

```bash
make build
./bin/backfill-embeddings
```

This generates embeddings for all existing messages. Progress is logged:
```
INFO starting backfill total_messages=1500
INFO progress processed=100 total=1500 remaining=1400 percent=6.7%
...
INFO backfill summary total_messages=1500 processed=1500 errors=0 success_rate=100.0%
```

**Step 3: Enable RAG in .env**

```bash
RAG_ENABLED=true
```

**Step 4: Restart Mimir**

```bash
make run
```

New messages will automatically get embeddings generated in the background.

### What RAG Gives You

- 🧠 **Semantic search:** Finds conceptually similar conversations ("anxious" matches "worried", "stressed")
- 📅 **Long-term memory:** Remembers facts from months/years ago
- 🎯 **Hybrid search:** Combines semantic similarity + keyword matching + recency
- ⚡ **Fast:** Sub-second search even with 10,000+ messages
- 💾 **Efficient:** ~200 MB storage for 5 years of conversations

### RAG Architecture

See [docs/rag-architecture.md](docs/rag-architecture.md) for:
- Complete technical architecture
- Glossary explaining embeddings, vectors, hybrid search
- Performance tuning guide
- Storage and performance estimates

---

## Configuration

All configuration is via environment variables (loaded from `.env` or system environment):

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `TELEGRAM_BOT_TOKEN` | ✅ | — | Telegram bot token from @BotFather |
| `ALLOWED_USER_IDS` | ✅ | — | Comma-separated Telegram user IDs (e.g., `123456789,987654321`) |
| `DATABASE_URL` | ✅ | — | PostgreSQL connection string (default in .env.example works with Docker Compose) |
| `OLLAMA_BASE_URL` | ❌ | `http://localhost:11434` | Ollama API URL |
| `OLLAMA_MODEL` | ❌ | `llama3.1` | Model name (must support tool calling) |
| `LOG_LEVEL` | ❌ | `info` | Logging level: `debug`, `info`, `warn`, `error` |
| `LOG_DIR` | ❌ | `logs` | Directory for daily log files (tailed by Promtail) |
| `CONVERSATION_WINDOW` | ❌ | `20` | Number of recent messages sent to LLM |
| `METRICS_ADDR` | ❌ | `:9090` | Prometheus metrics endpoint |

### RAG (Long-Term Memory) Configuration

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `RAG_ENABLED` | ❌ | `false` | Enable RAG for long-term conversational memory |
| `EMBEDDING_MODEL` | ❌ | `nomic-embed-text` | Ollama embedding model (768 dimensions) |
| `RAG_RETRIEVED_COUNT` | ❌ | `5` | Number of messages to retrieve from history |
| `RAG_RECENT_COUNT` | ❌ | `10` | Number of recent messages to include |
| `RAG_MIN_SIMILARITY` | ❌ | `0.5` | Minimum cosine similarity threshold (0-1) |
| `RAG_WEIGHT_SEMANTIC` | ❌ | `0.5` | Semantic similarity weight in hybrid search |
| `RAG_WEIGHT_KEYWORD` | ❌ | `0.3` | Keyword matching weight in hybrid search |
| `RAG_WEIGHT_RECENCY` | ❌ | `0.2` | Recency boost weight in hybrid search |
| `RAG_RECENCY_DECAY_DAYS` | ❌ | `30` | Days for recency score decay |

**Note:** Weights must sum to 1.0. See [RAG Architecture](docs/rag-architecture.md) for details.

---

## Observability

### Grafana Dashboards

Open **[http://localhost:3000](http://localhost:3000)** (login: `admin` / `admin`)

**Pre-provisioned dashboard:** `Mimir`

**Metrics tracked:**
- Messages received, processed (success/error), unauthorized attempts
- LLM latency (p50/p95/p99 + heatmap)
- Telegram send errors
- Message processing rate over time

**RAG metrics (if enabled):**
- RAG searches performed (success/error/fallback)
- Hybrid search latency
- Number of messages retrieved per query
- Context token usage
- Embedding generation latency and errors

### Logs

Query structured logs in **Grafana → Explore → Loki**:

```logql
{job="mimir"}
```

Filter by correlation ID:
```logql
{job="mimir"} |= "123456-789"
```

Logs include:
- `correlation_id` — unique per message (`chat_id-message_id`)
- `chat_id` — Telegram chat identifier
- Structured fields for errors, latency, etc.

### Metrics Endpoint

Raw Prometheus metrics: **[http://localhost:9090/metrics](http://localhost:9090/metrics)**

---

## Development

### Build

```bash
make build
# Binary output: ./bin/assistant
```

### Run Tests

```bash
make test
```

### Run Linter

```bash
make lint
# Requires golangci-lint installed
```

### Project Structure

```
mimir/
├── cmd/assistant/          # Main entry point
├── internal/
│   ├── config/             # Configuration loading
│   ├── telegram/           # Telegram bot client + poller
│   ├── orchestrator/       # Message flow coordinator
│   ├── llm/ollama/         # Ollama LLM client
│   ├── store/              # PostgreSQL access + migrations
│   └── observability/      # Logging, metrics, tracing
├── observability/          # Prometheus, Grafana, Loki configs
└── docs/                   # Architecture documentation
```

---

## Security

### User Allowlist

Only Telegram users listed in `ALLOWED_USER_IDS` can send messages. Unauthorized attempts are:
- Silently dropped
- Logged as warnings
- Tracked in Prometheus (`assistant_unauthorized_messages_total`)

### Circuit Breakers

- **Ollama:** Opens after 3 consecutive failures (30s cooldown)
- **Telegram:** Opens after 5 consecutive send failures (10s cooldown)

### Data Privacy

- No data sent to external LLM providers
- OAuth tokens (future: Google Calendar/Drive) encrypted at rest
- No personal data in logs (only correlation IDs)

---

## Why "Mimir"?

In Norse mythology, [Mimir](https://en.wikipedia.org/wiki/M%C3%ADmir) was renowned for his knowledge and wisdom. He advised Odin, the Allfather, serving as a counselor and source of insight.

In *God of War* (2018), Mimir is portrayed as Kratos's companion — a disembodied head carried everywhere, offering guidance, lore, and witty commentary. This project channels that spirit: a personal, ever-present advisor who's always ready with an answer.

Unlike cloud AI assistants, Mimir stays with you, runs locally, and never shares your conversations with anyone. With RAG-powered long-term memory, Mimir truly remembers your conversations—making it a genuine personal assistant that grows wiser over time.

---

## Roadmap

**Phase 1 & 2: ✅ Complete**
- Telegram ↔ LLM conversation
- Full observability stack
- User access control

**Phase 2.5: ✅ Complete**
- RAG (Retrieval-Augmented Generation) for long-term memory
- Hybrid search (semantic + keyword + recency)
- Embedding generation and backfill tools
- pgvector integration with HNSW indexes

**Phase 3: Pending**
- Google Calendar integration via MCP
- Google Drive RAG (document search)
- Scheduled reminders

**Phase 4: Pending**
- Cloud deployment (VPS/AWS)
- Automated backups

See [docs/development-phases.md](docs/development-phases.md) and [docs/rag-architecture.md](docs/rag-architecture.md) for details.

---

## Troubleshooting

### "DATABASE_URL is required"
Ensure `.env` file exists and contains `DATABASE_URL`. Run `cp .env.example .env` if missing.

### "model does not support native tool calling"
Switch to a supported model: `llama3.1`, `llama3.2`, `qwen2.5`, etc. Update `OLLAMA_MODEL` in `.env`.

### Bot doesn't respond on Telegram
1. Check `make run` logs for errors
2. Verify your Telegram user ID is in `ALLOWED_USER_IDS`
3. Confirm Ollama is running: `ollama list`

### Grafana shows no data
1. Wait 30s for first scrape
2. Verify Prometheus is scraping: [http://localhost:9091/targets](http://localhost:9091/targets)
3. Check app is exposing metrics: `curl localhost:9090/metrics`

---

## Contributing

This is a personal project, but suggestions are welcome via GitHub Issues.

### Development Setup

1. Fork and clone
2. Run `make up` to start dependencies
3. Run `make test` before committing
4. Follow conventional commit format: `feat:`, `fix:`, `docs:`, etc.

---

## License

This project is licensed under the MIT License.  
You are free to use, modify, and distribute this software in accordance with the terms of the license.

---

## Acknowledgments

Built with:
- [Ollama](https://ollama.com/) — Local LLM runtime
- [Telegram Bot API](https://core.telegram.org/bots/api) — Chat interface
- [Prometheus](https://prometheus.io/) + [Grafana](https://grafana.com/) — Observability
- [Loki](https://grafana.com/oss/loki/) — Log aggregation
- [pgvector](https://github.com/pgvector/pgvector) — PostgreSQL vector extension (future RAG)

Inspired by the local-first and privacy-first software movement, and by Mimir's wisdom in Norse mythology.

