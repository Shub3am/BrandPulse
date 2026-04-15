// Command bp-clusterer serves the topic-clustering agent over A2A.
//
// Wiring only. The handler is stateless: unlike bp-enricher there is nothing
// worth caching here, because clustering is free and the labels depend on
// which mentions landed together this window.
package main

import (
	"context"
	"log"

	"brandpulse/internal/a2a"
	"brandpulse/internal/obs"
)

const cardPath = "/AgentCard.json"

func main() {
	shutdown, err := obs.Setup("bp-clusterer")
	if err != nil {
		log.Fatalf("bp-clusterer: observability setup: %v", err)
	}
	defer func() {
		if err := shutdown(context.Background()); err != nil {
			log.Printf("bp-clusterer: observability shutdown: %v", err)
		}
	}()

	card, err := a2a.LoadCard(cardPath)
	if err != nil {
		log.Fatalf("bp-clusterer: load agent card: %v", err)
	}
	if err := a2a.Serve(card, NewClustererHandler()); err != nil {
		log.Fatalf("bp-clusterer: serve: %v", err)
	}
}
