// bp-detector fires the five statistical alert rules over one window of
// enriched mentions.
//
// It must never call a language model, open a socket or touch Postgres. Every
// rule is arithmetic over DetectInput, which is what lets an alert state the
// number and the threshold that fired it instead of asserting that an AI
// detected something. The rules are in rules.go; this file is the handler and
// the server, and nothing else.

package main

import (
	"context"
	"log"

	"brandpulse/internal/a2a"
	"brandpulse/internal/models"
	"brandpulse/internal/obs"
)

// cardPath is where the Dockerfile puts AgentCard.json. Building a Card literal
// here instead would skip every field the file carries, including
// supportedInterfaces, and a2a.Serve refuses a card without one.
const cardPath = "/AgentCard.json"

// DetectorHandler has no fields because the detector has no dependencies.
type DetectorHandler struct{}

// Handle returns every alert that fired, alongside how many rules ran.
//
// The error return is for malformed input only, and there is no malformed
// DetectInput: an empty window is a real answer, not a failure. A rule that
// produced an unusable alert reports itself through AlertSet.Errors so the
// other four rules' output still reaches the caller.
func (DetectorHandler) Handle(ctx context.Context, in models.DetectInput) (models.AlertSet, error) {
	return Evaluate(in), nil
}

func main() {
	shutdown, err := obs.Setup("bp-detector")
	if err != nil {
		log.Fatalf("bp-detector: observability setup failed: %v", err)
	}
	defer func() { _ = shutdown(context.Background()) }()

	card, err := a2a.LoadCard(cardPath)
	if err != nil {
		log.Fatalf("bp-detector: load agent card: %v", err)
	}

	if err := a2a.Serve(card, DetectorHandler{}); err != nil {
		log.Fatalf("bp-detector: %v", err)
	}
}
