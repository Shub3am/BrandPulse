// These are the claims about the corpus itself: that it is valid, labelled,
// deterministic and shaped the way the rules bucket.
//
// That those numbers actually fire the rule is a different claim, and it lives
// in agents/bp-detector/injection_test.go because only that package can see the
// real thresholds.
package crisis

import (
	"testing"
	"time"

	"brandpulse/internal/models"
)

// corpusNow is fixed so the corpus, its buckets and its timestamps are the same
// on every run of this test.
var corpusNow = time.Date(2026, 9, 20, 14, 40, 0, 0, time.UTC)

// Every mention has to survive the insert. Lang has no default in Go and
// Validate rejects an empty one, which is exactly the trap this data would
// otherwise fall into.
func TestEveryInjectedMentionIsValidAndMarkedSynthetic(t *testing.T) {
	corpus := Mentions("brd_demo", corpusNow)

	if len(corpus) != Count {
		t.Fatalf("corpus holds %d mentions, want %d", len(corpus), Count)
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
		if !IsSynthetic(em.Mention) {
			t.Errorf("mention %q is not marked synthetic", em.Mention.ID)
		}
		if seenHash[em.Mention.ContentHash] {
			t.Errorf("mention %q repeats a content_hash, so UNIQUE (brand_id, content_hash) would drop it",
				em.Mention.ID)
		}
		seenHash[em.Mention.ContentHash] = true

		if em.Mention.AuthorFollowers > 0 {
			influencers++
		}
		if em.Enrichment.SentimentLabel != models.SentimentNegative {
			t.Errorf("mention %q is not negative, so it does not belong in a crisis corpus", em.Mention.ID)
		}
	}
	if influencers != Influencers {
		t.Errorf("%d authors carry a follower count, want %d", influencers, Influencers)
	}
}

// The rules bucket by source and hour, so the shape that matters is how many
// mentions landed on each source inside one hour.
func TestTheSurgeIsSpreadOverEverySource(t *testing.T) {
	perSource := map[models.Source]int{}
	for _, em := range Mentions("brd_demo", corpusNow) {
		perSource[em.Mention.Source]++
	}

	if len(perSource) != len(Sources) {
		t.Fatalf("the surge landed on %d sources, want %d: %v", len(perSource), len(Sources), perSource)
	}
	// Count/len(Sources) rounded down: the remainder lands on the first
	// sources, so none may hold fewer than this.
	least := Count / len(Sources)
	for _, source := range Sources {
		if perSource[source] < least {
			t.Errorf("%s got %d mentions, fewer than the %d an even spread gives it",
				source, perSource[source], least)
		}
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
			for _, em := range Mentions("brd_demo", at) {
				if em.Mention.PostedAt.Before(hour) || em.Mention.PostedAt.After(at) {
					t.Fatalf("mention %q posted at %s, outside the hour from %s to %s",
						em.Mention.ID, em.Mention.PostedAt.Format(time.RFC3339),
						hour.Format(time.RFC3339), at.Format(time.RFC3339))
				}
			}
		})
	}
}

func TestEveryMentionBelongsToTheBrandItWasInjectedFor(t *testing.T) {
	for _, em := range Mentions("brd_other", corpusNow) {
		if em.Mention.BrandID != "brd_other" {
			t.Fatalf("mention %q carries brand %q", em.Mention.ID, em.Mention.BrandID)
		}
		if em.Enrichment.MentionID != em.Mention.ID {
			t.Fatalf("enrichment points at %q, mention is %q", em.Enrichment.MentionID, em.Mention.ID)
		}
	}
}

// Two brands injected on the same day must not collide on the primary key.
func TestTheIdsAreScopedToTheBrand(t *testing.T) {
	mine := Mentions("brd_demo", corpusNow)
	theirs := Mentions("brd_other", corpusNow)

	for i := range mine {
		if mine[i].Mention.ID == theirs[i].Mention.ID {
			t.Fatalf("both brands minted %q", mine[i].Mention.ID)
		}
	}
}

// Same input, same corpus, every time. A demo that differs between runs cannot
// be rehearsed.
func TestTheInjectionIsDeterministic(t *testing.T) {
	first := Mentions("brd_demo", corpusNow)
	again := Mentions("brd_demo", corpusNow)

	for i := range first {
		if again[i].Mention.ID != first[i].Mention.ID ||
			!again[i].Mention.PostedAt.Equal(first[i].Mention.PostedAt) ||
			again[i].Mention.ContentHash != first[i].Mention.ContentHash {
			t.Fatalf("position %d differs: %+v vs %+v", i, again[i].Mention, first[i].Mention)
		}
	}
}
