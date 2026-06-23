package news

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"brandpulse/internal/anakin"
	"brandpulse/internal/anakin/sources/sourcestest"
	"brandpulse/internal/models"
)

// liveResponse is the /v1/search shape recorded in
// docs/research/wire-schemas.md §5, dated into a window these tests control.
const liveResponse = `{
  "id": "search_30e33488c9d4549a201f1321b37e5bfc",
  "results": [
    {"url": "https://timesofindia.indiatimes.com/boat-airdopes-review",
     "title": "boAt Airdopes 191G review",
     "snippet": "The design is plasticky but the battery lasts two days.",
     "date": "2020-01-07",
     "last_updated": "2020-01-07"},
    {"url": "https://gadgets360.com/boat-old",
     "title": "boAt launches Airdopes",
     "snippet": "Launch coverage.",
     "date": "2019-06-14",
     "last_updated": "2019-06-14"}
  ]
}`

// window is a fortnight containing only the first of the two results. Every
// test here asserts against it rather than against time.Now() so a replayed
// fixture produces the same corpus on any day.
var (
	windowStart = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	windowEnd   = time.Date(2020, 1, 15, 0, 0, 0, 0, time.UTC)
)

func profile(keywords ...string) models.BrandProfile {
	return models.BrandProfile{BrandID: "boat-lifestyle", Keywords: keywords, Version: 1}
}

func TestPromptAsksForNews(t *testing.T) {
	if got := prompt("boAt"); got != "boAt news" {
		t.Errorf("prompt = %q, want %q", got, "boAt news")
	}
}

func TestFetchSearchesOncePerKeywordWithTheNewsPrompt(t *testing.T) {
	client := &sourcestest.Client{
		SearchFunc: func(ctx context.Context, prompt string, opt anakin.SearchOpt) (json.RawMessage, error) {
			return json.RawMessage(`{"id":"search_1","results":[]}`), nil
		},
	}

	if _, err := Fetch(context.Background(), client, profile("boAt", "boat headphones"), windowStart, windowEnd); err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	got := client.Queries()
	want := []string{"boAt news", "boat headphones news"}
	if len(got) != len(want) {
		t.Fatalf("got %d searches %v, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("search %d = %q, want %q", i, got[i], want[i])
		}
	}
	// Search is the only surface news touches. A Wire or Scrape call would make
	// sourcestest fail loudly, and the recorded methods prove it did not.
	for _, call := range client.Calls {
		if call.Method != "Search" {
			t.Errorf("news made a %s call", call.Method)
		}
	}
}

func TestFetchDropsResultsOutsideTheWindow(t *testing.T) {
	client := &sourcestest.Client{
		SearchFunc: func(ctx context.Context, prompt string, opt anakin.SearchOpt) (json.RawMessage, error) {
			return json.RawMessage(liveResponse), nil
		},
	}

	// Both results are outside this window, so the identity stamp never runs and
	// the whole date path is still exercised.
	out, err := Fetch(context.Background(), client, profile("boAt"), time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2021, 1, 15, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("got %d mentions, want 0: the window holds neither result", len(out))
	}
}

// An undated result is the common case rather than an edge case: 66 of the 120
// search results recorded on 2026-09-20 carry neither date field. Dropping them
// is what made a paid live run report zero news mentions, so the result is kept
// and dated into the window it was collected for.
func TestFetchKeepsAnUndatedResult(t *testing.T) {
	client := &sourcestest.Client{
		SearchFunc: func(ctx context.Context, prompt string, opt anakin.SearchOpt) (json.RawMessage, error) {
			return json.RawMessage(`{"id":"search_1","results":[{"url":"https://example.com/undated","title":"t","snippet":"s","date":"","last_updated":""}]}`), nil
		},
	}

	out, err := Fetch(context.Background(), client, profile("boAt"), windowStart, windowEnd)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("got %d mentions, want the undated result kept", len(out))
	}

	// The window is half-open, so surviving mentions.Filter is the assertion
	// that matters here.
	if out[0].PostedAt.Before(windowStart) || !out[0].PostedAt.Before(windowEnd) {
		t.Errorf("PostedAt = %v, want it inside [%v, %v)", out[0].PostedAt, windowStart, windowEnd)
	}
	if out[0].Raw["posted_at_precision"] != "unknown" {
		t.Errorf("Raw[posted_at_precision] = %v, want unknown so a consumer can tell the date is a guess",
			out[0].Raw["posted_at_precision"])
	}
}

func TestFetchPropagatesTheBudgetCeiling(t *testing.T) {
	var calls int
	client := &sourcestest.Client{
		SearchFunc: func(ctx context.Context, prompt string, opt anakin.SearchOpt) (json.RawMessage, error) {
			calls++
			return nil, fmt.Errorf("search: %w", anakin.ErrBudgetExceeded)
		},
	}

	_, err := Fetch(context.Background(), client, profile("boAt", "airdopes"), windowStart, windowEnd)
	if !errors.Is(err, anakin.ErrBudgetExceeded) {
		t.Fatalf("err = %v, want ErrBudgetExceeded to survive the error join", err)
	}
	if calls != 1 {
		t.Errorf("made %d searches, want it to stop at the ceiling", calls)
	}
}

func TestFetchReportsATotalFailure(t *testing.T) {
	client := &sourcestest.Client{
		SearchFunc: func(ctx context.Context, prompt string, opt anakin.SearchOpt) (json.RawMessage, error) {
			return nil, errors.New("upstream 503")
		},
	}

	out, err := Fetch(context.Background(), client, profile("boAt"), windowStart, windowEnd)
	if len(out) != 0 {
		t.Errorf("got %d mentions", len(out))
	}
	if err == nil {
		t.Fatal("want the 503 reported")
	}
}
