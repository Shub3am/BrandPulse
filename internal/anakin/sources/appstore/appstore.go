// Package appstore turns Apple's public customer review feed into mentions.
//
// Wire has no App Store action, so the source is the documented RSS feed at
// itunes.apple.com, fetched through the URL Scraper because Anakin is the data
// layer. The feed is keyed by app id, not by keyword: a brand's App Store
// reviews come from p.SourceHandles["appstore"] and nothing else, which is why
// this adapter makes exactly one call whatever the keyword list looks like.
//
// It must not know what a sentiment or a topic is. It emits raw mentions.
package appstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"brandpulse/internal/anakin"
	"brandpulse/internal/anakin/sources/mentions"
	"brandpulse/internal/anakin/sources/scrape"
	"brandpulse/internal/models"
)

// feedURL is the Indian storefront's most-recent review feed for one app.
//
// The storefront is fixed to /in/ because the product is Indian D2C and a
// review left in another country is not a mention this brand's team can act on.
const feedURL = "https://itunes.apple.com/in/rss/customerreviews/id=%s/sortBy=mostRecent/json"

// label is Apple's wrapper around every scalar in the feed.
type label struct {
	Label string `json:"label"`
}

// feedResponse is the review feed. entry is absent, not empty, for an app with
// no reviews in this storefront, and absent for many apps that plainly do have
// reviews; see Fetch.
type feedResponse struct {
	Feed struct {
		Entries []entry `json:"entry"`
	} `json:"feed"`
}

// entry is one customer review.
type entry struct {
	Author struct {
		Name label `json:"name"`
		URI  label `json:"uri"`
	} `json:"author"`
	Updated   label `json:"updated"`
	Rating    label `json:"im:rating"`
	Version   label `json:"im:version"`
	ID        label `json:"id"`
	Title     label `json:"title"`
	Content   label `json:"content"`
	VoteSum   label `json:"im:voteSum"`
	VoteCount label `json:"im:voteCount"`
	Link      struct {
		Attributes struct {
			Href string `json:"href"`
		} `json:"attributes"`
	} `json:"link"`
}

// Fetch collects App Store review mentions for p between start and end.
//
// An app id that returns no entry key is an error rather than an empty result.
// Measured on 2026-09-20: whether the feed carries entries depends on the exact
// URL form as much as on the app, and the same id answered 0 on one form and 50
// on another within the same session. Reporting that as a quiet fortnight would
// hide a broken handle.
func Fetch(ctx context.Context, c anakin.Client, p models.BrandProfile, start, end time.Time) ([]models.Mention, error) {
	appID := p.SourceHandles[string(models.SourceAppstore)]
	if appID == "" {
		return nil, fmt.Errorf("appstore: brand %q carries no appstore handle; the feed is keyed by app id, not by keyword", p.BrandID)
	}

	url := fmt.Sprintf(feedURL, appID)
	raw, err := c.Scrape(ctx, url, anakin.ScrapeOpt{Formats: []string{"html"}, Country: "in"})
	if err != nil {
		return nil, fmt.Errorf("appstore: scraping %s: %w", url, err)
	}

	resp, err := scrape.Decode(raw)
	if err != nil {
		return nil, fmt.Errorf("appstore: %w", err)
	}

	// html and only html. markdown escapes the feed's `[` as `\[` and
	// cleanedHtml turns every quote into &#34;, so both are JSON-shaped text
	// that json.Unmarshal rejects. Measured live on 2026-09-20.
	var feed feedResponse
	if err := json.Unmarshal([]byte(resp.HTML), &feed); err != nil {
		return nil, fmt.Errorf("appstore: decoding the feed for app %s: %w", appID, err)
	}
	if len(feed.Feed.Entries) == 0 {
		return nil, fmt.Errorf("appstore: the feed for app %s carries no entries; check the app id and the feed url form", appID)
	}

	var drafts []models.Mention
	var problems []error
	for _, e := range feed.Feed.Entries {
		draft, err := toDraft(e, appID)
		if err != nil {
			problems = append(problems, err)
			continue
		}
		drafts = append(drafts, draft)
	}

	out, dropped := mentions.Finalize(drafts, p, start, end)
	if len(dropped.Invalid) > 0 {
		problems = append(problems, fmt.Errorf("appstore: %d mentions failed validation: %s",
			len(dropped.Invalid), strings.Join(dropped.Invalid, "; ")))
	}
	return out, errors.Join(problems...)
}

// toDraft maps one review onto a mention with everything except its identity.
func toDraft(e entry, appID string) (models.Mention, error) {
	postedAt, err := time.Parse(time.RFC3339, e.Updated.Label)
	if err != nil {
		return models.Mention{}, fmt.Errorf("appstore: review %s: unparseable updated %q: %w", e.ID.Label, e.Updated.Label, err)
	}

	rating, err := strconv.ParseFloat(e.Rating.Label, 64)
	if err != nil {
		return models.Mention{}, fmt.Errorf("appstore: review %s: unparseable im:rating %q: %w", e.ID.Label, e.Rating.Label, err)
	}

	text := e.Content.Label
	if e.Title.Label != "" {
		text = e.Title.Label + "\n" + e.Content.Label
	}

	return models.Mention{
		Source:     models.SourceAppstore,
		ExternalID: e.ID.Label,
		URL:        e.Link.Attributes.Href,
		Author:     e.Author.Name.Label,
		// Apple exposes no follower or reviewer-reputation count.
		AuthorFollowers: 0,
		Text:            text,
		// The feed carries no language field. The /in/ storefront is read for
		// an Indian English corpus.
		Lang:     "en",
		PostedAt: postedAt.UTC(),
		Engagement: models.Engagement{
			// voteSum is the net helpful score, which is the only thing in the
			// feed that measures other people reacting to the review.
			Likes: votes(e.VoteSum.Label),
		},
		Rating: &rating,
		Raw: map[string]any{
			"app_id":      appID,
			"app_version": e.Version.Label,
			"vote_count":  e.VoteCount.Label,
			"author_uri":  e.Author.URI.Label,
		},
	}, nil
}

// votes parses one of Apple's string counters. An unparseable value is 0, which
// Engagement already means as "unknown".
func votes(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return n
}
