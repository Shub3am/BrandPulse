// Package reddit turns Wire's rt_search response into mentions.
//
// The response shape here was read live on 2026-09-20 and is written down in
// docs/research/wire-schemas.md §3. Every json tag below comes from that file.
// This package must not guess a field name: a guess unmarshals to a zero value
// with no error, which is an empty dashboard nobody notices until the demo.
//
// It must not know what a sentiment or a topic is. It emits raw mentions.
package reddit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"brandpulse/internal/anakin"
	"brandpulse/internal/anakin/sources/mentions"
	"brandpulse/internal/models"
)

// searchResponse is the rt_search payload, after internal/anakin has stripped
// the Wire job envelope.
type searchResponse struct {
	Query string `json:"query"`
	Posts []post `json:"posts"`

	// PostCount and PostsDroppedByFilter are how a caller tells "Reddit had
	// nothing" from "the match filter ate everything". Both read zero on the
	// first and the counter is non-zero on the second.
	PostCount            int `json:"post_count"`
	PostsDroppedByFilter int `json:"posts_dropped_by_filter"`
}

// post is one Reddit submission.
//
// score, upvote_ratio and num_comments are deliberately absent: they came back
// null on all 15 posts measured, because this path is backed by Reddit's RSS,
// which carries no counters. Mention.Engagement stays zeroed rather than
// inventing them, and recovering them would cost rt_post_details at 2 credits
// per post.
type post struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Subreddit  string `json:"subreddit"`
	Author     string `json:"author"`
	CreatedUTC string `json:"created_utc"`
	URL        string `json:"url"`
	Permalink  string `json:"permalink"`
	Selftext   string `json:"selftext"`
}

// rssTail matches the "submitted by /u/x to r/y [link] [comments]" footer that
// Reddit's RSS appends to every selftext. It has to go before the text is
// hashed, or the same post found by two keywords hashes to two rows.
var rssTail = regexp.MustCompile(`(?s)\s*submitted by\s+/u/.*$`)

// Fetch collects Reddit mentions for p between start and end.
//
// It issues one rt_search per keyword. That is not a missed optimisation:
// rt_search runs with match=true, which is a whole-phrase filter, so a query
// combining two brand terms is a guaranteed empty result. Measured 2026-09-20,
// "boAt vs Noise vs boult earbuds" returned post_count 0 with 22 posts dropped
// by the filter.
//
// p.Hashtags are not searched. Reddit has no hashtag convention, and a "#boAt"
// phrase filter matches nothing.
func Fetch(ctx context.Context, c anakin.Client, p models.BrandProfile, start, end time.Time) ([]models.Mention, error) {
	var drafts []models.Mention
	var problems []error

	for _, keyword := range p.Keywords {
		raw, err := c.Wire(ctx, "reddit", keyword, anakin.WireOpt{
			Action: "rt_search",
			Limit:  100,
			Sort:   "new",
			Time:   timeBucket(start, end),
			Params: map[string]any{"match": true},
		})
		if err != nil {
			// A budget ceiling is a stop, not a failure: every later query
			// would hit the same wall. The collector turns this into
			// MentionBatch.Truncated and keeps what came back.
			if errors.Is(err, anakin.ErrBudgetExceeded) {
				return finalize(drafts, p, start, end, problems, err)
			}
			problems = append(problems, fmt.Errorf("reddit: rt_search %q: %w", keyword, err))
			continue
		}

		var resp searchResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			problems = append(problems, fmt.Errorf("reddit: decoding rt_search %q: %w", keyword, err))
			continue
		}
		if len(resp.Posts) == 0 && resp.PostsDroppedByFilter > 0 {
			problems = append(problems, fmt.Errorf(
				"reddit: %q matched nothing but the match filter dropped %d posts, so the query is too specific, not the week",
				keyword, resp.PostsDroppedByFilter))
			continue
		}

		for _, post := range resp.Posts {
			draft, err := toDraft(post, keyword)
			if err != nil {
				problems = append(problems, err)
				continue
			}
			drafts = append(drafts, draft)
		}
	}

	return finalize(drafts, p, start, end, problems, nil)
}

// finalize applies the shared close and folds the collected problems into one
// error, keeping stop, the reason a run ended early, at the head of the chain
// so errors.Is finds it.
func finalize(drafts []models.Mention, p models.BrandProfile, start, end time.Time, problems []error, stop error) ([]models.Mention, error) {
	out, dropped := mentions.Finalize(drafts, p, start, end)
	if len(dropped.Invalid) > 0 {
		problems = append(problems, fmt.Errorf("reddit: %d mentions failed validation: %s",
			len(dropped.Invalid), strings.Join(dropped.Invalid, "; ")))
	}
	if stop != nil {
		return out, errors.Join(append([]error{stop}, problems...)...)
	}
	return out, errors.Join(problems...)
}

// toDraft maps one post onto a mention with everything except its identity,
// which mentions.Stamp adds.
func toDraft(p post, keyword string) (models.Mention, error) {
	// created_utc is an RFC 3339 string despite the name. It is not an epoch.
	postedAt, err := time.Parse(time.RFC3339, p.CreatedUTC)
	if err != nil {
		return models.Mention{}, fmt.Errorf("reddit: post %s has unparseable created_utc %q: %w", p.ID, p.CreatedUTC, err)
	}

	url := p.URL
	if url == "" && p.Permalink != "" {
		url = "https://www.reddit.com" + p.Permalink
	}

	return models.Mention{
		Source:     models.SourceReddit,
		ExternalID: p.ID,
		URL:        url,
		Author:     p.Author,
		Text:       text(p),
		// Reddit exposes no language field. The corpus is Indian English and
		// Hinglish, both of which the enricher reads as "en". There is no
		// default in Go, so this is set rather than left to the zero value.
		Lang:           "en",
		PostedAt:       postedAt.UTC(),
		MatchedKeyword: keyword,
		Raw:            map[string]any{"subreddit": p.Subreddit},
	}, nil
}

// text joins the title to the body because 3 of the 15 posts measured carried
// the complaint in the title and had a thin body.
func text(p post) string {
	body := strings.TrimSpace(rssTail.ReplaceAllString(p.Selftext, ""))
	if body == "" {
		return p.Title
	}
	return p.Title + "\n" + body
}

// timeBucket picks the smallest rt_search time window that still covers
// [start, end). The parameter is coarse, so mentions.Filter trims the result
// to the exact window afterwards.
func timeBucket(start, end time.Time) string {
	span := end.Sub(start)
	switch {
	case span <= time.Hour:
		return "hour"
	case span <= 24*time.Hour:
		return "day"
	case span <= 7*24*time.Hour:
		return "week"
	case span <= 31*24*time.Hour:
		return "month"
	case span <= 366*24*time.Hour:
		return "year"
	default:
		return "all"
	}
}
