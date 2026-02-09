package rag

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
)

// Searcher performs hybrid search combining semantic, keyword, and recency signals.
type Searcher struct {
	pool             *pgxpool.Pool
	semanticWeight   float64
	keywordWeight    float64
	recencyWeight    float64
	minSimilarity    float64
	recencyDecayDays float64
}

// SearchConfig configures the hybrid search behavior.
type SearchConfig struct {
	SemanticWeight   float64 // Weight for semantic similarity (default: 0.5)
	KeywordWeight    float64 // Weight for keyword matching (default: 0.3)
	RecencyWeight    float64 // Weight for recency boost (default: 0.2)
	MinSimilarity    float64 // Minimum cosine similarity threshold (default: 0.5)
	RecencyDecayDays float64 // Days for recency decay (default: 30)
}

// NewSearcher creates a new hybrid searcher.
func NewSearcher(pool *pgxpool.Pool, config SearchConfig) *Searcher {
	// Set defaults if not provided
	if config.SemanticWeight == 0 {
		config.SemanticWeight = 0.5
	}
	if config.KeywordWeight == 0 {
		config.KeywordWeight = 0.3
	}
	if config.RecencyWeight == 0 {
		config.RecencyWeight = 0.2
	}
	if config.MinSimilarity == 0 {
		config.MinSimilarity = 0.5
	}
	if config.RecencyDecayDays == 0 {
		config.RecencyDecayDays = 30
	}

	return &Searcher{
		pool:             pool,
		semanticWeight:   config.SemanticWeight,
		keywordWeight:    config.KeywordWeight,
		recencyWeight:    config.RecencyWeight,
		minSimilarity:    config.MinSimilarity,
		recencyDecayDays: config.RecencyDecayDays,
	}
}

// HybridSearch performs semantic + keyword + recency search.
func (s *Searcher) HybridSearch(
	ctx context.Context,
	sessionID int64,
	queryEmbedding []float32,
	queryText string,
	limit int,
) ([]SearchResult, error) {
	query := `
	WITH semantic_search AS (
		SELECT
			m.id,
			m.content,
			m.role,
			m.created_at,
			1 - (me.embedding <=> $1) AS semantic_score,
			ROW_NUMBER() OVER (ORDER BY me.embedding <=> $1) AS semantic_rank
		FROM messages m
		JOIN message_embeddings me ON me.message_id = m.id
		WHERE m.session_id = $2
			AND (1 - (me.embedding <=> $1)) >= $3
		ORDER BY me.embedding <=> $1
		LIMIT 20
	),
	keyword_search AS (
		SELECT
			m.id,
			ts_rank_cd(m.content_tsv, websearch_to_tsquery('english', $4)) AS keyword_score,
			ROW_NUMBER() OVER (
				ORDER BY ts_rank_cd(m.content_tsv, websearch_to_tsquery('english', $4)) DESC
			) AS keyword_rank
		FROM messages m
		WHERE
			m.session_id = $2
			AND m.content_tsv @@ websearch_to_tsquery('english', $4)
		ORDER BY keyword_score DESC
		LIMIT 20
	),
	recency_scores AS (
		SELECT
			m.id,
			EXP(-EXTRACT(EPOCH FROM (NOW() - m.created_at)) / (86400.0 * $5)) AS recency_score
		FROM messages m
		WHERE m.session_id = $2
	)
	SELECT
		COALESCE(ss.id, ks.id) AS message_id,
		COALESCE(ss.content, m.content) AS content,
		COALESCE(ss.role, m.role) AS role,
		COALESCE(ss.created_at, m.created_at) AS created_at,
		COALESCE(ss.semantic_score, 0) AS semantic_score,
		COALESCE(ks.keyword_score, 0) AS keyword_score,
		COALESCE(rs.recency_score, 0) AS recency_score,
		($6 * COALESCE(ss.semantic_score, 0) +
		 $7 * COALESCE(ks.keyword_score, 0) +
		 $8 * COALESCE(rs.recency_score, 0)) AS final_score
	FROM semantic_search ss
	FULL OUTER JOIN keyword_search ks ON ss.id = ks.id
	LEFT JOIN recency_scores rs ON COALESCE(ss.id, ks.id) = rs.id
	LEFT JOIN messages m ON m.id = COALESCE(ss.id, ks.id)
	ORDER BY final_score DESC
	LIMIT $9
	`

	embVector := pgvector.NewVector(queryEmbedding)

	rows, err := s.pool.Query(
		ctx,
		query,
		embVector,                // $1: query embedding
		sessionID,                // $2: session_id filter
		s.minSimilarity,          // $3: minimum similarity threshold
		queryText,                // $4: query text for keyword search
		s.recencyDecayDays,       // $5: recency decay days
		s.semanticWeight,         // $6: semantic weight
		s.keywordWeight,          // $7: keyword weight
		s.recencyWeight,          // $8: recency weight
		limit,                    // $9: result limit
	)
	if err != nil {
		return nil, fmt.Errorf("execute hybrid search: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		err := rows.Scan(
			&r.MessageID,
			&r.Content,
			&r.Role,
			&r.CreatedAt,
			&r.SemanticScore,
			&r.KeywordScore,
			&r.RecencyScore,
			&r.FinalScore,
		)
		if err != nil {
			return nil, fmt.Errorf("scan search result: %w", err)
		}
		results = append(results, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate search results: %w", err)
	}

	return results, nil
}

// SemanticSearchOnly performs only semantic similarity search (fallback mode).
func (s *Searcher) SemanticSearchOnly(
	ctx context.Context,
	sessionID int64,
	queryEmbedding []float32,
	limit int,
) ([]SearchResult, error) {
	query := `
		SELECT
			m.id,
			m.content,
			m.role,
			m.created_at,
			1 - (me.embedding <=> $1) AS semantic_score
		FROM messages m
		JOIN message_embeddings me ON me.message_id = m.id
		WHERE m.session_id = $2
			AND (1 - (me.embedding <=> $1)) >= $3
		ORDER BY me.embedding <=> $1
		LIMIT $4
	`

	embVector := pgvector.NewVector(queryEmbedding)

	rows, err := s.pool.Query(ctx, query, embVector, sessionID, s.minSimilarity, limit)
	if err != nil {
		return nil, fmt.Errorf("execute semantic search: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		err := rows.Scan(
			&r.MessageID,
			&r.Content,
			&r.Role,
			&r.CreatedAt,
			&r.SemanticScore,
		)
		if err != nil {
			return nil, fmt.Errorf("scan result: %w", err)
		}
		r.FinalScore = r.SemanticScore
		results = append(results, r)
	}

	return results, rows.Err()
}
