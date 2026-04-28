// Which sources a run actually calls, and which it drops.
//
// Nasiko's flow guard caps fan-out from one agent at 8 and it fails closed: the
// ninth call is dropped and returns an error, not a call that runs slowly. So
// the cap is applied here, before any call is made, and the drop is recorded
// rather than discovered.
//
// This file is pure. It does not call a peer, read a database or look at the
// clock, which is what lets its test assert the exact two sources a ten-source
// brand loses.

package main

import (
	"fmt"
	"sort"
	"strings"

	"brandpulse/internal/models"
)

// fanOutCap is the Nasiko flow guard's limit on calls out of one agent. Step 4
// is the only wide fan-out in the pipeline.
const fanOutCap = 8

// chooseSources ranks a brand's sources by yesterday's yield and keeps the best
// limit of them.
//
// yields is mentions per credit, keyed by source. A source with no row from
// yesterday is worth 0 here, which means a source enabled today waits a day for
// its first ranking. That is a real limitation and it is deliberate: the
// alternative is an exploration bonus, which spends credits on a guess.
//
// Ties break on the source name so the same brand drops the same two sources
// every run. A ranking that reorders under Go's randomised map iteration is a
// demo that cannot be rehearsed.
func chooseSources(profile models.BrandProfile, yields map[models.Source]float64, limit int) (chosen, skipped []models.Source) {
	ranked := make([]models.Source, 0, len(profile.Sources))
	seen := map[models.Source]bool{}
	for _, source := range profile.Sources {
		if seen[source] {
			continue
		}
		seen[source] = true
		ranked = append(ranked, source)
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		if yields[ranked[i]] != yields[ranked[j]] {
			return yields[ranked[i]] > yields[ranked[j]]
		}
		return ranked[i] < ranked[j]
	})

	if limit < 0 {
		limit = 0
	}
	if len(ranked) <= limit {
		return ranked, []models.Source{}
	}
	return ranked[:limit], ranked[limit:]
}

// degradedReason is what the dashboard shows when the cap bites. It names the
// cap and the sources that lost, because "degraded" on its own tells a founder
// nothing and tells a judge less.
func degradedReason(skipped []models.Source, limit int) string {
	if len(skipped) == 0 {
		return ""
	}
	names := make([]string, len(skipped))
	for i, source := range skipped {
		names[i] = string(source)
	}
	return fmt.Sprintf(
		"fan-out capped at %d sources, dropped the %d lowest-yielding: %s",
		limit, len(skipped), strings.Join(names, ", "),
	)
}

// splitCredits divides the day's budget evenly across the sources that ran.
//
// Evenly, not by yield: a high-yield source given the whole budget starves the
// rest, and one source's worth of mentions is not a brand's mention set. The
// remainder is dropped rather than handed to the first source, so two runs with
// the same budget give every source the same ceiling.
func splitCredits(budget, sources int) int {
	if sources <= 0 || budget <= 0 {
		return 0
	}
	return budget / sources
}
