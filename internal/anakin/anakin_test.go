// anakin_test.go runs the client the way CI does: replay mode, the canned
// fixtures, and NoNetwork() injected so a regression that reaches Anakin fails
// here instead of spending a credit.
//
// The canned fixtures are hand-written and deliberately obvious. They are not
// recordings and nothing reads them as data: they exist to prove the five
// methods route, cache and spend correctly. Real recordings are B2's, in
// fixtures/<source>/.
package anakin

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// cannedDir is fixtures/_canned from this package.
const cannedDir = "../../fixtures/_canned"

// The canned calls. They are constants because a query_hash is the fixture
// filename: changing an argument here orphans a file on disk.
const (
	cannedQuery   = "acme serum reviews"
	cannedURL     = "https://example.com/acme-review"
	cannedSite    = "https://example.com"
	cannedVideoID = "acme_video_1"
)

func replayClient(t *testing.T, maxCredits int) Client {
	t.Helper()

	c, err := NewHTTPClient(Config{
		Mode:       ModeReplay,
		MaxCredits: maxCredits,
		BrandID:    "brd_test",
		Day:        time.Now(),
		HTTPClient: NoNetwork(),
		FixtureDir: cannedDir,
	})
	if err != nil {
		t.Fatalf("NewHTTPClient: %v", err)
	}
	return c
}

// TestEveryMethodReturnsACannedResponse is the Phase 1 gate. Five tracks build
// on this interface, and a method that compiles but cannot complete one round
// trip through the cache, the budget and the fixture reader is not a surface
// anybody can build on.
func TestEveryMethodReturnsACannedResponse(t *testing.T) {
	ctx := context.Background()

	calls := []struct {
		name string
		call func(Client) (json.RawMessage, error)
	}{
		{"Search", func(c Client) (json.RawMessage, error) {
			return c.Search(ctx, cannedQuery, SearchOpt{Limit: 5})
		}},
		{"Wire", func(c Client) (json.RawMessage, error) {
			return c.Wire(ctx, "youtube", cannedVideoID, WireOpt{Action: "yt_comments", Limit: 20})
		}},
		{"Scrape", func(c Client) (json.RawMessage, error) {
			return c.Scrape(ctx, cannedURL, ScrapeOpt{Formats: []string{"markdown"}})
		}},
		{"Map", func(c Client) (json.RawMessage, error) {
			return c.Map(ctx, cannedSite, MapOpt{Limit: 100})
		}},
		{"Crawl", func(c Client) (json.RawMessage, error) {
			return c.Crawl(ctx, cannedSite, CrawlOpt{MaxPages: 5})
		}},
	}

	for _, call := range calls {
		t.Run(call.name, func(t *testing.T) {
			client := replayClient(t, 50)

			payload, err := call.call(client)
			if err != nil {
				t.Fatalf("%s: %v", call.name, err)
			}
			if !json.Valid(payload) {
				t.Fatalf("%s returned something that is not JSON: %s", call.name, payload)
			}
			if len(payload) == 0 {
				t.Fatalf("%s returned an empty payload", call.name)
			}
		})
	}
}

// TestACacheHitSpendsNoCredits is the whole reason the cache is in this package
// rather than in each collector. The second identical call must not reach the
// budget at all.
func TestACacheHitSpendsNoCredits(t *testing.T) {
	ctx := context.Background()
	client := replayClient(t, 50)

	first, err := client.Search(ctx, cannedQuery, SearchOpt{Limit: 5})
	if err != nil {
		t.Fatalf("first Search: %v", err)
	}
	second, err := client.Search(ctx, cannedQuery, SearchOpt{Limit: 5})
	if err != nil {
		t.Fatalf("second Search: %v", err)
	}

	if string(first) != string(second) {
		t.Error("the cached payload differs from the fetched one")
	}

	stats := client.Stats()
	if stats.CreditsUsed != costSearch {
		t.Errorf("CreditsUsed = %d after two identical searches, want %d: the second one was not a cache hit",
			stats.CreditsUsed, costSearch)
	}
	if stats.CacheHits != 1 {
		t.Errorf("CacheHits = %d, want 1", stats.CacheHits)
	}
}

// TestAnExceededBudgetReturnsErrBudgetExceeded covers the sentinel the
// collector switches on: a breached ceiling is MentionBatch.Truncated, not a
// failed run, and it can only be told apart by errors.Is.
func TestAnExceededBudgetReturnsErrBudgetExceeded(t *testing.T) {
	ctx := context.Background()

	// Exactly one search fits.
	client := replayClient(t, costSearch)

	if _, err := client.Search(ctx, cannedQuery, SearchOpt{Limit: 5}); err != nil {
		t.Fatalf("the first search did not fit its own ceiling: %v", err)
	}

	_, err := client.Scrape(ctx, cannedURL, ScrapeOpt{Formats: []string{"markdown"}})
	if !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("scrape past the ceiling returned %v, want ErrBudgetExceeded", err)
	}

	if used := client.Stats().CreditsUsed; used != costSearch {
		t.Errorf("CreditsUsed = %d, want %d: the refused call must not be counted", used, costSearch)
	}
}

// TestAMissingFixtureNamesThePath is the difference between a five-minute fix
// and a demo that quietly shows zero mentions. The path in the message is what
// someone records.
func TestAMissingFixtureNamesThePath(t *testing.T) {
	ctx := context.Background()
	client := replayClient(t, 50)

	_, err := client.Search(ctx, "a query nobody has ever recorded", SearchOpt{Limit: 5})
	if err == nil {
		t.Fatal("an unrecorded query returned a payload")
	}

	message := err.Error()
	if !strings.Contains(message, cannedDir+"/web/") || !strings.Contains(message, ".json") {
		t.Errorf("the error does not name the fixture path: %s", message)
	}
}

// TestNoNetworkRefusesEveryRoundTrip checks the enforcement mechanism itself.
// Go has no pytest-socket, so if this transport ever returned a response the
// whole zero-credit CI guarantee would be a comment in a README.
func TestNoNetworkRefusesEveryRoundTrip(t *testing.T) {
	resp, err := NoNetwork().Get("https://api.anakin.io/v1/search")
	if err == nil {
		resp.Body.Close()
		t.Fatal("NoNetwork performed a request")
	}
	if !strings.Contains(err.Error(), "api.anakin.io") {
		t.Errorf("the error does not name the URL that was attempted: %v", err)
	}
}

// TestReplayNeedsACeiling covers the one config that cannot work: replay has no
// pool, so there is nowhere to read brands.daily_credit_budget from. It fails
// at construction rather than on the first call.
func TestReplayNeedsACeiling(t *testing.T) {
	_, err := NewHTTPClient(Config{
		Mode:       ModeReplay,
		HTTPClient: NoNetwork(),
		FixtureDir: cannedDir,
	})
	if err == nil {
		t.Fatal("a replay client with no ceiling was built")
	}
}

// TestLiveModeNeedsAKey stops the failure mode where BP_FIXTURE_MODE is set to
// live on a machine with no key and every call fails one at a time with a 401.
func TestLiveModeNeedsAKey(t *testing.T) {
	_, err := NewHTTPClient(Config{Mode: ModeLive, MaxCredits: 10})
	if err == nil {
		t.Fatal("a live client with no API key was built")
	}
}

func TestWireCostsWhatTheResearchSays(t *testing.T) {
	costs := map[string]int{
		"yt_search":          1,
		"yt_video":           1,
		"yt_channel":         1,
		"yt_comments":        3,
		"rt_search":          2,
		"am_product_reviews": 2,
	}
	for action, want := range costs {
		if got := wireCost(action); got != want {
			t.Errorf("wireCost(%s) = %d, want %d", action, got, want)
		}
	}
}

// TestQueryHashIsStableAcrossOptions guards the fixture filenames. A hash that
// moved when an unrelated field was added would orphan every recording B2 made.
func TestQueryHashIsStableAcrossOptions(t *testing.T) {
	first, err := queryHash("search", cannedQuery, SearchOpt{Limit: 5})
	if err != nil {
		t.Fatalf("queryHash: %v", err)
	}
	second, err := queryHash("search", cannedQuery, SearchOpt{Limit: 5})
	if err != nil {
		t.Fatalf("queryHash: %v", err)
	}
	if first != second {
		t.Errorf("the same query hashed twice gave %s and %s", first, second)
	}

	other, err := queryHash("search", cannedQuery, SearchOpt{Limit: 10})
	if err != nil {
		t.Fatalf("queryHash: %v", err)
	}
	if other == first {
		t.Error("a different limit hashed the same; two queries would share one fixture")
	}
}
