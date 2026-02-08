package observability

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	MessagesReceived = promauto.NewCounter(prometheus.CounterOpts{
		Name: "assistant_messages_received_total",
		Help: "Total number of messages received from Telegram.",
	})

	UnauthorizedMessages = promauto.NewCounter(prometheus.CounterOpts{
		Name: "assistant_unauthorized_messages_total",
		Help: "Total number of messages rejected from unauthorized Telegram users.",
	})

	MessagesProcessed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "assistant_messages_processed_total",
		Help: "Total number of messages processed, by status (success|error).",
	}, []string{"status"})

	LLMLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "assistant_llm_duration_seconds",
		Help:    "LLM chat completion latency in seconds.",
		Buckets: prometheus.DefBuckets,
	})

	TelegramSendErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "assistant_telegram_send_errors_total",
		Help: "Total number of errors sending Telegram messages.",
	})
)

// StartMetricsServer starts the Prometheus /metrics HTTP server on the given
// address (e.g. ":9090"). It shuts down gracefully when ctx is cancelled.
func StartMetricsServer(ctx context.Context, addr string) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		slog.Info("metrics server listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("metrics server error", "error", err)
		}
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		slog.Info("metrics server stopped")
	}()
}
