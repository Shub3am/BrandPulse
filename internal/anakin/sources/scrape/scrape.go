// Package scrape decodes a URL Scraper response body.
//
// It exists so the three adapters that scrape, web, appstore and playstore,
// share one definition of the response. The shape is documented in
// docs/research/anakin.md §4.
//
// POST /v1/url-scraper/scrape returns these fields at the top level with no
// envelope, read live on 2026-09-20. The one assumption left is that
// internal/anakin passes the body through, which is what it does for Wire once
// the job envelope is off.
package scrape

import (
	"encoding/json"
	"fmt"
)

// Response is what POST /v1/url-scraper/scrape returns.
//
// Which field an adapter reads is not a matter of taste. cleanedHtml is lossy:
// on a Play Store listing it dropped the per-review star rating, which lives
// in an aria-label on an empty div, and the reviewer name. Markdown drops the
// rating too and reorders the review date next to the developer's reply. An
// adapter that needs an attribute rather than a text node must take HTML.
//
// The same holds for a URL that serves JSON rather than a page, measured on the
// App Store review feed on 2026-09-20. Only html carries the document verbatim:
// markdown escapes `[` as `\[` and cleanedHtml turns every quote into `&#34;`,
// so both are JSON-shaped text that json.Unmarshal rejects.
type Response struct {
	ID            string          `json:"id"`
	Status        string          `json:"status"`
	URL           string          `json:"url"`
	Country       string          `json:"country"`
	HTML          string          `json:"html"`
	CleanedHTML   string          `json:"cleanedHtml"`
	Markdown      string          `json:"markdown"`
	GeneratedJSON json.RawMessage `json:"generatedJson"`
	Cached        bool            `json:"cached"`
	DurationMS    int             `json:"durationMs"`
}

// Decode unmarshals a scrape response and refuses one that carries no content
// at all.
//
// A 200 with every format empty is the failure that looks like success: the
// adapter above would report a quiet week instead of a broken selector or a
// blocked fetch.
func Decode(raw json.RawMessage) (Response, error) {
	var resp Response
	if err := json.Unmarshal(raw, &resp); err != nil {
		return Response{}, fmt.Errorf("scrape: decoding response: %w", err)
	}
	if resp.HTML == "" && resp.Markdown == "" && resp.CleanedHTML == "" {
		return Response{}, fmt.Errorf("scrape: response carries no html, cleanedHtml or markdown")
	}
	return resp, nil
}
