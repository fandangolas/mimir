package embeddings

import (
	"context"
	"fmt"
	"time"
)

// RetryEmbedder wraps an Embedder with exponential backoff retry logic.
type RetryEmbedder struct {
	embedder   Embedder
	maxRetries int
	baseDelay  time.Duration
}

// NewRetryEmbedder creates an embedder with retry logic.
func NewRetryEmbedder(embedder Embedder, maxRetries int, baseDelay time.Duration) *RetryEmbedder {
	if maxRetries < 1 {
		maxRetries = 3
	}
	if baseDelay == 0 {
		baseDelay = time.Second
	}

	return &RetryEmbedder{
		embedder:   embedder,
		maxRetries: maxRetries,
		baseDelay:  baseDelay,
	}
}

// Embed generates an embedding with retry logic.
func (r *RetryEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	var lastErr error

	for attempt := 0; attempt < r.maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s, ...
			delay := r.baseDelay * time.Duration(1<<uint(attempt-1))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		embedding, err := r.embedder.Embed(ctx, text)
		if err == nil {
			return embedding, nil
		}

		lastErr = err
	}

	return nil, fmt.Errorf("failed after %d attempts: %w", r.maxRetries, lastErr)
}

// EmbedBatch generates embeddings for multiple texts with retry logic.
func (r *RetryEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	var lastErr error

	for attempt := 0; attempt < r.maxRetries; attempt++ {
		if attempt > 0 {
			delay := r.baseDelay * time.Duration(1<<uint(attempt-1))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		embeddings, err := r.embedder.EmbedBatch(ctx, texts)
		if err == nil {
			return embeddings, nil
		}

		lastErr = err
	}

	return nil, fmt.Errorf("batch failed after %d attempts: %w", r.maxRetries, lastErr)
}
