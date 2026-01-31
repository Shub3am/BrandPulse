package web

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

var (
	windowStart = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	windowEnd   = time.Date(2020, 1, 15, 0, 0, 0, 0, time.UTC)
)

func profile(keywords ...string) models.BrandProfile {
	return models.BrandProfile{BrandID: "boat-lifestyle", Keywords: keywords, Version: 1}
}

// searchWith builds a /v1/search response of results carrying the given
// snippets, in the shape recorded in docs/research/wire-schemas.md §5.
func searchWith(snippets ...string) json.RawMessage {
	results := make([]searchapiResult, 0, len(snippets))
	for i, snippet := range snippets {
		results = append(results, searchapiResult{
			URL:     fmt.Sprintf("https://example.com/%d", i),
			Title:   "",
			Snippet: snippet,
			Date:    "2019-06-14",
		})
	}
	body, err := json.Marshal(map[string]any{"id": "search_1", "results": results})
	if err != nil {
		panic(err)
	}
	return body
}

type searchapiResult struct {
	URL     string `json:"url"`
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
	Date    string `json:"date"`
}

func longSnippet() string  { return strings.Repeat("a", minSnippet) }
func shortSnippet() string { return strings.Repeat("a", minSnippet-1) }

func TestPromptIsThePlainKeyword(t *testing.T) {
	// The prompt carries no sentiment-bearing word. news owns " news"; anything
	// like " complaints" here would skew the distribution the product reports.
	if got := prompt("boAt"); got != "boAt" {
		t.Errorf("prompt = %q, want %q", got, "boAt")
	}
}

func TestFetchScrapesOnlyShortSnippets(t *testing.T) {
	client := &sourcestest.Client{
		SearchFunc: func(ctx context.Context, prompt string, opt anakin.SearchOpt) (json.RawMessage, error) {
			return searchWith(longSnippet(), shortSnippet(), longSnippet()), nil
		},
		ScrapeFunc: func(ctx context.Context, url string, opt anakin.ScrapeOpt) (json.RawMessage, error) {
			return json.RawMessage(`{"markdown":"the full page body","cached":false,"durationMs":900}`), nil
		},
	}

	if _, err := Fetch(context.Background(), client, profile("boAt"), windowStart, windowEnd); err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	var scraped []string
	for _, call := range client.Calls {
		if call.Method == "Scrape" {
			scraped = append(scraped, call.URL)
		}
	}
	if len(scraped) != 1 || scraped[0] != "https://example.com/1" {
		t.Fatalf("scraped %v, want only the short result", scraped)
	}
}

func TestFetchAsksForMarkdown(t *testing.T) {
	client := &sourcestest.Client{
		SearchFunc: func(ctx context.Context, prompt string, opt anakin.SearchOpt) (json.RawMessage, error) {
			return searchWith(shortSnippet()), nil
		},
		ScrapeFunc: func(ctx context.Context, url string, opt anakin.ScrapeOpt) (json.RawMessage, error) {
			if len(opt.Formats) != 1 || opt.Formats[0] != "markdown" {
				t.Errorf("Formats = %v, want [markdown]", opt.Formats)
			}
			if opt.Country != "in" {
				t.Errorf("Country = %q, want \"in\"", opt.Country)
			}
			return json.RawMessage(`{"markdown":"the full page body"}`), nil
		},
	}

	if _, err := Fetch(context.Background(), client, profile("boAt"), windowStart, windowEnd); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
}

func TestEnrichReplacesTextAndRecordsTheScrape(t *testing.T) {
	client := &sourcestest.Client{
		ScrapeFunc: func(ctx context.Context, url string, opt anakin.ScrapeOpt) (json.RawMessage, error) {
			return json.RawMessage(`{"markdown":"the full page body"}`), nil
		},
	}
	drafts := []models.Mention{{URL: "https://example.com/0", Text: shortSnippet(), Raw: map[string]any{}}}

	stop, problems := enrich(context.Background(), client, drafts)
	if stop != nil || len(problems) != 0 {
		t.Fatalf("stop = %v, problems = %v", stop, problems)
	}
	if drafts[0].Text != "the full page body" {
		t.Errorf("Text = %q, want the scraped body", drafts[0].Text)
	}
	if drafts[0].Raw["scraped"] != true {
		t.Errorf("Raw = %v, want the scrape recorded", drafts[0].Raw)
	}
}

func TestEnrichKeepsTheSnippetWhenAScrapeFails(t *testing.T) {
	client := &sourcestest.Client{
		ScrapeFunc: func(ctx context.Context, url string, opt anakin.ScrapeOpt) (json.RawMessage, error) {
			return nil, errors.New("upstream 502")
		},
	}
	drafts := []models.Mention{{URL: "https://example.com/0", Text: "thin", Raw: map[string]any{}}}

	stop, problems := enrich(context.Background(), client, drafts)
	if stop != nil {
		t.Fatalf("stop = %v, want a failed scrape not to end the run", stop)
	}
	// A thin mention beats no mention, but the failure is still reported.
	if drafts[0].Text != "thin" {
		t.Errorf("Text = %q, want the snippet kept", drafts[0].Text)
	}
	if len(problems) != 1 || !strings.Contains(problems[0].Error(), "upstream 502") {
		t.Fatalf("problems = %v, want the 502 reported", problems)
	}
}

func TestEnrichRejectsAnEmptyScrape(t *testing.T) {
	client := &sourcestest.Client{
		ScrapeFunc: func(ctx context.Context, url string, opt anakin.ScrapeOpt) (json.RawMessage, error) {
			return json.RawMessage(`{"html":"","cleanedHtml":"","markdown":""}`), nil
		},
	}
	drafts := []models.Mention{{URL: "https://example.com/0", Text: "thin", Raw: map[string]any{}}}

	stop, problems := enrich(context.Background(), client, drafts)
	if stop != nil {
		t.Fatalf("stop = %v", stop)
	}
	if drafts[0].Text != "thin" {
		t.Errorf("Text = %q, want the snippet kept", drafts[0].Text)
	}
	// A 200 with every format empty is the failure that looks like success.
	if len(problems) != 1 {
		t.Fatalf("problems = %v, want the empty response reported", problems)
	}
}

func TestEnrichStopsAtTheScrapeCap(t *testing.T) {
	var calls int
	client := &sourcestest.Client{
		ScrapeFunc: func(ctx context.Context, url string, opt anakin.ScrapeOpt) (json.RawMessage, error) {
			calls++
			return json.RawMessage(`{"markdown":"body"}`), nil
		},
	}
	drafts := make([]models.Mention, maxEnrichments+3)
	for i := range drafts {
		drafts[i] = models.Mention{URL: fmt.Sprintf("https://example.com/%d", i), Text: "thin", Raw: map[string]any{}}
	}

	if stop, problems := enrich(context.Background(), client, drafts); stop != nil || len(problems) != 0 {
		t.Fatalf("stop = %v, problems = %v", stop, problems)
	}
	if calls != maxEnrichments {
		t.Errorf("made %d scrapes, want the budgeted %d", calls, maxEnrichments)
	}
	if drafts[maxEnrichments].Text != "thin" {
		t.Errorf("draft past the cap was enriched anyway: %q", drafts[maxEnrichments].Text)
	}
}

func TestEnrichStopsOnTheBudgetCeiling(t *testing.T) {
	client := &sourcestest.Client{
		ScrapeFunc: func(ctx context.Context, url string, opt anakin.ScrapeOpt) (json.RawMessage, error) {
			return nil, fmt.Errorf("scrape: %w", anakin.ErrBudgetExceeded)
		},
	}
	drafts := []models.Mention{
		{URL: "https://example.com/0", Text: "thin", Raw: map[string]any{}},
		{URL: "https://example.com/1", Text: "thin", Raw: map[string]any{}},
	}

	stop, _ := enrich(context.Background(), client, drafts)
	if !errors.Is(stop, anakin.ErrBudgetExceeded) {
		t.Fatalf("stop = %v, want ErrBudgetExceeded", stop)
	}
	if len(client.Calls) != 1 {
		t.Errorf("made %d scrapes, want it to stop at the ceiling", len(client.Calls))
	}
}

func TestFetchPropagatesTheBudgetCeilingFromSearch(t *testing.T) {
	client := &sourcestest.Client{
		SearchFunc: func(ctx context.Context, prompt string, opt anakin.SearchOpt) (json.RawMessage, error) {
			return nil, fmt.Errorf("search: %w", anakin.ErrBudgetExceeded)
		},
	}

	_, err := Fetch(context.Background(), client, profile("boAt"), windowStart, windowEnd)
	if !errors.Is(err, anakin.ErrBudgetExceeded) {
		t.Fatalf("err = %v, want ErrBudgetExceeded to survive the error join", err)
	}
	// A ceiling hit during Search must not then spend on scrapes.
	for _, call := range client.Calls {
		if call.Method == "Scrape" {
			t.Errorf("scraped after the budget ceiling: %s", call.URL)
		}
	}
}
