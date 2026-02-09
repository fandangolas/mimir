package rag

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/fandangolas/mimir/internal/embeddings"
	"github.com/fandangolas/mimir/internal/llm"
	"github.com/fandangolas/mimir/internal/observability"
	"github.com/fandangolas/mimir/internal/store"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Pipeline coordinates the full RAG workflow.
type Pipeline struct {
	embedder       embeddings.Embedder
	searcher       *Searcher
	contextManager *ContextManager
	db             *store.DB
	enabled        bool
	retrievedCount int
	recentCount    int
}

// PipelineConfig configures the RAG pipeline.
type PipelineConfig struct {
	Enabled        bool         // Whether RAG is enabled
	RetrievedCount int          // Number of messages to retrieve from history
	RecentCount    int          // Number of recent messages to include
	SearchConfig   SearchConfig // Hybrid search configuration
	MaxTokens      int          // Maximum context tokens (default: 4096)
	SystemTokens   int          // Tokens reserved for system prompt (default: 200)
	QueryTokens    int          // Tokens reserved for query (default: 500)
	ReplyTokens    int          // Tokens reserved for reply (default: 1000)
}

// NewPipeline creates a new RAG pipeline.
func NewPipeline(
	embedder embeddings.Embedder,
	pool *pgxpool.Pool,
	db *store.DB,
	config PipelineConfig,
) *Pipeline {
	// Set defaults
	if config.RetrievedCount == 0 {
		config.RetrievedCount = 5
	}
	if config.RecentCount == 0 {
		config.RecentCount = 10
	}
	if config.MaxTokens == 0 {
		config.MaxTokens = 4096
	}
	if config.SystemTokens == 0 {
		config.SystemTokens = 200
	}
	if config.QueryTokens == 0 {
		config.QueryTokens = 500
	}
	if config.ReplyTokens == 0 {
		config.ReplyTokens = 1000
	}

	return &Pipeline{
		embedder:       embedder,
		searcher:       NewSearcher(pool, config.SearchConfig),
		contextManager: NewContextManager(
			config.MaxTokens,
			config.SystemTokens,
			config.QueryTokens,
			config.ReplyTokens,
		),
		db:             db,
		enabled:        config.Enabled,
		retrievedCount: config.RetrievedCount,
		recentCount:    config.RecentCount,
	}
}

// BuildContextWithRAG builds LLM context using RAG retrieval.
func (p *Pipeline) BuildContextWithRAG(
	ctx context.Context,
	sessionID int64,
	userQuery string,
) ([]llm.Message, error) {
	log := slog.Default().With("session_id", sessionID)

	// If RAG is disabled, fall back to recent messages only
	if !p.enabled {
		log.Debug("RAG disabled, using recent messages only")
		return p.buildContextWithoutRAG(ctx, sessionID)
	}

	// 1. Generate query embedding
	log.Debug("generating query embedding")
	embStart := time.Now()
	queryEmbedding, err := p.embedder.Embed(ctx, userQuery)
	observability.EmbeddingDuration.Observe(time.Since(embStart).Seconds())
	if err != nil {
		observability.EmbeddingErrors.Inc()
		observability.RAGSearches.WithLabelValues("fallback").Inc()
		log.Warn("failed to generate query embedding, falling back to keyword search", "error", err)
		return p.buildContextWithoutRAG(ctx, sessionID)
	}
	observability.EmbeddingsGenerated.Inc()

	// 2. Perform hybrid search
	log.Debug("performing hybrid search", "query", userQuery)
	searchStart := time.Now()
	retrieved, err := p.searcher.HybridSearch(
		ctx,
		sessionID,
		queryEmbedding,
		userQuery,
		p.retrievedCount,
	)
	observability.RAGSearchDuration.Observe(time.Since(searchStart).Seconds())

	if err != nil {
		observability.RAGSearches.WithLabelValues("error").Inc()
		log.Warn("hybrid search failed, falling back to recent messages", "error", err)
		return p.buildContextWithoutRAG(ctx, sessionID)
	}

	observability.RAGSearches.WithLabelValues("success").Inc()
	observability.RAGRetrievedMessages.Observe(float64(len(retrieved)))
	log.Debug("hybrid search complete", "retrieved_count", len(retrieved))

	// 3. Get recent messages
	recentMessages, err := p.db.GetRecentMessages(ctx, sessionID, p.recentCount)
	if err != nil {
		return nil, fmt.Errorf("get recent messages: %w", err)
	}

	log.Debug("retrieved recent messages", "recent_count", len(recentMessages))

	// 4. Truncate to fit token budget
	retrieved, recentMessages = p.contextManager.TruncateMessages(retrieved, recentMessages)

	log.Debug("truncated messages to fit budget",
		"final_retrieved", len(retrieved),
		"final_recent", len(recentMessages))

	// 5. Build context
	contextText := p.contextManager.BuildContext(
		"You are Mimir, a helpful AI assistant. Use the provided conversation history to give contextually relevant and personalized responses.",
		retrieved,
		recentMessages,
		userQuery,
	)

	// Track context size
	contextTokens := p.contextManager.EstimateTokens(contextText)
	observability.RAGContextTokens.Observe(float64(contextTokens))

	// Convert to LLM message format
	messages := []llm.Message{
		{
			Role:    llm.RoleSystem,
			Content: contextText,
		},
	}

	return messages, nil
}

// buildContextWithoutRAG builds context using only recent messages (fallback).
func (p *Pipeline) buildContextWithoutRAG(
	ctx context.Context,
	sessionID int64,
) ([]llm.Message, error) {
	recentMessages, err := p.db.GetRecentMessages(ctx, sessionID, p.recentCount)
	if err != nil {
		return nil, fmt.Errorf("get recent messages: %w", err)
	}

	// Build simple context without RAG
	messages := []llm.Message{
		{
			Role:    llm.RoleSystem,
			Content: "You are Mimir, a helpful AI assistant. Be concise and direct.",
		},
	}

	// Add recent messages
	for _, msg := range recentMessages {
		messages = append(messages, llm.Message{
			Role:    llm.Role(msg.Role),
			Content: msg.Content,
		})
	}

	return messages, nil
}

// StoreMessageWithEmbedding stores a message and generates its embedding asynchronously.
func (p *Pipeline) StoreMessageWithEmbedding(
	ctx context.Context,
	sessionID int64,
	role, content string,
) (int64, error) {
	// Store message first
	if err := p.db.SaveMessage(ctx, sessionID, role, content); err != nil {
		return 0, fmt.Errorf("save message: %w", err)
	}

	// Get the message ID (get most recent message for this session)
	messages, err := p.db.GetRecentMessages(ctx, sessionID, 1)
	if err != nil {
		return 0, fmt.Errorf("get message ID: %w", err)
	}
	if len(messages) == 0 {
		return 0, fmt.Errorf("no message found after save")
	}

	messageID := messages[0].ID

	// Generate embedding asynchronously (don't block)
	if p.enabled && content != "" {
		go func() {
			bgCtx := context.Background()
			log := slog.Default().With("message_id", messageID)

			start := time.Now()
			embedding, err := p.embedder.Embed(bgCtx, content)
			observability.EmbeddingDuration.Observe(time.Since(start).Seconds())

			if err != nil {
				observability.EmbeddingErrors.Inc()
				log.Error("failed to generate embedding", "error", err)
				return
			}

			observability.EmbeddingsGenerated.Inc()

			if err := p.db.SaveEmbedding(bgCtx, messageID, embedding); err != nil {
				log.Error("failed to save embedding", "error", err)
			} else {
				log.Debug("embedding saved successfully")
			}
		}()
	} else if content == "" {
		slog.Default().With("message_id", messageID).Warn("skipping embedding for empty message")
	}

	return messageID, nil
}

// IsEnabled returns whether RAG is enabled.
func (p *Pipeline) IsEnabled() bool {
	return p.enabled
}
