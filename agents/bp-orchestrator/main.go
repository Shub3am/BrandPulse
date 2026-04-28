// bp-orchestrator runs the pipeline for one brand and one trigger.
//
// It is the only agent that calls peers, the only one that fans out, and the
// only place in this repo where a data race could hide. It uses no language
// model: every decision it makes is a count, a cap or a bucket.
//
// This file builds the handler and serves it. The pipeline is in pipeline.go,
// the source ranking in sources.go and the SQL in store.go, and none of that
// logic belongs here.

package main

import (
	"context"
	"log"

	"brandpulse/internal/a2a"
	"brandpulse/internal/db"
	"brandpulse/internal/obs"
)

func main() {
	shutdown, err := obs.Setup("bp-orchestrator")
	if err != nil {
		log.Fatalf("bp-orchestrator: observability setup failed: %v", err)
	}
	defer func() { _ = shutdown(context.Background()) }()

	pool, err := db.Pool(context.Background())
	if err != nil {
		log.Fatalf("bp-orchestrator: %v", err)
	}
	defer db.Close()

	handler := NewOrchestratorHandler(NewPostgresStore(pool))
	if err := a2a.Serve(a2a.Card{Name: "bp-orchestrator"}, handler); err != nil {
		log.Fatalf("bp-orchestrator: %v", err)
	}
}
