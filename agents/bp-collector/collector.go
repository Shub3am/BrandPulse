// collector.go holds everything bp-collector does: pick the adapter for one
// source, run it under one credit ceiling, drop the mentions we already have,
// and report what it cost.
//
// It must not fan out across sources, decide how many sources run, or retry a
// source. One CollectInput is one source and the orchestrator owns the rest.
// It must not know what a sentiment, a topic or an alert is.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"brandpulse/internal/anakin"
	"brandpulse/internal/anakin/sources"
	"brandpulse/internal/models"
)

// CollectorHandler fetches one source's mentions for one window.
//
// Its three collaborators are fields rather than package-level calls so a test
// needs no Anakin key, no fixture directory and no Postgres. That is not
// ceremony: the Anakin client is already an interface for the same reason, and
// the dedupe lookup is the only SQL this agent runs.
type CollectorHandler struct {
	// Adapters is the source-to-adapter lookup, sources.Adapters in
	// production. It is a field because the behaviour this agent owns, dedupe
	// and the budget ceiling, is not reachable through a real adapter: every
	// adapter's last step is mentions.Stamp, and ids.New still panics in B1's
	// tree. A test supplies a fetch that returns known mentions instead.
	Adapters map[models.Source]sources.FetchFunc

	// NewClient builds the Anakin client for one call, bound to that call's
	// MaxCredits. Per call and not per process: the ceiling is an input field
	// and a client carries its own spend counter.
	NewClient func(ctx context.Context, in models.CollectInput) (anakin.Client, error)

	// SeenHashes reports which of the given content hashes the brand already
	// has stored.
	SeenHashes func(ctx context.Context, brandID string, hashes []string) (map[string]bool, error)
}

// New builds the handler main() serves, wired to the real registry, the real
// Anakin client and the shared pool.
func New(pool *pgxpool.Pool) *CollectorHandler {
	return &CollectorHandler{
		Adapters:   sources.Adapters,
		NewClient:  anakinClient(pool),
		SeenHashes: storedHashes(pool),
	}
}

// Handle collects one source for one window.
//
// It returns a nil error for everything except a caller mistake, per
// CONTRACTS §1: a dead source, a blown budget and a failed dedupe lookup all
// come back as a populated MentionBatch. The A2A error return is for malformed
// input, and nothing here can produce one, so it is always nil.
func (h *CollectorHandler) Handle(ctx context.Context, in models.CollectInput) (models.MentionBatch, error) {
	batch := models.MentionBatch{
		Source:   in.Source,
		Mentions: []models.Mention{},
		Errors:   []string{},
	}

	fetch, ok := h.Adapters[in.Source]
	if !ok {
		// An unknown source is a routing mistake upstream, not a crash here.
		// Naming it matters: "amazon" reaching this agent means somebody put a
		// dead source back in a BrandProfile.
		batch.Errors = append(batch.Errors, fmt.Sprintf("no adapter is registered for source %q", in.Source))
		return batch, nil
	}

	client, err := h.NewClient(ctx, in)
	if err != nil {
		batch.Errors = append(batch.Errors, fmt.Sprintf("building the anakin client: %v", err))
		return batch, nil
	}

	collected, fetchErr := fetch(ctx, client, in.Profile, in.WindowStart, in.WindowEnd)
	if fetchErr != nil {
		// The ceiling is recorded as well as flagged. Truncated says the
		// window is incomplete; the message says which keyword it stopped on,
		// which is the only thing that makes a short run diagnosable.
		batch.Truncated = errors.Is(fetchErr, anakin.ErrBudgetExceeded)
		batch.Errors = append(batch.Errors, fmt.Sprintf("%s: %v", in.Source, fetchErr))
	}

	kept, err := h.withoutStored(ctx, in.Profile.BrandID, dedupe(collected))
	if err != nil {
		batch.Errors = append(batch.Errors, fmt.Sprintf("checking stored hashes: %v", err))
	}
	batch.Mentions = kept

	spend := client.Stats()
	batch.CreditsUsed = spend.CreditsUsed
	batch.CacheHits = spend.CacheHits
	return batch, nil
}

// dedupe removes repeats within one batch, keeping the first occurrence.
//
// Repeats inside one batch are normal rather than a bug: the Reddit and Search
// adapters run one query per keyword, and a brand with "boAt" and "boAt
// Airdopes" in its profile gets the same post back twice. mentions.Stamp gives
// the two copies different ids, so ContentHash is the only thing that catches
// them.
func dedupe(collected []models.Mention) []models.Mention {
	seen := make(map[string]bool, len(collected))
	out := make([]models.Mention, 0, len(collected))
	for _, m := range collected {
		if seen[m.ContentHash] {
			continue
		}
		seen[m.ContentHash] = true
		out = append(out, m)
	}
	return out
}

// withoutStored drops the mentions the brand already has.
//
// On a lookup failure it returns the batch unfiltered along with the error:
// mentions has a UNIQUE (brand_id, content_hash) constraint, so a duplicate
// that slips through is rejected at the insert, while dropping the whole batch
// would lose a window nobody re-collects.
func (h *CollectorHandler) withoutStored(ctx context.Context, brandID string, batch []models.Mention) ([]models.Mention, error) {
	if len(batch) == 0 {
		return batch, nil
	}

	hashes := make([]string, len(batch))
	for i, m := range batch {
		hashes[i] = m.ContentHash
	}
	stored, err := h.SeenHashes(ctx, brandID, hashes)
	if err != nil {
		return batch, err
	}

	out := make([]models.Mention, 0, len(batch))
	for _, m := range batch {
		if stored[m.ContentHash] {
			continue
		}
		out = append(out, m)
	}
	return out, nil
}

// storedHashes asks Postgres which of hashes brandID already holds.
//
// One statement with ANY($2) rather than a query per mention: a full window is
// a few hundred mentions and the orchestrator runs six collectors at once.
func storedHashes(pool *pgxpool.Pool) func(context.Context, string, []string) (map[string]bool, error) {
	return func(ctx context.Context, brandID string, hashes []string) (map[string]bool, error) {
		rows, err := pool.Query(ctx,
			`SELECT content_hash FROM mentions WHERE brand_id = $1 AND content_hash = ANY($2)`,
			brandID, hashes)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		stored := make(map[string]bool)
		for rows.Next() {
			var hash string
			if err := rows.Scan(&hash); err != nil {
				return nil, err
			}
			stored[hash] = true
		}
		return stored, rows.Err()
	}
}

// anakinClient builds one Anakin client per call, bound to that call's ceiling.
//
// An unset BP_FIXTURE_MODE is replay, not live. The repo rule is zero-credit
// CI, and a missing env var must not be the thing that starts spending money.
func anakinClient(pool *pgxpool.Pool) func(context.Context, models.CollectInput) (anakin.Client, error) {
	return func(ctx context.Context, in models.CollectInput) (anakin.Client, error) {
		mode := anakin.Mode(os.Getenv("BP_FIXTURE_MODE"))
		if mode == "" {
			mode = anakin.ModeReplay
		}
		return anakin.NewHTTPClient(anakin.Config{
			APIKey:     os.Getenv("ANAKIN_API_KEY"),
			Mode:       mode,
			MaxCredits: in.MaxCredits,
			BrandID:    in.Profile.BrandID,
			// The ceiling is scoped to a brand-day, and the day being
			// collected is the window's, not the clock's. A backfill run must
			// not spend against today's budget.
			Day:  in.WindowEnd,
			Pool: pool,
		})
	}
}
