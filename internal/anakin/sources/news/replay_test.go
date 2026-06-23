// replay_test.go drives the adapter through the real replay client over the
// Search responses recorded live on 2026-09-20.
//
// news makes no Wire and no Scrape call, so it is the cheapest end-to-end proof
// that a recorded response reaches the dashboard as a mention. The recording
// produced none, because every result whose date fields were blank was dropped.
package news

import (
	"context"
	"testing"
	"time"

	"brandpulse/internal/anakin"
	"brandpulse/internal/models"
)

var (
	replayEnd   = time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	replayStart = replayEnd.AddDate(0, 0, -21)
)

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
	got, err := Fetch(context.Background(), replayClient(t), profile("Dot & Key"), replayStart, replayEnd)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("got 0 mentions from a recorded response; the undated results are being dropped again")
	}

	for _, m := range got {
		if m.Source != models.SourceNews {
			t.Errorf("Source = %q, want news", m.Source)
		}
		if m.PostedAt.Before(replayStart) || !m.PostedAt.Before(replayEnd) {
			t.Errorf("%s is dated %v, outside [%v, %v)", m.URL, m.PostedAt, replayStart, replayEnd)
		}
		if m.ContentHash == "" || m.Text == "" {
			t.Errorf("mention %+v is missing a field mentions.Stamp should have filled", m)
		}
	}
	t.Logf("%d mentions from the recorded news search", len(got))
}
