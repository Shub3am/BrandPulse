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

	if err := a2a.Serve(a2a.Card{Name: "bp-detector"}, DetectorHandler{}); err != nil {
		log.Fatalf("bp-detector: %v", err)
	}
}
