// The five alert rules, and nothing else.
//
// Every rule is a pure function over models.DetectInput with the same
// signature, so Evaluate is a loop and a test calls one rule with no server, no
// context plumbing and no mocks.
//
// This file must not import brandpulse/internal/llm, now or ever. An alert that
// cannot explain itself statistically is worth nothing to a founder at 11pm,
// and "our AI decided" is not a defensible answer. TestDetectorImportsNoLLM
// enforces it.

package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"brandpulse/internal/ids"
	"brandpulse/internal/models"
	"brandpulse/internal/stats"
)

// Thresholds. Every number the rules compare against is declared here, so the
// evidence a rule emits and the constant it was compared against cannot drift
// apart, and so a tuning conversation is one screen rather than five greps.
const (
	spikeZ     = 3.0 // hourly volume z-score that fires spike
	spikeHighZ = 5.0 // ...and the z-score that raises it to high

	crisisZ             = 3.0 // crisis needs the spike z-score
	crisisNegativeShare = 0.6 // ...and this share of the hour negative
	crisisCriticalShare = 0.8 // ...and this share makes it critical

	reviewBombMinReviews    = 5   // rated mentions in one hour on a review source
	reviewBombMaxMeanRating = 2.0 // ...whose observed mean is at or below this

	influencerFollowers = 50000.0 // author_followers that make a mention an influencer's

	competitorZ = 3.0 // competitor mention volume z-score

	// maxSamples caps the mentions attached to an alert. The wire carries whole
	// mentions, not ids, so an unbounded slice would put a whole hour of a
	// crisis into a WhatsApp payload.
	maxSamples = 3

	// baselineWindow labels the window every evidence entry was measured over.
	baselineWindow = "1h"
)

// rule is the shape every one of the five shares.
type rule func(in models.DetectInput) []models.Alert

// rules is the whole detector. RulesEvaluated is its length, always, even when
// nothing fires: "we checked and found nothing" is a different statement from
// "we did not check".
var rules = []rule{
	ruleSpike,
	ruleCrisis,
	ruleReviewBomb,
	ruleInfluencerMention,
	ruleCompetitorMove,
}

// Evaluate runs every rule and collects what fired.
//
// An alert that fails Validate is dropped into Errors rather than returned: a
// malformed alert would fail the NOT NULL insert downstream, and losing one
// alert must not lose the other four rules' output.
func Evaluate(in models.DetectInput) models.AlertSet {
	out := models.AlertSet{
		Alerts:         []models.Alert{},
		RulesEvaluated: len(rules),
		Errors:         []string{},
	}
	for _, r := range rules {
		for _, alert := range r(in) {
			if err := alert.Validate(); err != nil {
				out.Errors = append(out.Errors, err.Error())
				continue
			}
			out.Alerts = append(out.Alerts, alert)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// The rules
// ---------------------------------------------------------------------------

// ruleSpike fires when an hour's volume on one source is at least spikeZ
// standard deviations above that source's 14-day hourly mean.
//
// It counts every mention in the hour, not only the ones about the brand,
// because BaselineStats.PerSourceHourlyMean is the raw per-source hourly count.
// Comparing a filtered numerator against an unfiltered baseline would fire on
// any brand whose keyword set got more precise.
func ruleSpike(in models.DetectInput) []models.Alert {
	var alerts []models.Alert
	for _, g := range groupBySourceHour(in.Enriched, nil) {
		z := hourlyZScore(in.Baseline, g)
		if z < spikeZ {
			continue
		}
		severity := models.SeverityMedium
		if z >= spikeHighZ {
			severity = models.SeverityHigh
		}

		alert := newAlert(in, models.AlertSpike, severity, g)
		alert.Title = fmt.Sprintf("Volume spike on %s", g.source)
		alert.Why = fmt.Sprintf(
			"%d mentions on %s in the hour from %s, a z-score of %.1f against a 14-day mean of %.1f per hour (fires at %.1f).",
			len(g.mentions), g.source, g.bucket, z, in.Baseline.PerSourceHourlyMean[g.source], spikeZ,
		)
		alert.Evidence = []models.AlertEvidence{{
			Metric:    "volume_zscore",
			Value:     round1(z),
			Threshold: spikeZ,
			Window:    baselineWindow,
			Detail: fmt.Sprintf("%d mentions against a mean of %.1f, std %.1f, over %d days",
				len(g.mentions), in.Baseline.PerSourceHourlyMean[g.source],
				in.Baseline.PerSourceHourlyStd[g.source], in.Baseline.Days),
		}}
		alerts = append(alerts, alert)
	}
	return alerts
}

// ruleCrisis fires when a spike and a negative majority land in the same hour
// on the same source. It is the alert the whole product is built around, so it
// carries all three numbers: the z-score, the negative share and the count.
func ruleCrisis(in models.DetectInput) []models.Alert {
	var alerts []models.Alert
	for _, g := range groupBySourceHour(in.Enriched, nil) {
		z := hourlyZScore(in.Baseline, g)
		share := negativeShare(g)
		if z < crisisZ || share < crisisNegativeShare {
			continue
		}
		severity := models.SeverityHigh
		if share >= crisisCriticalShare {
			severity = models.SeverityCritical
		}

		alert := newAlert(in, models.AlertCrisis, severity, g)
		alert.Title = fmt.Sprintf("Negative surge on %s", g.source)
		alert.Why = fmt.Sprintf(
			"Volume on %s hit z=%.1f (fires at %.1f) and %.0f%% of the %d mentions in the hour from %s were negative (fires at %.0f%%).",
			g.source, z, crisisZ, share*100, len(g.mentions), g.bucket, crisisNegativeShare*100,
		)
		alert.Evidence = []models.AlertEvidence{
			{
				Metric:    "volume_zscore",
				Value:     round1(z),
				Threshold: crisisZ,
				Window:    baselineWindow,
				Detail: fmt.Sprintf("mean %.1f, std %.1f, over %d days",
					in.Baseline.PerSourceHourlyMean[g.source],
					in.Baseline.PerSourceHourlyStd[g.source], in.Baseline.Days),
			},
			{
				Metric:    "negative_share",
				Value:     round2(share),
				Threshold: crisisNegativeShare,
				Window:    baselineWindow,
				Detail: fmt.Sprintf("14-day baseline share %.2f, std %.2f",
					in.Baseline.NegativeShareMean, in.Baseline.NegativeShareStd),
			},
			{
				Metric:    "mention_count",
				Value:     float64(len(g.mentions)),
				Threshold: in.Baseline.PerSourceHourlyMean[g.source],
				Window:    baselineWindow,
				Detail:    fmt.Sprintf("%d negative of %d", countNegative(g), len(g.mentions)),
			},
		}
		alerts = append(alerts, alert)
	}
	return alerts
}

// ruleReviewBomb fires on a review source whose ratings collapse inside one
// hour.
//
// The operand is the observed hour's mean, computed from the non-nil
// Mention.Rating values in that hour, per the ruling in CONTRACTS §2.
// BaselineStats.MeanRating is context and rides along in the evidence detail
// when it is set; it is a *float64 because "no review source ran in the
// baseline window" and "the average was genuinely 0.0" are different facts, and
// dereferencing it unguarded is the likeliest panic in this agent.
//
// A mention with no rating is not a review, so it counts towards neither the
// five-review threshold nor the mean. Treating a missing rating as zero would
// read as the worst possible review and fire on a quiet source.
func ruleReviewBomb(in models.DetectInput) []models.Alert {
	var alerts []models.Alert
	for _, g := range groupBySourceHour(in.Enriched, onlyReviewSources) {
		rated, mean := ratedMean(g)
		if rated < reviewBombMinReviews || mean > reviewBombMaxMeanRating {
			continue
		}

		alert := newAlert(in, models.AlertReviewBomb, models.SeverityHigh, g)
		alert.Title = fmt.Sprintf("Review bomb on %s", g.source)
		alert.Why = fmt.Sprintf(
			"%d reviews on %s in the hour from %s averaged %.1f stars (fires at %d reviews averaging %.1f or less).",
			rated, g.source, g.bucket, mean, reviewBombMinReviews, reviewBombMaxMeanRating,
		)
		alert.Evidence = []models.AlertEvidence{
			{
				Metric:    "review_count",
				Value:     float64(rated),
				Threshold: reviewBombMinReviews,
				Window:    baselineWindow,
				Detail:    fmt.Sprintf("%d of %d mentions in the hour carried a rating", rated, len(g.mentions)),
			},
			{
				Metric:    "mean_rating",
				Value:     round2(mean),
				Threshold: reviewBombMaxMeanRating,
				Window:    baselineWindow,
				Detail:    baselineRatingDetail(in.Baseline.MeanRating),
			},
		}
		alerts = append(alerts, alert)
	}
	return alerts
}

// ruleInfluencerMention fires on an hour containing at least one author above
// the follower threshold. It groups rather than firing per mention because
// DedupeKey is kind:source:hour, so three influencers in one hour are one alert
// with three samples, not three alerts that collapse to one on insert.
func ruleInfluencerMention(in models.DetectInput) []models.Alert {
	var alerts []models.Alert
	for _, g := range groupBySourceHour(in.Enriched, nil) {
		influencers := filter(g.mentions, func(em models.EnrichedMention) bool {
			return float64(em.Mention.AuthorFollowers) >= influencerFollowers
		})
		if len(influencers) == 0 {
			continue
		}

		peak := 0
		negative := 0
		for _, em := range influencers {
			if em.Mention.AuthorFollowers > peak {
				peak = em.Mention.AuthorFollowers
			}
			if em.Enrichment.SentimentLabel == models.SentimentNegative {
				negative++
			}
		}
		severity := models.SeverityMedium
		if negative > 0 {
			severity = models.SeverityHigh
		}

		alert := newAlert(in, models.AlertInfluencerMention, severity, group{
			source:   g.source,
			bucket:   g.bucket,
			at:       g.at,
			mentions: influencers,
		})
		alert.Title = fmt.Sprintf("High-follower author on %s", g.source)
		alert.Why = fmt.Sprintf(
			"%d author(s) with at least %.0f followers posted on %s in the hour from %s, the largest at %d followers; %d of them negative.",
			len(influencers), influencerFollowers, g.source, g.bucket, peak, negative,
		)
		alert.Evidence = []models.AlertEvidence{{
			Metric:    "author_followers",
			Value:     float64(peak),
			Threshold: influencerFollowers,
			Window:    baselineWindow,
			Detail:    fmt.Sprintf("%d high-follower author(s), %d negative", len(influencers), negative),
		}}
		alerts = append(alerts, alert)
	}
	return alerts
}

// ruleCompetitorMove fires when mentions about a competitor spike on a source.
// It is low severity on purpose: it is intelligence, not an incident.
//
// It scores against the same PerSourceHourlyMean as ruleSpike, because that is
// the only baseline in the contract. The consequence to know is that a source
// dominated by competitor chatter reaches the threshold on a smaller move than
// one where the brand is most of the volume.
func ruleCompetitorMove(in models.DetectInput) []models.Alert {
	var alerts []models.Alert
	for _, g := range groupBySourceHour(in.Enriched, aboutACompetitor) {
		z := hourlyZScore(in.Baseline, g)
		if z < competitorZ {
			continue
		}

		alert := newAlert(in, models.AlertCompetitorMove, models.SeverityLow, g)
		alert.Title = fmt.Sprintf("Competitor volume up on %s", g.source)
		alert.Why = fmt.Sprintf(
			"%d competitor mentions on %s in the hour from %s, a z-score of %.1f against that source's 14-day hourly mean of %.1f (fires at %.1f).",
			len(g.mentions), g.source, g.bucket, z, in.Baseline.PerSourceHourlyMean[g.source], competitorZ,
		)
		alert.Evidence = []models.AlertEvidence{{
			Metric:    "competitor_volume_zscore",
			Value:     round1(z),
			Threshold: competitorZ,
			Window:    baselineWindow,
			Detail:    fmt.Sprintf("competitors seen: %s", strings.Join(competitorNames(g), ", ")),
		}}
		alerts = append(alerts, alert)
	}
	return alerts
}

// ---------------------------------------------------------------------------
// Grouping and arithmetic the rules share
// ---------------------------------------------------------------------------

// group is one source's mentions inside one UTC hour, which is the unit every
// rule reasons about and the unit DedupeKey is built from.
type group struct {
	source   models.Source
	bucket   string
	at       time.Time
	mentions []models.EnrichedMention
}

// groupBySourceHour buckets mentions by source and UTC hour, optionally
// filtered, and returns the groups in a stable order.
//
// The sort is not cosmetic: Go randomises map iteration, so without it the same
// input produces the alerts in a different order on every run and the demo
// cannot be rehearsed.
func groupBySourceHour(enriched []models.EnrichedMention, keep func(models.EnrichedMention) bool) []group {
	byKey := map[string]*group{}
	for _, em := range enriched {
		if keep != nil && !keep(em) {
			continue
		}
		at := em.Mention.PostedAt.UTC().Truncate(time.Hour)
		key := string(em.Mention.Source) + "|" + stats.HourBucket(at)
		g, ok := byKey[key]
		if !ok {
			g = &group{source: em.Mention.Source, bucket: stats.HourBucket(at), at: at}
			byKey[key] = g
		}
		g.mentions = append(g.mentions, em)
	}

	out := make([]group, 0, len(byKey))
	for _, g := range byKey {
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].bucket != out[j].bucket {
			return out[i].bucket < out[j].bucket
		}
		return out[i].source < out[j].source
	})
	return out
}

func onlyReviewSources(em models.EnrichedMention) bool {
	return em.Mention.Source.IsReviewSource()
}

func aboutACompetitor(em models.EnrichedMention) bool {
	return em.Enrichment.AboutCompetitor != ""
}

// hourlyZScore scores this group's volume against its source's baseline.
// stats.ZScore returns 0.0 when std is zero, which is what keeps a quiet brand
// with no variance from spiking on its first two mentions.
func hourlyZScore(b models.BaselineStats, g group) float64 {
	return stats.ZScore(float64(len(g.mentions)), b.PerSourceHourlyMean[g.source], b.PerSourceHourlyStd[g.source])
}

func countNegative(g group) int {
	n := 0
	for _, em := range g.mentions {
		if em.Enrichment.SentimentLabel == models.SentimentNegative {
			n++
		}
	}
	return n
}

func negativeShare(g group) float64 {
	if len(g.mentions) == 0 {
		return 0
	}
	return float64(countNegative(g)) / float64(len(g.mentions))
}

// ratedMean returns how many of the group's mentions carry a rating and the
// mean of those. Nils are skipped, never counted as zero.
func ratedMean(g group) (int, float64) {
	sum, n := 0.0, 0
	for _, em := range g.mentions {
		if em.Mention.Rating == nil {
			continue
		}
		sum += *em.Mention.Rating
		n++
	}
	if n == 0 {
		return 0, 0
	}
	return n, sum / float64(n)
}

// baselineRatingDetail reads the one pointer in the contract without
// dereferencing a nil. Nil means no review source ran in the baseline window,
// which is a fact worth printing rather than a zero worth inventing.
func baselineRatingDetail(meanRating *float64) string {
	if meanRating == nil {
		return "no review source in the 14-day baseline window"
	}
	return fmt.Sprintf("14-day baseline mean rating %.2f", *meanRating)
}

func competitorNames(g group) []string {
	seen := map[string]bool{}
	var out []string
	for _, em := range g.mentions {
		name := em.Enrichment.AboutCompetitor
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func filter(ems []models.EnrichedMention, keep func(models.EnrichedMention) bool) []models.EnrichedMention {
	var out []models.EnrichedMention
	for _, em := range ems {
		if keep(em) {
			out = append(out, em)
		}
	}
	return out
}

// newAlert builds the parts every rule fills in identically: the id, the
// dedupe key, the samples and the timestamp. Evidence, Title and Why are the
// rule's own and are left empty here, which Alert.Validate then enforces.
func newAlert(in models.DetectInput, kind models.AlertKind, severity models.Severity, g group) models.Alert {
	dedupeKey := fmt.Sprintf("%s:%s:%s", kind, g.source, g.bucket)
	alert := models.NewAlert(ids.New("alr"), in.BrandID, kind, severity, dedupeKey, in.Now)
	alert.SampleMentions = samples(g)
	return alert
}

// samples picks the most engaged mentions in the group, capped, so an alert
// shows the loudest posts rather than whichever arrived first.
func samples(g group) []models.Mention {
	ranked := make([]models.EnrichedMention, len(g.mentions))
	copy(ranked, g.mentions)
	sort.SliceStable(ranked, func(i, j int) bool {
		return ranked[i].Mention.Engagement.Total() > ranked[j].Mention.Engagement.Total()
	})

	n := len(ranked)
	if n > maxSamples {
		n = maxSamples
	}
	out := make([]models.Mention, 0, n)
	for _, em := range ranked[:n] {
		out = append(out, em.Mention)
	}
	return out
}

func round1(v float64) float64 { return float64(int(v*10+0.5)) / 10 }
func round2(v float64) float64 { return float64(int(v*100+0.5)) / 100 }
