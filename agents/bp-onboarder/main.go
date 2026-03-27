// main.go is bp-onboarder's process entry point and nothing else: telemetry,
// the pool, the card, serve. Every line of behaviour is in onboarder.go, and
// if this file grows then logic has leaked out of the handler.
package main

import (
	"context"
	"fmt"
	"log"

	"brandpulse/internal/a2a"
	"brandpulse/internal/db"
	"brandpulse/internal/obs"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("bp-onboarder: %v", err)
	}
}

// run exists so the deferred shutdowns actually run. log.Fatal skips defers,
// and a2a.Serve blocks until it fails, so a Fatal inside main would mean the
// OTel exporter is never flushed and the last trace of a crash is lost.
func run() error {
	ctx := context.Background()

	shutdown, err := obs.Setup("bp-onboarder")
	if err != nil {
		return fmt.Errorf("obs.Setup: %w", err)
	}
	defer func() {
		if err := shutdown(ctx); err != nil {
			log.Printf("otel shutdown: %v", err)
		}
	}()

	pool, err := db.Pool(ctx)
	if err != nil {
		return fmt.Errorf("db.Pool: %w", err)
	}
	defer db.Close()

	card, err := a2a.LoadCard("AgentCard.json")
	if err != nil {
		return fmt.Errorf("a2a.LoadCard: %w", err)
	}

	return a2a.Serve(card, New(pool))
}
