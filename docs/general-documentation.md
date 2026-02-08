# Local-First Personal Assistant (Go + LLM + RAG + Telegram)

## 1. Project Overview

This project is a **local-first personal assistant** designed to run entirely under the user's control, prioritizing **privacy, data sovereignty, and auditability**.

Key characteristics:

* **No personal data is ever sent to hosted LLMs**
* **No public inbound endpoints**
* **All personal data stays local**
* **LLM is Ollama-only (local open-source models)**
* **Telegram is the primary interface (chat + notifications)**
* **Google Calendar and Google Drive are used as data sources**
* **RAG (Retrieval-Augmented Generation) is used instead of training**
* **Designed to start on macOS and later migrate to cloud infrastructure**

---

## 2. Core Principles

### 2.1 Privacy First

* No data used for model training
* No outbound LLM calls (Ollama runs locally with no internet access)
* Minimal data exposure even to local models
* Least-privilege OAuth scopes
* No raw personal data in logs

### 2.2 Local-First

* Runs fully on a local machine (macOS Apple Silicon M4 initially)
* No dependency on public endpoints
* All processing is local

### 2.3 Replaceable Intelligence

* LLM is an **implementation detail**, not a core dependency
* Local models via Ollama can be swapped (DeepSeek, LLaMA, Mistral, Qwen, etc.)
* Only models with native tool-calling support are used (e.g., llama3.1+, qwen2.5)

### 2.4 Deterministic Tools, Probabilistic Language

* LLMs are used **only** for:
  * Natural language understanding
  * Summarization
  * Formatting
* All **data access and actions** are deterministic and auditable

---

## 3. High-Level Architecture

```
┌────────────────────────────┐
│        Telegram App         │
└─────────────▲──────────────┘
              │ (outbound HTTPS, long polling)
┌─────────────┴──────────────┐
│   Go Assistant Service      │
│   (Modular Monolith)        │
│                             │
│  ┌─────────────────────┐    │
│  │  internal/telegram   │    │
│  │  internal/orchestr.. │    │
│  │  internal/rag        │    │
│  │  internal/scheduler  │    │
│  │  internal/mcp        │    │
│  │  internal/llm        │    │
│  │  internal/google     │    │
│  │  internal/store      │    │
│  └─────────────────────┘    │
└───────▲───────────▲────────┘
        │           │
┌───────┴──────┐ ┌──┴─────────────┐
│  PostgreSQL   │ │   Ollama       │
│  + pgvector   │ │ (local models) │
└──────────────┘ └────────────────┘

┌─────────────────────────────────┐
│  Observability (Docker Compose) │
│  Prometheus + Grafana           │
└─────────────────────────────────┘
```

### Key Properties

* **No inbound connections** — Telegram uses long polling
* **LLM has no internet access** — Ollama runs fully local
* **Modular monolith** — single binary with clean internal package boundaries
* **Database is the system of record**

---

## 4. Technology Stack

### Runtime

* **Language:** Go
* **OS:** macOS (Apple Silicon M4)
* **Process model:** Single binary (modular monolith)

### LLM

* **Runtime:** Ollama (local only)
* **Models:** Configurable, must support native tool calling (llama3.1+, qwen2.5, etc.)
* **Embeddings:** Local embedding model via Ollama

### Storage

* **Database:** PostgreSQL
* **Vector store:** pgvector
* **Search:** Hybrid (vector + full-text)

### External APIs

* **Telegram Bot API:** chat UI + notifications
* **Google APIs:** Calendar + Drive (read-only, daily sync)

### Observability

* **Logging:** `slog` (stdlib) with structured JSON output
* **Metrics:** Prometheus (via Docker Compose)
* **Dashboards:** Grafana (via Docker Compose)
* **Tracing:** OpenTelemetry (instrumentation library)

---

## 5. Trust & Security Model

### Trust Boundaries

| Component        | Trust Level           |
| ---------------- | --------------------- |
| Assistant binary | Fully trusted         |
| PostgreSQL       | Fully trusted         |
| Local LLM        | Fully trusted         |
| Telegram         | Untrusted transport   |
| Google APIs      | Untrusted data source |
| Internet         | Untrusted             |

### Security Guarantees

* No public endpoints
* No external LLM calls
* OAuth tokens encrypted at rest (key stored in OS keychain or env)
* Firewall-friendly design
* Explicit audit logs for tool usage
* No raw personal data in logs — use correlation IDs

---

## 6. Telegram Integration

### Why Telegram

* Reliable push notifications
* Simple bot interface
* Works well with long polling
* Acts as both UI and output channel

### Long Polling

* Assistant initiates all connections
* Telegram never calls back into the system
* No need for tunnels, webhooks, or public IPs
* Circuit breaker pattern for Telegram API failures
* Message persistence before processing (idempotency)

### Capabilities

* Chat with assistant
* Receive reminders
* Receive daily / weekly reports
* Future: interactive confirmations

---

## 7. LLM Integration (Ollama-Only)

### Goals

* Local-only guarantees — no data leaves the machine
* Model swappability within Ollama ecosystem

### Requirements

* Only models with **native tool-calling support** are supported
* No prompt-based function calling fallback needed
* Model capability verified at startup

### Core Interfaces

**Chat provider**

* Input: structured messages + tool definitions
* Output: text or structured tool calls

**Embedding provider**

* Input: text chunks
* Output: vectors

### Tool Abstraction

```go
type Tool struct {
    Name        string
    Description string
    Parameters  JSONSchema
    Handler     func(ctx context.Context, params map[string]any) (string, error)
}
```

All tool calls are deterministic and auditable. The LLM decides *which* tool to call, but the handler executes the action.

---

## 8. Data Model Overview

### Chat

* Sessions mapped to Telegram chat IDs
* Messages persisted for context
* Memory window is configurable

### Calendar

* Synced from multiple Google accounts (daily sync)
* Stored as structured records
* Optional embeddings for fuzzy queries

### Drive

* Restricted folder(s) as "assistant vault"
* Documents chunked and embedded
* Hybrid search (semantic + keyword)

### Jobs

* Reminders
* Scheduled reports
* Sync tasks (daily Google sync)

---

## 9. RAG Strategy

### Why RAG Instead of Training

* Immediate updates
* No data baked into weights
* Full auditability
* Lower risk

### Retrieval Flow

```
User Query
    │
    ▼
Intent Classification (calendar? drive? both?)
    │
    ▼
Structured Retrieval (SQL for exact/date queries)
    │
    ▼
Semantic Retrieval (pgvector similarity search)
    │
    ▼
Re-ranking (cosine similarity threshold > 0.7)
    │
    ▼
Context Budget Management (≤50% of model context window)
    │
    ▼
LLM formats response
```

### Implementation Details

* Chunk size: 512–1024 tokens
* Retrieval: top-K with cosine similarity cutoff
* Context budget: 40–50% of model context window
* Multi-hop retrieval: max 2 rounds
* Hybrid search: pgvector (semantic) + PostgreSQL FTS (keyword)

### Non-Goals

* No autonomous browsing
* No model fine-tuning on personal data

---

## 10. MCP (Model Context Protocol)

### Purpose

* Clean separation between assistant logic and external tools
* Explicit, inspectable, auditable tool calls

### Planned MCP Servers

* Telegram (send messages)
* Google Calendar
* Google Drive
* Scheduler / jobs

### Benefits

* Auditable
* Testable
* Easy to sandbox tools

---

## 11. Concurrency and Job Scheduling

### Go Concurrency Patterns Used

* **Channels** for inter-component communication
* **Context cancellation** for graceful shutdown
* **Mutexes** for shared state protection
* **sync.Once** for initialization guarantees

### Job Safety

* Idempotency keys for all scheduled jobs
* Deduplication on restart to prevent double-firing
* Graceful shutdown with in-flight job completion
* Failed job retry with exponential backoff

---

## 12. Data Lifecycle

### Retention Policies

| Data Type     | Retention        |
|---------------|-----------------|
| Chat messages | 90 days default  |
| Calendar events | 1 year          |
| Drive embeddings | Synced, pruned on delete |
| Job history   | 30 days          |
| Logs          | 7 days           |

### Storage Growth Mitigation

* Periodic cleanup jobs for expired data
* Archived old embeddings before deletion
* Log rotation

---

## 13. Deployment Strategy

### Phase 1: macOS Laptop

* Local Postgres
* Local Ollama
* Long polling Telegram
* Docker Compose for Prometheus + Grafana
* Manual startup

### Phase 2: Always-On Host

* Home mini-PC or NAS
* Same binary + same Docker Compose
* Same architecture

### Phase 3: Cloud (Optional)

* AWS or GCP in private VPC
* No public endpoints required

---

## 14. Non-Goals (Explicitly Out of Scope)

* Training models on personal data
* Auto-sending emails/messages without confirmation
* Cloud-only dependency
* Consumer SaaS multi-tenant support
* Voice interface (for now)
* External LLM providers (OpenAI, Anthropic, Google, etc.)

---

## 15. Future Extensions

* Read/write calendar events
* Financial data ingestion
* Local web UI
* Multi-modal (images, PDFs)
* Multiple assistants/profiles
* Optional encrypted backups

---

## 16. Guiding Rule

> **If a model ever misbehaves, the worst it should do is generate a bad sentence — never access data it shouldn't or perform an irreversible action.**

All actions that modify data or send messages require explicit confirmation or use deterministic, auditable code paths separate from LLM output.
