// This file is the per-agent request and response envelopes from
// docs/CONTRACTS.md §2. It belongs to B1 (their Task 1) and is transcribed here
// verbatim so track B4 has something to compile against; drop it in favour of
// B1's version the moment that lands.
//
// Field names are the json tags. Do not tidy one: DronaHQ and the Fastify BFF
// bind to these strings.

package models

import "time"

// ---------------------------------------------------------------------------
// bp-onboarder
// ---------------------------------------------------------------------------

// OnboardInput asks for a keyword set built from a brand's own website.
type OnboardInput struct {
	BrandID     string   `json:"brand_id"`
	Name        string   `json:"name"`
	Website     string   `json:"website,omitempty"`
	Competitors []string `json:"competitors"`
	MaxPages    int      `json:"max_pages"` // 0 means 25; the agent applies the default
}

// ---------------------------------------------------------------------------
// bp-collector
// ---------------------------------------------------------------------------

// CollectInput is one source for one window. The orchestrator fans out; the
// collector never decides how many sources run.
type CollectInput struct {
	Profile     BrandProfile `json:"profile"`
	Source      Source       `json:"source"`
	WindowStart time.Time    `json:"window_start"`
	WindowEnd   time.Time    `json:"window_end"`
	MaxCredits  int          `json:"max_credits"`
	RunID       string       `json:"run_id"`
}

// MentionBatch is one source's harvest. Truncated means the credit ceiling was
// reached, which is a stop, not a failure.
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

// EnrichInput carries the mentions to classify. BatchSize 0 means 50.
type EnrichInput struct {
	Mentions  []Mention    `json:"mentions"`
	Profile   BrandProfile `json:"profile"`
	BatchSize int          `json:"batch_size"` // 0 means 50
}

// EnrichmentBatch holds classifier output. Order is not guaranteed; join on
// MentionID.
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

// ClusterInput carries one window of enriched mentions plus the prior window's
// per-label counts, which is what Topic.Trend is computed against.
type ClusterInput struct {
	Enriched          []EnrichedMention `json:"enriched"`
	BrandID           string            `json:"brand_id"`
	WindowStart       time.Time         `json:"window_start"`
	WindowEnd         time.Time         `json:"window_end"`
	PriorWindowCounts map[string]int    `json:"prior_window_counts"`
	MinClusterSize    int               `json:"min_cluster_size"` // 0 means 3
}

// TopicSet is the clusterer's output. Unclustered holds mention ids that fell
// into noise.
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

// SOVInput is pure counting input: no LLM, no network.
type SOVInput struct {
	Enriched    []EnrichedMention `json:"enriched"`
	Profile     BrandProfile      `json:"profile"`
	WindowStart time.Time         `json:"window_start"`
	WindowEnd   time.Time         `json:"window_end"`
}

// ---------------------------------------------------------------------------
// bp-detector
// ---------------------------------------------------------------------------

// DetectInput is everything the five rules need. There is no database handle
// here on purpose: the rules are pure functions over this struct.
type DetectInput struct {
	BrandID  string            `json:"brand_id"`
	Enriched []EnrichedMention `json:"enriched"`
	Baseline BaselineStats     `json:"baseline"`
	Now      time.Time         `json:"now"`
}

// BaselineStats is the 14-day behaviour the current window is compared against.
//
// MeanRating is a pointer because "no review source ran in the window" and
// "the average rating was 0.0" are different facts. It is context for the
// brief, not the operand review_bomb compares: that rule uses the observed
// hour's mean, computed from the non-nil Mention.Rating values in that hour.
type BaselineStats struct {
	PerSourceHourlyMean map[Source]float64 `json:"per_source_hourly_mean"`
	PerSourceHourlyStd  map[Source]float64 `json:"per_source_hourly_std"`
	NegativeShareMean   float64            `json:"negative_share_mean"`
	NegativeShareStd    float64            `json:"negative_share_std"`
	MeanRating          *float64           `json:"mean_rating"` // nil = no review source in the window
	Days                int                `json:"days"`
}

// AlertSet is the detector's output. RulesEvaluated is 5, always, even when
// nothing fires: "we checked and found nothing" is a different statement from
// "we did not check".
type AlertSet struct {
	Alerts         []Alert  `json:"alerts"`
	RulesEvaluated int      `json:"rules_evaluated"`
	Errors         []string `json:"errors"`
}

// ---------------------------------------------------------------------------
// bp-responder
// ---------------------------------------------------------------------------

// RespondInput drafts a reply to exactly one of Alert or Mention. Both nil or
// both set is an input error.
type RespondInput struct {
	Alert   *Alert       `json:"alert"`
	Mention *Mention     `json:"mention"`
	Profile BrandProfile `json:"profile"`
	Channel Channel      `json:"channel"`
}

// ---------------------------------------------------------------------------
// bp-briefer
// ---------------------------------------------------------------------------

// BriefInput carries everything the brief needs. The briefer does not query the
// database; the orchestrator hands it every number.
type BriefInput struct {
	BrandID     string       `json:"brand_id"`
	Profile     BrandProfile `json:"profile"`
	Period      string       `json:"period"` // "daily" | "weekly"
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

// RunInput triggers one pipeline run. WindowHours 0 means 24 and the agent
// applies that default; there is no *int in this contract.
type RunInput struct {
	BrandID     string  `json:"brand_id"`
	Trigger     RunKind `json:"trigger"`
	WindowHours int     `json:"window_hours"` // 0 means 24
	Force       bool    `json:"force"`
}
