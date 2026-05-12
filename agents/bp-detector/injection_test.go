// The demo injection has to fire the real rules.
//
// What lives here, rather than under demo/, is every claim that couples the
// corpus to the rules, because only this package can see the unexported
// thresholds and the handler. A claim about the corpus alone is tested beside
// the corpus, in demo/crisis.
//
// Special-casing the detector to make a demo work would make every alert on
// stage worthless, so the demo is proved against the rules instead.
//
// It needs no database and no network.

package main

import (
	"context"
	"slices"
	"testing"
	"time"

	"brandpulse/demo/crisis"
	"brandpulse/internal/models"
)

// injectionNow is fixed so the corpus, the buckets and the alert ids that
// depend on them are the same on every run.
var injectionNow = time.Date(2026, 9, 20, 14, 40, 0, 0, time.UTC)

// assumedBaseline is what the demo assumes the seeded corpus looks like: a few
// mentions an hour per source, mostly not negative. It is the detector's half
// of the claim, which is why it is here and not in demo/crisis: forty mentions
// in ten minutes is only a crisis relative to a brand that normally sees four
// an hour.
//
// On stage the real baseline comes from stats.ComputeBaseline over the seeded
// corpus. That it matches these numbers is not checked anywhere yet, because
// ComputeBaseline is still B1's stub.
func assumedBaseline() models.BaselineStats {
	stats := baseline(crisis.Sources[0], 4.0, 2.0, crisis.Sources[1:]...)
	stats.NegativeShareMean = 0.18
	stats.NegativeShareStd = 0.07
	return stats
}

func injectionInput() models.DetectInput {
	return models.DetectInput{
		BrandID:  "brd_demo",
		Enriched: crisis.Mentions("brd_demo", injectionNow),
		Baseline: assumedBaseline(),
		Now:      injectionNow,
	}
}

func TestTheInjectedCrisisFiresTheRealCrisisRule(t *testing.T) {
	set, err := DetectorHandler{}.Handle(context.Background(), injectionInput())
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(set.Errors) > 0 {
		t.Fatalf("the injected corpus produced malformed alerts: %v", set.Errors)
	}

	var fired []models.Alert
	for _, alert := range set.Alerts {
		if alert.Kind == models.AlertCrisis {
			fired = append(fired, alert)
		}
	}
	if len(fired) == 0 {
		t.Fatalf("no crisis alert fired; the injection is wrong, not the rule. alerts: %v", whys(set.Alerts))
	}

	// One per source, since DedupeKey is kind:source:hour.
	if len(fired) != len(crisis.Sources) {
		t.Errorf("%d crisis alerts, want one per source (%d)", len(fired), len(crisis.Sources))
	}

	for _, alert := range fired {
		if alert.Severity != models.SeverityCritical {
			t.Errorf("crisis on %s is %q, want critical: the whole corpus is negative",
				alert.DedupeKey, alert.Severity)
		}
		// The numbers the rule fired on, which is what an alert has to carry
		// instead of a sentence.
		if !slices.Equal(metrics(alert), crisisMetrics) {
			t.Errorf("crisis on %s carries %v, want %v", alert.DedupeKey, metrics(alert), crisisMetrics)
		}
		for _, metric := range crisisMetrics {
			e := evidence(t, alert, metric)
			if e.Window == "" {
				t.Errorf("crisis on %s reports %s over an unlabelled window: %+v", alert.DedupeKey, metric, e)
			}
		}
	}

	// The artifact the PR carries. It asserts nothing the block above does not
	// already cover; read it with -v.
	t.Logf("the demo's alert set on the wire:\n%s", alertJSON(t, set))
}

func TestTheInjectedCrisisAlsoFiresInfluencerMention(t *testing.T) {
	set, err := DetectorHandler{}.Handle(context.Background(), injectionInput())
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	influencer := 0
	for _, alert := range set.Alerts {
		if alert.Kind != models.AlertInfluencerMention {
			continue
		}
		influencer++
		if alert.Severity != models.SeverityHigh {
			t.Errorf("influencer alert on %s is %q, want high: the influencers are negative",
				alert.DedupeKey, alert.Severity)
		}
		followers := evidence(t, alert, "author_followers")
		if followers.Value < followers.Threshold {
			t.Errorf("influencer alert on %s reports %v followers against a threshold of %v",
				alert.DedupeKey, followers.Value, followers.Threshold)
		}
	}
	if influencer == 0 {
		t.Errorf("no influencer_mention alert fired, so the high-follower authors did nothing: %v", whys(set.Alerts))
	}
}

// This is the one place that can see both sides of the coupling: the corpus's
// follower count and the threshold it has to clear. Neither file restates the
// other's number, so tuning the rule fails here rather than on stage.
func TestTheInjectedInfluencersClearTheRealThreshold(t *testing.T) {
	above := 0
	for _, em := range crisis.Mentions("brd_demo", injectionNow) {
		if float64(em.Mention.AuthorFollowers) >= influencerFollowers {
			above++
		}
	}
	if above != crisis.Influencers {
		t.Errorf("%d injected authors clear the %v follower threshold, the corpus promises %d",
			above, influencerFollowers, crisis.Influencers)
	}
}
