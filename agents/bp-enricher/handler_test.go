// Tests for batching, joining, caching and the redaction guardrail.
//
// Every test stubs the LLM. CI has no key and no network, so a test that
// reaches a provider is a broken test, not a thorough one.
//
// The handler calls redact.PII for real rather than through an injected seam,
// because a guardrail tested against a stub of itself is not a guardrail.
// redact.PII is still B1's compile-target stub, so these skip until it lands;
// see requireRedactor.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"brandpulse/internal/llm"
	"brandpulse/internal/models"
	"brandpulse/internal/redact"
)

// requireRedactor skips when redact.PII is still the panicking stub. A skip
// reports as a skip: nothing here claims to pass until B1 Task 3 lands, at
// which point every one of these starts enforcing with no edit.
func requireRedactor(t *testing.T) {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Skipf("redact.PII is still B1's stub (%v); this suite enforces once B1 Task 3 lands", recovered)
		}
	}()
	redact.PII("probe")
}

// stubLLM records every prompt it is given and replies from a scripted
// function, so a test can assert on what the model was asked as well as on
// what the handler did with the answer.
type stubLLM struct {
	prompts []string
	calls   int
	reply   func(batchNumber int, prompt string) (json.RawMessage, llm.Usage, error)
}

func (s *stubLLM) chat(_ context.Context, prompt string, _ any, _ llm.Opt) (json.RawMessage, llm.Usage, error) {
	s.calls++
	s.prompts = append(s.prompts, prompt)
	return s.reply(s.calls, prompt)
}

func newHandler(stub *stubLLM) *EnricherHandler {
	h := NewEnricherHandler()
	h.chat = stub.chat
	return h
}

// answerAll replies with one well-formed enrichment per mention id found in
// the prompt, in the order the prompt listed them. The out-of-order case builds
// its own closure around reverse, because that is the one thing it is testing.
func answerAll(_ int, prompt string) (json.RawMessage, llm.Usage, error) {
	ids := idsInPrompt(prompt)
	enrichments := make([]replyEnrichment, 0, len(ids))
	for _, id := range ids {
		enrichments = append(enrichments, replyEnrichment{
			MentionID:      id,
			Sentiment:      -0.8,
			SentimentLabel: models.SentimentNegative,
			Emotion:        models.EmotionAnger,
			Intent:         models.IntentComplaint,
			Aspects:        []string{"delivery"},
			IsAboutBrand:   true,
		})
	}
	raw, err := json.Marshal(enrichmentReply{Enrichments: enrichments})
	usage := llm.Usage{PromptTokens: 100, CompletionTokens: 50, CostPaise: 12}
	return raw, usage, err
}

// answerAllExcept replies for every mention in the prompt but the named ones,
// which is what a model dropping an id out of a large batch looks like.
func answerAllExcept(skipped ...string) func(int, string) (json.RawMessage, llm.Usage, error) {
	return func(_ int, prompt string) (json.RawMessage, llm.Usage, error) {
		enrichments := make([]replyEnrichment, 0)
		for _, id := range idsInPrompt(prompt) {
			if slices.Contains(skipped, id) {
				continue
			}
			enrichments = append(enrichments, replyEnrichment{
				MentionID:      id,
				Sentiment:      -0.8,
				SentimentLabel: models.SentimentNegative,
				Emotion:        models.EmotionAnger,
				Intent:         models.IntentComplaint,
				Aspects:        []string{"delivery"},
				IsAboutBrand:   true,
			})
		}
		raw, err := json.Marshal(enrichmentReply{Enrichments: enrichments})
		return raw, llm.Usage{PromptTokens: 100, CompletionTokens: 50, CostPaise: 12}, err
	}
}

// idsInPrompt pulls the mention ids back out of the rendered prompt. The stub
// has to learn them the same way the model does, or the test proves nothing
// about what was actually sent.
func idsInPrompt(prompt string) []string {
	var ids []string
	for _, line := range strings.Split(prompt, "\n") {
		if _, after, found := strings.Cut(line, `"mention_id": "`); found {
			if id, _, ok := strings.Cut(after, `"`); ok {
				ids = append(ids, id)
			}
		}
	}
	return ids
}

func reverse(ids []string) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[len(ids)-1-i] = id
	}
	return out
}

func testMentions(count int) []models.Mention {
	mentions := make([]models.Mention, 0, count)
	for i := range count {
		mentions = append(mentions, models.Mention{
			ID:          fmt.Sprintf("mn_%03d", i),
			BrandID:     "brd_01J",
			Source:      models.SourceX,
			Text:        fmt.Sprintf("delivery bahut late thi order %d", i),
			ContentHash: fmt.Sprintf("hash_%03d", i),
		})
	}
	return mentions
}

func testProfile() models.BrandProfile {
	profile := models.NewBrandProfile("brd_01J", "Mamaearth")
	profile.Products = []string{"onion hair oil"}
	profile.Keywords = []string{"mamaearth"}
	profile.NegativeKeywords = []string{"mama earth day"}
	profile.Competitors = []string{"Plum"}
	return profile
}

func TestHandleClassifiesEveryMention(t *testing.T) {
	requireRedactor(t)
	stub := &stubLLM{reply: answerAll}

	out, err := newHandler(stub).Handle(context.Background(), models.EnrichInput{
		Mentions: testMentions(3),
		Profile:  testProfile(),
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if len(out.Enrichments) != 3 {
		t.Errorf("got %d enrichments, want 3", len(out.Enrichments))
	}
	if len(out.Errors) != 0 {
		t.Errorf("Errors = %v, want none", out.Errors)
	}
	if out.TokensUsed != 150 {
		t.Errorf("TokensUsed = %d, want 150: the real llm.Usage, not an estimate", out.TokensUsed)
	}
	if out.CostPaise != 12 {
		t.Errorf("CostPaise = %v, want 12", out.CostPaise)
	}
}

// The contract says reply order is not guaranteed. A stub that answers
// backwards is the cheapest way to catch a handler that zips by index.
func TestHandleJoinsOnMentionIDNotPosition(t *testing.T) {
	requireRedactor(t)
	stub := &stubLLM{reply: func(call int, prompt string) (json.RawMessage, llm.Usage, error) {
		ids := reverse(idsInPrompt(prompt))
		enrichments := make([]replyEnrichment, 0, len(ids))
		for i, id := range ids {
			// A distinct sentiment per id, so a positional join produces the
			// wrong number rather than merely the wrong order.
			enrichments = append(enrichments, replyEnrichment{
				MentionID:      id,
				Sentiment:      float64(i) / 10,
				SentimentLabel: models.SentimentNeutral,
				Emotion:        models.EmotionNeutral,
				Intent:         models.IntentOther,
				IsAboutBrand:   true,
			})
		}
		raw, err := json.Marshal(enrichmentReply{Enrichments: enrichments})
		return raw, llm.Usage{}, err
	}}

	out, err := newHandler(stub).Handle(context.Background(), models.EnrichInput{
		Mentions: testMentions(3),
		Profile:  testProfile(),
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	// The stub reversed mn_000, mn_001, mn_002 into mn_002, mn_001, mn_000 and
	// numbered them 0.0, 0.1, 0.2 in that order.
	want := map[string]float64{"mn_002": 0.0, "mn_001": 0.1, "mn_000": 0.2}
	for _, e := range out.Enrichments {
		if got := e.Sentiment; got != want[e.MentionID] {
			t.Errorf("%s: Sentiment = %v, want %v: the reply was joined by position", e.MentionID, got, want[e.MentionID])
		}
	}
}

// The guardrail. redact.PII runs before the prompt is built, so a stub that
// captures the prompt can prove no contact detail reached the model.
func TestHandleRedactsBeforeThePrompt(t *testing.T) {
	requireRedactor(t)
	stub := &stubLLM{reply: answerAll}

	leaks := []string{"shubham@tvaram.com", "+91 98765 43210", "9876543210"}
	mentions := []models.Mention{
		{ID: "mn_email", ContentHash: "h1", Text: "order kahan hai, mail me at shubham@tvaram.com"},
		{ID: "mn_phone", ContentHash: "h2", Text: "call me +91 98765 43210 please"},
		{ID: "mn_bare", ContentHash: "h3", Text: "whatsapp 9876543210 for refund"},
	}

	if _, err := newHandler(stub).Handle(context.Background(), models.EnrichInput{
		Mentions: mentions,
		Profile:  testProfile(),
	}); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if len(stub.prompts) == 0 {
		t.Fatal("no prompt was captured, so nothing was asserted")
	}
	for _, prompt := range stub.prompts {
		for _, leak := range leaks {
			if strings.Contains(prompt, leak) {
				t.Errorf("%q reached the prompt", leak)
			}
		}
	}
}

func TestHandleBatchesAtMostFiftyPerCall(t *testing.T) {
	requireRedactor(t)
	cases := []struct {
		name      string
		mentions  int
		batchSize int
		wantCalls int
	}{
		{"zero means fifty", 120, 0, 3},
		{"explicit smaller size", 10, 4, 3},
		{"cannot exceed fifty", 120, 500, 3},
		{"single short batch", 7, 0, 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := &stubLLM{reply: answerAll}
			out, err := newHandler(stub).Handle(context.Background(), models.EnrichInput{
				Mentions:  testMentions(tc.mentions),
				Profile:   testProfile(),
				BatchSize: tc.batchSize,
			})
			if err != nil {
				t.Fatalf("Handle returned error: %v", err)
			}
			if stub.calls != tc.wantCalls {
				t.Errorf("made %d LLM calls, want %d", stub.calls, tc.wantCalls)
			}
			if len(out.Enrichments) != tc.mentions {
				t.Errorf("got %d enrichments, want %d", len(out.Enrichments), tc.mentions)
			}
		})
	}
}

// The property the warm cost number in eval/ depends on.
func TestHandleSecondRunSpendsNothing(t *testing.T) {
	requireRedactor(t)
	stub := &stubLLM{reply: answerAll}
	handler := newHandler(stub)
	in := models.EnrichInput{Mentions: testMentions(5), Profile: testProfile()}

	if _, err := handler.Handle(context.Background(), in); err != nil {
		t.Fatalf("first Handle returned error: %v", err)
	}
	callsAfterFirst := stub.calls

	out, err := handler.Handle(context.Background(), in)
	if err != nil {
		t.Fatalf("second Handle returned error: %v", err)
	}

	if stub.calls != callsAfterFirst {
		t.Errorf("second run made %d extra LLM calls, want 0", stub.calls-callsAfterFirst)
	}
	if out.CacheHits != 5 {
		t.Errorf("CacheHits = %d, want 5", out.CacheHits)
	}
	if out.TokensUsed != 0 || out.CostPaise != 0 {
		t.Errorf("a fully cached run reported %d tokens and %v paise, want 0 and 0", out.TokensUsed, out.CostPaise)
	}
	if len(out.Enrichments) != 5 {
		t.Errorf("got %d enrichments, want 5: a cache hit is still an answer", len(out.Enrichments))
	}
}

// is_about_brand and about_competitor are judged entirely against the profile,
// so the same tweet is a different question for a different brand. The content
// hash is computed from the source and the text alone and knows nothing about
// either, and this handler is long-lived and shared across requests, so an
// unscoped cache would answer the second brand with the first brand's verdict.
func TestHandleDoesNotServeOneBrandsVerdictToAnother(t *testing.T) {
	requireRedactor(t)
	stub := &stubLLM{reply: answerAll}
	handler := newHandler(stub)

	first := testMentions(5)
	if _, err := handler.Handle(context.Background(), models.EnrichInput{
		Mentions: first,
		Profile:  testProfile(),
	}); err != nil {
		t.Fatalf("first brand's Handle returned error: %v", err)
	}
	callsAfterFirst := stub.calls

	// The same texts, so the same content hashes, asked about by a second
	// brand. Only BrandID differs.
	second := testMentions(5)
	for i := range second {
		second[i].BrandID = "brd_other"
	}
	otherProfile := models.NewBrandProfile("brd_other", "Plum")
	otherProfile.Keywords = []string{"plum"}

	out, err := handler.Handle(context.Background(), models.EnrichInput{
		Mentions: second,
		Profile:  otherProfile,
	})
	if err != nil {
		t.Fatalf("second brand's Handle returned error: %v", err)
	}

	if out.CacheHits != 0 {
		t.Errorf("CacheHits = %d, want 0: these hashes were classified for a different brand", out.CacheHits)
	}
	if stub.calls == callsAfterFirst {
		t.Error("the second brand made no LLM call, so it was served the first brand's classification")
	}
}

func TestHandleRetriesOnceThenDegradesToNeutral(t *testing.T) {
	requireRedactor(t)
	cases := []struct {
		name      string
		reply     func(int, string) (json.RawMessage, llm.Usage, error)
		wantCalls int
	}{
		{
			name: "recovers on the retry",
			reply: func(call int, prompt string) (json.RawMessage, llm.Usage, error) {
				if call == 1 {
					return json.RawMessage(`{"enrichments":[]}`), llm.Usage{PromptTokens: 10}, nil
				}
				return answerAll(call, prompt)
			},
			wantCalls: 2,
		},
		{
			name: "malformed twice",
			reply: func(call int, _ string) (json.RawMessage, llm.Usage, error) {
				return json.RawMessage(`{"enrichments":[{"mention_id":"mn_ghost"}]}`), llm.Usage{PromptTokens: 10}, nil
			},
			wantCalls: 2,
		},
		{
			name: "provider errors twice",
			reply: func(int, string) (json.RawMessage, llm.Usage, error) {
				return nil, llm.Usage{}, fmt.Errorf("upstream 503")
			},
			wantCalls: 2,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := &stubLLM{reply: tc.reply}
			out, err := newHandler(stub).Handle(context.Background(), models.EnrichInput{
				Mentions: testMentions(2),
				Profile:  testProfile(),
			})
			if err != nil {
				t.Fatalf("Handle returned error, but one bad batch must not fail a run: %v", err)
			}
			if stub.calls != tc.wantCalls {
				t.Errorf("made %d calls, want %d", stub.calls, tc.wantCalls)
			}
			if len(out.Enrichments) != 2 {
				t.Fatalf("got %d enrichments, want 2: a failed batch still answers for its mentions", len(out.Enrichments))
			}
			if tc.name == "recovers on the retry" {
				if len(out.Errors) != 0 {
					t.Errorf("Errors = %v, want none after a successful retry", out.Errors)
				}
				return
			}
			if len(out.Errors) != 1 {
				t.Errorf("Errors = %v, want exactly one entry naming the failed batch", out.Errors)
			}
			for _, e := range out.Enrichments {
				if e.SentimentLabel != models.SentimentNeutral || e.IsAboutBrand {
					t.Errorf("%s degraded to %+v, want neutral and not-about-brand", e.MentionID, e)
				}
			}
		})
	}
}

// The live failure: a model that drops one id out of a batch. The other
// mentions were classified correctly and must survive; only the dropped one
// degrades, and its id has to be in Errors or nobody can find it again.
func TestHandleKeepsTheAnsweredMentionsWhenOneIsOmitted(t *testing.T) {
	requireRedactor(t)
	stub := &stubLLM{reply: answerAllExcept("mn_001")}

	out, err := newHandler(stub).Handle(context.Background(), models.EnrichInput{
		Mentions: testMentions(3),
		Profile:  testProfile(),
	})
	if err != nil {
		t.Fatalf("Handle returned error, but an omission must not fail a run: %v", err)
	}
	if len(out.Enrichments) != 3 {
		t.Fatalf("got %d enrichments, want 3", len(out.Enrichments))
	}

	byID := map[string]models.Enrichment{}
	for _, e := range out.Enrichments {
		byID[e.MentionID] = e
	}
	for _, id := range []string{"mn_000", "mn_002"} {
		if got := byID[id].Sentiment; got != -0.8 {
			t.Errorf("%s: Sentiment = %v, want -0.8: an answered mention was thrown away with the omitted one", id, got)
		}
	}
	if degraded := byID["mn_001"]; degraded.SentimentLabel != models.SentimentNeutral || degraded.IsAboutBrand {
		t.Errorf("mn_001 = %+v, want neutral and not-about-brand", degraded)
	}
	if len(out.Errors) != 1 || !strings.Contains(out.Errors[0], "mn_001") {
		t.Errorf("Errors = %v, want one entry naming mn_001", out.Errors)
	}
}

// Re-sending the whole batch pays for the answers already in hand and gives the
// model another chance to drop a different id.
func TestHandleRetriesOnlyTheOmittedMentions(t *testing.T) {
	requireRedactor(t)
	stub := &stubLLM{reply: func(call int, prompt string) (json.RawMessage, llm.Usage, error) {
		if call == 1 {
			return answerAllExcept("mn_001")(call, prompt)
		}
		return answerAll(call, prompt)
	}}

	out, err := newHandler(stub).Handle(context.Background(), models.EnrichInput{
		Mentions: testMentions(3),
		Profile:  testProfile(),
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if stub.calls != 2 {
		t.Fatalf("made %d calls, want 2", stub.calls)
	}
	if got := idsInPrompt(stub.prompts[1]); !slices.Equal(got, []string{"mn_001"}) {
		t.Errorf("retry asked about %v, want only [mn_001]", got)
	}
	if len(out.Errors) != 0 {
		t.Errorf("Errors = %v, want none after a successful retry", out.Errors)
	}
	for _, e := range out.Enrichments {
		if e.SentimentLabel == models.SentimentNeutral {
			t.Errorf("%s degraded to neutral although the retry answered it", e.MentionID)
		}
	}
}

// A degraded enrichment is a placeholder, not an answer. Caching it would make
// the mention permanently neutral on every later run.
func TestHandleDoesNotCacheADegradedMention(t *testing.T) {
	requireRedactor(t)
	stub := &stubLLM{reply: answerAllExcept("mn_001")}
	handler := newHandler(stub)
	in := models.EnrichInput{Mentions: testMentions(3), Profile: testProfile()}

	if _, err := handler.Handle(context.Background(), in); err != nil {
		t.Fatalf("first Handle returned error: %v", err)
	}

	stub.reply = answerAll
	out, err := handler.Handle(context.Background(), in)
	if err != nil {
		t.Fatalf("second Handle returned error: %v", err)
	}
	if out.CacheHits != 2 {
		t.Errorf("CacheHits = %d, want 2: the two answered mentions, and not the degraded one", out.CacheHits)
	}
	for _, e := range out.Enrichments {
		if e.MentionID == "mn_001" && e.SentimentLabel == models.SentimentNeutral {
			t.Error("mn_001 came back neutral on the second run, so the degraded enrichment was cached")
		}
	}
}

// Labelling, not filtering. A mention the model judged irrelevant still comes
// back, or the eval set has nothing to score it against.
func TestHandleReturnsMentionsThatAreNotAboutTheBrand(t *testing.T) {
	requireRedactor(t)
	stub := &stubLLM{reply: func(_ int, prompt string) (json.RawMessage, llm.Usage, error) {
		var enrichments []replyEnrichment
		for i, id := range idsInPrompt(prompt) {
			enrichments = append(enrichments, replyEnrichment{
				MentionID:      id,
				SentimentLabel: models.SentimentNeutral,
				Emotion:        models.EmotionNeutral,
				Intent:         models.IntentOther,
				IsAboutBrand:   i == 0,
			})
		}
		raw, err := json.Marshal(enrichmentReply{Enrichments: enrichments})
		return raw, llm.Usage{}, err
	}}

	out, err := newHandler(stub).Handle(context.Background(), models.EnrichInput{
		Mentions: testMentions(3),
		Profile:  testProfile(),
	})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if len(out.Enrichments) != 3 {
		t.Errorf("got %d enrichments, want 3: off-brand mentions are labelled, not dropped", len(out.Enrichments))
	}
}

func TestHandleEmptyInput(t *testing.T) {
	requireRedactor(t)
	stub := &stubLLM{reply: answerAll}

	out, err := newHandler(stub).Handle(context.Background(), models.EnrichInput{Profile: testProfile()})
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if stub.calls != 0 {
		t.Errorf("made %d LLM calls on empty input, want 0", stub.calls)
	}
	if out.Enrichments == nil || out.Errors == nil {
		t.Error("nil slices marshal as null; the contract's zero value is an empty list")
	}
}

// The prompt is the embedded one, and it carries the disambiguation context
// the model needs to judge IsAboutBrand.
func TestPromptCarriesTheProfileContext(t *testing.T) {
	requireRedactor(t)
	stub := &stubLLM{reply: answerAll}

	if _, err := newHandler(stub).Handle(context.Background(), models.EnrichInput{
		Mentions: testMentions(1),
		Profile:  testProfile(),
	}); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	prompt := stub.prompts[0]
	for _, want := range []string{"Mamaearth", "onion hair oil", "mama earth day", "Plum", "mn_000"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt is missing %q", want)
		}
	}
	if strings.Contains(prompt, "{{.") {
		t.Error("prompt still contains an unrendered template placeholder")
	}
}
