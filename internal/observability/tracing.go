package observability

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/trace"
)

// SetupTracing initialises a stdout trace exporter.
// For Phase 2 this writes spans to stdout (JSON). In a later phase this can be
// swapped for an OTLP exporter (Jaeger, Tempo, etc.) without changing call sites.
func SetupTracing(ctx context.Context) (func(context.Context) error, error) {
	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		return nil, err
	}

	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
	)

	otel.SetTracerProvider(tp)

	slog.Info("tracing initialised", "exporter", "stdout")

	return tp.Shutdown, nil
}

