// Package llm is the only path to a language model in this repository.
//
// Every call goes through the Nasiko router at OPENAI_BASE_URL. No agent
// configures a provider SDK, no agent holds a key, and no agent picks a
// provider: per-agent model choice is set with "nasiko llm-config" outside
// this code.
//
// Two consequences follow from the router, and both are load-bearing:
//
//   - The router ignores the model named in the request body. Usage.CostPaise
//     is therefore priced from the model the router reports back in the
//     response, not from Opt.Model. Costing the requested model would report a
//     number we never paid.
//   - bp-detector must not import this package. Alerting is statistics, and
//     every alert carries the numbers that fired it.
//
// STUB: signatures only, bodies panic. B1 Task 7 implements this.
package llm

import (
	"context"
	"encoding/json"
)

// Opt carries the per-call knobs. Both fields are optional: a zero Model lets
// the router's per-agent configuration decide, and a zero MaxTokens lets the
// provider default apply.
type Opt struct {
	Model     string
	MaxTokens int
}

// Usage is what one call actually cost. CostPaise is priced from the model the
// router reports in its response, never from the model requested.
type Usage struct {
	PromptTokens, CompletionTokens int
	CostPaise                      float64
}

// ChatJSON sends prompt and returns a response constrained to schema by a
// strict JSON-schema response format. schema is any Go value whose shape
// describes the expected reply; the raw JSON comes back undecoded so the
// caller owns the unmarshal.
//
// A reply that does not parse is retried once with a "return only valid JSON"
// nudge, then returned as an error.
func ChatJSON(ctx context.Context, prompt string, schema any, opt Opt) (json.RawMessage, Usage, error) {
	panic("not implemented")
}

// Embed returns one vector per text, batched at 100 per request.
//
// Nothing on the critical path calls this: the router lists chat models only
// and bp-clusterer clusters on local TF-IDF. The signature exists so the
// contract is complete.
func Embed(ctx context.Context, texts []string, model string) ([][]float32, error) {
	panic("not implemented")
}
