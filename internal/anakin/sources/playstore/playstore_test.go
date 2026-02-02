package playstore

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"brandpulse/internal/anakin"
	"brandpulse/internal/anakin/sources/sourcestest"
	"brandpulse/internal/models"
)

// testdata/listing.html holds two review blocks lifted out of the live scrape
// of com.boAt.hearables on 2026-09-20, with the star SVGs, the avatar images
// and the jsaction/jscontroller attributes stripped. Every class name, data
// attribute and text node this adapter reads is the real one, including the
// developer reply block that the markdown format flattens into the review.
func listing(t *testing.T) string {
	t.Helper()
	document, err := os.ReadFile("testdata/listing.html")
	if err != nil {
		t.Fatalf("reading the listing fixture: %v", err)
	}
	return string(document)
}

// scrapeOf wraps a document in the URL Scraper response shape.
func scrapeOf(document string) json.RawMessage {
	body, err := json.Marshal(map[string]any{
		"status":   "completed",
		"country":  "in",
		"html":     document,
		"markdown": "# boAt Hearables\n\n(the rating never survives this format)",
	})
	if err != nil {
		panic(err)
	}
	return body
}

func profile() models.BrandProfile {
	return models.BrandProfile{
		BrandID:       "boat-lifestyle",
		Keywords:      []string{"boAt"},
		SourceHandles: map[string]string{"playstore": "com.boAt.hearables"},
		Version:       1,
	}
}

// window holds neither review, so mentions.Stamp never runs. ids.New still
// panics in B1's tree; everything this package owns is exercised before that.
var (
	windowStart = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	windowEnd   = time.Date(2020, 1, 15, 0, 0, 0, 0, time.UTC)
)

func TestParseReviewsReadsEveryFieldFromTheLiveDOM(t *testing.T) {
	found, err := parseReviews(listing(t))
	if err != nil {
		t.Fatalf("parseReviews: %v", err)
	}
	if len(found) != 2 {
		t.Fatalf("got %d reviews, want 2", len(found))
	}

	first := found[0]
	if first.ID != "f82e5cc1-7726-4443-a3dd-89dd5f5deb5f" {
		t.Errorf("ID = %q", first.ID)
	}
	if first.Author != "Bilal Musani" {
		t.Errorf("Author = %q", first.Author)
	}
	if first.Rating != "Rated 3 stars out of five stars" {
		t.Errorf("Rating = %q", first.Rating)
	}
	if first.Date != "16 July 2026" {
		t.Errorf("Date = %q", first.Date)
	}
	if first.Thumbs != "11" {
		t.Errorf("Thumbs = %q", first.Thumbs)
	}
	if !strings.HasPrefix(first.Text, "Please add dB values to the Custom EQ sliders.") {
		t.Errorf("Text = %q", first.Text)
	}
	// The developer's reply sits inside the same container. Reading the whole
	// container's text, which is what markdown effectively does, would glue the
	// reply onto the review.
	if strings.Contains(first.Text, "thanks for the feedback") {
		t.Errorf("Text swallowed the developer reply: %q", first.Text)
	}
}

func TestParseReviewsTakesTheReviewDateNotTheReplyDate(t *testing.T) {
	// The two dates coincide on the live fixture, so this fragment separates
	// them. It is the trap that made markdown unusable: the reply date sits
	// under the developer's name and reads as the review's own.
	const fragment = `<div class="EGFGHd"><header class="c1bOId" data-review-id="r1">` +
		`<div class="X5PpBb">Asha</div>` +
		`<div aria-label="Rated 2 stars out of five stars" role="img" class="iXRFPc"></div>` +
		`<span class="bp9Aid">3 March 2026</span></header>` +
		`<div class="h3YV2d">Pairing drops constantly.</div>` +
		`<div data-review-id="r1" data-original-thumbs-up-count="7"></div>` +
		`<div class="ocpBU"><div class="I6j64d">Imagine Marketing Limited</div>` +
		`<div class="I9Jtec">19 August 2026</div></div></div>`

	found, err := parseReviews(fragment)
	if err != nil {
		t.Fatalf("parseReviews: %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("got %d reviews, want 1", len(found))
	}
	if found[0].Date != "3 March 2026" {
		t.Errorf("Date = %q, want the review's own date, not the reply's", found[0].Date)
	}

	draft, err := toDraft(found[0], "com.boAt.hearables", "https://play.google.com/x")
	if err != nil {
		t.Fatalf("toDraft: %v", err)
	}
	if want := time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC); !draft.PostedAt.Equal(want) {
		t.Errorf("PostedAt = %v, want %v", draft.PostedAt, want)
	}
}

func TestToDraftMapsTheLiveReview(t *testing.T) {
	found, err := parseReviews(listing(t))
	if err != nil {
		t.Fatalf("parseReviews: %v", err)
	}

	draft, err := toDraft(found[0], "com.boAt.hearables", "https://play.google.com/store/apps/details?id=com.boAt.hearables&hl=en_IN&gl=IN")
	if err != nil {
		t.Fatalf("toDraft: %v", err)
	}

	if draft.Source != models.SourcePlaystore {
		t.Errorf("Source = %q", draft.Source)
	}
	if draft.ExternalID != "f82e5cc1-7726-4443-a3dd-89dd5f5deb5f" {
		t.Errorf("ExternalID = %q", draft.ExternalID)
	}
	if draft.Rating == nil || *draft.Rating != 3 {
		t.Errorf("Rating = %v, want 3", draft.Rating)
	}
	if draft.Engagement.Likes != 11 {
		t.Errorf("Likes = %d, want the thumbs-up count 11", draft.Engagement.Likes)
	}
	if want := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC); !draft.PostedAt.Equal(want) {
		t.Errorf("PostedAt = %v, want %v", draft.PostedAt, want)
	}
	if draft.Lang != "en" {
		t.Errorf("Lang = %q, want \"en\"", draft.Lang)
	}
	if draft.AuthorFollowers != 0 {
		t.Errorf("AuthorFollowers = %d, want 0", draft.AuthorFollowers)
	}
	if draft.Raw["package"] != "com.boAt.hearables" {
		t.Errorf("Raw = %v", draft.Raw)
	}
}

func TestToDraftRefusesAMissingField(t *testing.T) {
	complete := review{
		ID:     "r1",
		Author: "Asha",
		Text:   "Pairing drops constantly.",
		Rating: "Rated 2 stars out of five stars",
		Date:   "3 March 2026",
		Thumbs: "7",
	}

	cases := []struct {
		name  string
		mutee func(*review)
	}{
		{"no review id", func(r *review) { r.ID = "" }},
		{"no rating label", func(r *review) { r.Rating = "" }},
		{"a rating label Google changed", func(r *review) { r.Rating = "3 out of 5" }},
		{"no date", func(r *review) { r.Date = "" }},
		{"a date in another layout", func(r *review) { r.Date = "2026-03-03" }},
		{"no text", func(r *review) { r.Text = "   " }},
		{"no thumbs attribute", func(r *review) { r.Thumbs = "" }},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := complete
			c.mutee(&r)
			if _, err := toDraft(r, "com.boAt.hearables", "https://play.google.com/x"); err == nil {
				t.Fatal("want an error, got a mention")
			}
		})
	}
}

func TestFetchAsksForTheIndianListingWithABrowser(t *testing.T) {
	client := &sourcestest.Client{
		ScrapeFunc: func(ctx context.Context, url string, opt anakin.ScrapeOpt) (json.RawMessage, error) {
			return scrapeOf(listing(t)), nil
		},
	}

	if _, err := Fetch(context.Background(), client, profile(), windowStart, windowEnd); err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if len(client.Calls) != 1 {
		t.Fatalf("made %d calls, want exactly one: the listing is keyed by package name", len(client.Calls))
	}
	call := client.Calls[0]
	want := "https://play.google.com/store/apps/details?id=com.boAt.hearables&hl=en_IN&gl=IN"
	if call.URL != want {
		t.Errorf("url = %q, want %q", call.URL, want)
	}
	// Without a browser the reviews are not in the DOM at all.
	if !call.ScrapeOpt.UseBrowser {
		t.Error("UseBrowser = false, want true")
	}
	if len(call.ScrapeOpt.Formats) != 2 || call.ScrapeOpt.Formats[1] != "html" {
		t.Errorf("Formats = %v, want the verified [markdown html]", call.ScrapeOpt.Formats)
	}
}

func TestFetchErrorsWhenTheSelectorsMatchNothing(t *testing.T) {
	// Google's class names are obfuscated build output and they rotate. A
	// rotation that returned an empty batch would be indistinguishable from a
	// week with no reviews, which is the failure the whole probe existed to
	// prevent.
	client := &sourcestest.Client{
		ScrapeFunc: func(ctx context.Context, url string, opt anakin.ScrapeOpt) (json.RawMessage, error) {
			return scrapeOf(`<!doctype html><html><body><div class="AaBbCc">Ratings and reviews</div></body></html>`), nil
		},
	}

	_, err := Fetch(context.Background(), client, profile(), windowStart, windowEnd)
	if err == nil {
		t.Fatal("want an error, got a silent empty batch")
	}
	if !strings.Contains(err.Error(), "no reviews") {
		t.Errorf("err = %v, want it to say no reviews were found", err)
	}
}

func TestFetchRefusesABrandWithNoPackageName(t *testing.T) {
	client := &sourcestest.Client{}
	p := profile()
	p.SourceHandles = nil

	if _, err := Fetch(context.Background(), client, p, windowStart, windowEnd); err == nil {
		t.Fatal("want an error, got a silent empty result")
	}
	if len(client.Calls) != 0 {
		t.Errorf("spent a credit on a brand with no package name: %v", client.Calls)
	}
}

func TestFetchPropagatesTheBudgetCeiling(t *testing.T) {
	client := &sourcestest.Client{
		ScrapeFunc: func(ctx context.Context, url string, opt anakin.ScrapeOpt) (json.RawMessage, error) {
			return nil, anakin.ErrBudgetExceeded
		},
	}

	_, err := Fetch(context.Background(), client, profile(), windowStart, windowEnd)
	if !errors.Is(err, anakin.ErrBudgetExceeded) {
		t.Fatalf("err = %v, want ErrBudgetExceeded to survive the wrap", err)
	}
}
