// Command bp-sov serves the share-of-voice agent over A2A.
//
// This file wires and nothing else. Any logic here is logic that escaped
// handler.go, and agents/CLAUDE.md is explicit that every agent's main() is
// the same twenty lines.
package main

import (
	"context"
	"log"

	"brandpulse/internal/a2a"
	"brandpulse/internal/obs"
)

// cardPath is absolute because the Dockerfile puts AgentCard.json at the image
// root and Nasiko does not promise a working directory.
const cardPath = "/AgentCard.json"

func main() {
	shutdown, err := obs.Setup("bp-sov")
	if err != nil {
		log.Fatalf("bp-sov: observability setup: %v", err)
	}
	defer func() {
		if err := shutdown(context.Background()); err != nil {
			log.Printf("bp-sov: observability shutdown: %v", err)
		}
	}()

	card, err := a2a.LoadCard(cardPath)
	if err != nil {
		log.Fatalf("bp-sov: load agent card: %v", err)
	}
	if err := a2a.Serve(card, SOVHandler{}); err != nil {
		log.Fatalf("bp-sov: serve: %v", err)
	}
}
