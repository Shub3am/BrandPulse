// onboarder_test.go drives the handler with a stub Anakin client and a stub
// model, so no test needs a key, a fixture directory or Postgres.
//
// The stub client lives here rather than coming from
// internal/anakin/sources/sourcestest, whose package doc says nothing under
// agents/ may import it.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"brandpulse/internal/anakin"
	"brandpulse/internal/llm"
	"brandpulse/internal/models"
)

// errNoNetwork is what every unstubbed surface returns. There is no
// http.Client to block because there is no transport at all.
var errNoNetwork = errors.New("stub client: no network in a test")

// stubClient records what it was asked for and replays what the test set.
type stubClient struct {
	links    []string
	markdown map[string]string

	mapErr    error
	scrapeErr error

	mapCalls    []string
	scrapeCalls []string
	credits     int
}

func (c *stubClient) Map(ctx context.Context, site string, opt anakin.MapOpt) (json.RawMessage, error) {
	c.mapCalls = append(c.mapCalls, site)
	if c.mapErr != nil {
		return nil, c.mapErr
	}
	c.credits++
	return json.Marshal(map[string]any{"links": c.links, "totalLinks": len(c.links)})
}

func (c *stubClient) Scrape(ctx context.Context, page string, opt anakin.ScrapeOpt) (json.RawMessage, error) {
	c.scrapeCalls = append(c.scrapeCalls, page)
	if c.scrapeErr != nil {
		return nil, c.scrapeErr
	}
	md, ok := c.markdown[page]
	if !ok {
		md = "# " + page + "\n\nAirdopes 141 wireless earbuds.\n"
	}
	c.credits++
	return json.Marshal(map[string]any{"id": "s1", "status": "completed", "url": page, "markdown": md})
}

func (c *stubClient) Search(ctx context.Context, q string, opt anakin.SearchOpt) (json.RawMessage, error) {
	return nil, errNoNetwork
}

func (c *stubClient) Wire(ctx context.Context, platform, q string, opt anakin.WireOpt) (json.RawMessage, error) {
	return nil, errNoNetwork
}

func (c *stubClient) Crawl(ctx context.Context, u string, opt anakin.CrawlOpt) (json.RawMessage, error) {
	return nil, errNoNetwork
}

func (c *stubClient) Stats() anakin.Stats {
	return anakin.Stats{CreditsUsed: c.credits}
}

// answering builds a Chat stub that returns set and records the prompt it saw.
func answering(set keywordSet, prompt *string, calls *int) func(context.Context, string, any, llm.Opt) (json.RawMessage, llm.Usage, error) {
	return func(ctx context.Context, p string, schema any, opt llm.Opt) (json.RawMessage, llm.Usage, error) {
		*calls++
		if prompt != nil {
			*prompt = p
		}
		raw, err := json.Marshal(set)
		return raw, llm.Usage{}, err
	}
}

// handlerFor wires a handler to one stub client and one stub model. Redact is
// the identity by default so a test asserting on prompt text reads what the
// scrape returned; the redaction test replaces it.
func handlerFor(client *stubClient, chat func(context.Context, string, any, llm.Opt) (json.RawMessage, llm.Usage, error)) *OnboarderHandler {
	return &OnboarderHandler{
		NewClient: func(ctx context.Context, in models.OnboardInput) (anakin.Client, error) { return client, nil },
		Chat:      chat,
		Redact:    func(s string) string { return s },
	}
}

func input() models.OnboardInput {
	return models.OnboardInput{
		BrandID:     "boat",
		Name:        "boAt",
		Website:     "https://www.boat-lifestyle.com",
		Competitors: []string{"Noise", "boult"},
	}
}

func fullSet() keywordSet {
	return keywordSet{
		Keywords:         []string{"boAt", "bo at", "airdopes 141"},
		Products:         []string{"Airdopes 141"},
		Hashtags:         []string{"#boatheads"},
		NegativeKeywords: []string{"boat rental", "sailing"},
	}
}

// links builds a products-first link list like the live map returns.
func links(products, collections, pages int) []string {
	var out []string
	for i := 0; i < products; i++ {
		out = append(out, fmt.Sprintf("https://www.boat-lifestyle.com/products/p%d", i))
	}
	for i := 0; i < collections; i++ {
		out = append(out, fmt.Sprintf("https://www.boat-lifestyle.com/collections/c%d", i))
	}
	for i := 0; i < pages; i++ {
		out = append(out, fmt.Sprintf("https://www.boat-lifestyle.com/pages/a%d", i))
	}
	return out
}

func TestHandleBuildsAProfileFromTheSite(t *testing.T) {
	client := &stubClient{links: links(3, 2, 1)}
	calls := 0
	h := handlerFor(client, answering(fullSet(), nil, &calls))

	profile, err := h.Handle(context.Background(), input())
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if profile.BrandID != "boat" || profile.Name != "boAt" {
		t.Errorf("profile identifies %q/%q, want boat/boAt", profile.BrandID, profile.Name)
	}
	if profile.Website != "https://www.boat-lifestyle.com" {
		t.Errorf("Website is %q", profile.Website)
	}
	if got := len(profile.Keywords); got != 3 {
		t.Errorf("got %d keywords, want the 3 the model returned", got)
	}
	if len(profile.NegativeKeywords) != 2 {
		t.Errorf("NegativeKeywords is %v, want the model's 2", profile.NegativeKeywords)
	}
	if !reflectEqual(profile.Competitors, []string{"Noise", "boult"}) {
		t.Errorf("Competitors is %v, want the input's", profile.Competitors)
	}
}

// Version 0 means "never onboarded" in the database and fails Validate, so a
// hand-built literal in profileFrom would be a silent data bug.
func TestProfileCarriesVersionOneAndValidates(t *testing.T) {
	client := &stubClient{links: links(2, 1, 1)}
	calls := 0
	h := handlerFor(client, answering(fullSet(), nil, &calls))

	profile, err := h.Handle(context.Background(), input())
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if profile.Version != 1 {
		t.Errorf("Version is %d, want 1", profile.Version)
	}
	if err := profile.Validate(); err != nil {
		t.Errorf("the profile does not validate: %v", err)
	}
}

// The budget is one map plus one scrape per page, so the ceiling is a page
// count. A caller asking for 500 pages must not get 500 credits spent.
func TestPageBudgetIsCappedAtTheCreditCeiling(t *testing.T) {
	for _, tc := range []struct {
		name      string
		requested int
		want      int
	}{
		{"zero means the contract default", 0, defaultMaxPages},
		{"negative means the contract default", -3, defaultMaxPages},
		{"a small ask is honoured", 5, 5},
		{"a huge ask is capped", 500, maxScrapedPages},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := pageBudget(tc.requested); got != tc.want {
				t.Errorf("pageBudget(%d) = %d, want %d", tc.requested, got, tc.want)
			}
		})
	}
}

func TestHandleNeverSpendsMoreThanThirtyCredits(t *testing.T) {
	client := &stubClient{links: links(200, 200, 200)}
	calls := 0
	h := handlerFor(client, answering(fullSet(), nil, &calls))

	in := input()
	in.MaxPages = 500
	if _, err := h.Handle(context.Background(), in); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if client.credits > maxAnakinCredits {
		t.Errorf("spent %d credits, the ceiling is %d", client.credits, maxAnakinCredits)
	}
	if got := len(client.scrapeCalls); got != maxScrapedPages {
		t.Errorf("scraped %d pages, want the full budget of %d", got, maxScrapedPages)
	}
	if calls != 1 {
		t.Errorf("made %d LLM calls, the budget is 1", calls)
	}
}

func TestOnlyOneLLMCallPerOnboarding(t *testing.T) {
	client := &stubClient{links: links(4, 4, 4)}
	calls := 0
	h := handlerFor(client, answering(fullSet(), nil, &calls))

	if _, err := h.Handle(context.Background(), input()); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if calls != 1 {
		t.Errorf("made %d LLM calls, want exactly 1", calls)
	}
}

// A products-first map truncated at the head gives no about page at all. The
// live map returned 48 product, 35 collection and 13 about links in that order.
func TestSelectPagesTakesSomeOfEachKind(t *testing.T) {
	picked := selectPages("https://s.com", links(48, 35, 13), 10)

	if len(picked) != 10 {
		t.Fatalf("picked %d pages, want the full budget of 10", len(picked))
	}
	counts := map[pageKind]int{}
	for _, page := range picked[1:] {
		counts[classify(page)]++
	}
	for kind, name := range map[pageKind]string{kindAbout: "about", kindProduct: "product", kindCollection: "collection"} {
		if counts[kind] == 0 {
			t.Errorf("no %s page was picked out of %v", name, picked)
		}
	}
}

// The homepage is the one page that always names the brand and its categories.
func TestSelectPagesAlwaysStartsAtTheHomepage(t *testing.T) {
	picked := selectPages("https://s.com", links(5, 5, 5), 4)
	if picked[0] != "https://s.com" {
		t.Errorf("first page is %q, want the site root", picked[0])
	}
}

// Account, cart and checkout pages carry no brand vocabulary and cost a credit
// each. The live 10-page crawl probe spent 3 of its 10 on exactly these.
func TestSelectPagesSkipsAccountAndCartPages(t *testing.T) {
	junk := []string{
		"https://s.com/account",
		"https://s.com/account/login",
		"https://s.com/cart",
		"https://s.com/checkout",
		"https://s.com/policies/refund-policy",
		"https://s.com/search?q=x",
	}
	picked := selectPages("https://s.com", append(junk, links(2, 0, 0)...), 20)

	for _, page := range picked {
		if classify(page) == kindSkip {
			t.Errorf("picked %q, which is not worth a credit", page)
		}
	}
	if len(picked) != 3 {
		t.Errorf("picked %v, want the root and the 2 product pages", picked)
	}
}

// The live map returns both "https://site.com" and "https://site.com/", and
// the crawl probe paid for both.
func TestSelectPagesPaysForTheHomepageOnce(t *testing.T) {
	picked := selectPages("https://s.com", []string{"https://s.com", "https://s.com/", "https://s.com/pages/about"}, 10)

	if len(picked) != 2 {
		t.Errorf("picked %v, want the root once plus the about page", picked)
	}
}

func TestSelectPagesStopsWhenTheSiteIsSmallerThanTheBudget(t *testing.T) {
	picked := selectPages("https://s.com", links(1, 1, 0), 25)
	if len(picked) != 3 {
		t.Errorf("picked %v, want the root and the 2 real links", picked)
	}
}

// A brand on a non-Shopify site has its story at /our-story, under no path this
// recognises, so an unrecognised path must be read rather than dropped.
func TestUnrecognisedPathsCountAsAboutPages(t *testing.T) {
	if got := classify("https://s.com/our-story"); got != kindAbout {
		t.Errorf("classify(/our-story) = %v, want kindAbout", got)
	}
}

func TestScrapedTextIsRedactedBeforeItReachesThePrompt(t *testing.T) {
	client := &stubClient{
		links:    []string{"https://www.boat-lifestyle.com/pages/contact"},
		markdown: map[string]string{"https://www.boat-lifestyle.com/pages/contact": "write to priya@example.com"},
	}
	var prompt string
	calls := 0
	h := handlerFor(client, answering(fullSet(), &prompt, &calls))
	h.Redact = func(s string) string { return strings.ReplaceAll(s, "priya@example.com", "[EMAIL]") }

	if _, err := h.Handle(context.Background(), input()); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if strings.Contains(prompt, "priya@example.com") {
		t.Error("the prompt carries an unredacted email address")
	}
	if !strings.Contains(prompt, "[EMAIL]") {
		t.Error("the redacted text never reached the prompt")
	}
}

// The prompt is the embedded file plus the brand's facts. An inlined prompt
// would pass a text assertion but breaks the rule that a prompt change shows up
// as a prose diff.
func TestPromptCarriesTheEmbeddedFileAndTheBrandFacts(t *testing.T) {
	client := &stubClient{links: links(1, 0, 1)}
	var prompt string
	calls := 0
	h := handlerFor(client, answering(fullSet(), &prompt, &calls))

	if _, err := h.Handle(context.Background(), input()); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	embedded, err := os.ReadFile("../../internal/prompts/onboarder.md")
	if err != nil {
		t.Fatalf("reading the prompt file: %v", err)
	}
	if !strings.Contains(prompt, strings.TrimSpace(string(embedded))) {
		t.Error("the prompt does not carry internal/prompts/onboarder.md verbatim")
	}
	for _, want := range []string{"boAt", "https://www.boat-lifestyle.com", "Noise", "boult"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the prompt never mentions %q", want)
		}
	}
}

// The prompt drops whole pages, never half of one: a fragment ending
// mid-sentence gets read as a product name.
func TestPromptTruncatesWholePages(t *testing.T) {
	big := strings.Repeat("x", maxPromptChars/2+1)
	built := buildPrompt(input(), []string{big, big, big})

	if strings.Count(built, "### Page") != 1 {
		t.Errorf("kept %d pages, want only the one that fits", strings.Count(built, "### Page"))
	}
}

// A profile with no keywords is not a degraded result. Every collector would
// search for nothing and the run would report a quiet week.
func TestEmptyKeywordSetIsAnError(t *testing.T) {
	client := &stubClient{links: links(2, 0, 0)}
	calls := 0
	h := handlerFor(client, answering(keywordSet{}, nil, &calls))

	if _, err := h.Handle(context.Background(), input()); err == nil {
		t.Error("an empty keyword set returned no error")
	}
}

func TestOneDeadPageDoesNotFailTheOnboarding(t *testing.T) {
	client := &stubClient{
		links:    links(2, 0, 0),
		markdown: map[string]string{"https://www.boat-lifestyle.com/products/p0": ""},
	}
	calls := 0
	h := handlerFor(client, answering(fullSet(), nil, &calls))

	profile, err := h.Handle(context.Background(), input())
	if err != nil {
		t.Fatalf("one empty page failed the whole onboarding: %v", err)
	}
	if len(profile.Keywords) == 0 {
		t.Error("the profile came back with no keywords")
	}
}

func TestNoScrapablePageIsAnError(t *testing.T) {
	client := &stubClient{links: links(2, 0, 0), scrapeErr: errNoNetwork}
	calls := 0
	h := handlerFor(client, answering(fullSet(), nil, &calls))

	if _, err := h.Handle(context.Background(), input()); err == nil {
		t.Error("a site where nothing scraped returned no error")
	}
	if calls != 0 {
		t.Error("the model was called with no site text at all")
	}
}

func TestEmptyMapIsAnError(t *testing.T) {
	client := &stubClient{links: []string{}}
	calls := 0
	h := handlerFor(client, answering(fullSet(), nil, &calls))

	if _, err := h.Handle(context.Background(), input()); err == nil {
		t.Error("a map with no links returned no error")
	}
}

func TestMissingWebsiteIsRejectedBeforeAnySpend(t *testing.T) {
	client := &stubClient{links: links(2, 0, 0)}
	calls := 0
	h := handlerFor(client, answering(fullSet(), nil, &calls))

	in := input()
	in.Website = ""
	if _, err := h.Handle(context.Background(), in); err == nil {
		t.Error("a brand with no website returned no error")
	}
	if client.credits != 0 {
		t.Errorf("spent %d credits with no site to map", client.credits)
	}
}

// "boAt" and "boat" are two paid search queries for one set of posts.
func TestKeywordsAreDeduplicatedCaseInsensitively(t *testing.T) {
	client := &stubClient{links: links(2, 0, 0)}
	calls := 0
	set := keywordSet{Keywords: []string{"boAt", "boat", " boAt ", "", "airdopes"}}
	h := handlerFor(client, answering(set, nil, &calls))

	profile, err := h.Handle(context.Background(), input())
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if !reflectEqual(profile.Keywords, []string{"boAt", "airdopes"}) {
		t.Errorf("Keywords is %v, want the first spelling of each term", profile.Keywords)
	}
}

// An artifact whose lists marshal as null instead of [] breaks DronaHQ's
// binding, which iterates them.
func TestEmptyListsMarshalAsArrays(t *testing.T) {
	client := &stubClient{links: links(2, 0, 0)}
	calls := 0
	h := handlerFor(client, answering(keywordSet{Keywords: []string{"boAt"}}, nil, &calls))

	profile, err := h.Handle(context.Background(), input())
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	raw, err := json.Marshal(profile)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	for _, want := range []string{`"products":[]`, `"hashtags":[]`, `"negative_keywords":[]`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("the artifact does not carry %s: %s", want, raw)
		}
	}
}

func TestUnbuildableClientIsReported(t *testing.T) {
	h := &OnboarderHandler{
		NewClient: func(ctx context.Context, in models.OnboardInput) (anakin.Client, error) {
			return nil, errors.New("no ANAKIN_API_KEY")
		},
		Chat:   answering(fullSet(), nil, new(int)),
		Redact: func(s string) string { return s },
	}
	if _, err := h.Handle(context.Background(), input()); err == nil {
		t.Error("a client that could not be built returned no error")
	}
}

func TestMapFailureIsReported(t *testing.T) {
	client := &stubClient{mapErr: errNoNetwork}
	calls := 0
	h := handlerFor(client, answering(fullSet(), nil, &calls))

	if _, err := h.Handle(context.Background(), input()); err == nil {
		t.Error("a failed map returned no error")
	}
}

// nasiko validate's required fields, from cli/src/commands/validate.rs. A card
// missing one of them deploys and is then invisible to routing.
func TestAgentCardCarriesWhatNasikoValidateRequires(t *testing.T) {
	raw, err := os.ReadFile("AgentCard.json")
	if err != nil {
		t.Fatalf("reading AgentCard.json: %v", err)
	}
	var card map[string]any
	if err := json.Unmarshal(raw, &card); err != nil {
		t.Fatalf("AgentCard.json is not valid JSON: %v", err)
	}

	for _, field := range []string{"name", "description", "url", "version", "capabilities", "skills", "protocolVersion", "preferredTransport"} {
		if _, ok := card[field]; !ok {
			t.Errorf("AgentCard.json has no %q", field)
		}
	}
	// The Nasiko example ships "0.2.9" and a real cluster rejects it with
	// -32009 VersionNotSupported.
	if card["protocolVersion"] != "1.0" {
		t.Errorf("protocolVersion is %v, want \"1.0\"", card["protocolVersion"])
	}
	skills, ok := card["skills"].([]any)
	if !ok || len(skills) == 0 {
		t.Error("skills is empty, so the agent is invisible to routing")
	}
}

func reflectEqual(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
