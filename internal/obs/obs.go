// Package obs self-instruments a Go agent for OpenTelemetry. Nasiko
// auto-injects OTel into Python containers only, so every Go agent's main()
// calls Setup or it is invisible to `nasiko observe`.
//
// This package belongs to B1 (their Task 10). Only the no-collector path is
// implemented here, because that is the path every test and every local run
// takes and an agent that refuses to start without a collector is an agent
// that never runs in a test. The exporter itself is B1's.
package obs

import (
	"context"
	"fmt"
	"os"
)

// Setup builds the trace provider and returns its shutdown.
//
// With OTEL_EXPORTER_OTLP_ENDPOINT unset it returns a no-op shutdown and a nil
// error: local dev and CI have no collector. The returned shutdown is safe to
// call more than once.
func Setup(serviceName string) (shutdown func(context.Context) error, err error) {
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		return func(context.Context) error { return nil }, nil
	}
	return nil, fmt.Errorf(
		"obs: OTLP export is not implemented, B1 Task 10 owns it; unset OTEL_EXPORTER_OTLP_ENDPOINT to run %s without a collector",
		serviceName,
	)
}
