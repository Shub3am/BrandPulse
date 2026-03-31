// Package sourcestest is a stub anakin.Client for adapter tests.
//
// It exists so every adapter test runs with no API key, no fixture directory
// and no network, and so the same twenty lines are not copied into six test
// files. It is test-only despite being an ordinary package, in the same way as
// net/http/httptest: nothing under agents/ or demo/ may import it.
//
// It records what it was asked for, because half of what an adapter test needs
// to prove is that the right action id and the right parameters went out, not
// only that the response came back parsed.
package sourcestest

import (
	"context"
	"encoding/json"
	"fmt"

	"brandpulse/internal/anakin"
)

// Call is one request an adapter made. Only the fields belonging to Method are
// populated.
type Call struct {
	Method    string
	Platform  string
	Query     string
	URL       string
	WireOpt   anakin.WireOpt
	SearchOpt anakin.SearchOpt
	ScrapeOpt anakin.ScrapeOpt
}

// Client answers from the funcs a test sets and records every call. A nil func
// means the test did not expect that surface to be used, and reaching it is an
// error rather than a zero value, so a misrouted adapter fails loudly.
type Client struct {
	WireFunc   func(ctx context.Context, platform, query string, opt anakin.WireOpt) (json.RawMessage, error)
	SearchFunc func(ctx context.Context, prompt string, opt anakin.SearchOpt) (json.RawMessage, error)
	ScrapeFunc func(ctx context.Context, url string, opt anakin.ScrapeOpt) (json.RawMessage, error)

	// Spend is what Stats reports, so a test can assert a collector read it.
	Spend anakin.Stats

	Calls []Call
}

// Wire records the call and answers from WireFunc.
func (c *Client) Wire(ctx context.Context, platform, query string, opt anakin.WireOpt) (json.RawMessage, error) {
	c.Calls = append(c.Calls, Call{Method: "Wire", Platform: platform, Query: query, WireOpt: opt})
	if c.WireFunc == nil {
		return nil, fmt.Errorf("sourcestest: unexpected Wire(%q, %q)", platform, query)
	}
	return c.WireFunc(ctx, platform, query, opt)
}

// Search records the call and answers from SearchFunc.
func (c *Client) Search(ctx context.Context, prompt string, opt anakin.SearchOpt) (json.RawMessage, error) {
	c.Calls = append(c.Calls, Call{Method: "Search", Query: prompt, SearchOpt: opt})
	if c.SearchFunc == nil {
		return nil, fmt.Errorf("sourcestest: unexpected Search(%q)", prompt)
	}
	return c.SearchFunc(ctx, prompt, opt)
}

// Scrape records the call and answers from ScrapeFunc.
func (c *Client) Scrape(ctx context.Context, url string, opt anakin.ScrapeOpt) (json.RawMessage, error) {
	c.Calls = append(c.Calls, Call{Method: "Scrape", URL: url, ScrapeOpt: opt})
	if c.ScrapeFunc == nil {
		return nil, fmt.Errorf("sourcestest: unexpected Scrape(%q)", url)
	}
	return c.ScrapeFunc(ctx, url, opt)
}

// Map is never used by an adapter; bp-onboarder is the only caller.
func (c *Client) Map(ctx context.Context, url string, opt anakin.MapOpt) (json.RawMessage, error) {
	return nil, fmt.Errorf("sourcestest: Map is not stubbed")
}

// Crawl is never used by an adapter; bp-onboarder is the only caller.
func (c *Client) Crawl(ctx context.Context, url string, opt anakin.CrawlOpt) (json.RawMessage, error) {
	return nil, fmt.Errorf("sourcestest: Crawl is not stubbed")
}

// Stats reports Spend.
func (c *Client) Stats() anakin.Stats { return c.Spend }

// Queries returns the query or prompt of every recorded call, which is the
// assertion most adapter tests actually want.
func (c *Client) Queries() []string {
	out := make([]string, 0, len(c.Calls))
	for _, call := range c.Calls {
		if call.Method == "Scrape" {
			out = append(out, call.URL)
			continue
		}
		out = append(out, call.Query)
	}
	return out
}
