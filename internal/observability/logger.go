package observability

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

type contextKey string

const correlationIDKey contextKey = "correlation_id"

// SetupLogger configures the global slog logger with JSON output.
// If logDir is non-empty, logs are written to both stdout and a daily log file
// in that directory (for Promtail to tail).
func SetupLogger(level, logDir string) error {
	var l slog.Level
	switch level {
	case "debug":
		l = slog.LevelDebug
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: l}

	var w io.Writer = os.Stdout

	if logDir != "" {
		if err := os.MkdirAll(logDir, 0o755); err != nil {
			return fmt.Errorf("creating log directory: %w", err)
		}
		logFile := filepath.Join(logDir, fmt.Sprintf("assistant-%s.log", time.Now().Format("2006-01-02")))
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return fmt.Errorf("opening log file: %w", err)
		}
		w = io.MultiWriter(os.Stdout, f)
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(w, opts)))
	return nil
}

// WithCorrelationID returns a context carrying the given correlation ID.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationIDKey, id)
}

// FromContext returns a logger enriched with the correlation ID from ctx.
func FromContext(ctx context.Context) *slog.Logger {
	id, _ := ctx.Value(correlationIDKey).(string)
	if id == "" {
		return slog.Default()
	}
	return slog.Default().With("correlation_id", id)
}
