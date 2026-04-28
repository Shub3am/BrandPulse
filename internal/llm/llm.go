// Package llm is the only path to a language model in this repo. Every call
// goes through the Nasiko router at OPENAI_BASE_URL; no agent configures a
// provider SDK.
//
// This package belongs to B1 (their Task 7). Only the signatures from
// CONTRACTS §3 are here, with panicking bodies, so track B4 compiles. The
// agents that use it take a ChatJSONFunc rather than calling the package
// function directly, which is how their tests stub a model without a network.
package llm

import (
	"context"
	"encoding/json"
)

// Opt carries the per-call knobs. The Nasiko router discards the request's
// model field and per-agent choice is set with `nasiko llm-config`, so Model
// here is what the cost table is keyed against, not a routing instruction.
type Opt struct {
	Model     string
	MaxTokens int
}

// Usage is what one call actually cost. CostPaise is what eval/cost sums, so it
// carries a measured number and never an estimate.
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	CostPaise        float64
}

// ChatJSONFunc is the shape of ChatJSON, declared so an agent can hold it as a
// dependency and a test can substitute a canned response.
//
// It exists because ChatJSON is a package function: without this, every test of
// an LLM-using agent would need a live router, which is the definition of a
// broken test in this repo.
type ChatJSONFunc func(ctx context.Context, prompt string, schema any, opt Opt) (json.RawMessage, Usage, error)

// ChatJSON asks the router for a completion constrained to schema and returns
// the raw JSON body alongside what it cost.
func ChatJSON(ctx context.Context, prompt string, schema any, opt Opt) (json.RawMessage, Usage, error) {
	panic("not implemented: B1 Task 7 owns llm.ChatJSON")
}

// Embed returns one vector per text. It is on no critical path: the router
// lists chat models only and bp-clusterer uses TF-IDF.
func Embed(ctx context.Context, texts []string, model string) ([][]float32, error) {
	panic("not implemented: B1 Task 7 owns llm.Embed")
}
