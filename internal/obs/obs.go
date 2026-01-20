// Package obs sets up OpenTelemetry for a Go agent.
//
// Nasiko auto-injects OTel only into Dockerfiles with a "FROM python" base, so
// the nine Go agents instrument themselves. That is ~150 lines once, here,
// rather than nine copies.
//
// Every agent's main() calls Setup and defers the shutdown it returns. Nothing
// else in the repository configures a tracer provider.
//
// It must not instrument anything. Setup builds the provider and stops;
// wrapping an HTTP handler is otelhttp's job and internal/a2a does it once, so
// no agent wires a middleware either.
package obs

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
)

// Setup installs a tracer provider exporting to OTEL_EXPORTER_OTLP_ENDPOINT
// and returns the function that flushes and stops it.
//
// When OTEL_EXPORTER_OTLP_ENDPOINT is unset it returns a no-op shutdown and a
// nil error. Local dev and CI have no collector, and an agent that refuses to
// start without one is an agent that never runs in a test.
//
// The returned shutdown is safe to call more than once and from more than one
// goroutine, which every agent needs because main() defers it and a signal
// handler also calls it. Both paths get that for free: the no-op does nothing,
// and sdktrace.TracerProvider.Shutdown guards itself with a compare-and-swap
// and returns nil on every call after the first. obs_test.go pins that so an
// SDK upgrade cannot quietly withdraw it.
func Setup(serviceName string) (shutdown func(context.Context) error, err error) {
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		return noShutdown, nil
	}

	ctx := context.Background()

	// The exporter reads OTEL_EXPORTER_OTLP_ENDPOINT, and the rest of the
	// OTEL_* variables, itself. Passing the endpoint explicitly would mean
	// this code decides what the spec already decides, and would then need its
	// own opinion on OTEL_EXPORTER_OTLP_TRACES_ENDPOINT overriding it.
	exporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return noShutdown, fmt.Errorf("obs: build the OTLP exporter for %s: %w", serviceName, err)
	}

	// resource.Default carries the SDK's own attributes and its schema URL.
	// Merging is what keeps service.name alongside them instead of replacing
	// them, and it fails loudly when the two schema URLs disagree, which is
	// the version-skew bug this would otherwise hide.
	attributes, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL, semconv.ServiceName(serviceName)),
	)
	if err != nil {
		return noShutdown, fmt.Errorf("obs: build the resource for %s: %w", serviceName, err)
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(attributes),
	)
	otel.SetTracerProvider(provider)

	// W3C traceparent, so a span raised in bp-collector is the same trace as
	// the span bp-orchestrator raised when it called it.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return provider.Shutdown, nil
}

func noShutdown(context.Context) error { return nil }
