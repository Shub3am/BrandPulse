// replay_test.go drives the adapter through the real replay client over the
// fixtures recorded live on 2026-09-20.
//
// The stubbed tests in reddit_test.go check the mapping. This one checks the
// thing they cannot: that the bytes on disk, envelope and all, come out the
// other end as mentions. The recording session produced 0 mentions with every
// stubbed test passing, so this file is the one that would have caught it.
package reddit

import (
	"context"
	"testing"
	"time"

	"brandpulse/internal/anakin"
	"brandpulse/internal/models"
)

// The recorder ran a 21-day window ending at the moment of the run, so
// timeBucket resolved to "month". That bucket is part of the request the
// fixture is filed under: a window of a different width asks for a file that
// was never recorded.
var (
	replayEnd   = time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	replayStart = replayEnd.AddDate(0, 0, -21)
)

// replayClient reads fixtures/ and cannot reach the network.
func replayClient(t *testing.T) anakin.Client {
	t.Helper()

	c, err := anakin.NewHTTPClient(anakin.Config{
		Mode:       anakin.ModeReplay,
		MaxCredits: 300,
		BrandID:    "brd_replay",
		Day:        replayEnd,
		HTTPClient: anakin.NoNetwork(),
		FixtureDir: "../../../../fixtures",
	})
	if err != nil {
		t.Fatalf("NewHTTPClient: %v", err)
	}
	return c
}

func TestFetchMapsTheRecordedSearch(t *testing.T) {
	// "Dot & Key" is a competitor profile, so its keyword list is the one name.
	// Its recording holds 4 posts, the smallest non-empty one.
	profile := models.BrandProfile{BrandID: "dot-key", Keywords: []string{"Dot & Key"}, Version: 1}

	got, err := Fetch(context.Background(), replayClient(t), profile, replayStart, replayEnd)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("got 0 mentions from a recording holding 4 posts")
	}

	for _, m := range got {
		if m.Source != models.SourceReddit {
			t.Errorf("Source = %q, want reddit", m.Source)
		}
		if m.PostedAt.Before(replayStart) || !m.PostedAt.Before(replayEnd) {
			t.Errorf("%s is dated %v, outside [%v, %v)", m.ExternalID, m.PostedAt, replayStart, replayEnd)
		}
		if m.Text == "" || m.ExternalID == "" || m.ContentHash == "" {
			t.Errorf("mention %+v is missing a field mentions.Stamp should have filled", m)
		}
	}
}
