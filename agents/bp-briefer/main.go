// bp-briefer writes the daily and the weekly brief for one brand.
//
// One model call, and it produces exactly three things: a headline, a narrative
// and a short list of suggested actions. Every number in the output is
// formatted by render.go from models.BriefNumbers. The prompt forbids the model
// from writing a number at all, because a restated percentage is wrong slowly
// and then wrong on stage.
//
// This agent does not query the database. internal/db is not imported here: the
// orchestrator hands it every topic, alert, SOV figure and number it needs,
// which is what makes it pure and testable.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"text/template"

	"brandpulse/internal/a2a"
	"brandpulse/internal/llm"
	"brandpulse/internal/models"
	"brandpulse/internal/obs"
	"brandpulse/internal/prompts"
	"brandpulse/internal/redact"
)

const (
	// brieferModel is what the cost table is keyed against. The Nasiko router
	// discards the model field in the request body and per-agent choice is set
	// with `nasiko llm-config`, so this is a label, not a routing instruction.
	brieferModel = "bp-briefer-default"

	// brieferMaxTokens covers a headline, three short paragraphs and four
	// actions with room to spare. A brief that runs longer is not a brief.
	brieferMaxTokens = 1400

	// maxSuggestedActions matches what the prompt asks for. The model is asked
	// for two to four; this is the ceiling actually enforced.
	maxSuggestedActions = 4

	// promptTopics and promptExamples cap what the model reads. Beyond this the
	// tail of a long topic list contributes noise and tokens, not insight.
	promptTopics   = 8
	promptExamples = 2

	periodDaily  = "daily"
	periodWeekly = "weekly"

	// cardPath is where the Dockerfile puts AgentCard.json. Building a Card
	// literal here instead would skip every field the file carries, including
	// supportedInterfaces, and a2a.Serve refuses a card without one.
	cardPath = "/AgentCard.json"
)

// BrieferHandler holds the model call as a dependency rather than calling
// llm.ChatJSON directly, so a test can hand it a canned response instead of a
// live router.
type BrieferHandler struct {
	ChatJSON llm.ChatJSONFunc

	prompt *template.Template
}

// NewBrieferHandler parses the embedded prompt once. A malformed prompt is a
// startup failure, not a per-request one.
func NewBrieferHandler(chat llm.ChatJSONFunc) BrieferHandler {
	return BrieferHandler{
		ChatJSON: chat,
		prompt:   template.Must(template.New("briefer").Parse(prompts.Load("briefer"))),
	}
}

// briefResponse is the JSON shape the model is asked for. There is no numbers
// field on purpose: the model is never given a way to hand one back.
type briefResponse struct {
	Headline         string   `json:"headline"`
	Narrative        string   `json:"narrative"`
	SuggestedActions []string `json:"suggested_actions"`
}

// promptData is what briefer.md is rendered against. Every field is a string
// that Go has already formatted.
type promptData struct {
	Period      string
	Brand       string
	PeriodStart string
	PeriodEnd   string
	Numbers     string
	Topics      string
	Alerts      string
	Competitors string
	Language    string
}

// Handle writes one brief.
//
// The error return is for malformed input and for a failed model call. Unlike a
// batch agent there is no partial brief worth returning: a brief missing its
// narrative is not a degraded brief, it is an empty screen, and the caller
// needs to know the difference.
func (h BrieferHandler) Handle(ctx context.Context, in models.BriefInput) (models.DailyBrief, error) {
	if in.BrandID == "" {
		return models.DailyBrief{}, fmt.Errorf("bp-briefer: BriefInput.brand_id is required")
	}

	brief := models.NewDailyBrief(in.BrandID, in.PeriodStart, in.PeriodEnd)
	brief.Numbers = in.Numbers
	brief.TopTopics = in.Topics
	brief.Alerts = in.Alerts
	brief.CompetitorWatch = competitorWatch(in)

	written, err := h.write(ctx, in)
	if err != nil {
		return models.DailyBrief{}, err
	}

	brief.Headline = strings.TrimSpace(written.Headline)
	brief.SuggestedActions = cleanActions(written.SuggestedActions)
	brief.Markdown = renderMarkdown(brief, periodOf(in), in.Profile.Name, written.Narrative)
	brief.WhatsappShort = renderWhatsapp(brief)

	if err := brief.Validate(); err != nil {
		return models.DailyBrief{}, fmt.Errorf("bp-briefer: %w", err)
	}
	return brief, nil
}

// write renders the prompt and makes the one model call.
func (h BrieferHandler) write(ctx context.Context, in models.BriefInput) (briefResponse, error) {
	var rendered strings.Builder
	if err := h.prompt.Execute(&rendered, buildPromptData(in)); err != nil {
		return briefResponse{}, fmt.Errorf("bp-briefer: rendering the prompt: %w", err)
	}

	body, _, err := h.ChatJSON(ctx, rendered.String(), briefResponse{}, llm.Opt{
		Model:     brieferModel,
		MaxTokens: brieferMaxTokens,
	})
	if err != nil {
		return briefResponse{}, fmt.Errorf("bp-briefer: writing the brief: %w", err)
	}

	var written briefResponse
	if err := json.Unmarshal(body, &written); err != nil {
		return briefResponse{}, fmt.Errorf("bp-briefer: the model did not return the brief shape: %w", err)
	}
	if strings.TrimSpace(written.Headline) == "" {
		return briefResponse{}, fmt.Errorf("bp-briefer: the model returned no headline")
	}
	return written, nil
}

// ---------------------------------------------------------------------------
// Prompt assembly. Everything here is a string the model reads, never a
// structure it is asked to compute over.
// ---------------------------------------------------------------------------

func buildPromptData(in models.BriefInput) promptData {
	return promptData{
		Period:      periodOf(in),
		Brand:       in.Profile.Name,
		PeriodStart: in.PeriodStart.UTC().Format("2 Jan 2006 15:04 UTC"),
		PeriodEnd:   in.PeriodEnd.UTC().Format("2 Jan 2006 15:04 UTC"),
		Numbers:     formatNumbersLine(in.Numbers),
		Topics:      promptTopicList(in.Topics),
		Alerts:      promptAlertList(in.Alerts),
		Competitors: strings.Join(competitorWatch(in), ", "),
		Language:    languageOf(in),
	}
}

// promptTopicList carries a sample quote per topic, which is what lets the
// narrative say why something moved rather than only that it moved.
//
// Every quote goes through redact.PII first. A phone number in a complaint is
// not something to hand to a router.
func promptTopicList(topics []models.Topic) string {
	var b strings.Builder
	for i, topic := range firstN(topics, promptTopics) {
		fmt.Fprintf(&b, "%d. %s, %d mentions, %s. %s\n",
			i+1, topic.Label, topic.Size, formatTrend(topic.Trend), topic.Summary)
		for j, example := range topic.TopExamples {
			if j >= promptExamples {
				break
			}
			fmt.Fprintf(&b, "   - (%s) %s\n", example.Source, redact.PII(example.Text))
		}
	}
	return strings.TrimSpace(b.String())
}

// promptAlertList carries each alert's numbers and thresholds, so the narrative
// can explain an alert without the model having to reconstruct one.
func promptAlertList(alerts []models.Alert) string {
	var b strings.Builder
	for _, alert := range mostSevere(alerts, len(alerts)) {
		fmt.Fprintf(&b, "- %s (%s): %s\n", alert.Kind, alert.Severity, alert.Why)
		for _, e := range alert.Evidence {
			fmt.Fprintf(&b, "  - %s: %s, fires at %s, over %s\n",
				e.Metric, trimFloat(e.Value), trimFloat(e.Threshold), e.Window)
		}
		for j, sample := range alert.SampleMentions {
			if j >= promptExamples {
				break
			}
			fmt.Fprintf(&b, "  - (%s) %s\n", sample.Source, redact.PII(sample.Text))
		}
	}
	return strings.TrimSpace(b.String())
}

// ---------------------------------------------------------------------------
// Field defaults. Go has no field defaults and a zero value here is a brief
// that reads wrong rather than one that fails.
// ---------------------------------------------------------------------------

// periodOf defaults an empty Period to daily. The contract allows only "daily"
// and "weekly", and an unrecognised value is passed through rather than
// silently relabelled: if a caller invents a period, the brief should say so.
func periodOf(in models.BriefInput) string {
	if in.Period == "" {
		return periodDaily
	}
	return in.Period
}

func languageOf(in models.BriefInput) string {
	if in.Profile.Voice.Language == "" {
		return "English"
	}
	return in.Profile.Voice.Language
}

// competitorWatch is the competitors this brand tracks that actually appeared
// in the window, sorted so the brief renders the same way twice. A competitor
// with no share is not watched: naming one that nobody mentioned pads the brief
// and invites the model to invent a reason.
func competitorWatch(in models.BriefInput) []string {
	watched := make([]string, 0, len(in.Profile.Competitors))
	for _, name := range in.Profile.Competitors {
		if in.SOV.CompetitorShares[name] > 0 {
			watched = append(watched, name)
		}
	}
	sort.Strings(watched)
	return watched
}

// cleanActions trims the model's list to what the brief asks for and drops
// blanks, which a model produces when it has fewer than the minimum to say.
func cleanActions(actions []string) []string {
	cleaned := make([]string, 0, len(actions))
	for _, action := range actions {
		action = strings.TrimSpace(action)
		if action == "" {
			continue
		}
		cleaned = append(cleaned, action)
		if len(cleaned) == maxSuggestedActions {
			break
		}
	}
	return cleaned
}

func main() {
	shutdown, err := obs.Setup("bp-briefer")
	if err != nil {
		log.Fatalf("bp-briefer: observability setup failed: %v", err)
	}
	defer func() { _ = shutdown(context.Background()) }()

	card, err := a2a.LoadCard(cardPath)
	if err != nil {
		log.Fatalf("bp-briefer: load agent card: %v", err)
	}

	handler := NewBrieferHandler(llm.ChatJSON)
	if err := a2a.Serve(card, handler); err != nil {
		log.Fatalf("bp-briefer: %v", err)
	}
}
