// Package playstore turns a Play Store listing into review mentions.
//
// Wire has no Play action, so the source is the public listing fetched through
// the URL Scraper with a real browser. It is keyed by package name, not by
// keyword: reviews come from p.SourceHandles["playstore"] and nothing else.
//
// The listing renders exactly three reviews and no method moved that number,
// measured three ways in docs/research/playstore-probe.md. This source is
// coverage, not volume, and nothing downstream should read three reviews as a
// trend.
//
// It must not know what a sentiment or a topic is. It emits raw mentions.
package playstore

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"

	"brandpulse/internal/anakin"
	"brandpulse/internal/anakin/sources/mentions"
	"brandpulse/internal/anakin/sources/scrape"
	"brandpulse/internal/models"
)

// listingURL is the app's Play listing in the Indian English locale.
//
// hl and gl are not decoration. Without them the listing can serve another
// country's review set, which is the failure that killed the Amazon source in
// Task 1: reviews in the wrong language for a brand that does not sell there.
const listingURL = "https://play.google.com/store/apps/details?id=%s&hl=en_IN&gl=IN"

// The class names below are Google's obfuscated build output and they will
// rotate. They are not an API, and when they change these selectors match
// nothing, which is why a parse that yields no reviews is an error rather than
// an empty batch. The fixtures recorded in Task 7 keep replay mode green
// regardless.
const (
	reviewClass = "EGFGHd" // the container for one review
	authorClass = "X5PpBb"
	ratingClass = "iXRFPc"
	dateClass   = "bp9Aid"
	bodyClass   = "h3YV2d"
)

// reviewIDAttr and thumbsAttr are data attributes rather than classes, and are
// the two selectors here that read like something Google meant for a consumer.
const (
	reviewIDAttr = "data-review-id"
	thumbsAttr   = "data-original-thumbs-up-count"
)

// review is one review as it comes out of the DOM, before any parsing. Every
// field is a string because every field in HTML is.
type review struct {
	ID     string
	Author string
	Text   string
	Rating string
	Date   string
	Thumbs string
}

// Fetch collects Play Store review mentions for p between start and end.
func Fetch(ctx context.Context, c anakin.Client, p models.BrandProfile, start, end time.Time) ([]models.Mention, error) {
	pkg := p.SourceHandles[string(models.SourcePlaystore)]
	if pkg == "" {
		return nil, fmt.Errorf("playstore: brand %q carries no playstore handle; the listing is keyed by package name, not by keyword", p.BrandID)
	}

	url := fmt.Sprintf(listingURL, pkg)
	// useBrowser is mandatory and costs nothing extra: without it the reviews
	// are not in the DOM at all. Both formats are requested because that is the
	// call shape verified live; markdown is then ignored, because it drops the
	// star rating entirely and moves the review date next to the developer's
	// reply where it reads as the reply date.
	raw, err := c.Scrape(ctx, url, anakin.ScrapeOpt{
		Formats:    []string{"markdown", "html"},
		UseBrowser: true,
		Country:    "in",
	})
	if err != nil {
		return nil, fmt.Errorf("playstore: scraping %s: %w", url, err)
	}

	resp, err := scrape.Decode(raw)
	if err != nil {
		return nil, fmt.Errorf("playstore: %w", err)
	}

	// cleanedHtml is not a substitute either: Anakin's cleaner strips the
	// aria-label star div and the reviewer name.
	found, err := parseReviews(resp.HTML)
	if err != nil {
		return nil, fmt.Errorf("playstore: parsing the listing for %s: %w", pkg, err)
	}
	if len(found) == 0 {
		return nil, fmt.Errorf("playstore: the listing for %s yielded no reviews; Google's class names have most likely rotated", pkg)
	}

	var drafts []models.Mention
	var problems []error
	for _, r := range found {
		draft, err := toDraft(r, pkg, url)
		if err != nil {
			problems = append(problems, err)
			continue
		}
		drafts = append(drafts, draft)
	}

	out, dropped := mentions.Finalize(drafts, p, start, end)
	if len(dropped.Invalid) > 0 {
		problems = append(problems, fmt.Errorf("playstore: %d mentions failed validation: %s",
			len(dropped.Invalid), strings.Join(dropped.Invalid, "; ")))
	}
	return out, errors.Join(problems...)
}

// parseReviews pulls every review out of a listing's HTML.
func parseReviews(document string) ([]review, error) {
	root, err := html.Parse(strings.NewReader(document))
	if err != nil {
		return nil, err
	}

	var out []review
	for _, container := range findByClass(root, reviewClass) {
		out = append(out, review{
			ID:     firstAttr(container, reviewIDAttr),
			Author: textOf(firstByClass(container, authorClass)),
			Text:   textOf(firstByClass(container, bodyClass)),
			Rating: attr(firstByClass(container, ratingClass), "aria-label"),
			Date:   textOf(firstByClass(container, dateClass)),
			Thumbs: firstAttr(container, thumbsAttr),
		})
	}
	return out, nil
}

// ratingRE reads the integer out of "Rated 3 stars out of five stars".
//
// The number comes from this label and not from counting filled star SVGs: the
// filled and empty stars use the same path and differ only by a wrapper class,
// which is one more obfuscated name to depend on.
var ratingRE = regexp.MustCompile(`^Rated (\d+(?:\.\d+)?) stars? out of five stars$`)

// toDraft maps one review onto a mention with everything except its identity.
func toDraft(r review, pkg, url string) (models.Mention, error) {
	if r.ID == "" {
		return models.Mention{}, fmt.Errorf("playstore: a review on %s carries no %s", pkg, reviewIDAttr)
	}

	// Play states the date as "16 July 2026" with no time and no zone. Midnight
	// UTC is the only reading that does not invent precision the page never had.
	postedAt, err := time.Parse("2 January 2006", strings.TrimSpace(r.Date))
	if err != nil {
		return models.Mention{}, fmt.Errorf("playstore: review %s: unparseable date %q: %w", r.ID, r.Date, err)
	}

	match := ratingRE.FindStringSubmatch(strings.TrimSpace(r.Rating))
	if match == nil {
		return models.Mention{}, fmt.Errorf("playstore: review %s: unrecognised rating label %q", r.ID, r.Rating)
	}
	rating, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return models.Mention{}, fmt.Errorf("playstore: review %s: unparseable rating %q: %w", r.ID, match[1], err)
	}

	if strings.TrimSpace(r.Text) == "" {
		return models.Mention{}, fmt.Errorf("playstore: review %s carries no text", r.ID)
	}

	thumbs, err := strconv.Atoi(strings.TrimSpace(r.Thumbs))
	if err != nil {
		return models.Mention{}, fmt.Errorf("playstore: review %s: unparseable %s %q: %w", r.ID, thumbsAttr, r.Thumbs, err)
	}

	return models.Mention{
		Source:     models.SourcePlaystore,
		ExternalID: r.ID,
		// Play exposes no per-review permalink on the listing, so the listing
		// is where a human goes to find this review.
		URL: url,
		// Real names, not handles. redact.PII runs before any prompt.
		Author: r.Author,
		// Play exposes no reviewer reputation or follower count.
		AuthorFollowers: 0,
		Text:            strings.TrimSpace(r.Text),
		// The listing is fetched in the en_IN locale.
		Lang:       "en",
		PostedAt:   postedAt.UTC(),
		Engagement: models.Engagement{Likes: thumbs},
		Rating:     &rating,
		Raw:        map[string]any{"package": pkg},
	}, nil
}

// ---------------------------------------------------------------------------
// DOM helpers. x/net/html is a tokeniser with a tree on top and no selector
// engine, so these four are what stands in for one.
// ---------------------------------------------------------------------------

// attr returns an attribute's value, or "" when the node or attribute is absent.
func attr(n *html.Node, key string) string {
	if n == nil {
		return ""
	}
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// hasClass reports whether n carries want as one of its classes. Play's class
// attributes hold several names, so a substring match would match the wrong
// element.
func hasClass(n *html.Node, want string) bool {
	for _, name := range strings.Fields(attr(n, "class")) {
		if name == want {
			return true
		}
	}
	return false
}

// findByClass returns every element under root carrying the class.
func findByClass(root *html.Node, class string) []*html.Node {
	var out []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && hasClass(n, class) {
			out = append(out, n)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return out
}

// firstByClass returns the first element under root carrying the class, or nil.
func firstByClass(root *html.Node, class string) *html.Node {
	found := findByClass(root, class)
	if len(found) == 0 {
		return nil
	}
	return found[0]
}

// firstAttr returns the first non-empty value of key on root or any descendant.
// The review id sits on the header and the thumbs count on a sibling div, so
// neither is reachable from the container by class.
func firstAttr(root *html.Node, key string) string {
	if value := attr(root, key); value != "" {
		return value
	}
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		if value := firstAttr(child, key); value != "" {
			return value
		}
	}
	return ""
}

// textOf returns the concatenated text under n.
func textOf(n *html.Node) string {
	if n == nil {
		return ""
	}
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			b.WriteString(node.Data)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(n)
	return b.String()
}
