// Package crisis builds the synthetic negative surge the demo injects. It is
// the rows and nothing else: no baseline, no thresholds, no database.
//
// It exists as an importable package rather than living inside
// demo/cmd/injectcrisis so that bp-detector's own test can import this corpus
// and prove the real rules fire on it, which is the one thing that makes the
// injection honest. The alternative, splitting the detector's rules into an
// importable package so the dependency ran the other way, is the worse breach:
// it would turn the thresholds into repo-wide API and put demo/ inside an
// agent's internals in the production graph rather than in one test file.
//
// This package must never be imported by a non-test file under agents/. It
// writes nothing, reads nothing and has no clock of its own: every mention is
// derived from the `now` the caller passes.
package crisis

import (
	"fmt"
	"time"

	"brandpulse/internal/hashing"
	"brandpulse/internal/models"
)

const (
	// Count is the size of the surge. It is well above the per-source hourly
	// mean of a demo brand, which is what makes the z-score clear the bar.
	Count = 40

	// Spread is how far back the surge reaches. A tight window is the point:
	// the rules bucket by hour and forty mentions in ten minutes is what a real
	// crisis looks like from the outside.
	Spread = 10 * time.Minute

	// Influencers is how many of the mentions come from an account above the
	// detector's follower threshold, so influencer_mention fires alongside
	// crisis.
	Influencers = 3

	// influencerFollowers is comfortably over bp-detector's threshold, rather
	// than sitting on it: a demo that depends on a boundary comparison is a
	// demo that breaks when the boundary is tuned. The number this has to
	// clear is not restated here, it is asserted against the real constant in
	// agents/bp-detector/injection_test.go.
	influencerFollowers = 128_400

	// SyntheticKey marks every injected mention. The dashboard reads it to show
	// the "injected demo data" badge, and -cleanup deletes by it. We are not
	// going to imply scraped data is synthetic or the reverse.
	SyntheticKey = "synthetic"
)

// Sources is where the surge lands. Three sources, so the demo shows a crisis
// crossing channels rather than one angry subreddit.
var Sources = []models.Source{models.SourceX, models.SourceReddit, models.SourceInstagram}

// complaints are one plausible product issue seen from forty angles. They are
// written as separate sentences rather than generated, because a generated
// corpus of "negative mention 7" reads as fake on a screen.
var complaints = []string{
	"ordered the sunscreen last week and the pump broke on the second day",
	"my sunscreen bottle arrived already leaking, half of it was gone",
	"second bottle in a row with a broken pump, what is happening",
	"the new batch smells completely different and it stings",
	"support has not replied in four days about the leaking bottle",
	"refund still not processed, it has been over a week now",
	"the sunscreen separated in the bottle, this cannot be normal",
	"pump jammed on day one and support told me to just squeeze it",
	"got a batch that is watery and does not spread at all",
	"this is my third damaged delivery this month, done ordering",
	"the seal was already broken when the box arrived",
	"rash on my face within an hour of the new batch",
	"same leaking pump problem here, clearly a manufacturing issue",
	"paid extra for express and got a half empty bottle",
	"customer care keeps closing my ticket without answering",
	"the texture is nothing like the one I bought in June",
	"bottle cracked in transit, the whole box was soaked",
	"two orders, two broken pumps, no response from support",
	"stopped using it after the reaction, waiting on a refund",
	"the batch code on mine matches the one everyone is complaining about",
}

// Mentions returns the injected corpus, newest last, all inside one hour
// bucket.
//
// The hour matters: bp-detector groups by source and hour, so a surge that
// straddles a boundary is two half-sized groups and may clear neither the
// z-score nor the negative share. When `now` is less than Spread past the top
// of the hour the window is clamped to the top of that hour, which compresses
// the surge rather than splitting it.
func Mentions(brandID string, now time.Time) []models.EnrichedMention {
	now = now.UTC()
	start := now.Add(-Spread)
	if hourStart := now.Truncate(time.Hour); start.Before(hourStart) {
		start = hourStart
	}

	step := now.Sub(start) / time.Duration(Count)
	out := make([]models.EnrichedMention, 0, Count)

	for i := 0; i < Count; i++ {
		source := Sources[i%len(Sources)]
		text := complaints[i%len(complaints)]
		postedAt := start.Add(time.Duration(i) * step)

		// The id is derived from the index and the brand rather than from
		// ids.New, because ids.New carries a clock and this corpus has to be
		// identical on every rehearsal. The brand is in it so injecting into
		// two brands does not collide on the primary key.
		id := fmt.Sprintf("mnt_%s_synthetic_%02d", brandID, i)

		mention := models.Mention{
			ID:         id,
			BrandID:    brandID,
			Source:     source,
			ExternalID: fmt.Sprintf("synthetic_%02d", i),
			Author:     fmt.Sprintf("demo_user_%02d", i),
			Text:       text,
			Lang:       "en",
			PostedAt:   postedAt,
			Engagement: models.Engagement{Likes: 4 + i, Replies: i % 7},
			// ContentHash includes the index through the text suffix below, so
			// forty distinct rows survive the UNIQUE (brand_id, content_hash).
			ContentHash:    hashing.ContentHash(fmt.Sprintf("%s #%02d", text, i), source),
			MatchedKeyword: "sunscreen",
			Raw:            map[string]any{SyntheticKey: true},
		}
		if i < Influencers {
			mention.Author = fmt.Sprintf("demo_creator_%02d", i)
			mention.AuthorFollowers = influencerFollowers + i*1000
		}

		out = append(out, models.EnrichedMention{
			Mention: mention,
			Enrichment: models.Enrichment{
				MentionID:      id,
				Sentiment:      -0.82,
				SentimentLabel: models.SentimentNegative,
				Emotion:        models.EmotionAnger,
				Intent:         models.IntentComplaint,
				Aspects:        []string{"packaging", "support"},
				IsAboutBrand:   true,
				Model:          "demo-injection",
			},
		})
	}
	return out
}

// IsSynthetic reports whether a mention came from this package, for a caller
// holding a models.Mention. The two callers that hold rows instead of structs,
// -cleanup and replay's corpus query, ask Postgres the same question as
// `raw ->> SyntheticKey`.
func IsSynthetic(mention models.Mention) bool {
	marked, ok := mention.Raw[SyntheticKey].(bool)
	return ok && marked
}
