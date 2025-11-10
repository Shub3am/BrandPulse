// agentio.go holds the per-agent request and response envelopes: the nine
// *Input types a caller sends and the four batch types that carry partial
// results back.
//
// It exists separately from models.go so the domain types stay readable on
// their own. It must not gain behaviour: like the rest of this package it is
// pure schema, and the rule that an envelope carries no constructor is
// deliberate. The batch types are built field by field by the agent that
// returns them, and an agent that forgets Errors returns an empty slice, which
// is the truthful answer.
//
// Field names on the wire are the json tags, copied verbatim from
// docs/CONTRACTS.md §2. Do not tidy one.
//
// Every int input whose Python default was non-zero is documented at the
// field: the agent applies the default, the caller may omit it. There are no
// *int fields here and none is to be added.
package models

import "time"

// ---------------------------------------------------------------------------
// bp-onboarder
// ---------------------------------------------------------------------------

// OnboardInput asks for a keyword set built from a brand's own website.
// Output is a BrandProfile with Version 1 and no confirmed_at: a human
// confirms it in DronaHQ, and confirmation does not re-run the agent.
type OnboardInput struct {
	BrandID     string   `json:"brand_id"`
	Name        string   `json:"name"`
	Website     string   `json:"website,omitempty"`
	Competitors []string `json:"competitors"`

	// MaxPages caps the Anakin crawl. 0 means 25; the agent applies the
	// default.
	MaxPages int `json:"max_pages"`
}

// ---------------------------------------------------------------------------
// bp-collector
// ---------------------------------------------------------------------------

// CollectInput is one source for one window. The orchestrator fans out across
// sources and is the only component that decides how many run.
type CollectInput struct {
	Profile     BrandProfile `json:"profile"`
	Source      Source       `json:"source"`
	WindowStart time.Time    `json:"window_start"`
	WindowEnd   time.Time    `json:"window_end"`
	MaxCredits  int          `json:"max_credits"`
	RunID       string       `json:"run_id"`
}

// MentionBatch is bp-collector's output. Truncated means the collector stopped
// at MaxCredits rather than at the end of the window, so the window is not
// fully covered and a later run should not treat it as complete.
type MentionBatch struct {
	Mentions    []Mention `json:"mentions"`
	CreditsUsed int       `json:"credits_used"`
	CacheHits   int       `json:"cache_hits"`
	Source      Source    `json:"source"`
	Truncated   bool      `json:"truncated"`
	Errors      []string  `json:"errors"`
}

// ---------------------------------------------------------------------------
// bp-enricher
// ---------------------------------------------------------------------------

// EnrichInput is a batch of mentions to classify.
type EnrichInput struct {
	Mentions []Mention    `json:"mentions"`
	Profile  BrandProfile `json:"profile"`

	// BatchSize is mentions per LLM call. 0 means 50; the agent applies the
	// default.
	BatchSize int `json:"batch_size"`
}

// EnrichmentBatch is bp-enricher's output. Order is not guaranteed: join on
// Enrichment.MentionID, never on position.
type EnrichmentBatch struct {
	Enrichments []Enrichment `json:"enrichments"`
	TokensUsed  int          `json:"tokens_used"`
	CostPaise   float64      `json:"cost_paise"`
	CacheHits   int          `json:"cache_hits"`
	Errors      []string     `json:"errors"`
}

// ---------------------------------------------------------------------------
// bp-clusterer
// ---------------------------------------------------------------------------

// ClusterInput is one window of enriched mentions plus the prior window's
// counts, which is what Topic.Trend is computed against.
type ClusterInput struct {
	Enriched    []EnrichedMention `json:"enriched"`
	BrandID     string            `json:"brand_id"`
	WindowStart time.Time         `json:"window_start"`
	WindowEnd   time.Time         `json:"window_end"`

	// PriorWindowCounts is keyed by topic label. A label absent here means no
	// prior window, and Trend is 1.0 rather than 0.
	PriorWindowCounts map[string]int `json:"prior_window_counts"`

	// MinClusterSize is the smallest group that becomes a Topic. 0 means 3;
	// the agent applies the default.
	MinClusterSize int `json:"min_cluster_size"`
}

// TopicSet is bp-clusterer's output. Unclustered holds mention ids that fell
// into noise; they are not lost, they are just not a topic.
type TopicSet struct {
	Topics      []Topic  `json:"topics"`
	Unclustered []string `json:"unclustered"`
	TokensUsed  int      `json:"tokens_used"`
	CostPaise   float64  `json:"cost_paise"`
	Errors      []string `json:"errors"`
}

// ---------------------------------------------------------------------------
// bp-sov
// ---------------------------------------------------------------------------

// SOVInput is one window of enriched mentions. Output is a ShareOfVoice.
type SOVInput struct {
	Enriched    []EnrichedMention `json:"enriched"`
	Profile     BrandProfile      `json:"profile"`
	WindowStart time.Time         `json:"window_start"`
	WindowEnd   time.Time         `json:"window_end"`
}

// ---------------------------------------------------------------------------
// bp-detector
// ---------------------------------------------------------------------------

// DetectInput is one window of enriched mentions measured against a baseline.
// Now is passed rather than read from the clock so a replayed run produces the
// same alerts it produced live.
type DetectInput struct {
	BrandID  string            `json:"brand_id"`
	Enriched []EnrichedMention `json:"enriched"`
	Baseline BaselineStats     `json:"baseline"`
	Now      time.Time         `json:"now"`
}

// BaselineStats is the 14-day history the detector compares the current hour
// against. Produced by stats.ComputeBaseline, which excludes the current hour
// so a spike cannot suppress itself.
type BaselineStats struct {
	PerSourceHourlyMean map[Source]float64 `json:"per_source_hourly_mean"`
	PerSourceHourlyStd  map[Source]float64 `json:"per_source_hourly_std"`
	NegativeShareMean   float64            `json:"negative_share_mean"`
	NegativeShareStd    float64            `json:"negative_share_std"`

	// MeanRating is nil when no review source ran in the window, which is a
	// different fact from an average rating of 0.0. It is context for the
	// brief and is not the operand the review_bomb rule compares: that rule
	// uses the observed hour's own mean.
	MeanRating *float64 `json:"mean_rating"`

	Days int `json:"days"`
}

// AlertSet is bp-detector's output. RulesEvaluated is how many of the five
// rules ran, so a UI can say "5 rules, 1 fired" instead of implying a model
// decided something.
type AlertSet struct {
	Alerts         []Alert  `json:"alerts"`
	RulesEvaluated int      `json:"rules_evaluated"`
	Errors         []string `json:"errors"`
}

// ---------------------------------------------------------------------------
// bp-responder
// ---------------------------------------------------------------------------

// RespondInput asks for one draft. Exactly one of Alert and Mention is
// non-nil; both nil or both set is an input error. The pointers mean "not this
// one", not "absent versus zero".
type RespondInput struct {
	Alert   *Alert       `json:"alert"`
	Mention *Mention     `json:"mention"`
	Profile BrandProfile `json:"profile"`
	Channel Channel      `json:"channel"`
}

// ---------------------------------------------------------------------------
// bp-briefer
// ---------------------------------------------------------------------------

// BriefInput carries everything the brief needs. The briefer does not query
// the database: the orchestrator hands it the topics, alerts and numbers, and
// the one LLM call writes prose around them rather than restating a number.
type BriefInput struct {
	BrandID string       `json:"brand_id"`
	Profile BrandProfile `json:"profile"`

	// Period is "daily" or "weekly".
	Period      string       `json:"period"`
	PeriodStart time.Time    `json:"period_start"`
	PeriodEnd   time.Time    `json:"period_end"`
	Topics      []Topic      `json:"topics"`
	Alerts      []Alert      `json:"alerts"`
	SOV         ShareOfVoice `json:"sov"`
	Numbers     BriefNumbers `json:"numbers"`
}

// ---------------------------------------------------------------------------
// bp-orchestrator
// ---------------------------------------------------------------------------

// RunInput triggers one pipeline execution. Output is a RunRecord.
type RunInput struct {
	BrandID string  `json:"brand_id"`
	Trigger RunKind `json:"trigger"`

	// WindowHours is how far back to collect. 0 means 24; the agent applies
	// the default.
	WindowHours int `json:"window_hours"`

	// Force re-runs a bucket that already has a runs row. Without it the
	// orchestrator returns the existing record unchanged.
	Force bool `json:"force"`
}
