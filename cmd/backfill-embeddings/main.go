package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/fandangolas/mimir/internal/config"
	"github.com/fandangolas/mimir/internal/embeddings"
	"github.com/fandangolas/mimir/internal/observability"
	"github.com/fandangolas/mimir/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	if err := observability.SetupLogger(cfg.LogLevel, cfg.LogDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error setting up logger: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()

	if err := run(ctx, cfg); err != nil {
		slog.Error("backfill failed", "error", err)
		os.Exit(1)
	}

	slog.Info("backfill completed successfully")
}

func run(ctx context.Context, cfg *config.Config) error {
	// Connect to database
	db, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer db.Close()

	slog.Info("database connected")

	// Check total count
	totalCount, err := db.CountMessagesWithoutEmbeddings(ctx)
	if err != nil {
		return fmt.Errorf("count messages: %w", err)
	}

	if totalCount == 0 {
		slog.Info("no messages need backfilling")
		return nil
	}

	slog.Info("starting backfill", "total_messages", totalCount)

	// Create embedder with retry logic
	baseEmbedder := embeddings.NewOllamaEmbedder(cfg.OllamaBaseURL, cfg.EmbeddingModel)
	embedder := embeddings.NewRetryEmbedder(baseEmbedder, 3, time.Second)

	// Backfill configuration
	batchSize := 100
	concurrency := 3

	// Process in batches
	processed := 0
	errors := 0

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for processed < int(totalCount) {
		// Fetch batch
		messages, err := db.GetMessagesWithoutEmbeddings(ctx, batchSize)
		if err != nil {
			return fmt.Errorf("fetch batch: %w", err)
		}

		if len(messages) == 0 {
			break // No more messages
		}

		// Process batch concurrently
		for _, msg := range messages {
			sem <- struct{}{}
			wg.Add(1)

			go func(m store.Message) {
				defer wg.Done()
				defer func() { <-sem }()

				if err := processMessage(ctx, embedder, db, m); err != nil {
					slog.Error("failed to process message",
						"message_id", m.ID,
						"error", err)
					errors++
				} else {
					processed++
				}
			}(msg)

			// Log progress every 100 messages
			if processed%100 == 0 && processed > 0 {
				remaining := int(totalCount) - processed
				slog.Info("progress",
					"processed", processed,
					"total", totalCount,
					"remaining", remaining,
					"errors", errors,
					"percent", fmt.Sprintf("%.1f%%", float64(processed)/float64(totalCount)*100))
			}
		}

		// Rate limiting: small pause between batches
		time.Sleep(100 * time.Millisecond)
	}

	// Wait for all in-flight work to complete
	wg.Wait()

	slog.Info("backfill summary",
		"total_messages", totalCount,
		"processed", processed,
		"errors", errors,
		"success_rate", fmt.Sprintf("%.1f%%", float64(processed)/float64(totalCount)*100))

	if errors > 0 {
		return fmt.Errorf("backfill completed with %d errors", errors)
	}

	return nil
}

func processMessage(
	ctx context.Context,
	embedder embeddings.Embedder,
	db *store.DB,
	msg store.Message,
) error {
	// Generate embedding
	embedding, err := embedder.Embed(ctx, msg.Content)
	if err != nil {
		return fmt.Errorf("generate embedding: %w", err)
	}

	// Save embedding
	if err := db.SaveEmbedding(ctx, msg.ID, embedding); err != nil {
		return fmt.Errorf("save embedding: %w", err)
	}

	return nil
}
