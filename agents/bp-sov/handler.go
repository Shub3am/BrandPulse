// SOVHandler counts how much of a window's conversation is about this brand
// rather than about a competitor it named.
//
// It must not enrich, fetch or persist anything. Every judgement about what a
// mention means was already made by bp-enricher and arrives in the
// Enrichment; this file only counts, and the counting rule is a contract
// (CONTRACTS.md §2, bp-sov) rather than a local choice.
package main

import (
	"context"
	"strings"

	"brandpulse/internal/models"
)

// SOVHandler is stateless. A2A gives one instance to every request.
type SOVHandler struct{}

// Handle returns the share of voice for in.Profile over in's window.
//
// TotalMentions is the denominator, not len(in.Enriched): mentions about
// neither the brand nor a listed competitor are excluded, so the two differ
// whenever keyword search matched something unrelated.
func (SOVHandler) Handle(ctx context.Context, in models.SOVInput) (models.ShareOfVoice, error) {
	out := models.NewShareOfVoice(in.Profile.BrandID, in.WindowStart, in.WindowEnd)

	competitors := canonicalCompetitors(in.Profile.Competitors)
	overall := newTally()
	perSource := map[models.Source]*tally{}

	for _, enriched := range in.Enriched {
		name, counts := attribute(enriched.Enrichment, in.Profile.Name, competitors)
		if !counts {
			continue
		}
		overall.add(name)
		source := enriched.Mention.Source
		if perSource[source] == nil {
			perSource[source] = newTally()
		}
		perSource[source].add(name)
	}

	out.TotalMentions = overall.total
	out.BrandShare, out.CompetitorShares = split(overall.shares(), in.Profile.Name)
	for source, t := range perSource {
		out.BySource[source] = t.shares()
	}
	return out, nil
}

// attribute names who a mention counts for, or reports that it counts for
// nobody.
//
// AboutCompetitor is checked first because the contract counts a mention for
// the brand only when it is empty: a comparison carries both flags and belongs
// to the competitor it names. A competitor the profile does not list is an
// unknown, so it is excluded rather than added as a new competitor, which is
// the difference between "30% share" and "30% of what we recognised".
func attribute(e models.Enrichment, brandName string, competitors map[string]string) (string, bool) {
	if e.AboutCompetitor != "" {
		canonical, listed := competitors[fold(e.AboutCompetitor)]
		return canonical, listed
	}
	if e.IsAboutBrand {
		return brandName, true
	}
	return "", false
}

// canonicalCompetitors maps a folded competitor name to the spelling the
// profile uses, so the output is keyed the way the brand wrote it however the
// model cased it.
func canonicalCompetitors(names []string) map[string]string {
	canonical := make(map[string]string, len(names))
	for _, name := range names {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			canonical[fold(trimmed)] = trimmed
		}
	}
	return canonical
}

func fold(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// tally counts countable mentions by the name they count for.
type tally struct {
	counts map[string]int
	total  int
}

func newTally() *tally {
	return &tally{counts: map[string]int{}}
}

func (t *tally) add(name string) {
	t.counts[name]++
	t.total++
}

// shares converts counts to percentages. An empty tally returns an empty map
// rather than dividing by zero: NaN does not fail here, it fails at
// json.Marshal as "unsupported value" once the artifact is built.
func (t *tally) shares() map[string]float64 {
	shares := make(map[string]float64, len(t.counts))
	if t.total == 0 {
		return shares
	}
	for name, count := range t.counts {
		shares[name] = 100 * float64(count) / float64(t.total)
	}
	return shares
}

// split lifts the brand's own share out of the shares map, leaving the
// competitors. An absent brand key is a genuine 0%, not a missing value.
func split(shares map[string]float64, brandName string) (float64, map[string]float64) {
	brandShare := shares[brandName]
	delete(shares, brandName)
	return brandShare, shares
}
