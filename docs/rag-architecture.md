# RAG Architecture for Chat History Memory

## Table of Contents

1. [Glossary & Core Concepts](#glossary--core-concepts)
2. [Overview](#overview)
3. [Architecture](#architecture)
4. [Technology Stack](#technology-stack)
5. [Database Schema](#database-schema)
6. [Hybrid Search Strategy](#hybrid-search-strategy)
7. [Re-Ranking with BM25](#re-ranking-with-bm25)
8. [Context Management](#context-management)
9. [Message Chunking](#message-chunking)
10. [Storage Requirements](#storage-requirements)
11. [Performance Targets](#performance-targets)
12. [Error Handling and Resilience](#error-handling-and-resilience)
13. [Backfilling Strategy](#backfilling-strategy)
14. [Monitoring and Observability](#monitoring-and-observability)
15. [Configuration](#configuration)
16. [Migration Path](#migration-path)
17. [Testing Strategy](#testing-strategy)
18. [Future Enhancements](#future-enhancements)

---

## Glossary & Core Concepts

### What is RAG?

**RAG (Retrieval-Augmented Generation)** is a technique that enhances AI responses by retrieving relevant information from a knowledge base before generating an answer.

**Simple analogy:** Instead of trying to remember everything (like memorizing a textbook), RAG is like having an open-book exam where you can look up relevant pages before answering.

**How it works:**
1. User asks a question
2. System searches your past conversations for relevant information
3. Retrieved information is added to the prompt sent to the LLM
4. LLM generates a response using both the current query and retrieved context

**Example:**
```
Without RAG:
You (3 months ago): "My birthday is May 15th"
You (today): "When is my birthday?"
Mimir: "I don't have that information" ❌

With RAG:
You (today): "When is my birthday?"
→ RAG retrieves: "My birthday is May 15th" (from 3 months ago)
Mimir: "Your birthday is May 15th" ✅
```

---

### Embeddings

**What they are:** Numerical representations of text that capture semantic meaning.

**Simple explanation:** Embeddings convert text into arrays of numbers (vectors) that computers can compare mathematically. Similar meanings produce similar numbers.

**Example:**
```
Text: "I'm feeling anxious"
Embedding: [0.23, -0.45, 0.67, ..., 0.12] (768 numbers)

Text: "I'm feeling worried"
Embedding: [0.25, -0.43, 0.65, ..., 0.10] (768 numbers)
                ↑ Very similar numbers = similar meanings

Text: "The weather is nice"
Embedding: [-0.82, 0.31, -0.15, ..., 0.93] (768 numbers)
                ↑ Very different numbers = different meanings
```

**Key properties:**
- **Dimension:** Number of values in the array (we use 768)
- **Semantic similarity:** Similar concepts have similar embeddings
- **Language-agnostic:** Can compare across different phrasings

**Why we need them:**
- Traditional search only matches exact words ("anxious" ≠ "worried")
- Embedding search finds conceptually similar content ("anxious" ≈ "worried" ≈ "stressed")

---

### Vector Database (pgvector)

**What it is:** A database that can store and efficiently search through embeddings.

**Why regular databases aren't enough:**
- Regular search: "Find messages containing the word 'anxious'" (exact match)
- Vector search: "Find messages similar to this feeling of anxiety" (semantic match)

**How pgvector works:**
1. Stores embeddings as a new data type: `VECTOR(768)`
2. Creates special indexes to quickly find similar vectors
3. Uses distance metrics to measure similarity

**Analogy:** Think of it like a library catalog system that understands concepts, not just book titles.

---

### Distance Functions

**Purpose:** Measure how similar two embeddings are.

#### Cosine Distance (What We Use)

**Formula:** Measures the angle between two vectors, ignoring their magnitude.

```
Cosine Similarity = (A · B) / (||A|| × ||B||)
Cosine Distance = 1 - Cosine Similarity

Range: 0 (identical) to 2 (opposite)
```

**Visual explanation:**
```
         Vector A
           ↗
          /  small angle = similar
         /
        /___________→ Vector B

         Vector A
           ↗
          /
         /  large angle = different
        /
       ↓
    Vector C
```

**Why cosine?**
- Focuses on direction (meaning) rather than magnitude
- Standard for text embeddings
- Range [0,1] after normalization (easy to interpret)

**Example:**
```
"I love pizza" vs "I adore pizza" → Distance: 0.05 (very similar)
"I love pizza" vs "Pizza is good" → Distance: 0.15 (similar)
"I love pizza" vs "I hate pizza" → Distance: 0.85 (opposite)
```

#### Other Distance Functions (Not Used)

**L2 Distance (Euclidean):** Physical distance between points
- Good for: Image embeddings, spatial data
- Not ideal for: Text embeddings (magnitude matters)

**Inner Product:** Dot product of vectors
- Good for: Pre-normalized embeddings
- Slight speed boost vs cosine

---

### Vector Indexes

#### HNSW (Hierarchical Navigable Small World)

**What it is:** A graph-based index that enables fast approximate nearest neighbor search.

**How it works (simplified):**
1. Creates a multi-layer graph of vectors
2. Upper layers have long-distance "highways" between points
3. Lower layers have short-distance "local roads"
4. Search starts at the top and navigates down

**Analogy:** Like using Google Maps navigation:
- First, take the highway to the right city (upper layers)
- Then, take local roads to the neighborhood (middle layers)
- Finally, walk to the exact address (bottom layer)

**Parameters:**
- **m (connections per node):** How many neighbors each vector connects to
  - Higher = more accurate but larger index
  - Default: 16 (good balance)

- **ef_construction (build quality):** How carefully to build the index
  - Higher = better quality but slower build
  - Default: 64 (good balance)

- **ef_search (query quality):** How carefully to search at query time
  - Higher = more accurate but slower queries
  - Default: 40 (can be tuned per query)

**Trade-offs:**
| Aspect | HNSW | IVFFlat (Alternative) |
|--------|------|----------------------|
| Query Speed | ⚡⚡⚡ Very fast (40 QPS) | 🐌 Slow (2.6 QPS) |
| Build Time | 🐌 Slow (4000s) | ⚡ Fast (128s) |
| Memory | 📊 High (729 MB) | 📊 Low (257 MB) |
| Accuracy | ✅ Excellent | ✅ Good |
| Updates | ✅ Incremental | ❌ Rebuild needed |

**Why we chose HNSW:**
- We build the index once, but query it thousands of times
- Speed matters more than build time for production
- Incremental updates (no rebuild when adding new messages)

---

### Hybrid Search

**What it is:** Combining multiple search methods to get the best of each.

**The three search methods:**

#### 1. Semantic Search (Vector Similarity)
**Finds:** Conceptually similar content

**Example:**
```
Query: "I'm feeling stressed about work"
Finds:
  ✅ "Work has been overwhelming lately"
  ✅ "I'm anxious about the project deadline"
  ✅ "Too much pressure from my boss"
  ❌ "I love my new desk setup" (mentions "work" but different meaning)
```

**Strength:** Understands meaning, handles synonyms
**Weakness:** May miss exact keyword matches

#### 2. Keyword Search (Full-Text Search)
**Finds:** Exact word/phrase matches

**Example:**
```
Query: "project proposal meeting"
Finds:
  ✅ "The project proposal meeting is on Thursday"
  ✅ "I reviewed the proposal for the new project"
  ❌ "I'm anxious about the upcoming presentation" (semantically related but no exact keywords)
```

**Strength:** Finds exact terms (names, technical jargon)
**Weakness:** Misses paraphrases and synonyms

#### 3. Recency Boost
**Finds:** Recent messages are weighted higher

**Why:** Recent context is often more relevant than old context

**Example:**
```
Query: "What's my work schedule?"

Option A (6 months ago): "I work 9-5 Monday-Friday"
Option B (yesterday): "My schedule changed to 10-6"

Result: Option B ranked higher (recency boost)
```

**Formula:**
```
recency_score = exp(-days_ago / decay_constant)
```

**Combining all three:**
```
Final Score = 0.5 × semantic_score
            + 0.3 × keyword_score
            + 0.2 × recency_score
```

---

### Reciprocal Rank Fusion (RRF)

**What it is:** A method to combine rankings from different search methods.

**The problem it solves:**
- Semantic search ranks messages: [A, B, C, D, E]
- Keyword search ranks messages: [C, E, A, F, G]
- How do we combine these into one ranked list?

**RRF Formula:**
```
RRF_score(message) = Σ 1 / (k + rank_in_method_i)

where k = 60 (constant)
```

**Example:**
```
Message C appears in both rankings:
- Rank 3 in semantic search → score = 1/(60+3) = 0.0159
- Rank 1 in keyword search → score = 1/(60+1) = 0.0164
- Combined RRF score = 0.0159 + 0.0164 = 0.0323

Message A appears in both rankings:
- Rank 1 in semantic search → score = 1/(60+1) = 0.0164
- Rank 3 in keyword search → score = 1/(60+3) = 0.0159
- Combined RRF score = 0.0164 + 0.0159 = 0.0323

Messages with similar combined scores!
```

**Why RRF instead of averaging scores?**
- Works without normalizing scores (different search methods have different scales)
- Robust to outliers
- Well-researched and proven effective

---

### BM25 Re-Ranking

**What it is:** An algorithm that ranks documents by keyword relevance.

**Full name:** Best Matching 25 (25th iteration of the algorithm)

**What it does better than simple keyword matching:**
1. **Term Frequency (TF):** More mentions = more relevant, but with diminishing returns
2. **Inverse Document Frequency (IDF):** Rare words matter more than common words
3. **Document Length Normalization:** Shorter documents aren't penalized

**Simple example:**

```
Query: "birthday cake recipe"

Document A (short): "This birthday cake recipe is amazing!" (mentions "birthday cake recipe" once in 6 words)
Document B (long): "...baking tips...birthday...cake ingredients...recipe steps..." (mentions terms scattered in 500 words)

Simple keyword match: Both mention all terms → tie
BM25: Document A scores higher (terms are concentrated, not just mentioned)
```

**Parameters:**
- **k1 = 1.2:** Controls term frequency saturation (standard value)
- **b = 0.75:** Controls length normalization (standard value)

**When we use it:** After hybrid search retrieves 15-20 candidates, BM25 re-ranks them to pick the best 5.

---

### Tokens and Context Windows

**What is a token?**
A token is a piece of text that the LLM processes. Roughly:
- 1 token ≈ 4 characters for English
- 1 token ≈ ¾ of a word on average

**Examples:**
```
"Hello" → 1 token
"Hello, world!" → 3 tokens
"I'm feeling great today!" → 5 tokens
```

**What is a context window?**
The maximum amount of text an LLM can process at once.

**Example:**
```
Model: llama3.1 with 8K context window
= Can process ~8,000 tokens at once
= Roughly 6,000 words
= About 12 pages of text
```

**Why it matters for RAG:**
We need to fit:
- System prompt (200 tokens)
- Retrieved messages from RAG (2,720 tokens)
- Recent conversation (2,720 tokens)
- Current query (1,360 tokens)
- Space for response (1,000 tokens)
- **Total: ~8,000 tokens** (fits in 8K window)

**What happens if we exceed the limit?**
- LLM truncates (cuts off) the beginning
- We lose context
- Responses become incoherent

**Our solution:** Carefully manage token budget (allocate percentages to each section).

---

### Asynchronous vs Synchronous Processing

**Synchronous (Blocking):**
```
User sends message
  ↓
Store message in database
  ↓
Generate embedding (wait 500ms) ⏳
  ↓
Send to LLM
  ↓
Return response to user

Total time: ~2 seconds
```

**Asynchronous (Non-Blocking):**
```
User sends message
  ↓
Store message in database
  ↓
Send to LLM immediately
  ↓
Return response to user (1.5s) ✅
  ↓
(Meanwhile, in background)
  ↓
Generate embedding (500ms) 🔄
  ↓
Store embedding

Total user-facing time: 1.5 seconds (500ms faster!)
```

**Why it matters:**
- User doesn't wait for embedding generation
- System is more responsive
- If embedding fails, user still gets their response

**Trade-off:** New message isn't immediately searchable (takes ~1 second to appear in search results).

---

### Graceful Degradation

**What it is:** System continues working at reduced capacity when components fail.

**Example cascade:**
```
Scenario: Ollama embedding service crashes

Without graceful degradation:
  User sends message → Try to generate embedding → FAIL → Return error to user ❌

With graceful degradation:
  User sends message
    → Try to generate embedding → FAIL
    → Fall back to keyword-only search ✅
    → Return response to user (slightly worse quality, but works!)
    → Queue embedding for retry later 🔄
```

**Fallback hierarchy for Mimir:**
1. **Full RAG:** Semantic + keyword + recency (best quality)
2. **Keyword-only:** Just full-text search (good quality)
3. **Recent messages only:** No retrieval, just last 10 messages (basic quality)
4. **Error message:** Only if all else fails

**Philosophy:** Degrade gracefully rather than fail completely.

---

### Backfilling

**What it is:** Processing historical data that existed before a feature was added.

**Our scenario:**
- Mimir has 10,000 existing messages in the database
- We add RAG feature (requires embeddings)
- Need to generate embeddings for all 10,000 old messages

**Backfill process:**
```
1. Find messages without embeddings: SELECT messages WHERE no embedding exists
2. Process in batches (100 at a time)
3. Generate embedding for each message
4. Store embedding in database
5. Repeat until all messages processed
```

**Why batches?**
- Prevents memory overload
- Allows progress tracking
- Can resume if interrupted
- Doesn't overwhelm Ollama

**Estimated time:**
- 10,000 messages × 500ms per embedding = 5,000 seconds
- With 3 parallel workers = ~30 minutes
- With batching overhead = ~45 minutes total

---

## Overview

This document describes the Retrieval-Augmented Generation (RAG) system that gives Mimir long-term conversational memory. Instead of relying on a limited sliding window, RAG enables Mimir to remember and retrieve relevant information from weeks, months, or years of past conversations.

**Key Principle:** RAG provides true long-term memory by retrieving actual past conversations, not lossy summaries.

**Now that you understand the concepts above, the rest of this document explains how we implement them in Mimir.**

---

## Architecture

### High-Level Flow

```
User Query
    ↓
1. Generate Query Embedding (Ollama: nomic-embed-text)
    ↓
2. Hybrid Search
    ├─→ Semantic Search (pgvector cosine similarity)
    ├─→ Keyword Search (PostgreSQL full-text search)
    └─→ Recency Boost (exponential decay)
    ↓
3. Reciprocal Rank Fusion (RRF)
    ↓
4. Re-ranking (BM25)
    ↓
5. Deduplication (remove messages already in recent context)
    ↓
6. Context Assembly
    ├─→ System Prompt
    ├─→ Retrieved Messages (3-5 from RAG)
    ├─→ Recent Messages (5-10 from current session)
    └─→ Current Query
    ↓
7. LLM Generation (Ollama)
    ↓
8. Store Response + Generate Embedding (async)
```

---

## Technology Stack

### Embedding Model: **nomic-embed-text**

**Selected because:**
- **Dimensions:** 768 (optimal balance of accuracy and performance)
- **Context Length:** 8192 tokens (handles long messages)
- **Performance:** Fast inference on CPU/GPU
- **Quality:** Strong performance on conversational text
- **Local:** Runs entirely via Ollama, no external API
- **License:** Apache 2.0 (MIT-compatible)

**Installation:**
```bash
ollama pull nomic-embed-text
```

**Alternatives considered:**
- `all-MiniLM-L6-v2`: Smaller (384 dims) but lower quality
- `mxbai-embed-large`: Higher quality but slower

### Vector Database: **pgvector**

Already installed in PostgreSQL container. Provides:
- **Vector storage:** Native `vector(768)` data type
- **Index types:** HNSW (production) and IVFFlat (development)
- **Distance functions:** Cosine, L2, inner product

### Search Index: **HNSW** (Hierarchical Navigable Small World)

**Selected because:**
- **Speed:** 15x faster queries than IVFFlat (40.5 QPS vs 2.6 QPS)
- **Accuracy:** Comparable or better than IVFFlat
- **Updates:** Supports incremental updates (no rebuild needed)
- **Production-ready:** Battle-tested in large-scale systems

**Trade-offs:**
- Slower index build (32x slower than IVFFlat)
- Higher memory usage (2.8x more)
- Worth it: We build once, query thousands of times

**Parameters:**
```sql
CREATE INDEX message_embeddings_hnsw_idx ON message_embeddings
USING hnsw (embedding vector_cosine_ops)
WITH (
    m = 16,              -- Links per node (higher = more accurate but larger index)
    ef_construction = 64 -- Build quality (higher = better quality, slower build)
);

-- Query-time tuning
SET hnsw.ef_search = 40;  -- Search quality (higher = more accurate, slower)
```

---

## Database Schema

### New Table: `message_embeddings`

```sql
CREATE TABLE message_embeddings (
    id BIGSERIAL PRIMARY KEY,
    message_id BIGINT NOT NULL UNIQUE REFERENCES messages(id) ON DELETE CASCADE,
    embedding VECTOR(768) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX message_embeddings_hnsw_idx ON message_embeddings
USING hnsw (embedding vector_cosine_ops)
WITH (m = 16, ef_construction = 64);
```

### Updated Table: `messages`

```sql
-- Add full-text search support
ALTER TABLE messages
ADD COLUMN content_tsv TSVECTOR
GENERATED ALWAYS AS (to_tsvector('english', content)) STORED;

CREATE INDEX messages_fts_idx ON messages USING GIN(content_tsv);
```

---

## Hybrid Search Strategy

### Why Hybrid?

Different search methods excel at different tasks:
- **Semantic:** Finds conceptually similar conversations ("anxious" → "worried", "stressed")
- **Keyword:** Finds exact matches ("project proposal" → "project proposal")
- **Recency:** Prefers recent context over old context

**Combining all three yields the best results.**

### Weighted Scoring

Default weights (tunable via config):
- **Semantic:** 50% - Primary signal for conceptual similarity
- **Keyword:** 30% - Catches exact term matches
- **Recency:** 20% - Recent context is often more relevant

### Reciprocal Rank Fusion (RRF)

Combines rankings from different search methods:

```
RRF_score = 1 / (k + rank_semantic) + 1 / (k + rank_keyword)

where k = 60 (constant that balances importance)
```

**Why RRF?**
- Works without score normalization
- Robust to outliers
- Simple and effective

---

## Re-Ranking with BM25

After hybrid search retrieves top 15-20 candidates, BM25 re-ranks them for final selection.

**BM25 Formula:**
```
score(D, Q) = Σ IDF(qi) * (f(qi, D) * (k1 + 1)) / (f(qi, D) + k1 * (1 - b + b * |D| / avgdl))

where:
- IDF(qi) = inverse document frequency of query term
- f(qi, D) = term frequency in document
- k1 = 1.2 (term saturation parameter)
- b = 0.75 (length normalization)
```

**Purpose:** BM25 is excellent at ranking documents by keyword relevance, improving final result quality.

---

## Context Management

### Token Budget Allocation

For a model with 8K context window:
- **System Prompt:** 200 tokens (2.5%)
- **Response Buffer:** 1000 tokens (12.5%)
- **Available for Context:** 6800 tokens (85%)

**Context breakdown:**
- **Current Query:** 20% → 1360 tokens
- **Recent Messages:** 40% → 2720 tokens (5-10 messages)
- **Retrieved Messages:** 40% → 2720 tokens (3-5 messages)

### Deduplication

Retrieved messages may overlap with recent messages. We deduplicate by message ID to avoid redundancy.

**Example:**
```
Recent messages: [101, 102, 103, 104, 105]
Retrieved messages: [50, 75, 104, 120]
After dedup: [50, 75, 120]  ← Message 104 removed
```

### Context Assembly Format

```
You are Mimir, a helpful AI assistant. Use conversation history to provide contextually relevant responses.

## Relevant conversation history:

[Jan 15 10:30] user: I'm allergic to peanuts
[Jan 15 10:31] assistant: I've noted that you're allergic to peanuts. I'll remember this.
[Feb 02 14:20] user: What restaurants do you recommend?
[Feb 02 14:21] assistant: I'll suggest places with allergy-friendly menus since you have a peanut allergy.

## Recent conversation:

user: I'm planning dinner tonight
assistant: Great! What kind of cuisine are you interested in?
user: Thai food

User: Do you remember any dietary restrictions I have?
Assistant:
```

---

## Message Chunking

Most chat messages are short (<500 tokens) and don't need chunking. For rare long messages, we use **recursive character splitting**.

### Chunking Parameters

- **Max Chunk Size:** 512 tokens (~2048 characters)
- **Overlap:** 50 tokens (~200 characters)
- **Separators (priority order):**
  1. `\n\n` (paragraphs)
  2. `\n` (lines)
  3. `. ` (sentences)
  4. `! ` (exclamations)
  5. `? ` (questions)
  6. `; ` (semicolons)
  7. `, ` (commas)
  8. ` ` (spaces)

**Why overlap?** Prevents losing context at chunk boundaries.

---

## Storage Requirements

### Per-Message Storage

**Without embeddings:**
- Message data: ~234 bytes
- Index overhead: ~50 bytes
- **Total: 284 bytes**

**With embeddings:**
- Message data: 234 bytes
- Embedding (768 floats): 3072 bytes
- Index overhead: ~500 bytes
- **Total: 3806 bytes**

**Multiplier: ~13x**

### Projected Storage Growth

| Time Period | Messages | Without Embeddings | With Embeddings | Total Storage |
|-------------|----------|-------------------|-----------------|---------------|
| 1 month | 1,500 | 426 KB | 5.4 MB | 5.8 MB |
| 1 year | 18,000 | 5.1 MB | 65 MB | 70 MB |
| 5 years | 90,000 | 25.6 MB | 325 MB | 351 MB |
| 10 years | 180,000 | 51.1 MB | 651 MB | 702 MB |

**Conclusion:** Storage is negligible. Even 10 years of daily use = <1 GB.

---

## Performance Targets

### Embedding Generation
- **Target:** <500ms per message
- **Batch Processing:** 3-5 parallel workers
- **Async:** Non-blocking for user requests

### Hybrid Search
- **Target:** <200ms for top-5 results
- **HNSW Performance:** 40+ queries per second
- **Full-Text Search:** <50ms with GIN index

### End-to-End RAG Latency
- **Target:** <1 second total
  - Query embedding: 200ms
  - Hybrid search: 200ms
  - Re-ranking: 50ms
  - Context assembly: 50ms
  - LLM generation: 500ms (depends on model/hardware)

---

## Error Handling and Resilience

### Graceful Degradation

**If embedding service fails:**
1. Retry with exponential backoff (3 attempts)
2. Fall back to keyword-only search
3. Queue embedding generation for later
4. Log error and emit metrics

**If search fails:**
1. Fall back to recent messages only (no RAG)
2. Log error and alert
3. User still gets a response

**If context exceeds budget:**
1. Truncate retrieved messages first
2. Preserve recent messages (critical for coherence)
3. Log warning

### Asynchronous Embedding Generation

**New messages:**
1. Store message in database immediately
2. Queue embedding generation (async)
3. Return response to user without waiting
4. Embedding becomes searchable within seconds

**Benefits:**
- No latency impact on user
- Resilient to temporary Ollama failures
- Automatic retry on failure

---

## Backfilling Strategy

### For Existing Messages

When deploying RAG, existing messages have no embeddings. We backfill them using a batch process.

**Strategy:**
1. **Batch size:** 100 messages per batch
2. **Concurrency:** 3 parallel workers
3. **Rate limiting:** 100ms between batches (avoid overloading Ollama)
4. **Resumability:** Skip messages that already have embeddings
5. **Progress tracking:** Log every 1000 messages

**Estimated time:**
- 10,000 messages: ~30 minutes
- 50,000 messages: ~2.5 hours

**Index creation:**
- Build HNSW index **after** backfilling (much faster)
- Use `CREATE INDEX CONCURRENTLY` to avoid blocking production

---

## Monitoring and Observability

### New Prometheus Metrics

```
# Embedding generation
mimir_embeddings_generated_total
mimir_embedding_generation_duration_seconds (histogram)
mimir_embedding_errors_total

# RAG retrieval
mimir_rag_searches_total
mimir_rag_search_duration_seconds (histogram)
mimir_rag_retrieved_messages (histogram)
mimir_rag_context_tokens (histogram)

# Performance
mimir_rag_e2e_latency_seconds (histogram)
mimir_rag_cache_hits_total
```

### Logging

All RAG operations include:
- `correlation_id` - Links to original message
- `search_duration_ms` - Time spent in hybrid search
- `retrieved_count` - Number of messages retrieved
- `context_tokens` - Total tokens in assembled context

---

## Configuration

### Environment Variables

```bash
# Embedding model (Ollama)
EMBEDDING_MODEL=nomic-embed-text

# RAG behavior
RAG_ENABLED=true
RAG_RETRIEVED_COUNT=5          # Number of messages to retrieve from history
RAG_RECENT_COUNT=10            # Number of recent messages to include
RAG_MIN_SIMILARITY=0.5         # Minimum cosine similarity threshold

# Search weights (must sum to 1.0)
RAG_WEIGHT_SEMANTIC=0.5
RAG_WEIGHT_KEYWORD=0.3
RAG_WEIGHT_RECENCY=0.2

# Performance
RAG_EMBEDDING_WORKERS=3        # Parallel embedding generation
RAG_SEARCH_TIMEOUT_MS=500      # Abort search if too slow
```

---

## Migration Path

### Phase 1: Deploy Schema (Week 1)
- Add `message_embeddings` table
- Add `content_tsv` column to `messages`
- Create indexes
- **No code changes yet** - backward compatible

### Phase 2: Deploy Code with Feature Flag (Week 1)
- Deploy RAG code with `RAG_ENABLED=false`
- Verify no regressions
- Start generating embeddings for new messages (silently)

### Phase 3: Backfill Embeddings (Week 2)
- Run `backfill-embeddings` tool for existing messages
- Monitor Ollama resource usage
- Build HNSW index after backfill completes

### Phase 4: Enable RAG (Week 2)
- Set `RAG_ENABLED=true`
- Monitor metrics: latency, accuracy, errors
- Tune search weights based on user feedback

### Phase 5: Optimize (Ongoing)
- Tune `ef_search` for accuracy/speed balance
- Adjust context token budgets
- Implement caching for frequent queries (if needed)

---

## Testing Strategy

### Unit Tests
- `embeddings`: Mock Ollama, test retry logic
- `rag/search`: Test hybrid search SQL correctness
- `rag/rerank`: Test BM25 scoring
- `rag/context`: Test deduplication and token budgets

### Integration Tests
- Full RAG pipeline end-to-end
- Test with real pgvector database
- Test with real Ollama (local)

### Load Tests
- 100 concurrent queries
- Measure p50, p95, p99 latency
- Ensure <1s p95 end-to-end

### Accuracy Tests
- Manually curated test cases
- "What did I say about X?" → Verify correct retrieval
- Test semantic similarity (synonyms, paraphrasing)
- Test keyword matching (exact terms)

---

## Future Enhancements

### Potential Improvements (Not in Initial Scope)

1. **Caching:** Cache frequent query embeddings
2. **Multi-turn context:** Track conversation threads (not just recency)
3. **User feedback:** "Was this response helpful?" → fine-tune weights
4. **Metadata filtering:** Search by date range, topic tags, sentiment
5. **Cross-encoder re-ranking:** Higher quality but slower (1.5s latency)
6. **Conversational memory management:** Automatic summarization of very old conversations
7. **Multi-user support:** Separate embedding indexes per user (tenant isolation)

---

## References

### Research Papers
- "Retrieval-Augmented Generation for Knowledge-Intensive NLP Tasks" (Lewis et al., 2020)
- "HNSW: Efficient and robust approximate nearest neighbor search" (Malkov & Yashunin, 2018)
- "Reciprocal Rank Fusion outperforms Condorcet and individual Rank Learning Methods" (Cormack et al., 2009)

### Tools & Libraries
- [pgvector](https://github.com/pgvector/pgvector) - PostgreSQL vector extension
- [Ollama](https://ollama.com/) - Local LLM runtime
- [nomic-embed-text](https://huggingface.co/nomic-ai/nomic-embed-text-v1) - Embedding model

### Best Practices
- [Pinecone: Understanding Hybrid Search](https://www.pinecone.io/learn/hybrid-search/)
- [OpenAI: Embedding Best Practices](https://platform.openai.com/docs/guides/embeddings/what-are-embeddings)
- [PostgreSQL: Full-Text Search](https://www.postgresql.org/docs/current/textsearch.html)

---

## Summary

**What RAG gives Mimir:**
- ✅ True long-term memory (months/years)
- ✅ Semantic search (concepts, not just keywords)
- ✅ Hybrid retrieval (best of all methods)
- ✅ Scalable (millions of messages, <1 GB storage)
- ✅ Fast (sub-second queries)
- ✅ Local-first (no external APIs)

**What RAG doesn't do:**
- ❌ Doesn't replace recent context (we keep both)
- ❌ Doesn't guarantee perfect recall (similarity threshold)
- ❌ Doesn't understand user intent (LLM still does reasoning)

**Bottom line:** RAG transforms Mimir from a chatbot with short-term memory into a true personal assistant that never forgets.
