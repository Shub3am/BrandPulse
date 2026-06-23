// Package searchapi turns POST /v1/search responses into mention drafts.
//
// It exists because news and web are the same adapter with a different prompt
// and a different Source. The Search API has no server-side source, locale or
// freshness filter, verified live on 2026-09-20: unknown parameters are
// accepted with a 200 and silently ignored. The prompt text is the only lever
// either adapter has, so the only honest way to express them is as one
// mechanism parameterised by the prompt.
//
// It must not know which source it is collecting for. The caller says.
package searchapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"brandpulse/internal/anakin"
	"brandpulse/internal/models"
)

// Limit is results per call. The API caps it at 20, and with no server-side
// recency filter the adapter over-fetches and discards client-side, so asking
// for less only loses mentions at the same price.
const Limit = 20

// response is the Search API response body.
type response struct {
	ID      string   `json:"id"`
	Results []Result `json:"results"`
}

// Result is one search hit. Snippet is not the short line the name suggests:
// a Times of India result measured 2 966 characters of cleaned page text,
// which is ample for sentiment and intent classification.
type Result struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Snippet     string `json:"snippet"`
	Date        string `json:"date"`
	LastUpdated string `json:"last_updated"`
}

// undatedOffset is how far before the window end an undated result is dated.
//
// The window is half-open, [start, end), so a mention dated exactly at end is
// filtered out as being in the future. One second inside it is the smallest
// offset that keeps the result.
const undatedOffset = time.Second

// UndatedAt is the instant an undated result is dated to, given the collection
// window's end. Exported because a caller that maps a Result itself needs the
// same instant Collect would have used.
func UndatedAt(windowEnd time.Time) time.Time {
	return windowEnd.Add(-undatedOffset).UTC()
}

// Collect issues one Search per keyword and maps the results to drafts.
//
// windowEnd is the end of the collection window, which is what an undated
// result is dated to. It is the window rather than time.Now() so a replayed
// fixture produces the same corpus every run.
//
// stop is non-nil only for a budget ceiling, which ends the run. problems
// holds everything else that went wrong without ending it, so one bad keyword
// costs only its own results.
func Collect(ctx context.Context, c anakin.Client, source models.Source, keywords []string, prompt func(keyword string) string, windowEnd time.Time) (drafts []models.Mention, stop error, problems []error) {
	undatedAt := UndatedAt(windowEnd)

	for _, keyword := range keywords {
		raw, err := c.Search(ctx, prompt(keyword), anakin.SearchOpt{Limit: Limit})
		if err != nil {
			if errors.Is(err, anakin.ErrBudgetExceeded) {
				return drafts, err, problems
			}
			problems = append(problems, fmt.Errorf("%s: search %q: %w", source, keyword, err))
			continue
		}

		var resp response
		if err := json.Unmarshal(raw, &resp); err != nil {
			problems = append(problems, fmt.Errorf("%s: decoding search %q: %w", source, keyword, err))
			continue
		}

		for _, result := range resp.Results {
			draft, err := ToDraft(result, source, keyword, undatedAt)
			if err != nil {
				problems = append(problems, err)
				continue
			}
			drafts = append(drafts, draft)
		}
	}
	return drafts, nil, problems
}

// ToDraft maps one result onto a mention with everything except its identity,
// which mentions.Stamp adds. undatedAt is what a result carrying no date is
// dated to; see UndatedAt.
func ToDraft(r Result, source models.Source, keyword string, undatedAt time.Time) (models.Mention, error) {
	postedAt, precision, err := publishedAt(r, undatedAt)
	if err != nil {
		return models.Mention{}, fmt.Errorf("%s: %s: %w", source, r.URL, err)
	}

	text := r.Snippet
	if r.Title != "" {
		text = r.Title + "\n" + r.Snippet
	}

	return models.Mention{
		Source: source,
		// Search returns no id of its own. The URL is what identifies a result
		// and is what a second run would match it on.
		ExternalID: r.URL,
		URL:        r.URL,
		Text:       text,
		// Search exposes no language field. The corpus is Indian English.
		Lang:           "en",
		PostedAt:       postedAt,
		MatchedKeyword: keyword,
		Raw: map[string]any{
			"date":         r.Date,
			"last_updated": r.LastUpdated,
			// "unknown" marks a mention whose PostedAt is the window end
			// rather than a publish date, so a consumer that needs real dates
			// can exclude it without re-reading the raw fields.
			"posted_at_precision": precision,
		},
	}, nil
}

// publishedAt reads a result's date, preferring date over last_updated, and
// returns the instant alongside the precision it was stated at.
//
// Both fields are blank far more often than the first measurement suggested:
// 66 of the 120 results recorded on 2026-09-20 carry neither. Dropping those
// is how a live recording produced zero web and zero news mentions, so an
// undated result is dated to undatedAt and marked "unknown" instead. Search
// has no recency filter, so a result was returned for a query asked now and
// the window end is the only defensible instant available.
//
// A non-empty value that will not parse stays an error. That is a shape this
// adapter has not seen, and guessing at it would put a real article on the
// wrong day rather than on a day flagged as a guess.
func publishedAt(r Result, undatedAt time.Time) (time.Time, string, error) {
	for _, value := range []string{r.Date, r.LastUpdated} {
		if value == "" {
			continue
		}
		parsed, err := time.Parse(time.DateOnly, value)
		if err != nil {
			return time.Time{}, "", fmt.Errorf("unparseable date %q", value)
		}
		return parsed.UTC(), "day", nil
	}
	return undatedAt, "unknown", nil
}
