// Package obs sets up OpenTelemetry for a Go agent.
//
// Nasiko auto-injects OTel only into Dockerfiles with a "FROM python" base, so
// the nine Go agents instrument themselves. That is ~150 lines once, here,
// rather than nine copies.
//
// Every agent's main() calls Setup and defers the shutdown it returns. Nothing
// else in the repository configures a tracer provider.
//
// STUB: signature only, body panics. B1 Task 10 implements this.
package obs

import "context"

// Setup installs a tracer provider exporting to OTEL_EXPORTER_OTLP_ENDPOINT
// and returns the function that flushes and stops it.
//
// When OTEL_EXPORTER_OTLP_ENDPOINT is unset it returns a no-op shutdown and a
// nil error. Local dev and CI have no collector, and an agent that refuses to
// start without one is an agent that never runs in a test.
//
// The returned shutdown is safe to call more than once.
func Setup(serviceName string) (shutdown func(context.Context) error, err error) {
	panic("not implemented")
}
