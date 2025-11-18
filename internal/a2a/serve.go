// Package a2a wires a plain Go handler to the A2A protocol, so every agent's
// main() is the same twenty lines and nobody hand-rolls the artifact envelope.
//
// # The error rule, stated once
//
// A malformed input part is an A2A task failure. A non-nil error out of Handle
// is also a task failure. An agent that hit trouble but still has a partial
// answer does not return an error: it returns its normal output struct with
// Errors populated and a nil error. A dead source must not kill a run.
//
// The four agents whose output is a bare domain type have no Errors field and
// are not getting one, so they do fail the task. The orchestrator records that
// in RunRecord.DegradedReason and carries on.
//
// STUB: signatures only, bodies panic. B1 Task 11 implements this.
package a2a

import "context"

// Card is one agent's AgentCard.json.
//
// Agents never build one by hand; they call LoadCard and pass the result
// straight to Serve. That indirection is why Task 11 can reconcile this type
// with the SDK's own card without changing a single agent's main().
type Card struct {
	// ProtocolVersion must be "1.0". The Nasiko example ships "0.2.9" and a
	// real cluster rejects that with -32009 VersionNotSupported, so LoadCard
	// checks it rather than letting the failure surface at deploy time.
	ProtocolVersion string `json:"protocolVersion"`

	Name        string `json:"name"`
	Description string `json:"description"`
	URL         string `json:"url"`
	Version     string `json:"version"`
}

// Handler is what every agent implements: one method, its own input type in,
// its own output type out. No agent implements the SDK's executor interface
// directly.
type Handler[In, Out any] interface {
	Handle(ctx context.Context, in In) (Out, error)
}

// Artifact is the single application/json artifact an agent replies with.
type Artifact struct {
	MimeType string `json:"mimeType"`
	Name     string `json:"name"`
	Body     []byte `json:"body"`
}

// LoadCard reads an AgentCard.json from path and rejects a protocolVersion
// other than "1.0".
func LoadCard(path string) (Card, error) {
	panic("not implemented")
}

// Serve reads PORT, registers the health path, wraps the mux in otelhttp and
// blocks.
//
// The type parameters are inferred from h, so the call site is
// a2a.Serve(card, handler) with no type arguments written out and every
// agent's main() keeps the shape docs/CONTRACTS.md §3 prints.
func Serve[In, Out any](card Card, h Handler[In, Out]) error {
	panic("not implemented")
}

// Call reaches a peer agent through the Nasiko proxy. bp-orchestrator is the
// only caller.
//
// No agent constructs a peer URL: the proxy address and the routing header are
// Nasiko's, and a hardcoded one breaks on the next redeploy.
func Call(ctx context.Context, agent string, in any, out any) error {
	panic("not implemented")
}
