// Package web collects general web mentions through the Search API and fills
// in a body for the few results that come back thin.
//
// It is searchapi with the plain keyword as the prompt, plus the Search to URL
// Scraper chain SOURCE-STRATEGY.md calls for. Task 1 narrowed that chain: a
// Search snippet measured 2 966 characters of cleaned page text, which is ample
// for classification, so scraping every result would be paying a credit each to
// replace text the classifier was already going to be happy with. The chain
// runs only where a snippet came back short.
//
// It must not know what a sentiment or a topic is. It emits raw mentions.
package web

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"brandpulse/internal/anakin"
	"brandpulse/internal/anakin/sources/mentions"
	"brandpulse/internal/anakin/sources/scrape"
	"brandpulse/internal/anakin/sources/searchapi"
	"brandpulse/internal/models"
)

// minSnippet is the length below which a result is worth a scrape, in runes.
//
// A headline plus a one-line lede lands around 200; the full-page snippets that
// make scraping unnecessary measured thousands. Anything in between is a
// judgement call and this constant is where it is written down.
const minSnippet = 500

// maxEnrichments caps scrapes per Fetch. URL Scraper is 1 credit a call and the
// recording budget in docs/research/anakin.md §6 allots 5 to this chain, so the
// cap is the budget rather than a guess. Short results are scraped in the order
// Search returned them, which is relevance order.
const maxEnrichments = 5

// Fetch collects web mentions for p between start and end.
func Fetch(ctx context.Context, c anakin.Client, p models.BrandProfile, start, end time.Time) ([]models.Mention, error) {
	drafts, stop, problems := searchapi.Collect(ctx, c, models.SourceWeb, p.Keywords, prompt)

	// Bodies are filled in before the window and negative-keyword filters run,
	// so both read the same text the classifier will, and so a page whose
	// snippet hid a negative keyword is still caught.
	if stop == nil {
		enrichStop, errs := enrich(ctx, c, drafts)
		problems = append(problems, errs...)
		stop = enrichStop
	}

	out, dropped := mentions.Finalize(drafts, p, start, end)
	if len(dropped.Invalid) > 0 {
		problems = append(problems, fmt.Errorf("web: %d mentions failed validation: %s",
			len(dropped.Invalid), strings.Join(dropped.Invalid, "; ")))
	}
	if stop != nil {
		return out, errors.Join(append([]error{stop}, problems...)...)
	}
	return out, errors.Join(problems...)
}

// prompt searches the plain keyword.
//
// news already owns the press-coverage prompt, and this adapter is the rest of
// the web. Steering it with words like "review" or "complaint" would bias the
// corpus towards one sentiment, and every number this product reports is a
// distribution over that corpus.
func prompt(keyword string) string { return keyword }

// enrich replaces the text of short drafts with the scraped page, in place.
//
// stop is non-nil only for a budget ceiling. A page that fails to scrape keeps
// its snippet: a thin mention is worth more than no mention, and the failure is
// reported rather than swallowed.
func enrich(ctx context.Context, c anakin.Client, drafts []models.Mention) (stop error, problems []error) {
	var spent int
	for i := range drafts {
		if spent == maxEnrichments {
			return nil, problems
		}
		if utf8.RuneCountInString(drafts[i].Text) >= minSnippet {
			continue
		}

		raw, err := c.Scrape(ctx, drafts[i].URL, anakin.ScrapeOpt{Formats: []string{"markdown"}, Country: "in"})
		spent++
		if err != nil {
			if errors.Is(err, anakin.ErrBudgetExceeded) {
				return err, problems
			}
			problems = append(problems, fmt.Errorf("web: scraping %s: %w", drafts[i].URL, err))
			continue
		}

		resp, err := scrape.Decode(raw)
		if err != nil {
			problems = append(problems, fmt.Errorf("web: scraping %s: %w", drafts[i].URL, err))
			continue
		}
		if resp.Markdown == "" {
			problems = append(problems, fmt.Errorf("web: scraping %s: response carries no markdown", drafts[i].URL))
			continue
		}

		drafts[i].Text = resp.Markdown
		drafts[i].Raw["scraped"] = true
	}
	return nil, problems
}
