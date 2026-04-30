// The demo injection has to fire the real rules.
//
// This test lives here, not under demo/, because bp-detector is `package main`
// and nothing can import it. Running the corpus through the handler is the only
// way to prove the claim without special-casing the detector, and
// special-casing the detector to make a demo work would make every alert on
// stage worthless.
//
// It needs no database and no network.

package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"brandpulse/demo/crisis"
	"brandpulse/internal/models"
)

// injectionNow is fixed so the corpus, the buckets and the alert ids that
// depend on them are the same on every run.
var injectionNow = time.Date(2026, 9, 20, 14, 40, 0, 0, time.UTC)

func injectionInput() models.DetectInput {
	return models.DetectInput{
		BrandID:  "brd_demo",
		Enriched: crisis.Mentions("brd_demo", injectionNow),
		Baseline: crisis.Baseline(),
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
		t.Fatalf("no crisis alert fired; the injection is wrong, not the rule. alerts: %+v", set.Alerts)
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
		if len(alert.Evidence) != 3 {
			t.Errorf("crisis on %s carries %d pieces of evidence, want the z-score, the negative share and the count",
				alert.DedupeKey, len(alert.Evidence))
		}
		for _, e := range alert.Evidence {
			if e.Metric == "" || e.Window == "" {
				t.Errorf("crisis on %s has an unlabelled evidence entry: %+v", alert.DedupeKey, e)
			}
		}
	}
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
		if alert.Evidence[0].Value < alert.Evidence[0].Threshold {
			t.Errorf("influencer alert reports %v followers against a threshold of %v",
				alert.Evidence[0].Value, alert.Evidence[0].Threshold)
		}
	}
	if influencer == 0 {
		t.Errorf("no influencer_mention alert fired, so the three high-follower authors did nothing: %+v", set.Alerts)
	}
}

// Every mention has to survive the insert. Lang has no default in Go and
// Validate rejects an empty one, which is exactly the trap this data would
// otherwise fall into.
func TestEveryInjectedMentionIsValidAndMarkedSynthetic(t *testing.T) {
	corpus := crisis.Mentions("brd_demo", injectionNow)

	if len(corpus) != crisis.Count {
		t.Fatalf("corpus holds %d mentions, want %d", len(corpus), crisis.Count)
	}

	seenHash := map[string]bool{}
	influencers := 0
	for _, em := range corpus {
		if err := em.Mention.Validate(); err != nil {
			t.Errorf("mention %q: %v", em.Mention.ID, err)
		}
		if em.Mention.Lang == "" {
			t.Errorf("mention %q has no lang", em.Mention.ID)
		}
		if !crisis.IsSynthetic(em.Mention) {
			t.Errorf("mention %q is not marked synthetic", em.Mention.ID)
		}
		if seenHash[em.Mention.ContentHash] {
			t.Errorf("mention %q repeats a content_hash, so UNIQUE (brand_id, content_hash) would drop it",
				em.Mention.ID)
		}
		seenHash[em.Mention.ContentHash] = true

		if em.Mention.AuthorFollowers >= 50000 {
			influencers++
		}
		if em.Enrichment.SentimentLabel != models.SentimentNegative {
			t.Errorf("mention %q is not negative, so it does not belong in a crisis corpus", em.Mention.ID)
		}
	}
	if influencers != crisis.Influencers {
		t.Errorf("%d high-follower authors, want %d", influencers, crisis.Influencers)
	}
}

// The surge must not straddle an hour boundary: the rules bucket by hour and
// two half-sized groups may clear neither threshold.
func TestTheSurgeStaysInsideOneHourBucket(t *testing.T) {
	for _, at := range []time.Time{
		time.Date(2026, 9, 20, 14, 40, 0, 0, time.UTC), // mid hour
		time.Date(2026, 9, 20, 14, 3, 0, 0, time.UTC),  // three minutes past
		time.Date(2026, 9, 20, 14, 0, 30, 0, time.UTC), // thirty seconds past
	} {
		t.Run(at.Format("15:04:05"), func(t *testing.T) {
			hour := at.Truncate(time.Hour)
			for _, em := range crisis.Mentions("brd_demo", at) {
				if em.Mention.PostedAt.Before(hour) || em.Mention.PostedAt.After(at) {
					t.Fatalf("mention %q posted at %s, outside the hour from %s to %s",
						em.Mention.ID, em.Mention.PostedAt.Format(time.RFC3339), hour.Format(time.RFC3339), at.Format(time.RFC3339))
				}
			}
		})
	}
}

// Same input, same corpus, every time. A demo that differs between runs cannot
// be rehearsed.
func TestTheInjectionIsDeterministic(t *testing.T) {
	first := crisis.Mentions("brd_demo", injectionNow)
	for i := 0; i < 5; i++ {
		again := crisis.Mentions("brd_demo", injectionNow)
		for j := range first {
			if again[j].Mention.ID != first[j].Mention.ID ||
				!again[j].Mention.PostedAt.Equal(first[j].Mention.PostedAt) ||
				again[j].Mention.ContentHash != first[j].Mention.ContentHash {
				t.Fatalf("run %d position %d differs: %+v vs %+v", i, j, again[j].Mention, first[j].Mention)
			}
		}
	}
}

// TestPrintTheCrisisAlertJSON exists to produce the artifact the PR asks for.
// It asserts nothing the tests above do not already cover; run it with -v.
func TestPrintTheCrisisAlertJSON(t *testing.T) {
	set, err := DetectorHandler{}.Handle(context.Background(), injectionInput())
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	for _, alert := range set.Alerts {
		if alert.Kind != models.AlertCrisis {
			continue
		}
		out, err := json.MarshalIndent(alert, "", "  ")
		if err != nil {
			t.Fatalf("marshalling the alert: %v", err)
		}
		t.Logf("crisis alert:\n%s", out)
		return
	}
	t.Fatal("no crisis alert to print")
}
