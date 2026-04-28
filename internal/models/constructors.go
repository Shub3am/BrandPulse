// This file holds the two domain constructors docs/CONTRACTS.md §3 says B1
// owes and models.go does not yet carry. They belong beside the other six in
// models.go; they live here only so the "construct through the New*
// constructor" rule is satisfiable for the two types track B4 produces. Fold
// them into models.go when B1 lands.

package models

import "time"

// NewAlert returns an open alert with its slices ready, so a rule appends
// evidence rather than allocating it. Kind and Severity are arguments because
// there is no sensible default for either: every rule knows both at the moment
// it fires.
//
// DedupeKey is deliberately left empty. It is the idempotency key behind
// alerts UNIQUE (brand_id, dedupe_key) and only the firing rule knows the
// source and hour bucket that compose it, so Validate rejecting an empty one is
// the intended guard rather than a constructor guess.
func NewAlert(id, brandID string, kind AlertKind, severity Severity) Alert {
	return Alert{
		ID:             id,
		BrandID:        brandID,
		Kind:           kind,
		Severity:       severity,
		Evidence:       []AlertEvidence{},
		SampleMentions: []Mention{},
		Status:         AlertStatusOpen,
	}
}

// NewDailyBrief returns a brief with its slices ready for a period. Numbers
// stays zero: BriefNumbers is computed deterministically by the briefer's
// renderer and a zeroed set is the correct answer for a period with no
// mentions.
func NewDailyBrief(brandID string, periodStart, periodEnd time.Time) DailyBrief {
	return DailyBrief{
		BrandID:          brandID,
		PeriodStart:      periodStart,
		PeriodEnd:        periodEnd,
		TopTopics:        []Topic{},
		Alerts:           []Alert{},
		CompetitorWatch:  []string{},
		SuggestedActions: []string{},
	}
}
