package core

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
	"go.opentelemetry.io/otel/trace"
)

// InitTracer sets up the OpenTelemetry tracer provider and registers it globally.
// otlpEndpoint is the HTTP endpoint of the OTel collector (e.g. "http://otel-collector:4318").
// Call this once in main before starting any handlers. The returned shutdown function
// must be deferred to flush spans on exit.
func InitTracer(ctx context.Context, serviceName, otlpEndpoint string) (shutdown func(context.Context) error, err error) {
	exp, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpointURL(otlpEndpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("InitTracer: create OTLP exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(semconv.ServiceName(serviceName)),
		resource.WithFromEnv(),
	)
	if err != nil {
		return nil, fmt.Errorf("InitTracer: create resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	return tp.Shutdown, nil
}

// StartSpan starts a new span with the given name and returns the enriched context.
// The span name should be "HandlerName" or "repo.OperationName".
// Always defer span.End() immediately after calling StartSpan.
func StartSpan(ctx context.Context, spanName string) (context.Context, trace.Span) {
	return otel.Tracer("").Start(ctx, spanName)
}
