package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/fandangolas/mimir/internal/config"
	"github.com/fandangolas/mimir/internal/llm/ollama"
	"github.com/fandangolas/mimir/internal/observability"
	"github.com/fandangolas/mimir/internal/orchestrator"
	"github.com/fandangolas/mimir/internal/store"
	"github.com/fandangolas/mimir/internal/telegram"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.New(slog.NewJSONHandler(os.Stdout, nil)).Error("fatal error", "error", err)
		os.Exit(1)
	}

	if err := observability.SetupLogger(cfg.LogLevel, cfg.LogDir); err != nil {
		slog.New(slog.NewJSONHandler(os.Stdout, nil)).Error("failed to setup logger", "error", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := run(ctx, cfg); err != nil {
		slog.Error("fatal error", "error", err)
		os.Exit(1)
	}

	slog.Info("assistant stopped")
}

func run(ctx context.Context, cfg *config.Config) error {
	slog.Info("configuration loaded", "model", cfg.OllamaModel, "ollama_url", cfg.OllamaBaseURL)

	// Tracing
	shutdownTracing, err := observability.SetupTracing(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = shutdownTracing(context.Background()) }()

	// Start metrics server (non-blocking)
	observability.StartMetricsServer(ctx, cfg.MetricsAddr)

	// Database
	db, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	slog.Info("database connected and migrated")

	// Ollama LLM client
	ollamaClient := ollama.New(cfg.OllamaBaseURL, cfg.OllamaModel)

	if err := ollamaClient.Ping(ctx); err != nil {
		return err
	}
	slog.Info("ollama reachable", "model", cfg.OllamaModel)

	if err := ollamaClient.SupportsToolCalling(ctx); err != nil {
		return err
	}
	slog.Info("model supports tool calling", "model", cfg.OllamaModel)

	// Telegram client
	tgClient, err := telegram.New(cfg.TelegramBotToken)
	if err != nil {
		return err
	}
	slog.Info("telegram bot connected", "username", tgClient.Username())

	// Orchestrator
	orch := orchestrator.New(tgClient, ollamaClient, db, cfg.ConversationWindow)

	// Start polling — blocks until ctx is cancelled
	tgClient.Poll(ctx, cfg.AllowedUserIDs, orch.Handle)

	return nil
}
