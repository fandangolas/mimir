package rag

import "time"

// SearchResult represents a single message retrieved from hybrid search.
type SearchResult struct {
	MessageID     int64
	Content       string
	Role          string
	CreatedAt     time.Time
	SemanticScore float64
	KeywordScore  float64
	RecencyScore  float64
	FinalScore    float64
}
