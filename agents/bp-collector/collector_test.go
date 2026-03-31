package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"brandpulse/internal/anakin"
	"brandpulse/internal/anakin/sources"
	"brandpulse/internal/models"
)

// errNoNetwork is what every fetch surface of the stub returns. bp-collector
// never calls one itself, it only hands the client to an adapter, so reaching
// one from this package is a bug rather than a case to stub out.
var errNoNetwork = errors.New("collector_test: the stub client reaches no network")

// stubClient is an anakin.Client that cannot call out. This is the no-network
// guarantee for this package: there is no http.Client to block because there
// is no transport at all. anakin.NoNetwork() is the equivalent one layer down,
// inside the real client B1 still has to implement.
type stubClient struct{ spend anakin.Stats }

func (c stubClient) Search(context.Context, string, anakin.SearchOpt) (json.RawMessage, error) {
	return nil, errNoNetwork
}

func (c stubClient) Wire(context.Context, string, string, anakin.WireOpt) (json.RawMessage, error) {
	return nil, errNoNetwork
}

func (c stubClient) Scrape(context.Context, string, anakin.ScrapeOpt) (json.RawMessage, error) {
	return nil, errNoNetwork
}

func (c stubClient) Map(context.Context, string, anakin.MapOpt) (json.RawMessage, error) {
	return nil, errNoNetwork
}

func (c stubClient) Crawl(context.Context, string, anakin.CrawlOpt) (json.RawMessage, error) {
	return nil, errNoNetwork
}

func (c stubClient) Stats() anakin.Stats { return c.spend }

// fetching builds a handler whose one adapter returns what the test says, with
// no stored hashes and a client that spends nothing.
func fetching(source models.Source, out []models.Mention, err error) *CollectorHandler {
	return &CollectorHandler{
		Adapters: map[models.Source]sources.FetchFunc{
			source: func(context.Context, anakin.Client, models.BrandProfile, time.Time, time.Time) ([]models.Mention, error) {
				return out, err
			},
		},
		NewClient: func(context.Context, models.CollectInput) (anakin.Client, error) {
			return stubClient{}, nil
		},
		SeenHashes: func(context.Context, string, []string) (map[string]bool, error) {
			return nil, nil
		},
	}
}

// mention is a mention as an adapter hands it over: already stamped, already
// validated, identified only by its content hash as far as this agent cares.
func mention(hash string) models.Mention {
	return models.Mention{
		ID:          "mnt_" + hash,
		BrandID:     "boat-lifestyle",
		Source:      models.SourceReddit,
		ContentHash: hash,
		Text:        "the case rattles",
		Lang:        "en",
		PostedAt:    time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC),
	}
}

func input() models.CollectInput {
	return models.CollectInput{
		Profile:     models.BrandProfile{BrandID: "boat-lifestyle", Keywords: []string{"boAt"}, Version: 1},
		Source:      models.SourceReddit,
		WindowStart: time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC),
		WindowEnd:   time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC),
		MaxCredits:  40,
		RunID:       "run_01",
	}
}

func hashesOf(batch models.MentionBatch) []string {
	out := make([]string, 0, len(batch.Mentions))
	for _, m := range batch.Mentions {
		out = append(out, m.ContentHash)
	}
	return out
}

func TestHandleReturnsWhatTheAdapterCollected(t *testing.T) {
	h := fetching(models.SourceReddit, []models.Mention{mention("a"), mention("b"), mention("c")}, nil)

	batch, err := h.Handle(context.Background(), input())
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if got := hashesOf(batch); strings.Join(got, ",") != "a,b,c" {
		t.Errorf("hashes = %v, want [a b c] in order", got)
	}
	if batch.Source != models.SourceReddit {
		t.Errorf("Source = %q", batch.Source)
	}
	if batch.Truncated {
		t.Error("Truncated = true on a clean run")
	}
	if len(batch.Errors) != 0 {
		t.Errorf("Errors = %v, want none", batch.Errors)
	}
}

func TestHandleReportsAnUnknownSourceInsteadOfPanicking(t *testing.T) {
	// A nil out of a map is a nil func call, which is a panic that takes the
	// whole run down. Naming the source matters too: "amazon" arriving here
	// means a dead source went back into a BrandProfile.
	h := fetching(models.SourceReddit, nil, nil)
	in := input()
	in.Source = models.SourceAmazon

	batch, err := h.Handle(context.Background(), in)
	if err != nil {
		t.Fatalf("Handle returned an error for an unknown source: %v", err)
	}
	if len(batch.Errors) != 1 || !strings.Contains(batch.Errors[0], "amazon") {
		t.Errorf("Errors = %v, want one naming amazon", batch.Errors)
	}
	if len(batch.Mentions) != 0 {
		t.Errorf("Mentions = %v, want none", batch.Mentions)
	}
	if batch.Source != models.SourceAmazon {
		t.Errorf("Source = %q, want the source that was asked for", batch.Source)
	}
}

func TestHandleTreatsTheBudgetCeilingAsTruncationNotFailure(t *testing.T) {
	// The half the run paid for is worth keeping. Truncated is what stops a
	// later pass treating the window as covered.
	stop := fmt.Errorf("reddit: rt_search %q: %w", "boAt Airdopes", anakin.ErrBudgetExceeded)
	h := fetching(models.SourceReddit, []models.Mention{mention("a"), mention("b")}, stop)

	batch, err := h.Handle(context.Background(), input())
	if err != nil {
		t.Fatalf("a budget ceiling failed the task: %v", err)
	}
	if !batch.Truncated {
		t.Error("Truncated = false after ErrBudgetExceeded")
	}
	if len(batch.Mentions) != 2 {
		t.Errorf("kept %d mentions, want the 2 the run already paid for", len(batch.Mentions))
	}
	if len(batch.Errors) != 1 || !strings.Contains(batch.Errors[0], "boAt Airdopes") {
		t.Errorf("Errors = %v, want the ceiling recorded with the keyword it stopped on", batch.Errors)
	}
}

func TestHandleKeepsThePartialBatchWhenASourceFails(t *testing.T) {
	// One dead source must not kill a run, per CONTRACTS §1.
	h := fetching(models.SourceReddit, []models.Mention{mention("a")}, errors.New("reddit: decoding rt_search: unexpected EOF"))

	batch, err := h.Handle(context.Background(), input())
	if err != nil {
		t.Fatalf("an adapter error failed the task: %v", err)
	}
	if batch.Truncated {
		t.Error("Truncated = true, but this was a failure and not a ceiling")
	}
	if len(batch.Mentions) != 1 {
		t.Errorf("kept %d mentions, want the partial batch", len(batch.Mentions))
	}
	if len(batch.Errors) != 1 || !strings.Contains(batch.Errors[0], "unexpected EOF") {
		t.Errorf("Errors = %v", batch.Errors)
	}
}

func TestHandleDedupesWithinTheBatchKeepingTheFirst(t *testing.T) {
	// Two of a brand's keywords return the same post. Stamp gives the copies
	// different ids, so only the hash catches them.
	first := mention("a")
	first.MatchedKeyword = "boAt"
	second := mention("a")
	second.ID = "mnt_a2"
	second.MatchedKeyword = "boAt Airdopes"

	h := fetching(models.SourceReddit, []models.Mention{first, second, mention("b")}, nil)

	batch, err := h.Handle(context.Background(), input())
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if got := hashesOf(batch); strings.Join(got, ",") != "a,b" {
		t.Errorf("hashes = %v, want [a b]", got)
	}
	if batch.Mentions[0].MatchedKeyword != "boAt" {
		t.Errorf("MatchedKeyword = %q, want the first occurrence kept", batch.Mentions[0].MatchedKeyword)
	}
}

func TestHandleSkipsHashesTheBrandAlreadyHas(t *testing.T) {
	h := fetching(models.SourceReddit, []models.Mention{mention("a"), mention("b"), mention("c")}, nil)

	var askedFor []string
	var askedBrand string
	h.SeenHashes = func(_ context.Context, brandID string, hashes []string) (map[string]bool, error) {
		askedBrand, askedFor = brandID, hashes
		return map[string]bool{"b": true}, nil
	}

	batch, err := h.Handle(context.Background(), input())
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if got := hashesOf(batch); strings.Join(got, ",") != "a,c" {
		t.Errorf("hashes = %v, want [a c]", got)
	}
	// The lookup is scoped by brand: the same article collected for two brands
	// is two mentions, and mentions is unique on (brand_id, content_hash).
	if askedBrand != "boat-lifestyle" {
		t.Errorf("looked up hashes for brand %q", askedBrand)
	}
	if strings.Join(askedFor, ",") != "a,b,c" {
		t.Errorf("asked for %v, want every hash in one statement", askedFor)
	}
}

func TestHandleKeepsTheBatchWhenTheDedupeLookupFails(t *testing.T) {
	// mentions has UNIQUE (brand_id, content_hash), so a duplicate that slips
	// through is rejected at the insert. Dropping the batch would lose a
	// window nobody re-collects.
	h := fetching(models.SourceReddit, []models.Mention{mention("a"), mention("b")}, nil)
	h.SeenHashes = func(context.Context, string, []string) (map[string]bool, error) {
		return nil, errors.New("pool exhausted")
	}

	batch, err := h.Handle(context.Background(), input())
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(batch.Mentions) != 2 {
		t.Errorf("kept %d mentions, want both", len(batch.Mentions))
	}
	if len(batch.Errors) != 1 || !strings.Contains(batch.Errors[0], "pool exhausted") {
		t.Errorf("Errors = %v, want the lookup failure recorded", batch.Errors)
	}
}

func TestHandleDoesNotAskTheDatabaseAboutAnEmptyBatch(t *testing.T) {
	h := fetching(models.SourceReddit, nil, nil)
	h.SeenHashes = func(context.Context, string, []string) (map[string]bool, error) {
		return nil, errors.New("SeenHashes was called with nothing to look up")
	}

	batch, err := h.Handle(context.Background(), input())
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(batch.Errors) != 0 {
		t.Errorf("Errors = %v, want none", batch.Errors)
	}
}

func TestHandleReadsTheCountersFromTheClient(t *testing.T) {
	// The cost table lives in internal/anakin because Wire costs vary per
	// action. This agent must not price a call itself.
	h := fetching(models.SourceReddit, []models.Mention{mention("a")}, nil)
	h.NewClient = func(context.Context, models.CollectInput) (anakin.Client, error) {
		return stubClient{spend: anakin.Stats{CreditsUsed: 17, CacheHits: 4}}, nil
	}

	batch, err := h.Handle(context.Background(), input())
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if batch.CreditsUsed != 17 || batch.CacheHits != 4 {
		t.Errorf("CreditsUsed = %d, CacheHits = %d, want 17 and 4", batch.CreditsUsed, batch.CacheHits)
	}
}

func TestHandleBindsTheClientToThisCallsCeilingAndWindow(t *testing.T) {
	h := fetching(models.SourceReddit, nil, nil)

	var saw models.CollectInput
	h.NewClient = func(_ context.Context, in models.CollectInput) (anakin.Client, error) {
		saw = in
		return stubClient{}, nil
	}

	in := input()
	in.MaxCredits = 12
	if _, err := h.Handle(context.Background(), in); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if saw.MaxCredits != 12 {
		t.Errorf("client built with MaxCredits %d, want 12: the ceiling is per call", saw.MaxCredits)
	}
	if saw.Profile.BrandID != "boat-lifestyle" {
		t.Errorf("client built for brand %q", saw.Profile.BrandID)
	}
}

func TestHandlePassesTheWindowStraightToTheAdapter(t *testing.T) {
	var gotStart, gotEnd time.Time
	h := fetching(models.SourceReddit, nil, nil)
	h.Adapters[models.SourceReddit] = func(_ context.Context, _ anakin.Client, _ models.BrandProfile, start, end time.Time) ([]models.Mention, error) {
		gotStart, gotEnd = start, end
		return nil, nil
	}

	in := input()
	if _, err := h.Handle(context.Background(), in); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if !gotStart.Equal(in.WindowStart) || !gotEnd.Equal(in.WindowEnd) {
		t.Errorf("adapter got [%v, %v), want [%v, %v)", gotStart, gotEnd, in.WindowStart, in.WindowEnd)
	}
}

func TestHandleReportsAClientItCannotBuild(t *testing.T) {
	h := fetching(models.SourceReddit, []models.Mention{mention("a")}, nil)
	h.NewClient = func(context.Context, models.CollectInput) (anakin.Client, error) {
		return nil, errors.New("ANAKIN_API_KEY is empty")
	}

	batch, err := h.Handle(context.Background(), input())
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(batch.Errors) != 1 || !strings.Contains(batch.Errors[0], "ANAKIN_API_KEY") {
		t.Errorf("Errors = %v", batch.Errors)
	}
	if len(batch.Mentions) != 0 {
		t.Error("returned mentions without a client")
	}
}

func TestBatchMarshalsEmptyListsAsArrays(t *testing.T) {
	// DronaHQ binds a table straight to this artifact. A null where a list is
	// declared is a render error there, not a Go problem.
	h := fetching(models.SourceReddit, nil, nil)

	batch, err := h.Handle(context.Background(), input())
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	body, err := json.Marshal(batch)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	for _, want := range []string{`"mentions":[]`, `"errors":[]`} {
		if !strings.Contains(string(body), want) {
			t.Errorf("artifact is missing %s: %s", want, body)
		}
	}
}

func TestNewWiresTheRealRegistry(t *testing.T) {
	// The seam exists for the tests above; it must not quietly change which
	// sources production collects.
	h := New(nil)
	if len(h.Adapters) != len(sources.Adapters) {
		t.Fatalf("New wired %d adapters, the registry has %d", len(h.Adapters), len(sources.Adapters))
	}
	for source := range sources.Adapters {
		if h.Adapters[source] == nil {
			t.Errorf("New did not wire %q", source)
		}
	}
	if h.NewClient == nil || h.SeenHashes == nil {
		t.Error("New left a collaborator nil, which is a nil func call at the first request")
	}
}

func TestAgentCardCarriesWhatNasikoValidateRequires(t *testing.T) {
	// nasiko validate reads this file, and its required-field list is ground
	// truth. A deploy-time rejection is the most expensive place to find out.
	raw, err := os.ReadFile("AgentCard.json")
	if err != nil {
		t.Fatalf("reading AgentCard.json: %v", err)
	}

	var card struct {
		Name               string          `json:"name"`
		Description        string          `json:"description"`
		URL                string          `json:"url"`
		Version            string          `json:"version"`
		Capabilities       json.RawMessage `json:"capabilities"`
		ProtocolVersion    string          `json:"protocolVersion"`
		PreferredTransport string          `json:"preferredTransport"`
		Skills             []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"skills"`
		LLMProvider json.RawMessage `json:"llm_provider"`
	}
	if err := json.Unmarshal(raw, &card); err != nil {
		t.Fatalf("AgentCard.json is not valid JSON: %v", err)
	}

	required := map[string]string{
		"name":               card.Name,
		"description":        card.Description,
		"url":                card.URL,
		"version":            card.Version,
		"preferredTransport": card.PreferredTransport,
	}
	for field, value := range required {
		if value == "" {
			t.Errorf("%s is empty", field)
		}
	}
	if len(card.Capabilities) == 0 {
		t.Error("capabilities is absent")
	}

	// The Nasiko example ships "0.2.9" and a real cluster answers -32009
	// VersionNotSupported.
	if card.ProtocolVersion != "1.0" {
		t.Errorf("protocolVersion = %q, want \"1.0\"", card.ProtocolVersion)
	}
	// An empty skills list is only a validate warning, but it makes the agent
	// invisible to routing, which is a silent failure on stage.
	if len(card.Skills) == 0 {
		t.Fatal("skills is empty, so the orchestrator cannot route to this agent")
	}
	for _, skill := range card.Skills {
		if skill.ID == "" || skill.Name == "" || skill.Description == "" {
			t.Errorf("skill %+v has an empty field", skill)
		}
	}
	// bp-collector makes no LLM call, and the card is what says so.
	if string(card.LLMProvider) != "null" {
		t.Errorf("llm_provider = %s, want null for a deterministic agent", card.LLMProvider)
	}
}
