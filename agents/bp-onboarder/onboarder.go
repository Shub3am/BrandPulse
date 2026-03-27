// onboarder.go holds everything bp-onboarder does: map a brand's site, pick a
// budgeted spread of its pages, scrape them, and turn the text into one
// keyword set through one LLM call.
//
// It must not collect mentions, confirm the profile it returns, or write to
// Postgres. Confirmation is a human's job in DronaHQ and does not re-run this
// agent, so the profile comes back with Version 1 and no confirmed_at.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"brandpulse/internal/anakin"
	"brandpulse/internal/anakin/sources/scrape"
	"brandpulse/internal/llm"
	"brandpulse/internal/models"
	"brandpulse/internal/prompts"
	"brandpulse/internal/redact"
)

const (
	// maxAnakinCredits is the ceiling from the B2 brief. Map costs one credit
	// per job and Scrape one per URL, so the page count is the ceiling minus
	// the map.
	maxAnakinCredits = 30
	mapCredits       = 1
	maxScrapedPages  = maxAnakinCredits - mapCredits

	// defaultMaxPages is CONTRACTS §2: MaxPages 0 means 25 and this agent
	// applies it. It is applied here and not left to Anakin, because
	// /v1/crawl treats a zero maxPages as its own default of 10 rather than
	// as an error (docs/research/anakin.md §5) and a zero-valued Go int
	// reaching the wire spends credits on a silently different plan.
	defaultMaxPages = 25

	// maxPromptChars caps the site text handed to the model. Twelve pages of
	// Shopify markdown already exceeded a small model's context in Task 6
	// sizing, and a truncated prompt is a worse failure than a smaller
	// sample: it drops whichever pages sorted last.
	maxPromptChars = 60000
)

// OnboarderHandler builds a BrandProfile from a brand's own website.
//
// Its three collaborators are fields for the same reason bp-collector's are:
// llm.ChatJSON and redact.PII both still panic in B1's tree, so a test that
// called them through the package would panic before reaching anything this
// agent owns. The Anakin client is already an interface.
type OnboarderHandler struct {
	// NewClient builds the Anakin client for one onboarding, bound to this
	// agent's credit ceiling.
	NewClient func(ctx context.Context, in models.OnboardInput) (anakin.Client, error)

	// Chat is llm.ChatJSON in production. One call per onboarding, no retry
	// loop: the budget is one LLM call and llm.ChatJSON already retries a
	// reply that does not parse.
	Chat func(ctx context.Context, prompt string, schema any, opt llm.Opt) (json.RawMessage, llm.Usage, error)

	// Redact is redact.PII in production. It runs on every scraped page
	// before the text reaches the prompt, per the repo rule.
	Redact func(string) string
}

// New builds the handler main() serves.
func New(pool *pgxpool.Pool) *OnboarderHandler {
	return &OnboarderHandler{
		NewClient: anakinClient(pool),
		Chat:      llm.ChatJSON,
		Redact:    redact.PII,
	}
}

// keywordSet is the JSON shape the model must return. It is both the schema
// passed to llm.ChatJSON and the target of the unmarshal, so the contract with
// the model has one definition.
type keywordSet struct {
	Keywords         []string `json:"keywords"`
	Products         []string `json:"products"`
	Hashtags         []string `json:"hashtags"`
	NegativeKeywords []string `json:"negative_keywords"`
}

// Handle maps the brand's site, scrapes a spread of its pages, and asks the
// model once for the keyword set.
//
// Unlike bp-collector this returns a real error on failure. A BrandProfile has
// no Errors field to put a partial failure in, and a profile with an empty
// keyword list is not a degraded result: every downstream collector would
// search for nothing and report a quiet week.
func (h *OnboarderHandler) Handle(ctx context.Context, in models.OnboardInput) (models.BrandProfile, error) {
	if in.Website == "" {
		return models.BrandProfile{}, fmt.Errorf("onboarder: brand %q carries no website; there is nothing to map", in.BrandID)
	}

	client, err := h.NewClient(ctx, in)
	if err != nil {
		return models.BrandProfile{}, fmt.Errorf("onboarder: building the anakin client: %w", err)
	}

	pages, err := h.readSite(ctx, client, in.Website, pageBudget(in.MaxPages))
	if err != nil {
		return models.BrandProfile{}, err
	}

	set, err := h.askForKeywords(ctx, in, pages)
	if err != nil {
		return models.BrandProfile{}, err
	}

	return profileFrom(in, set), nil
}

// pageBudget resolves how many pages one onboarding may scrape.
//
// Two separate caps and both matter: the contract default for an unset field,
// and this agent's own credit ceiling for a caller that asks for 500.
func pageBudget(requested int) int {
	if requested <= 0 {
		requested = defaultMaxPages
	}
	if requested > maxScrapedPages {
		return maxScrapedPages
	}
	return requested
}

// readSite maps the site, picks the pages worth reading, and scrapes them.
//
// Map then Scrape rather than Crawl: Crawl takes one seed URL and follows links
// from it, so it cannot be handed a chosen list, and it charges the same credit
// per page while picking the pages itself. The 10-page probe in
// docs/research/anakin.md §5 spent 3 of its 10 credits on /account, /cart and a
// login page.
func (h *OnboarderHandler) readSite(ctx context.Context, client anakin.Client, site string, budget int) ([]string, error) {
	raw, err := client.Map(ctx, site, anakin.MapOpt{})
	if err != nil {
		return nil, fmt.Errorf("onboarder: mapping %s: %w", site, err)
	}

	// links is a flat []string and externalLinks is absent unless asked for,
	// both measured live. A []struct{URL string} here would decode without an
	// error into a slice of empty structs.
	var mapped struct {
		Links []string `json:"links"`
	}
	if err := json.Unmarshal(raw, &mapped); err != nil {
		return nil, fmt.Errorf("onboarder: decoding the map of %s: %w", site, err)
	}
	if len(mapped.Links) == 0 {
		return nil, fmt.Errorf("onboarder: the map of %s returned no links", site)
	}

	var pages []string
	for _, link := range selectPages(site, mapped.Links, budget) {
		text, err := scrapePage(ctx, client, link)
		if err != nil {
			// One dead page does not fail an onboarding. The keyword set comes
			// from the whole sample and the ceiling below catches the case
			// where too little of it survived.
			continue
		}
		pages = append(pages, h.Redact(text))
	}
	if len(pages) == 0 {
		return nil, fmt.Errorf("onboarder: no page of %s could be scraped", site)
	}
	return pages, nil
}

// scrapePage fetches one page's markdown.
//
// Markdown and not html: this text is read by a language model, and a Shopify
// product page's html is two orders of magnitude larger than its markdown for
// the same prose. The number-escaping in Anakin's markdown ("1\. Copyright")
// affects list numbering, which no keyword is drawn from.
func scrapePage(ctx context.Context, client anakin.Client, page string) (string, error) {
	raw, err := client.Scrape(ctx, page, anakin.ScrapeOpt{Formats: []string{"markdown"}, Country: "in"})
	if err != nil {
		return "", err
	}
	resp, err := scrape.Decode(raw)
	if err != nil {
		return "", err
	}
	if resp.Markdown == "" {
		return "", fmt.Errorf("onboarder: %s returned no markdown", page)
	}
	return resp.Markdown, nil
}

// pageKind buckets a link by what it tells us about the brand.
type pageKind int

const (
	kindAbout pageKind = iota
	kindProduct
	kindCollection
	kindSkip
)

// classify buckets one path.
//
// Account, cart, checkout and policy pages carry no brand vocabulary and cost
// a credit each. Everything unrecognised is an about-page: a brand on a
// non-Shopify site has its story at /our-story, not under a path this knows.
func classify(link string) pageKind {
	parsed, err := url.Parse(link)
	if err != nil {
		return kindSkip
	}
	path := strings.ToLower(parsed.Path)

	for _, skip := range []string{"/account", "/cart", "/checkout", "/login", "/register", "/policies", "/search", "/blogs/tagged"} {
		if strings.HasPrefix(path, skip) {
			return kindSkip
		}
	}
	switch {
	case strings.HasPrefix(path, "/products"):
		return kindProduct
	case strings.HasPrefix(path, "/collections"):
		return kindCollection
	default:
		return kindAbout
	}
}

// selectPages picks up to budget links, taking turns between the three kinds.
//
// Round-robin and not head-of-list: a real site map is products-first. The live
// map of boat-lifestyle.com returned 48 product, 35 collection and 13 about
// links in that order, so taking the first 25 gives 25 product pages, no about
// page and a keyword set with no brand story in it.
//
// The homepage goes first and is never subject to the quota: it is the one page
// that always names the brand, its categories and its flagship products.
func selectPages(site string, links []string, budget int) []string {
	if budget <= 0 {
		return nil
	}

	buckets := map[pageKind][]string{}
	for _, link := range dedupe(links) {
		if kind := classify(link); kind != kindSkip {
			buckets[kind] = append(buckets[kind], link)
		}
	}

	picked := []string{site}
	seen := map[string]bool{canonical(site): true}

	// About first: on a small site it is the only bucket with anything in it.
	order := []pageKind{kindAbout, kindProduct, kindCollection}
	for taken := true; taken && len(picked) < budget; {
		taken = false
		for _, kind := range order {
			if len(picked) >= budget {
				break
			}
			for len(buckets[kind]) > 0 {
				next := buckets[kind][0]
				buckets[kind] = buckets[kind][1:]
				if seen[canonical(next)] {
					continue
				}
				seen[canonical(next)] = true
				picked = append(picked, next)
				taken = true
				break
			}
		}
	}
	return picked
}

// canonical strips the trailing slash so "…/" and "…" are one page.
//
// The live map returns both forms of the homepage, and the 10-page crawl probe
// paid for both.
func canonical(link string) string {
	return strings.TrimSuffix(link, "/")
}

// dedupe removes repeated links while keeping the map's ordering, which is what
// selectPages's round-robin depends on.
func dedupe(links []string) []string {
	seen := make(map[string]bool, len(links))
	out := make([]string, 0, len(links))
	for _, link := range links {
		if seen[canonical(link)] {
			continue
		}
		seen[canonical(link)] = true
		out = append(out, link)
	}
	return out
}

// askForKeywords makes the single LLM call.
func (h *OnboarderHandler) askForKeywords(ctx context.Context, in models.OnboardInput, pages []string) (keywordSet, error) {
	raw, _, err := h.Chat(ctx, buildPrompt(in, pages), keywordSet{}, llm.Opt{})
	if err != nil {
		return keywordSet{}, fmt.Errorf("onboarder: asking for %s's keywords: %w", in.Name, err)
	}

	var set keywordSet
	if err := json.Unmarshal(raw, &set); err != nil {
		return keywordSet{}, fmt.Errorf("onboarder: decoding the keyword set for %s: %w", in.Name, err)
	}
	if len(set.Keywords) == 0 {
		return keywordSet{}, fmt.Errorf("onboarder: the keyword set for %s is empty; every collector would search for nothing", in.Name)
	}
	return set, nil
}

// buildPrompt assembles the instructions, the brand facts and the site text.
//
// The site text is truncated whole pages at a time rather than mid-page: half a
// product page ends mid-sentence and the model treats the fragment as a product
// name.
func buildPrompt(in models.OnboardInput, pages []string) string {
	var b strings.Builder
	b.WriteString(prompts.Load("onboarder"))
	b.WriteString("\n\n## This brand\n\n")
	fmt.Fprintf(&b, "Name: %s\n", in.Name)
	fmt.Fprintf(&b, "Website: %s\n", in.Website)
	if len(in.Competitors) > 0 {
		fmt.Fprintf(&b, "Competitors, for telling their products apart from this brand's: %s\n", strings.Join(in.Competitors, ", "))
	}

	b.WriteString("\n## Site text\n")
	budget := maxPromptChars
	for i, page := range pages {
		if len(page) > budget {
			break
		}
		budget -= len(page)
		fmt.Fprintf(&b, "\n### Page %d\n\n%s\n", i+1, page)
	}
	return b.String()
}

// profileFrom turns the model's answer into the profile the rest of the system
// stores.
//
// models.NewBrandProfile and not a literal: it sets Version 1, and a profile
// built by hand gets Version 0, which means "never onboarded" in the database
// and fails Validate().
func profileFrom(in models.OnboardInput, set keywordSet) models.BrandProfile {
	profile := models.NewBrandProfile(in.BrandID, in.Name)
	profile.Website = in.Website
	profile.Keywords = clean(set.Keywords)
	profile.Products = clean(set.Products)
	profile.Hashtags = clean(set.Hashtags)
	profile.NegativeKeywords = clean(set.NegativeKeywords)
	profile.Competitors = clean(in.Competitors)
	return profile
}

// clean drops blanks and case-insensitive repeats from a model-supplied list,
// and returns an empty slice rather than nil so the JSON artifact carries [].
//
// The model is asked for no duplicates and mostly obliges, but "boAt" and
// "boat" arrive as two keywords and each one is a paid search query.
func clean(terms []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, term := range terms {
		trimmed := strings.TrimSpace(term)
		key := strings.ToLower(trimmed)
		if trimmed == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, trimmed)
	}
	return out
}

// anakinClient builds the Anakin client for one onboarding.
//
// An unset BP_FIXTURE_MODE is replay, not live: the repo rule is zero-credit
// CI and a missing env var must not be the thing that starts spending. The
// pool is passed because the ceiling is persisted per brand-day, so a crash
// halfway through an onboarding does not hand the retry a fresh 30 credits.
func anakinClient(pool *pgxpool.Pool) func(context.Context, models.OnboardInput) (anakin.Client, error) {
	return func(ctx context.Context, in models.OnboardInput) (anakin.Client, error) {
		mode := anakin.Mode(os.Getenv("BP_FIXTURE_MODE"))
		if mode == "" {
			mode = anakin.ModeReplay
		}
		return anakin.NewHTTPClient(anakin.Config{
			APIKey:     os.Getenv("ANAKIN_API_KEY"),
			Mode:       mode,
			MaxCredits: maxAnakinCredits,
			BrandID:    in.BrandID,
			Day:        time.Now().UTC(),
			Pool:       pool,
		})
	}
}
