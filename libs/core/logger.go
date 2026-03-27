package core

import (
	"context"
	"io"
	"os"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/trace"
)

// NewLogger returns a structured JSON logger for the given service name.
// The logger always includes the service name. Use [WithTraceID] to attach trace context.
func NewLogger(service string) zerolog.Logger {
	return zerolog.New(os.Stdout).
		With().
		Timestamp().
		Str("service", service).
		Logger()
}

// NewLoggerWithWriter returns a logger writing to the provided writer (useful in tests).
func NewLoggerWithWriter(w io.Writer, service string) zerolog.Logger {
	return zerolog.New(w).
		With().
		Timestamp().
		Str("service", service).
		Logger()
}

// WithTraceID returns a child logger enriched with the trace_id from the context.
// Call this at the start of each handler to attach the active span's trace ID.
func WithTraceID(ctx context.Context, log zerolog.Logger) zerolog.Logger {
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().HasTraceID() {
		return log
	}
	return log.With().Str("trace_id", span.SpanContext().TraceID().String()).Logger()
}
