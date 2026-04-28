// Package a2a wires a BrandPulse handler to the A2A SDK, so every agent's
// main() is the same twenty lines and nobody hand-rolls an artifact envelope.
//
// This package belongs to B1 (their Task 11). Only the signatures from
// CONTRACTS §3 are here, with panicking bodies, so track B4 compiles. The
// generic Handler shape is B1's own refinement, recorded in their brief:
// inference keeps the call site exactly `a2a.Serve(card, handler)`.
//
// bp-orchestrator holds Call as a dependency rather than calling this package
// function, so its tests stub a peer without a proxy. No agent writes a peer
// URL anywhere: the proxy address and the routing header are Nasiko's, and a
// hardcoded one breaks on redeploy.
package a2a

import "context"

// Card identifies the agent being served. B1's loader will read the rest of
// AgentCard.json; the name is all a call site needs to write.
type Card struct {
	Name string
}

// Handler is the one shape every BrandPulse agent implements: one method, one
// typed input, one typed output.
type Handler[In, Out any] interface {
	Handle(context.Context, In) (Out, error)
}

// Artifact is the single application/json artifact an agent replies with.
type Artifact struct {
	MimeType string `json:"mimeType"`
	Name     string `json:"name"`
	Body     []byte `json:"body"`
}

// CallFunc is the shape of Call, declared so bp-orchestrator can hold it as a
// dependency and a test can substitute a stub peer.
type CallFunc func(ctx context.Context, agent string, in any, out any) error

// JSONArtifact builds the envelope from CONTRACTS §1: mimeType
// application/json, name the output type name, body json.Marshal(v).
func JSONArtifact(name string, v any) (Artifact, error) {
	panic("not implemented: B1 Task 11 owns a2a.JSONArtifact")
}

// Serve registers the handler and the health path, reads PORT, and blocks.
//
// A malformed input part is an A2A task failure, and so is a non-nil error out
// of Handle. An agent that hit trouble returns its normal output struct with
// Errors populated and a nil error: a dead source must not kill a run.
func Serve[In, Out any](card Card, h Handler[In, Out]) error {
	panic("not implemented: B1 Task 11 owns a2a.Serve")
}

// Call reaches a peer agent through the Nasiko proxy. bp-orchestrator is the
// only caller.
func Call(ctx context.Context, agent string, in any, out any) error {
	panic("not implemented: B1 Task 11 owns a2a.Call")
}
