// The tests for bp-briefer.
//
// The renderers are pure, so most of this file needs no model at all. The
// handler tests use a stub that returns canned JSON, which is what keeps the
// suite at zero credits under BP_FIXTURE_MODE=replay.
//
// What is actually being defended here: the numbers in a brief come from
// models.BriefNumbers and never from the model, the WhatsApp brief fits in
// models.WhatsappShortLimit with no tables and no links, and no mention text
// reaches a prompt un-redacted.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"strings"
	"testing"
	"time"

	"brandpulse/internal/llm"
	"brandpulse/internal/models"
)

var (
	periodStart = time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	periodEnd   = time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
)

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

// stubModel stands in for llm.ChatJSON. It records the prompt it was given, so
// a test can assert on what the model was allowed to see.
type stubModel struct {
	response briefResponse
	err      error

	calls   int
	prompts []string
}

func (s *stubModel) chatJSON(_ context.Context, prompt string, _ any, _ llm.Opt) (json.RawMessage, llm.Usage, error) {
	s.calls++
	s.prompts = append(s.prompts, prompt)
	if s.err != nil {
		return nil, llm.Usage{}, s.err
	}
	body, err := json.Marshal(s.response)
	if err != nil {
		return nil, llm.Usage{}, err
	}
	return body, llm.Usage{PromptTokens: 900, CompletionTokens: 200, CostPaise: 4}, nil
}

func okResponse() briefResponse {
	return briefResponse{
		Headline:         "Delivery delays dominated the week and sentiment turned sharply negative",
		Narrative:        "The volume rose because one delivery thread was shared widely.\n\nI am not sure whether the courier change caused it or only coincided with it.",
		SuggestedActions: []string{"Answer the delivery thread on Twitter today", "Ask the courier for a written delay report"},
	}
}

func testProfile() models.BrandProfile {
	profile := models.NewBrandProfile("brand_1", "Testbrand")
	profile.Competitors = []string{"Rivalco", "Neverseen"}
	profile.Voice = models.NewBrandVoice()
	return profile
}

func testNumbers() models.BriefNumbers {
	return models.BriefNumbers{
		Mentions:         1284,
		MentionsDeltaPct: 38.2,
		SentimentAvg:     -0.42,
		NegativeShare:    61.5,
		ShareOfVoice:     31.4,
	}
}

func testMention(text string) models.Mention {
	return models.Mention{
		ID:       "men_1",
		BrandID:  "brand_1",
		Source:   models.SourceX,
		Text:     text,
		Lang:     "en",
		PostedAt: periodEnd,
	}
}

func testTopic(label string, size int, trend float64) models.Topic {
	return models.Topic{
		ID:          "top_" + label,
		BrandID:     "brand_1",
		WindowStart: periodStart,
		WindowEnd:   periodEnd,
		Label:       label,
		Summary:     "People are talking about " + label + ".",
		Size:        size,
		Trend:       trend,
		TopExamples: []models.Mention{testMention("my order is late, call me on 9876543210")},
	}
}

func testAlert(kind models.AlertKind, severity models.Severity) models.Alert {
	return models.Alert{
		ID:       "alt_" + string(kind),
		BrandID:  "brand_1",
		Kind:     kind,
		Severity: severity,
		Title:    string(kind) + " on x",
		Why:      "85% of the 20 mentions on x in the last hour were negative (fires at 60%)",
		Evidence: []models.AlertEvidence{
			{Metric: "negative_share", Value: 0.85, Threshold: 0.6, Window: "1h"},
		},
		SampleMentions: []models.Mention{testMention("terrible, email me at ravi@example.com")},
		Status:         models.AlertStatusOpen,
		DedupeKey:      string(kind) + ":x:2026-09-20T14",
		CreatedAt:      periodEnd,
	}
}

func testInput(period string) models.BriefInput {
	sov := models.NewShareOfVoice("brand_1", periodStart, periodEnd)
	sov.BrandShare = 31.4
	sov.CompetitorShares = map[string]float64{"Rivalco": 22.0, "Neverseen": 0}

	return models.BriefInput{
		BrandID:     "brand_1",
		Profile:     testProfile(),
		Period:      period,
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
		Topics:      []models.Topic{testTopic("delivery delay", 140, 2.4), testTopic("packaging", 30, 1.0)},
		Alerts:      []models.Alert{testAlert(models.AlertCrisis, models.SeverityCritical)},
		SOV:         sov,
		Numbers:     testNumbers(),
	}
}

func handle(t *testing.T, in models.BriefInput, model *stubModel) models.DailyBrief {
	t.Helper()
	brief, err := NewBrieferHandler(model.chatJSON).Handle(context.Background(), in)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	return brief
}

// ---------------------------------------------------------------------------
// Number formatting
// ---------------------------------------------------------------------------

func TestFormatters(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"commas under a thousand", commas(987), "987"},
		{"commas at a thousand", commas(1000), "1,000"},
		{"commas in the millions", commas(1234567), "1,234,567"},
		{"commas on a negative delta count", commas(-4210), "-4,210"},

		{"a rise reads as up", formatDelta(38.2), "up 38.2%"},
		{"a fall drops the minus sign", formatDelta(-12.5), "down 12.5%"},
		{"exactly zero is flat, not up zero", formatDelta(0), "flat"},

		{"a percent rounds to one place", formatPercent(61.4999), "61.5%"},
		{"a whole percent loses its decimal", formatPercent(30.0), "30%"},

		{"sentiment keeps its sign and both decimals", formatSentiment(-0.4), "-0.40"},
		{"positive sentiment keeps both decimals", formatSentiment(0.125), "0.13"},

		{"a flat trend is flat", formatTrend(1.0), "flat"},
		{"a first window with no prior reads flat", formatTrend(1.04), "flat"},
		{"a doubled topic reads as up 100%", formatTrend(2.0), "up 100%"},
		{"a halved topic reads as down 50%", formatTrend(0.5), "down 50%"},

		{"an evidence integer loses its .0", trimFloat(4.0), "4"},
		{"an evidence decimal keeps it", trimFloat(4.2), "4.2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %q, want %q", tt.got, tt.want)
			}
		})
	}
}

func TestNumbersLineCarriesEveryFigure(t *testing.T) {
	line := formatNumbersLine(testNumbers())
	for _, want := range []string{"1,284", "up 38.2%", "61.5%", "-0.42", "31.4%"} {
		if !strings.Contains(line, want) {
			t.Errorf("numbers line %q is missing %q", line, want)
		}
	}
}

// ---------------------------------------------------------------------------
// The WhatsApp brief
// ---------------------------------------------------------------------------

func TestWhatsappBriefIsSendable(t *testing.T) {
	brief := handle(t, testInput(periodDaily), &stubModel{response: okResponse()})
	short := brief.WhatsappShort

	if strings.Contains(short, "|") {
		t.Errorf("the WhatsApp brief contains a table pipe:\n%s", short)
	}
	if strings.Contains(short, "](") {
		t.Errorf("the WhatsApp brief contains a markdown link:\n%s", short)
	}
	if len(short) > models.WhatsappShortLimit {
		t.Errorf("the WhatsApp brief is %d bytes, limit is %d", len(short), models.WhatsappShortLimit)
	}
	if !strings.Contains(short, "1,284 mentions") {
		t.Errorf("the WhatsApp brief lost its headline number:\n%s", short)
	}
}

// A label or an action arrives from a model, so either can carry markdown the
// renderer has to strip rather than pass through.
func TestWhatsappStripsMarkdownFromModelText(t *testing.T) {
	in := testInput(periodDaily)
	in.Topics = []models.Topic{testTopic("[delivery](https://x.test/t) | refunds", 140, 2.0)}

	model := &stubModel{response: briefResponse{
		Headline:         "Trouble on the [delivery thread](https://x.test/t)",
		Narrative:        "Short.",
		SuggestedActions: []string{"Fix the | pipe in the topic name"},
	}}
	brief := handle(t, in, model)

	if strings.Contains(brief.WhatsappShort, "](") || strings.Contains(brief.WhatsappShort, "|") {
		t.Errorf("markdown survived into the WhatsApp brief:\n%s", brief.WhatsappShort)
	}
	if !strings.Contains(brief.WhatsappShort, "delivery thread") {
		t.Errorf("stripping a link deleted its text:\n%s", brief.WhatsappShort)
	}
}

// The limit is enforced in render.go, not left to Validate. This input is long
// enough to force the cut and the cut has to land on a sentence.
func TestWhatsappTruncatesAtASentenceBoundary(t *testing.T) {
	in := testInput(periodDaily)
	in.Topics = []models.Topic{
		testTopic("delivery delays in tier two cities", 140, 2.4),
		testTopic("packaging arriving crushed and taped", 90, 1.8),
		testTopic("refund requests going unanswered for days", 60, 3.1),
	}

	model := &stubModel{response: briefResponse{
		Headline: "Delivery delays dominated the period and sentiment turned sharply negative across every source we watch today",
		SuggestedActions: []string{
			"Answer the pinned delivery thread on Twitter before the end of the day and say when the backlog clears",
			"Ask the courier for a written delay report covering every tier two city on the list",
			"Rewrite the packaging page so it stops promising next day delivery in cities that do not get it",
		},
	}}

	brief := handle(t, in, model)
	short := brief.WhatsappShort

	if len(short) > models.WhatsappShortLimit {
		t.Fatalf("truncation did not hold the limit: %d bytes, limit %d", len(short), models.WhatsappShortLimit)
	}
	if len(short) == models.WhatsappShortLimit {
		t.Errorf("the brief was cut at exactly the limit, which means it was cut mid-sentence:\n%s", short)
	}
	if last := short[len(short)-1]; !strings.ContainsRune(".!?", rune(last)) {
		t.Errorf("the brief does not end on a sentence, it ends on %q:\n%s", last, short)
	}
	if err := brief.Validate(); err != nil {
		t.Errorf("a truncated brief still failed validation: %v", err)
	}
}

func TestTruncateAtSentence(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		limit int
		want  string
	}{
		{"short enough is untouched", "One sentence.", 50, "One sentence."},
		{"cuts back to the last full stop", "One sentence. And a second one that runs over.", 20, "One sentence."},
		{"cuts back to the last question mark", "Who ordered this? Nobody knows for certain.", 25, "Who ordered this?"},
		{"falls back to a word boundary when there is no sentence end", "aaaa bbbb cccc dddd eeee", 12, "aaaa bbbb"},
		// Each of these letters is 3 bytes, so a 10-byte cut lands inside the
		// fourth one. What comes back is the three whole letters, never the
		// first byte of a fourth.
		{"never returns a partial rune", "देरी देरी देरी देरी देरी", 10, "देर"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateAtSentence(tt.text, tt.limit)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
			if len(got) > tt.limit {
				t.Errorf("got %d bytes, limit was %d", len(got), tt.limit)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// The markdown brief
// ---------------------------------------------------------------------------

func TestMarkdownCarriesTheNumbersAndTheEvidence(t *testing.T) {
	brief := handle(t, testInput(periodDaily), &stubModel{response: okResponse()})

	for _, want := range []string{
		"# Testbrand: daily brief",
		"19 Sep to 20 Sep 2026",
		"| Mentions | 1,284 (up 38.2%) |",
		"| Average sentiment | -0.42 |",
		"| Negative share | 61.5% |",
		"| Share of voice | 31.4% |",
		"**crisis** (critical)",
		"negative_share: 0.85, fires at 0.6, over 1h",
		"**delivery delay**, 140 mentions, up 140%",
		"Rivalco",
		"Answer the delivery thread on Twitter today",
	} {
		if !strings.Contains(brief.Markdown, want) {
			t.Errorf("the markdown brief is missing %q:\n%s", want, brief.Markdown)
		}
	}
}

func TestWeeklyUsesTheSamePath(t *testing.T) {
	model := &stubModel{response: okResponse()}
	brief := handle(t, testInput(periodWeekly), model)

	if !strings.Contains(brief.Markdown, "# Testbrand: weekly brief") {
		t.Errorf("a weekly brief did not label itself weekly:\n%s", brief.Markdown)
	}
	if !strings.Contains(model.prompts[0], "weekly") {
		t.Error("the prompt did not tell the model it was writing a weekly brief")
	}
	if brief.Numbers != testNumbers() {
		t.Errorf("weekly changed the numbers: got %+v", brief.Numbers)
	}
}

// ---------------------------------------------------------------------------
// The model never supplies a number
// ---------------------------------------------------------------------------

// The model is told never to write a number. This asserts the stronger thing:
// even when it ignores that, the figures in the brief are the measured ones.
func TestNumbersComeFromTheInputNotTheModel(t *testing.T) {
	model := &stubModel{response: briefResponse{
		Headline:         "Mentions rose 900% to 50,000 and sentiment hit -0.99",
		Narrative:        "Share of voice is now 88%.",
		SuggestedActions: []string{"Ignore the 900% figure above"},
	}}
	brief := handle(t, testInput(periodDaily), model)

	if brief.Numbers != testNumbers() {
		t.Errorf("the model's numbers leaked into BriefNumbers: got %+v, want %+v", brief.Numbers, testNumbers())
	}
	if !strings.Contains(brief.Markdown, "| Mentions | 1,284 (up 38.2%) |") {
		t.Errorf("the numbers table does not hold the measured figure:\n%s", brief.Markdown)
	}
	if !strings.Contains(brief.WhatsappShort, "1,284 mentions, up 38.2%") {
		t.Errorf("the WhatsApp brief does not hold the measured figure:\n%s", brief.WhatsappShort)
	}
}

func TestPromptForbidsWritingANumber(t *testing.T) {
	model := &stubModel{response: okResponse()}
	handle(t, testInput(periodDaily), model)

	if model.calls != 1 {
		t.Errorf("a brief took %d model calls, the contract is one", model.calls)
	}
	if !strings.Contains(model.prompts[0], "Never write a number.") {
		t.Errorf("the prompt does not carry the rule that stops a restated percentage:\n%s", model.prompts[0])
	}
}

// ---------------------------------------------------------------------------
// Redaction, input handling and the boundaries of this agent
// ---------------------------------------------------------------------------

func TestMentionTextIsRedactedBeforeThePrompt(t *testing.T) {
	model := &stubModel{response: okResponse()}
	handle(t, testInput(periodDaily), model)

	prompt := model.prompts[0]
	for _, raw := range []string{"9876543210", "ravi@example.com"} {
		if strings.Contains(prompt, raw) {
			t.Errorf("raw PII %q reached the prompt:\n%s", raw, prompt)
		}
	}
	for _, want := range []string{"[phone]", "[email]"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the prompt has no %s placeholder, so the sample text never arrived:\n%s", want, prompt)
		}
	}
}

func TestCompetitorWatchHoldsOnlyCompetitorsWhoAppeared(t *testing.T) {
	brief := handle(t, testInput(periodDaily), &stubModel{response: okResponse()})

	want := []string{"Rivalco"}
	if fmt.Sprint(brief.CompetitorWatch) != fmt.Sprint(want) {
		t.Errorf("competitor watch = %v, want %v: Neverseen had no share in the window", brief.CompetitorWatch, want)
	}
}

func TestHandleRejectsInputWithNoBrand(t *testing.T) {
	model := &stubModel{response: okResponse()}
	in := testInput(periodDaily)
	in.BrandID = ""

	if _, err := NewBrieferHandler(model.chatJSON).Handle(context.Background(), in); err == nil {
		t.Fatal("a brief with no brand id was accepted")
	}
	if model.calls != 0 {
		t.Errorf("malformed input still cost %d model calls", model.calls)
	}
}

func TestAFailedModelCallIsAnError(t *testing.T) {
	model := &stubModel{err: fmt.Errorf("router timed out")}

	brief, err := NewBrieferHandler(model.chatJSON).Handle(context.Background(), testInput(periodDaily))
	if err == nil {
		t.Fatal("a failed model call produced a brief")
	}
	if brief.Markdown != "" || brief.Headline != "" {
		t.Errorf("a failed call returned a half-written brief: %+v", brief)
	}
}

func TestAnEmptyPeriodDefaultsToDaily(t *testing.T) {
	brief := handle(t, testInput(""), &stubModel{response: okResponse()})
	if !strings.Contains(brief.Markdown, "daily brief") {
		t.Errorf("an empty Period did not default to daily:\n%s", brief.Markdown)
	}
}

func TestSuggestedActionsAreCappedAndBlanksDropped(t *testing.T) {
	model := &stubModel{response: briefResponse{
		Headline:         "Quiet period",
		SuggestedActions: []string{"One", "  ", "Two", "Three", "Four", "Five"},
	}}
	brief := handle(t, testInput(periodDaily), model)

	if len(brief.SuggestedActions) != maxSuggestedActions {
		t.Fatalf("got %d actions, want %d", len(brief.SuggestedActions), maxSuggestedActions)
	}
	for _, action := range brief.SuggestedActions {
		if strings.TrimSpace(action) == "" {
			t.Error("a blank action survived into the brief")
		}
	}
}

// The briefer is handed every figure it needs. The moment it can query, it
// stops being pure and its tests start needing a database.
func TestBrieferDoesNotTouchTheDatabase(t *testing.T) {
	forbidden := []string{"brandpulse/internal/db", "database/sql", "github.com/jackc/pgx"}

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parsing bp-briefer: %v", err)
	}

	for _, pkg := range pkgs {
		for name, file := range pkg.Files {
			for _, imported := range file.Imports {
				path := strings.Trim(imported.Path.Value, `"`)
				for _, bad := range forbidden {
					if strings.HasPrefix(path, bad) {
						t.Errorf("%s imports %s: the orchestrator hands the briefer its numbers", name, path)
					}
				}
			}
		}
	}
}
