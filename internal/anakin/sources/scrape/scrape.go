// Package scrape decodes a URL Scraper response body.
//
// It exists so the three adapters that scrape, web, appstore and playstore,
// share one definition of the response. The shape is documented in
// docs/research/anakin.md §4.
//
// UNVERIFIED: whether the response arrives at the top level or inside an
// envelope. internal/anakin strips the Wire job envelope before an adapter
// sees a Wire payload, and this package assumes it does the same here. The
// first live Scrape of Task 7 settles it, and settling it wrong is a one-line
// fix in this file rather than a change in three adapters. That is the whole
// reason this package exists rather than three copies of the struct.
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
type Response struct {
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
