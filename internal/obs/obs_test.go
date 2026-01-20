// obs_test.go is about the unconfigured case, because that is the one every
// other test in this repository runs inside.
//
// CI has no collector. If Setup returned an error there, every agent's main()
// would fail on line one and nine test suites would be testing nothing.
package obs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

func TestSetupWithNoCollectorIsNotAnError(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	shutdown, err := Setup("bp-test")
	if err != nil {
		t.Fatalf("Setup with no endpoint: %v, want nil; CI has no collector", err)
	}
	if shutdown == nil {
		t.Fatal("Setup returned a nil shutdown, which every agent's main() defers")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("the no-op shutdown returned %v, want nil", err)
	}
}

func TestTheNoOpShutdownIsSafeToCallTwice(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	shutdown, err := Setup("bp-test")
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	for attempt := 1; attempt <= 2; attempt++ {
		if err := shutdown(context.Background()); err != nil {
			t.Fatalf("shutdown call %d returned %v, want nil", attempt, err)
		}
	}
}

func TestAConfiguredShutdownIsSafeToCallTwice(t *testing.T) {
	// An agent defers shutdown and a signal handler calls it, so the second
	// call is the normal case, not an edge case. The SDK's TracerProvider
	// already returns nil after the first, so obs adds nothing here; this test
	// exists to catch an SDK upgrade that withdraws the guarantee, at which
	// point obs has to supply it.
	shutdown := setupAgainstACollector(t)

	for attempt := 1; attempt <= 2; attempt++ {
		if err := shutdown(context.Background()); err != nil {
			t.Fatalf("shutdown call %d returned %v, want nil", attempt, err)
		}
	}
}

func TestAConfiguredShutdownIsSafeUnderConcurrentCallers(t *testing.T) {
	// The defer and the signal handler are on different goroutines, so "twice"
	// above is not enough on its own. Run under -race.
	shutdown := setupAgainstACollector(t)

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := shutdown(context.Background()); err != nil {
				t.Errorf("concurrent shutdown returned %v, want nil", err)
			}
		}()
	}
	wg.Wait()
}

func TestAConfiguredSetupInstallsTheW3CPropagator(t *testing.T) {
	shutdown := setupAgainstACollector(t)
	defer shutdown(context.Background())

	// traceparent is how a span raised in bp-collector stays in the trace
	// bp-orchestrator started. Without it every agent gets its own root span
	// and the dashboard's waterfall is nine unrelated bars.
	fields := otel.GetTextMapPropagator().Fields()
	if !contains(fields, "traceparent") {
		t.Errorf("the propagator carries %v, want traceparent among them", fields)
	}
	if !contains(fields, "baggage") {
		t.Errorf("the propagator carries %v, want baggage among them", fields)
	}

	var carrier propagation.MapCarrier = map[string]string{}
	otel.GetTextMapPropagator().Inject(context.Background(), carrier)
	// An empty context has no span, so nothing is injected. The assertion is
	// that Inject does not panic on the propagator Setup installed.
	if len(carrier) != 0 {
		t.Errorf("injecting an empty context wrote %v, want nothing", carrier)
	}
}

// setupAgainstACollector points Setup at an httptest server that accepts every
// OTLP POST, which is the shortest way to get the configured branch of Setup
// without a collector in the test environment.
func setupAgainstACollector(t *testing.T) func(context.Context) error {
	t.Helper()

	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(collector.Close)
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", collector.URL)

	shutdown, err := Setup("bp-test")
	if err != nil {
		t.Fatalf("Setup against %s: %v", collector.URL, err)
	}
	return shutdown
}

func contains(haystack []string, needle string) bool {
	for _, straw := range haystack {
		if straw == needle {
			return true
		}
	}
	return false
}
