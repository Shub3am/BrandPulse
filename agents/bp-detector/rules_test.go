// One table per rule, and the rows that matter most are the ones that must not
// fire. A detector that alerts on everything is the same product as no
// detector, so every table pins a near miss a hair under the threshold.

package main

import (
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"sort"
	"strings"
	"testing"
	"time"

	"brandpulse/internal/models"
)

// testHour is a fixed UTC hour. Every fixture hangs off it so dedupe keys are
// literal strings a test can assert on.
var testHour = time.Date(2026, 9, 20, 14, 0, 0, 0, time.UTC)

func ptr(v float64) *float64 { return &v }

// mentionSpec is the shorthand a table row builds its window from. Anything
// left zero is filled in by buildMentions with a value that does not fire a
// rule, so a row names only the field it is about.
type mentionSpec struct {
	source     models.Source
	hourOffset int
	label      models.SentimentLabel
	rating     *float64
	followers  int
	competitor string
	likes      int
}

// repeat is how a table row says "eighteen of these", which is what a volume
// rule is actually about.
func repeat(n int, spec mentionSpec) []mentionSpec {
	out := make([]mentionSpec, n)
	for i := range out {
		out[i] = spec
	}
	return out
}

func join(groups ...[]mentionSpec) []mentionSpec {
	var out []mentionSpec
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}

func buildMentions(specs []mentionSpec) []models.EnrichedMention {
	out := make([]models.EnrichedMention, 0, len(specs))
	for i, spec := range specs {
		if spec.label == "" {
			spec.label = models.SentimentNeutral
		}
		id := fmt.Sprintf("mnt_test_%03d", i)
		out = append(out, models.EnrichedMention{
			Mention: models.Mention{
				ID:              id,
				BrandID:         "brd_test",
				Source:          spec.source,
				ExternalID:      id,
				Text:            "fixture mention " + id,
				Lang:            "en",
				AuthorFollowers: spec.followers,
				Rating:          spec.rating,
				// Minutes wrap inside the hour so every mention in a spec lands
				// in the same bucket however many there are.
				PostedAt:   testHour.Add(time.Duration(spec.hourOffset)*time.Hour + time.Duration(i%60)*time.Minute),
				Engagement: models.Engagement{Likes: spec.likes},
			},
			Enrichment: models.Enrichment{
				MentionID:       id,
				SentimentLabel:  spec.label,
				IsAboutBrand:    spec.competitor == "",
				AboutCompetitor: spec.competitor,
			},
		})
	}
	return out
}

// baseline builds a BaselineStats where the given sources all have the given
// mean and std. MeanRating stays nil unless a row sets it, because nil is the
// value that panics an unguarded rule.
func baseline(mean, std float64, sources ...models.Source) models.BaselineStats {
	stats := models.BaselineStats{
		PerSourceHourlyMean: map[models.Source]float64{},
		PerSourceHourlyStd:  map[models.Source]float64{},
		Days:                14,
	}
	for _, source := range sources {
		stats.PerSourceHourlyMean[source] = mean
		stats.PerSourceHourlyStd[source] = std
	}
	return stats
}

// crisisMetrics is what a crisis alert has to carry. The brief's demo line
// reads "volume z=4.1, negative share 78%", so all three terms are on the
// alert, not just the one that tipped it.
var crisisMetrics = []string{"volume_zscore", "negative_share", "mention_count"}

func detectInput(b models.BaselineStats, specs []mentionSpec) models.DetectInput {
	return models.DetectInput{
		BrandID:  "brd_test",
		Enriched: buildMentions(specs),
		Baseline: b,
		Now:      testHour.Add(time.Hour),
	}
}

// ---------------------------------------------------------------------------
// spike
// ---------------------------------------------------------------------------

func TestRuleSpike(t *testing.T) {
	tests := []struct {
		name         string
		baseline     models.BaselineStats
		specs        []mentionSpec
		wantAlerts   int
		wantSeverity models.Severity
		wantZ        float64
	}{
		{
			name:         "z exactly 3.0 fires medium",
			baseline:     baseline(10, 2, models.SourceX),
			specs:        repeat(16, mentionSpec{source: models.SourceX}),
			wantAlerts:   1,
			wantSeverity: models.SeverityMedium,
			wantZ:        3.0,
		},
		{
			name:         "z exactly 5.0 raises it to high",
			baseline:     baseline(10, 2, models.SourceX),
			specs:        repeat(20, mentionSpec{source: models.SourceX}),
			wantAlerts:   1,
			wantSeverity: models.SeverityHigh,
			wantZ:        5.0,
		},
		{
			name:       "near miss at z 2.9 does not fire",
			baseline:   baseline(10.2, 2, models.SourceX),
			specs:      repeat(16, mentionSpec{source: models.SourceX}),
			wantAlerts: 0,
		},
		{
			name: "quiet brand with std 0 does not fire",
			// stats.ZScore returns 0.0 rather than +Inf here. Without that
			// guard this row is a silent false alert, not a crash.
			baseline:   baseline(0, 0, models.SourceReddit),
			specs:      repeat(12, mentionSpec{source: models.SourceReddit}),
			wantAlerts: 0,
		},
		{
			name:       "the same volume split across two hours does not fire",
			baseline:   baseline(10, 2, models.SourceX),
			specs:      join(repeat(8, mentionSpec{source: models.SourceX}), repeat(8, mentionSpec{source: models.SourceX, hourOffset: 1})),
			wantAlerts: 0,
		},
		{
			name:       "two sources spiking produce one alert each",
			baseline:   baseline(10, 2, models.SourceX, models.SourceReddit),
			wantAlerts: 2,
			specs:      join(repeat(16, mentionSpec{source: models.SourceX}), repeat(16, mentionSpec{source: models.SourceReddit})),
		},
		{
			name:       "a source absent from the baseline does not fire",
			baseline:   baseline(10, 2, models.SourceX),
			specs:      repeat(16, mentionSpec{source: models.SourceYoutube}),
			wantAlerts: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			alerts := ruleSpike(detectInput(tc.baseline, tc.specs))
			if len(alerts) != tc.wantAlerts {
				t.Fatalf("got %d alerts, want %d: %v", len(alerts), tc.wantAlerts, whys(alerts))
			}
			if tc.wantAlerts != 1 {
				return
			}
			if alerts[0].Severity != tc.wantSeverity {
				t.Errorf("severity = %q, want %q", alerts[0].Severity, tc.wantSeverity)
			}
			if got := evidence(t, alerts[0], "volume_zscore"); got.Value != tc.wantZ {
				t.Errorf("volume_zscore value = %v, want %v", got.Value, tc.wantZ)
			} else if got.Threshold != spikeZ {
				t.Errorf("volume_zscore threshold = %v, want %v", got.Threshold, spikeZ)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// crisis
// ---------------------------------------------------------------------------

func TestRuleCrisis(t *testing.T) {
	negative := mentionSpec{source: models.SourceX, label: models.SentimentNegative}
	neutral := mentionSpec{source: models.SourceX}

	tests := []struct {
		name         string
		baseline     models.BaselineStats
		specs        []mentionSpec
		wantAlerts   int
		wantSeverity models.Severity
		wantShare    float64
	}{
		{
			name:         "z 4.0 with negative share exactly 0.6 fires high",
			baseline:     baseline(10, 2.5, models.SourceX),
			specs:        join(repeat(12, negative), repeat(8, neutral)),
			wantAlerts:   1,
			wantSeverity: models.SeverityHigh,
			wantShare:    0.6,
		},
		{
			name:         "negative share 0.8 is critical",
			baseline:     baseline(10, 2.5, models.SourceX),
			specs:        join(repeat(16, negative), repeat(4, neutral)),
			wantAlerts:   1,
			wantSeverity: models.SeverityCritical,
			wantShare:    0.8,
		},
		{
			name:       "near miss at negative share 0.59 does not fire",
			baseline:   baseline(40, 20, models.SourceX),
			specs:      join(repeat(59, negative), repeat(41, neutral)),
			wantAlerts: 0,
		},
		{
			name:       "near miss at z 2.9 does not fire however negative the hour is",
			baseline:   baseline(10.2, 2, models.SourceX),
			specs:      repeat(16, negative),
			wantAlerts: 0,
		},
		{
			name:       "a spike of happy customers is not a crisis",
			baseline:   baseline(10, 2.5, models.SourceX),
			specs:      join(repeat(2, negative), repeat(18, mentionSpec{source: models.SourceX, label: models.SentimentPositive})),
			wantAlerts: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			alerts := ruleCrisis(detectInput(tc.baseline, tc.specs))
			if len(alerts) != tc.wantAlerts {
				t.Fatalf("got %d alerts, want %d: %v", len(alerts), tc.wantAlerts, whys(alerts))
			}
			if tc.wantAlerts != 1 {
				return
			}
			if alerts[0].Severity != tc.wantSeverity {
				t.Errorf("severity = %q, want %q", alerts[0].Severity, tc.wantSeverity)
			}
			if got := evidence(t, alerts[0], "negative_share"); got.Value != tc.wantShare {
				t.Errorf("negative_share value = %v, want %v", got.Value, tc.wantShare)
			}
			for _, metric := range crisisMetrics {
				evidence(t, alerts[0], metric)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// review_bomb
// ---------------------------------------------------------------------------

func TestRuleReviewBomb(t *testing.T) {
	oneStar := mentionSpec{source: models.SourceAmazon, rating: ptr(1.0), label: models.SentimentNegative}

	tests := []struct {
		name        string
		meanRating  *float64
		specs       []mentionSpec
		wantAlerts  int
		wantMean    float64
		wantReviews float64
	}{
		{
			// nil means no review source ran in the baseline window. The rule
			// compares the observed hour, so it fires, and the point of the row
			// is that reading the pointer does not panic.
			name:        "five one-star reviews fire with a nil baseline MeanRating",
			meanRating:  nil,
			specs:       repeat(5, oneStar),
			wantAlerts:  1,
			wantMean:    1.0,
			wantReviews: 5,
		},
		{
			name:        "five one-star reviews fire with a baseline MeanRating of 0.0",
			meanRating:  ptr(0.0),
			specs:       repeat(5, oneStar),
			wantAlerts:  1,
			wantMean:    1.0,
			wantReviews: 5,
		},
		{
			name:       "four reviews is a near miss",
			specs:      repeat(4, oneStar),
			wantAlerts: 0,
		},
		{
			name: "five reviews averaging exactly 2.0 fire",
			specs: join(
				repeat(2, mentionSpec{source: models.SourceAmazon, rating: ptr(1.0)}),
				repeat(1, mentionSpec{source: models.SourceAmazon, rating: ptr(2.0)}),
				repeat(2, mentionSpec{source: models.SourceAmazon, rating: ptr(3.0)}),
			),
			wantAlerts:  1,
			wantMean:    2.0,
			wantReviews: 5,
		},
		{
			name: "five reviews averaging 2.1 do not fire",
			specs: join(
				repeat(4, mentionSpec{source: models.SourceAmazon, rating: ptr(2.0)}),
				repeat(1, mentionSpec{source: models.SourceAmazon, rating: ptr(2.5)}),
			),
			wantAlerts: 0,
		},
		{
			// A missing rating is not a zero-star review. Counting nils would
			// both pad the count to five and drag the mean to the floor.
			name:       "unrated mentions neither pad the count nor drag the mean",
			specs:      join(repeat(4, oneStar), repeat(3, mentionSpec{source: models.SourceAmazon, label: models.SentimentNegative})),
			wantAlerts: 0,
		},
		{
			name:       "a non-review source never fires however bad the ratings",
			specs:      repeat(6, mentionSpec{source: models.SourceX, rating: ptr(1.0)}),
			wantAlerts: 0,
		},
		{
			name:        "playstore is a review source too",
			specs:       repeat(5, mentionSpec{source: models.SourcePlaystore, rating: ptr(1.5)}),
			wantAlerts:  1,
			wantMean:    1.5,
			wantReviews: 5,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := baseline(10, 2, models.SourceAmazon)
			b.MeanRating = tc.meanRating

			alerts := ruleReviewBomb(detectInput(b, tc.specs))
			if len(alerts) != tc.wantAlerts {
				t.Fatalf("got %d alerts, want %d: %v", len(alerts), tc.wantAlerts, whys(alerts))
			}
			if tc.wantAlerts != 1 {
				return
			}
			if alerts[0].Severity != models.SeverityHigh {
				t.Errorf("severity = %q, want high", alerts[0].Severity)
			}
			if got := evidence(t, alerts[0], "mean_rating"); got.Value != tc.wantMean {
				t.Errorf("mean_rating value = %v, want %v", got.Value, tc.wantMean)
			}
			if got := evidence(t, alerts[0], "review_count"); got.Value != tc.wantReviews {
				t.Errorf("review_count value = %v, want %v", got.Value, tc.wantReviews)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// influencer_mention
// ---------------------------------------------------------------------------

func TestRuleInfluencerMention(t *testing.T) {
	tests := []struct {
		name         string
		specs        []mentionSpec
		wantAlerts   int
		wantSeverity models.Severity
		wantPeak     float64
		wantSamples  int
	}{
		{
			name:         "exactly 50000 followers fires medium",
			specs:        []mentionSpec{{source: models.SourceX, followers: 50000}},
			wantAlerts:   1,
			wantSeverity: models.SeverityMedium,
			wantPeak:     50000,
			wantSamples:  1,
		},
		{
			name:       "near miss at 49999 followers does not fire",
			specs:      []mentionSpec{{source: models.SourceX, followers: 49999}},
			wantAlerts: 0,
		},
		{
			name:         "a negative influencer is high",
			specs:        []mentionSpec{{source: models.SourceX, followers: 80000, label: models.SentimentNegative}},
			wantAlerts:   1,
			wantSeverity: models.SeverityHigh,
			wantPeak:     80000,
			wantSamples:  1,
		},
		{
			// DedupeKey is kind:source:hour, so three influencers in one hour
			// have to arrive as one alert with three samples. Three alerts
			// would collapse to one on insert and lose two of them.
			name: "three influencers in one hour are one alert with three samples",
			specs: []mentionSpec{
				{source: models.SourceX, followers: 60000, likes: 10},
				{source: models.SourceX, followers: 120000, likes: 30},
				{source: models.SourceX, followers: 55000, likes: 20},
			},
			wantAlerts:   1,
			wantSeverity: models.SeverityMedium,
			wantPeak:     120000,
			wantSamples:  3,
		},
		{
			name:       "a crowd of ordinary authors does not fire",
			specs:      repeat(40, mentionSpec{source: models.SourceX, followers: 300}),
			wantAlerts: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			alerts := ruleInfluencerMention(detectInput(baseline(10, 2, models.SourceX), tc.specs))
			if len(alerts) != tc.wantAlerts {
				t.Fatalf("got %d alerts, want %d: %v", len(alerts), tc.wantAlerts, whys(alerts))
			}
			if tc.wantAlerts != 1 {
				return
			}
			if alerts[0].Severity != tc.wantSeverity {
				t.Errorf("severity = %q, want %q", alerts[0].Severity, tc.wantSeverity)
			}
			if got := evidence(t, alerts[0], "author_followers"); got.Value != tc.wantPeak {
				t.Errorf("author_followers value = %v, want %v", got.Value, tc.wantPeak)
			}
			if len(alerts[0].SampleMentions) != tc.wantSamples {
				t.Errorf("got %d sample mentions, want %d", len(alerts[0].SampleMentions), tc.wantSamples)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// competitor_move
// ---------------------------------------------------------------------------

func TestRuleCompetitorMove(t *testing.T) {
	rival := mentionSpec{source: models.SourceX, competitor: "RivalCo"}

	tests := []struct {
		name       string
		baseline   models.BaselineStats
		specs      []mentionSpec
		wantAlerts int
		wantZ      float64
	}{
		{
			name:       "competitor volume at z 3.0 fires low",
			baseline:   baseline(10, 2, models.SourceX),
			specs:      repeat(16, rival),
			wantAlerts: 1,
			wantZ:      3.0,
		},
		{
			name:       "near miss at z 2.9 does not fire",
			baseline:   baseline(10.2, 2, models.SourceX),
			specs:      repeat(16, rival),
			wantAlerts: 0,
		},
		{
			name:       "brand mentions are not competitor volume",
			baseline:   baseline(10, 2, models.SourceX),
			specs:      repeat(16, mentionSpec{source: models.SourceX}),
			wantAlerts: 0,
		},
		{
			name:       "competitor chatter under the threshold does not fire",
			baseline:   baseline(10, 2, models.SourceX),
			specs:      join(repeat(12, rival), repeat(20, mentionSpec{source: models.SourceX})),
			wantAlerts: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			alerts := ruleCompetitorMove(detectInput(tc.baseline, tc.specs))
			if len(alerts) != tc.wantAlerts {
				t.Fatalf("got %d alerts, want %d: %v", len(alerts), tc.wantAlerts, whys(alerts))
			}
			if tc.wantAlerts != 1 {
				return
			}
			if alerts[0].Severity != models.SeverityLow {
				t.Errorf("severity = %q, want low", alerts[0].Severity)
			}
			if got := evidence(t, alerts[0], "competitor_volume_zscore"); got.Value != tc.wantZ {
				t.Errorf("competitor_volume_zscore value = %v, want %v", got.Value, tc.wantZ)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Evaluate
// ---------------------------------------------------------------------------

func TestEvaluateAlwaysReportsFiveRules(t *testing.T) {
	quiet := Evaluate(detectInput(baseline(10, 2, models.SourceX), nil))

	if quiet.RulesEvaluated != 5 {
		t.Errorf("RulesEvaluated = %d, want 5; checking and finding nothing is not the same as not checking", quiet.RulesEvaluated)
	}
	if len(quiet.Alerts) != 0 {
		t.Errorf("an empty window produced %d alerts: %v", len(quiet.Alerts), whys(quiet.Alerts))
	}
	if len(quiet.Errors) != 0 {
		t.Errorf("an empty window produced errors: %v", quiet.Errors)
	}
}

// crisisWindow is the demo fixture: a negative surge on X carrying three
// high-follower authors, which is the moment the product is sold on.
func crisisWindow() models.DetectInput {
	negative := mentionSpec{source: models.SourceX, label: models.SentimentNegative, likes: 5}
	return detectInput(
		baseline(10, 2.5, models.SourceX),
		join(
			repeat(14, negative),
			repeat(3, mentionSpec{source: models.SourceX, label: models.SentimentNegative, followers: 90000, likes: 400}),
			repeat(3, mentionSpec{source: models.SourceX}),
		),
	)
}

func TestEvaluateOnACrisisWindow(t *testing.T) {
	got := Evaluate(crisisWindow())

	if len(got.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", got.Errors)
	}

	wantKinds := map[models.AlertKind]models.Severity{
		// z is 4.0 here, which is a medium spike: high starts at 5.0. The
		// crisis rule is what makes this window critical, and that is the
		// point of having both rules rather than one.
		models.AlertSpike:             models.SeverityMedium,
		models.AlertCrisis:            models.SeverityCritical,
		models.AlertInfluencerMention: models.SeverityHigh,
	}
	if len(got.Alerts) != len(wantKinds) {
		t.Fatalf("got %d alerts, want %d: %v", len(got.Alerts), len(wantKinds), whys(got.Alerts))
	}
	for _, alert := range got.Alerts {
		want, ok := wantKinds[alert.Kind]
		if !ok {
			t.Errorf("unexpected alert kind %q", alert.Kind)
			continue
		}
		if alert.Severity != want {
			t.Errorf("%s severity = %q, want %q", alert.Kind, alert.Severity, want)
		}
		if err := alert.Validate(); err != nil {
			t.Errorf("%s did not validate: %v", alert.Kind, err)
		}
		if len(alert.SampleMentions) > maxSamples {
			t.Errorf("%s carries %d samples, cap is %d", alert.Kind, len(alert.SampleMentions), maxSamples)
		}
	}

	t.Log(alertJSON(t, got))
}

// TestEveryAlertShowsItsNumbers is the rule the whole agent exists for: a
// founder reading an alert at 11pm has to see the arithmetic, not a claim.
func TestEveryAlertShowsItsNumbers(t *testing.T) {
	for _, alert := range Evaluate(crisisWindow()).Alerts {
		if alert.Why == "" || alert.Title == "" {
			t.Errorf("%s has no title or why", alert.Kind)
		}
		if len(alert.Evidence) == 0 {
			t.Fatalf("%s carries no evidence", alert.Kind)
		}
		for _, e := range alert.Evidence {
			if e.Metric == "" {
				t.Errorf("%s has an evidence entry with no metric", alert.Kind)
			}
			if e.Window == "" {
				t.Errorf("%s evidence %q has no window", alert.Kind, e.Metric)
			}
			// The threshold has to be the constant the rule compared against,
			// so it must appear in the alert's own sentence.
			if !strings.Contains(alert.Why, fmt.Sprintf("%v", e.Value)) &&
				!strings.Contains(alert.Why, fmt.Sprintf("%.0f", e.Value)) &&
				!strings.Contains(alert.Why, fmt.Sprintf("%.1f", e.Value)) &&
				!strings.Contains(alert.Why, fmt.Sprintf("%.0f%%", e.Value*100)) {
				t.Errorf("%s evidence %q value %v does not appear in Why: %q", alert.Kind, e.Metric, e.Value, alert.Why)
			}
		}
	}
}

// TestDedupeKeysAreStableAcrossRuns is the idempotency guarantee: a sustained
// crisis re-detected every ten minutes must produce one alert per hour, not one
// per run. The unique index is on (brand_id, dedupe_key), so a second run
// producing a new key is a duplicate row in the UI.
func TestDedupeKeysAreStableAcrossRuns(t *testing.T) {
	in := crisisWindow()
	first := dedupeKeys(Evaluate(in).Alerts)
	second := dedupeKeys(Evaluate(in).Alerts)

	if len(first) == 0 {
		t.Fatal("the crisis window produced no alerts to dedupe")
	}
	for key := range second {
		if !first[key] {
			t.Errorf("the second run invented a new dedupe key: %q", key)
		}
	}
	if len(second) != len(first) {
		t.Errorf("run 1 produced %d keys, run 2 produced %d", len(first), len(second))
	}

	want := fmt.Sprintf("crisis:x:%s", testHour.Format("2006-01-02T15"))
	if !first[want] {
		t.Errorf("want dedupe key %q, got %v", want, keyList(first))
	}
}

// TestDedupeKeyIsPerHourNotPerRun pins the other half: the same rule firing in
// two different hours must produce two keys.
func TestDedupeKeyIsPerHourNotPerRun(t *testing.T) {
	in := detectInput(baseline(10, 2, models.SourceX), join(
		repeat(16, mentionSpec{source: models.SourceX}),
		repeat(16, mentionSpec{source: models.SourceX, hourOffset: 3}),
	))

	keys := dedupeKeys(ruleSpike(in))
	if len(keys) != 2 {
		t.Fatalf("got %d dedupe keys, want 2: %v", len(keys), keyList(keys))
	}
	for _, hour := range []time.Time{testHour, testHour.Add(3 * time.Hour)} {
		want := fmt.Sprintf("spike:x:%s", hour.Format("2006-01-02T15"))
		if !keys[want] {
			t.Errorf("missing dedupe key %q, got %v", want, keyList(keys))
		}
	}
}

// TestAlertOrderIsDeterministic guards the demo. Go randomises map iteration,
// so grouping without a sort gives a different alert order on every run and the
// crisis slide cannot be rehearsed.
func TestAlertOrderIsDeterministic(t *testing.T) {
	in := detectInput(
		baseline(10, 2, models.SourceX, models.SourceReddit, models.SourceYoutube),
		join(
			repeat(16, mentionSpec{source: models.SourceYoutube}),
			repeat(16, mentionSpec{source: models.SourceX}),
			repeat(16, mentionSpec{source: models.SourceReddit, hourOffset: 1}),
		),
	)

	want := keyList(dedupeKeys(Evaluate(in).Alerts))
	for run := 0; run < 20; run++ {
		got := keyList(dedupeKeys(Evaluate(in).Alerts))
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Fatalf("run %d produced %v, run 0 produced %v", run, got, want)
		}
	}
}

// TestDetectorImportsNoLLM is the belt-and-braces check the brief asks for.
// "No LLM in bp-detector" is a repo-wide rule, and a rule nothing enforces is a
// rule that lasts until the first person in a hurry.
//
// It parses the import block rather than grepping the source, because the files
// discuss the rule in their comments and a substring match fails on its own
// prose.
func TestDetectorImportsNoLLM(t *testing.T) {
	pkg, err := parser.ParseDir(token.NewFileSet(), ".", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parsing the agent directory: %v", err)
	}
	for _, p := range pkg {
		for name, file := range p.Files {
			for _, imported := range file.Imports {
				if strings.Trim(imported.Path.Value, `"`) == "brandpulse/internal/llm" {
					t.Errorf("%s imports internal/llm; bp-detector is statistics and must stay that way", name)
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// evidence fetches one metric off an alert and fails the test when it is
// missing, so a caller can assert on the value in one line.
func evidence(t *testing.T, alert models.Alert, metric string) models.AlertEvidence {
	t.Helper()
	for _, e := range alert.Evidence {
		if e.Metric == metric {
			return e
		}
	}
	t.Fatalf("alert %s carries no %q evidence, only %v", alert.Kind, metric, metrics(alert))
	return models.AlertEvidence{}
}

func metrics(alert models.Alert) []string {
	out := make([]string, 0, len(alert.Evidence))
	for _, e := range alert.Evidence {
		out = append(out, e.Metric)
	}
	return out
}

// whys renders alerts for a failure message. A count mismatch is unreadable
// without seeing which rules actually fired.
func whys(alerts []models.Alert) []string {
	out := make([]string, 0, len(alerts))
	for _, a := range alerts {
		out = append(out, string(a.Kind)+": "+a.Why)
	}
	return out
}

func dedupeKeys(alerts []models.Alert) map[string]bool {
	out := map[string]bool{}
	for _, a := range alerts {
		out[a.DedupeKey] = true
	}
	return out
}

func keyList(keys map[string]bool) []string {
	out := make([]string, 0, len(keys))
	for k := range keys {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// alertJSON prints what the demo's crisis alert actually looks like on the
// wire, which the PR has to carry.
func alertJSON(t *testing.T, set models.AlertSet) string {
	t.Helper()
	body, err := json.MarshalIndent(set, "", "  ")
	if err != nil {
		t.Fatalf("marshalling the alert set: %v", err)
	}
	return string(body)
}
