// Package wire strips the job envelope off a Client.Wire response.
//
// internal/anakin hands back the whole poll response on purpose: it must not
// know what any action's payload looks like. That response nests the real
// payload two "data" hops down, and a partial failure surfaces in the inner
// envelope rather than in the job's own status. Both were read live on
// 2026-09-20 and are written down in docs/research/wire-schemas.md §1.
//
// Unmarshalling one hop short is silent. The per-action struct decodes to zero
// values with no error, which is a dashboard reporting a quiet week for a
// source that returned 119 posts. That is the failure this package exists to
// stop, so every Wire-backed adapter goes through it and none of them keeps its
// own copy of the envelope.
//
// It must not know what any action's payload contains, and it must never invent
// one: an envelope carrying no data is an error, not an empty result.
package wire

import (
	"encoding/json"
	"fmt"
)

// envelope is the poll response. Its top-level keys are exactly credits_used,
// data, execution_ms and status; there is no "job" key despite what the path
// "job.data.data" in the research suggests.
//
// Error is a RawMessage rather than a string because every recorded response
// has it null and nothing has shown what a failure puts there. Reporting the
// bytes verbatim beats guessing at a shape and failing to decode a response
// that was fine.
type envelope struct {
	Status      string `json:"status"`
	CreditsUsed int    `json:"credits_used"`
	Data        struct {
		Status string          `json:"status"`
		Error  json.RawMessage `json:"error"`
		Data   json.RawMessage `json:"data"`
	} `json:"data"`
}

// Payload returns the per-action payload, the bytes an adapter's own struct is
// unmarshalled from.
func Payload(raw json.RawMessage) (json.RawMessage, error) {
	var resp envelope
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("wire: decoding the job envelope: %w", err)
	}
	if reported := string(resp.Data.Error); reported != "" && reported != "null" {
		return nil, fmt.Errorf("wire: the job succeeded but its payload reports %s", reported)
	}
	if len(resp.Data.Data) == 0 {
		return nil, fmt.Errorf("wire: the job envelope carries no data (job status %q, payload status %q)",
			resp.Status, resp.Data.Status)
	}
	return resp.Data.Data, nil
}
