package appstore

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"brandpulse/internal/anakin"
	"brandpulse/internal/anakin/sources/sourcestest"
	"brandpulse/internal/models"
)

// liveFeed carries the exact key nesting, key names and value formats of the
// feed for app 1592550875 (boAt Hearables) read on 2026-09-20, including the
// im: prefixes and Apple's label wrapper around every scalar. Two of its
// entries are real ones; the second review's text is shortened and both vote
// counters on the first were raised from 0 so the engagement mapping is
// actually exercised rather than passing on a zero value.
const liveFeed = `{"feed":{"author":{"name":{"label":"iTunes Store"},"uri":{"label":"http://www.apple.com/uk/itunes/"}},"entry":[
{"author":{"uri":{"label":"https://itunes.apple.com/in/reviews/id1605206013"},"name":{"label":"Fatluga"},"label":""},
 "updated":{"label":"2026-09-14T04:24:23-07:00"},
 "im:rating":{"label":"5"},
 "im:version":{"label":"1.8.0"},
 "id":{"label":"14548543799"},
 "title":{"label":"Airdopes app so good"},
 "content":{"label":"This app can order and connect to devices like airdopes I feel so good","attributes":{"type":"text"}},
 "link":{"attributes":{"rel":"related","href":"https://itunes.apple.com/in/review?id=1592550875&type=Purple%20Software"}},
 "im:voteSum":{"label":"3"},
 "im:contentType":{"attributes":{"term":"Application","label":"Application"}},
 "im:voteCount":{"label":"4"}},
{"author":{"uri":{"label":"https://itunes.apple.com/in/reviews/id9"},"name":{"label":"Ankit"},"label":""},
 "updated":{"label":"2026-09-10T11:02:00-07:00"},
 "im:rating":{"label":"1"},
 "im:version":{"label":"1.8.0"},
 "id":{"label":"14545746087"},
 "title":{"label":"There's no optimisation to it"},
 "content":{"label":"It takes an eternity to load and show the features.","attributes":{"type":"text"}},
 "link":{"attributes":{"rel":"related","href":"https://itunes.apple.com/in/review?id=1592550875&type=Purple%20Software"}},
 "im:voteSum":{"label":"0"},
 "im:contentType":{"attributes":{"term":"Application","label":"Application"}},
 "im:voteCount":{"label":"0"}}
],"updated":{"label":"2026-09-20T02:39:26-07:00"}}}`

// scrapeOf wraps a document in the URL Scraper response shape recorded live on
// 2026-09-20: the fields sit at the top level with no envelope.
func scrapeOf(document string) json.RawMessage {
	body, err := json.Marshal(map[string]any{
		"id":         "7599a32d-79c6-4cc6-a049-e988bd0c9391",
		"status":     "completed",
		"url":        "https://itunes.apple.com/in/rss/customerreviews/id=1592550875/sortBy=mostRecent/json",
		"jobType":    "url_scraper",
		"country":    "in",
		"html":       document,
		"markdown":   strings.ReplaceAll(document, "[", `\[`),
		"durationMs": 2470,
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
		SourceHandles: map[string]string{"appstore": "1592550875"},
		Version:       1,
	}
}

// window holds neither review, so mentions.Stamp never runs. ids.New still
// panics in B1's tree; everything this package owns is exercised before that.
var (
	windowStart = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	windowEnd   = time.Date(2020, 1, 15, 0, 0, 0, 0, time.UTC)
)

func TestToDraftMapsLiveFields(t *testing.T) {
	var feed feedResponse
	if err := json.Unmarshal([]byte(liveFeed), &feed); err != nil {
		t.Fatalf("decoding the live feed: %v", err)
	}
	if len(feed.Feed.Entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(feed.Feed.Entries))
	}

	draft, err := toDraft(feed.Feed.Entries[0], "1592550875")
	if err != nil {
		t.Fatalf("toDraft: %v", err)
	}

	if draft.Source != models.SourceAppstore {
		t.Errorf("Source = %q", draft.Source)
	}
	if draft.ExternalID != "14548543799" {
		t.Errorf("ExternalID = %q, want the review id", draft.ExternalID)
	}
	if draft.Author != "Fatluga" {
		t.Errorf("Author = %q", draft.Author)
	}
	if want := "Airdopes app so good\nThis app can order and connect to devices like airdopes I feel so good"; draft.Text != want {
		t.Errorf("Text = %q, want title and body joined", draft.Text)
	}
	if draft.Lang != "en" {
		t.Errorf("Lang = %q, want \"en\"", draft.Lang)
	}
	// Apple states the timestamp in Cupertino time; everything downstream reads UTC.
	if want := time.Date(2026, 9, 14, 11, 24, 23, 0, time.UTC); !draft.PostedAt.Equal(want) {
		t.Errorf("PostedAt = %v, want %v", draft.PostedAt, want)
	}
	if draft.PostedAt.Location() != time.UTC {
		t.Errorf("PostedAt location = %v, want UTC", draft.PostedAt.Location())
	}
	if draft.Rating == nil || *draft.Rating != 5 {
		t.Errorf("Rating = %v, want 5", draft.Rating)
	}
	if draft.Engagement.Likes != 3 {
		t.Errorf("Likes = %d, want the net helpful score 3", draft.Engagement.Likes)
	}
	if draft.AuthorFollowers != 0 {
		t.Errorf("AuthorFollowers = %d, want 0: Apple exposes no such count", draft.AuthorFollowers)
	}
	if draft.Raw["app_version"] != "1.8.0" || draft.Raw["app_id"] != "1592550875" {
		t.Errorf("Raw = %v", draft.Raw)
	}
}

func TestToDraftKeepsAOneStarRatingDistinctFromNoRating(t *testing.T) {
	var feed feedResponse
	if err := json.Unmarshal([]byte(liveFeed), &feed); err != nil {
		t.Fatalf("decoding the live feed: %v", err)
	}

	draft, err := toDraft(feed.Feed.Entries[1], "1592550875")
	if err != nil {
		t.Fatalf("toDraft: %v", err)
	}
	// A pointer, because a 0 in this field would read as the worst review there
	// is rather than as an absent one.
	if draft.Rating == nil || *draft.Rating != 1 {
		t.Fatalf("Rating = %v, want a pointer to 1", draft.Rating)
	}
}

func TestToDraftRefusesAnUnparseableRatingOrDate(t *testing.T) {
	cases := []struct {
		name  string
		entry entry
	}{
		{"no rating", func() entry {
			var e entry
			e.ID.Label, e.Updated.Label, e.Rating.Label = "1", "2026-09-14T04:24:23-07:00", ""
			return e
		}()},
		{"no date", func() entry {
			var e entry
			e.ID.Label, e.Updated.Label, e.Rating.Label = "1", "", "5"
			return e
		}()},
		{"date is not RFC3339", func() entry {
			var e entry
			e.ID.Label, e.Updated.Label, e.Rating.Label = "1", "14 Sep 2026", "5"
			return e
		}()},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := toDraft(c.entry, "1592550875"); err == nil {
				t.Fatal("want an error, got a mention")
			}
		})
	}
}

func TestFetchScrapesTheIndianFeedForTheHandle(t *testing.T) {
	client := &sourcestest.Client{
		ScrapeFunc: func(ctx context.Context, url string, opt anakin.ScrapeOpt) (json.RawMessage, error) {
			return scrapeOf(liveFeed), nil
		},
	}

	if _, err := Fetch(context.Background(), client, profile(), windowStart, windowEnd); err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if len(client.Calls) != 1 {
		t.Fatalf("made %d calls, want exactly one: the feed is keyed by app id, not by keyword", len(client.Calls))
	}
	want := "https://itunes.apple.com/in/rss/customerreviews/id=1592550875/sortBy=mostRecent/json"
	if client.Calls[0].URL != want {
		t.Errorf("url = %q, want %q", client.Calls[0].URL, want)
	}
	// Only html carries the feed verbatim; the other two formats mangle it.
	if len(client.Calls[0].ScrapeOpt.Formats) != 1 || client.Calls[0].ScrapeOpt.Formats[0] != "html" {
		t.Errorf("Formats = %v, want [html]", client.Calls[0].ScrapeOpt.Formats)
	}
}

func TestFetchReadsHTMLAndNothingElse(t *testing.T) {
	// A response whose markdown is a perfectly good feed and whose html is
	// empty. An adapter that fell back to markdown would find 2 reviews here.
	// The live markdown is never good: it escapes the entry array's `[` as
	// `\[`, so a fallback would decode nothing on a real response anyway.
	client := &sourcestest.Client{
		ScrapeFunc: func(ctx context.Context, url string, opt anakin.ScrapeOpt) (json.RawMessage, error) {
			return json.RawMessage(`{"html":"","cleanedHtml":"","markdown":` + mustQuote(liveFeed) + `}`), nil
		},
	}

	out, err := Fetch(context.Background(), client, profile(), windowStart, windowEnd)
	if len(out) != 0 {
		t.Fatalf("got %d mentions from markdown, want the adapter to read html only", len(out))
	}
	if err == nil {
		t.Fatal("want a decode error, got nil")
	}
}

// mustQuote renders s as a JSON string literal.
func mustQuote(s string) string {
	quoted, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(quoted)
}

func TestFetchRefusesAFeedWithNoEntries(t *testing.T) {
	client := &sourcestest.Client{
		ScrapeFunc: func(ctx context.Context, url string, opt anakin.ScrapeOpt) (json.RawMessage, error) {
			return scrapeOf(`{"feed":{"author":{"name":{"label":"iTunes Store"}},"id":{"label":"x"}}}`), nil
		},
	}

	_, err := Fetch(context.Background(), client, profile(), windowStart, windowEnd)
	// A missing entry key is a broken handle or a bad url form, not a quiet
	// fortnight. The same app id answered 0 on one url form and 50 on another.
	if err == nil {
		t.Fatal("want an error for a feed with no entry key")
	}
	if !strings.Contains(err.Error(), "no entries") {
		t.Errorf("err = %v, want it to name the missing entries", err)
	}
}

func TestFetchRefusesABrandWithNoAppID(t *testing.T) {
	client := &sourcestest.Client{}
	p := profile()
	p.SourceHandles = nil

	_, err := Fetch(context.Background(), client, p, windowStart, windowEnd)
	if err == nil {
		t.Fatal("want an error, got a silent empty result")
	}
	if len(client.Calls) != 0 {
		t.Errorf("spent a credit on a brand with no app id: %v", client.Calls)
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

func TestFetchRejectsAnEmptyScrape(t *testing.T) {
	client := &sourcestest.Client{
		ScrapeFunc: func(ctx context.Context, url string, opt anakin.ScrapeOpt) (json.RawMessage, error) {
			return json.RawMessage(`{"html":"","cleanedHtml":"","markdown":""}`), nil
		},
	}

	if _, err := Fetch(context.Background(), client, profile(), windowStart, windowEnd); err == nil {
		t.Fatal("want an error for a 200 with every format empty")
	}
}
