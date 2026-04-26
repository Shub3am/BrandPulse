// Command bp-enricher serves the classification agent over A2A.
//
// Wiring only. The handler is built once and shared, because the content-hash
// cache lives on it and a per-request handler would spend tokens re-answering
// what it already knows.
package main

import (
	"context"
	"log"

	"brandpulse/internal/a2a"
	"brandpulse/internal/obs"
)

const cardPath = "/AgentCard.json"

func main() {
	shutdown, err := obs.Setup("bp-enricher")
	if err != nil {
		log.Fatalf("bp-enricher: observability setup: %v", err)
	}
	defer func() {
		if err := shutdown(context.Background()); err != nil {
			log.Printf("bp-enricher: observability shutdown: %v", err)
		}
	}()

	card, err := a2a.LoadCard(cardPath)
	if err != nil {
		log.Fatalf("bp-enricher: load agent card: %v", err)
	}
	if err := a2a.Serve(card, NewEnricherHandler()); err != nil {
		log.Fatalf("bp-enricher: serve: %v", err)
	}
}
