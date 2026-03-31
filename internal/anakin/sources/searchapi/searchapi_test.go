package searchapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"brandpulse/internal/anakin"
	"brandpulse/internal/anakin/sources/sourcestest"
	"brandpulse/internal/models"
)

// liveResponse is the shape POST /v1/search returned on 2026-09-20, recorded in
// docs/research/wire-schemas.md §5. The three date values are the ones the live
// call actually produced: a dated result, a result with both date fields empty,
// and a three-year-old article. All three arrived in one response to a query
// asking for the past week, which is how the missing server-side freshness
// filter was proved.
const liveResponse = `{
  "id": "search_30e33488c9d4549a201f1321b37e5bfc",
  "results": [
    {"url": "https://timesofindia.indiatimes.com/boat-airdopes-review",
     "title": "boAt Airdopes 191G review",
     "snippet": "The design is plasticky but the battery lasts two days.",
     "date": "2025-01-29",
     "last_updated": "2025-01-29"},
    {"url": "https://example.com/undated",
     "title": "boAt in the news",
     "snippet": "No date on this one.",
     "date": "",
     "last_updated": ""},
    {"url": "https://gadgets360.com/boat-old",
     "title": "boAt launches Airdopes",
     "snippet": "Launch coverage.",
     "date": "2022-09-14",
     "last_updated": "2022-09-14"}
  ]
}`

func testPrompt(keyword string) string { return keyword + " news" }

func TestToDraftMapsLiveFields(t *testing.T) {
	var resp response
	if err := json.Unmarshal([]byte(liveResponse), &resp); err != nil {
		t.Fatalf("decoding the live response: %v", err)
	}
	if resp.ID != "search_30e33488c9d4549a201f1321b37e5bfc" {
		t.Errorf("id = %q", resp.ID)
	}
	if len(resp.Results) != 3 {
		t.Fatalf("got %d results, want 3", len(resp.Results))
	}

	draft, err := ToDraft(resp.Results[0], models.SourceNews, "boAt")
	if err != nil {
		t.Fatalf("ToDraft: %v", err)
	}

	if draft.Source != models.SourceNews {
		t.Errorf("Source = %q", draft.Source)
	}
	// Search returns no id of its own, so the URL has to serve as both.
	if draft.ExternalID != "https://timesofindia.indiatimes.com/boat-airdopes-review" {
		t.Errorf("ExternalID = %q", draft.ExternalID)
	}
	if draft.URL != draft.ExternalID {
		t.Errorf("URL = %q, want it to equal ExternalID", draft.URL)
	}
	if want := "boAt Airdopes 191G review\nThe design is plasticky but the battery lasts two days."; draft.Text != want {
		t.Errorf("Text = %q, want %q", draft.Text, want)
	}
	// Lang is the field pydantic used to default and Go does not.
	if draft.Lang != "en" {
		t.Errorf("Lang = %q, want \"en\"", draft.Lang)
	}
	if want := time.Date(2025, 1, 29, 0, 0, 0, 0, time.UTC); !draft.PostedAt.Equal(want) {
		t.Errorf("PostedAt = %v, want %v", draft.PostedAt, want)
	}
	if draft.PostedAt.Location() != time.UTC {
		t.Errorf("PostedAt location = %v, want UTC", draft.PostedAt.Location())
	}
	if draft.MatchedKeyword != "boAt" {
		t.Errorf("MatchedKeyword = %q", draft.MatchedKeyword)
	}
	if draft.Raw["date"] != "2025-01-29" || draft.Raw["last_updated"] != "2025-01-29" {
		t.Errorf("Raw = %v, want both date fields kept", draft.Raw)
	}
	// Search carries no engagement and no rating for news or web.
	if draft.AuthorFollowers != 0 || draft.Rating != nil {
		t.Errorf("AuthorFollowers = %d, Rating = %v, want 0 and nil", draft.AuthorFollowers, draft.Rating)
	}
}

func TestToDraftUsesSnippetAloneWhenTitleIsEmpty(t *testing.T) {
	draft, err := ToDraft(Result{URL: "https://example.com/a", Snippet: "body only", Date: "2025-01-29"}, models.SourceWeb, "boAt")
	if err != nil {
		t.Fatalf("ToDraft: %v", err)
	}
	if draft.Text != "body only" {
		t.Errorf("Text = %q, want the snippet with no leading newline", draft.Text)
	}
}

func TestPublishedAt(t *testing.T) {
	cases := []struct {
		name    string
		result  Result
		want    time.Time
		wantErr bool
	}{
		{"date wins over last_updated", Result{Date: "2025-01-29", LastUpdated: "2025-03-01"}, time.Date(2025, 1, 29, 0, 0, 0, 0, time.UTC), false},
		{"falls back to last_updated", Result{Date: "", LastUpdated: "2025-03-01"}, time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC), false},
		{"both empty is an error", Result{}, time.Time{}, true},
		{"unparseable is an error", Result{Date: "29 January 2025"}, time.Time{}, true},
		{"a timestamp is not a date", Result{Date: "2025-01-29T10:00:00Z"}, time.Time{}, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := publishedAt(c.result)
			if c.wantErr {
				if err == nil {
					t.Fatalf("got %v, want an error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("publishedAt: %v", err)
			}
			if !got.Equal(c.want) {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestCollectIssuesOneSearchPerKeywordAtTheCap(t *testing.T) {
	client := &sourcestest.Client{
		SearchFunc: func(ctx context.Context, prompt string, opt anakin.SearchOpt) (json.RawMessage, error) {
			if opt.Limit != Limit {
				t.Errorf("Limit = %d, want %d", opt.Limit, Limit)
			}
			return json.RawMessage(`{"id":"search_1","results":[]}`), nil
		},
	}

	_, stop, problems := Collect(context.Background(), client, models.SourceNews, []string{"boAt", "boat headphones"}, testPrompt)
	if stop != nil || len(problems) != 0 {
		t.Fatalf("stop = %v, problems = %v", stop, problems)
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
}

func TestCollectDropsUndatedResultsAndKeepsTheRest(t *testing.T) {
	client := &sourcestest.Client{
		SearchFunc: func(ctx context.Context, prompt string, opt anakin.SearchOpt) (json.RawMessage, error) {
			return json.RawMessage(liveResponse), nil
		},
	}

	drafts, stop, problems := Collect(context.Background(), client, models.SourceNews, []string{"boAt"}, testPrompt)
	if stop != nil {
		t.Fatalf("stop = %v", stop)
	}
	if len(drafts) != 2 {
		t.Fatalf("got %d drafts, want 2 of 3", len(drafts))
	}
	// The drop is counted, not silent, so a source that quietly loses half its
	// results shows up in the run log rather than as a quiet week.
	if len(problems) != 1 {
		t.Fatalf("got %d problems %v, want 1", len(problems), problems)
	}
	if !strings.Contains(problems[0].Error(), "https://example.com/undated") {
		t.Errorf("problem = %v, want it to name the dropped url", problems[0])
	}
}

func TestCollectStopsOnBudgetAndKeepsWhatItHas(t *testing.T) {
	var calls int
	client := &sourcestest.Client{
		SearchFunc: func(ctx context.Context, prompt string, opt anakin.SearchOpt) (json.RawMessage, error) {
			calls++
			if calls == 1 {
				return json.RawMessage(liveResponse), nil
			}
			return nil, fmt.Errorf("search: %w", anakin.ErrBudgetExceeded)
		},
	}

	drafts, stop, _ := Collect(context.Background(), client, models.SourceNews, []string{"boAt", "boat headphones", "airdopes"}, testPrompt)
	if !errors.Is(stop, anakin.ErrBudgetExceeded) {
		t.Fatalf("stop = %v, want ErrBudgetExceeded", stop)
	}
	if len(drafts) != 2 {
		t.Errorf("got %d drafts, want the 2 collected before the ceiling", len(drafts))
	}
	if calls != 2 {
		t.Errorf("made %d searches, want it to stop at the ceiling rather than try the third keyword", calls)
	}
}

func TestCollectContinuesPastOneFailedKeyword(t *testing.T) {
	client := &sourcestest.Client{
		SearchFunc: func(ctx context.Context, prompt string, opt anakin.SearchOpt) (json.RawMessage, error) {
			if prompt == "boAt news" {
				return nil, errors.New("upstream 503")
			}
			return json.RawMessage(liveResponse), nil
		},
	}

	drafts, stop, problems := Collect(context.Background(), client, models.SourceNews, []string{"boAt", "airdopes"}, testPrompt)
	if stop != nil {
		t.Fatalf("stop = %v, want a per-keyword failure not to end the run", stop)
	}
	if len(drafts) != 2 {
		t.Errorf("got %d drafts, want the second keyword's 2", len(drafts))
	}
	if len(problems) != 2 {
		t.Fatalf("got %d problems %v, want the 503 and the undated result", len(problems), problems)
	}
	if !strings.Contains(problems[0].Error(), "upstream 503") {
		t.Errorf("problems[0] = %v, want the 503", problems[0])
	}
}

func TestCollectRecordsADecodeFailure(t *testing.T) {
	client := &sourcestest.Client{
		SearchFunc: func(ctx context.Context, prompt string, opt anakin.SearchOpt) (json.RawMessage, error) {
			return json.RawMessage(`{"results":"not an array"}`), nil
		},
	}

	drafts, stop, problems := Collect(context.Background(), client, models.SourceWeb, []string{"boAt"}, testPrompt)
	if stop != nil || len(drafts) != 0 {
		t.Fatalf("stop = %v, drafts = %d", stop, len(drafts))
	}
	if len(problems) != 1 || !strings.Contains(problems[0].Error(), "decoding search") {
		t.Fatalf("problems = %v, want one decode failure", problems)
	}
}
