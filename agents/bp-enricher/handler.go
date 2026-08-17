// EnricherHandler classifies mentions in batches and remembers what it has
// already classified.
//
// Two rules here are guardrails rather than design choices. Every text goes
// through redact.PII before it can reach a prompt, and every reply is joined
// back to its request on MentionID and never on position. Both have tests that
// fail loudly, because both are silent when broken.
//
// It must not decide what a mention means, cluster anything, or drop a mention
// it judged irrelevant: an IsAboutBrand=false enrichment is an answer, and the
// eval set cannot score a mention that never came back.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sync"

	"brandpulse/internal/llm"
	"brandpulse/internal/models"
	"brandpulse/internal/redact"
)

// chatJSON is llm.ChatJSON's signature, held as a field so a test can supply a
// stub. CI has no key and no network, so a package-level call here would make
// this agent untestable rather than merely untested.
type chatJSON func(ctx context.Context, prompt string, schema any, opt llm.Opt) (json.RawMessage, llm.Usage, error)

// EnricherHandler holds the content-hash cache, so it is long-lived and shared
// across requests. A2A serves concurrently; the mutex is not optional.
type EnricherHandler struct {
	chat chatJSON

	mu     sync.Mutex
	cached map[string]models.Enrichment
}

func NewEnricherHandler() *EnricherHandler {
	return &EnricherHandler{chat: llm.ChatJSON, cached: map[string]models.Enrichment{}}
}

// Handle classifies every mention in in.Mentions and returns one Enrichment
// per mention, in no guaranteed order.
//
// A mention the model will not answer twice degrades to a neutral enrichment
// with its id named in Errors. Degrading is per mention, not per batch: the
// answers that did come back are kept. It does not return an error: per
// CONTRACTS §1 the error return is for malformed input, and one bad batch must
// not lose a run.
func (h *EnricherHandler) Handle(ctx context.Context, in models.EnrichInput) (models.EnrichmentBatch, error) {
	out := models.EnrichmentBatch{Enrichments: []models.Enrichment{}, Errors: []string{}}
	if len(in.Mentions) == 0 {
		return out, nil
	}

	// Redaction happens once, here, before anything else can see the text.
	redacted := make(map[string]string, len(in.Mentions))
	for _, m := range in.Mentions {
		redacted[m.ID] = redact.PII(m.Text)
	}

	uncached := make([]models.Mention, 0, len(in.Mentions))
	for _, m := range in.Mentions {
		if hit, ok := h.lookup(m); ok {
			out.Enrichments = append(out.Enrichments, hit)
			out.CacheHits++
			continue
		}
		uncached = append(uncached, m)
	}

	for batch := range slices.Chunk(uncached, batchSize(in.BatchSize)) {
		answered, unanswered, usage, err := h.classify(ctx, batch, redacted, in.Profile)
		if err != nil {
			out.Errors = append(out.Errors, err.Error())
		}
		out.TokensUsed += usage.PromptTokens + usage.CompletionTokens
		out.CostPaise += usage.CostPaise
		// Only the answered enrichments are cached. A degraded neutral is a
		// placeholder, and caching one would neutralise that mention on every
		// later run instead of re-asking for it.
		out.Enrichments = append(out.Enrichments, h.store(batch, price(answered, usage))...)
		out.Enrichments = append(out.Enrichments, neutralEnrichments(unanswered)...)
	}
	return out, nil
}

// maxAttempts counts the first call plus one retry of whatever it left out. It
// lives here rather than with the prompt because it is retry policy, which is
// classify's business. Two and not three: attempt 3 would send the same bytes
// as attempt 2, since llm.Opt carries no temperature and no seed, so it would
// pay again for the same question having already learned the answer is not
// coming.
const maxAttempts = 2

// classify runs one batch, retrying once, and returns the enrichments it got
// alongside the mentions still unanswered after the last attempt. llm.ChatJSON
// already retries a reply that is not valid JSON; this retry is for the other
// failure, a reply that parses but does not answer for every mention.
//
// The retry asks only about what is still missing. Re-sending the whole batch
// pays again for answers already in hand and gives the model a fresh chance to
// drop a different id, which is how one omission turns into a loop that never
// converges.
func (h *EnricherHandler) classify(ctx context.Context, batch []models.Mention, redacted map[string]string, profile models.BrandProfile) ([]models.Enrichment, []models.Mention, llm.Usage, error) {
	var answered []models.Enrichment
	var total llm.Usage
	var lastErr error

	unanswered := batch
	for attempt := range maxAttempts {
		prompt, err := buildPrompt(unanswered, redacted, profile)
		if err != nil {
			return answered, unanswered, total, err
		}

		asked := len(unanswered)
		raw, usage, err := h.chat(ctx, prompt, enrichmentReply{}, llm.Opt{})
		total.PromptTokens += usage.PromptTokens
		total.CompletionTokens += usage.CompletionTokens
		total.CostPaise += usage.CostPaise
		if err == nil {
			var got []models.Enrichment
			got, unanswered, err = parseReply(raw, unanswered)
			answered = append(answered, got...)
			if len(unanswered) == 0 {
				return answered, nil, total, nil
			}
		}
		// asked, not len(unanswered): parseReply has already narrowed that to
		// the remainder, and the number worth reading is how many were sent.
		lastErr = fmt.Errorf("asked %d, attempt %d: %w", asked, attempt+1, err)
	}
	return answered, unanswered, total, lastErr
}

// price spreads one call's cost evenly across the mentions it answered. The
// provider bills per call, not per mention, so an even split is the only
// division available; the batch total in EnrichmentBatch.CostPaise is the
// number the cost dashboard should trust.
func price(enrichments []models.Enrichment, usage llm.Usage) []models.Enrichment {
	if len(enrichments) == 0 {
		return enrichments
	}
	perMention := usage.CostPaise / float64(len(enrichments))
	for i := range enrichments {
		enrichments[i].CostPaise = perMention
	}
	return enrichments
}

// batchSize applies the contract's default. BatchSize is an int and not a
// *int, so 0 means "unset" and 50 is what unset means.
func batchSize(requested int) int {
	if requested <= 0 {
		return defaultBatchSize
	}
	return min(requested, defaultBatchSize)
}

// lookup returns a cached enrichment re-keyed to this mention. Two mentions
// with identical text share a hash, so the id has to be overwritten rather
// than carried over from whichever mention was classified first.
func (h *EnricherHandler) lookup(m models.Mention) (models.Enrichment, bool) {
	if m.ContentHash == "" {
		return models.Enrichment{}, false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	hit, ok := h.cached[m.ContentHash]
	if !ok {
		return models.Enrichment{}, false
	}
	hit.MentionID = m.ID
	// A cache hit spent no tokens, so it costs nothing. Leaving the original
	// call's cost here would bill the same tokens on every re-run and make the
	// warm cost number in eval/ a fiction.
	hit.CostPaise = 0
	return hit, true
}

// store caches each enrichment under its mention's content hash and returns
// them unchanged.
func (h *EnricherHandler) store(batch []models.Mention, enrichments []models.Enrichment) []models.Enrichment {
	hashOf := make(map[string]string, len(batch))
	for _, m := range batch {
		hashOf[m.ID] = m.ContentHash
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, e := range enrichments {
		if hash := hashOf[e.MentionID]; hash != "" {
			h.cached[hash] = e
		}
	}
	return enrichments
}
