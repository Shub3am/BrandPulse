// Everything about one LLM call: how many mentions go into it, what the prompt
// looks like, and what shape comes back.
//
// This file must not know about caching or about the handler's accounting. It
// also must not put a raw Mention.Text into a prompt: the text it receives has
// already been through redact.PII and it has no way to check, which is exactly
// why the redaction happens one layer up, in one place, with a test on it.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"text/template"

	"brandpulse/internal/models"
	"brandpulse/internal/prompts"
)

// defaultBatchSize is applied when EnrichInput.BatchSize is 0, per CONTRACTS
// §2. 50 is the contract's number, not a tuning knob.
const defaultBatchSize = 50

// enricherPrompt is parsed once. prompts.Load panics on a missing prompt, so a
// renamed file fails at process start rather than on the first classification.
var enricherPrompt = template.Must(template.New("enricher").Parse(prompts.Load("enricher")))

// promptMention is one mention as the model sees it: an id to copy back and
// text that has already been redacted. Nothing else, because the prompt tells
// the model not to infer from the author, the source or the follower count,
// and the cheapest way to enforce that is not to send them.
type promptMention struct {
	MentionID string `json:"mention_id"`
	Text      string `json:"text"`
}

// enrichmentReply is the schema handed to llm.ChatJSON and the shape parsed
// back out of it. One key, because a bare top-level array is the format models
// most often wrap in prose.
type enrichmentReply struct {
	Enrichments []replyEnrichment `json:"enrichments"`
}

type replyEnrichment struct {
	MentionID       string                `json:"mention_id"`
	Sentiment       float64               `json:"sentiment"`
	SentimentLabel  models.SentimentLabel `json:"sentiment_label"`
	Emotion         models.Emotion        `json:"emotion"`
	Intent          models.Intent         `json:"intent"`
	Aspects         []string              `json:"aspects"`
	IsAboutBrand    bool                  `json:"is_about_brand"`
	AboutCompetitor string                `json:"about_competitor"`
}

// buildPrompt renders the embedded template for one batch. redacted is keyed
// by mention id and is the only source of text: reaching into the Mention for
// it would bypass redaction.
func buildPrompt(batch []models.Mention, redacted map[string]string, profile models.BrandProfile) (string, error) {
	forModel := make([]promptMention, 0, len(batch))
	for _, m := range batch {
		forModel = append(forModel, promptMention{MentionID: m.ID, Text: redacted[m.ID]})
	}
	encoded, err := json.MarshalIndent(forModel, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode mentions for prompt: %w", err)
	}

	var rendered strings.Builder
	err = enricherPrompt.Execute(&rendered, struct {
		BrandName        string
		Products         string
		Keywords         string
		NegativeKeywords string
		Competitors      string
		Count            int
		Mentions         string
	}{
		BrandName:        profile.Name,
		Products:         list(profile.Products),
		Keywords:         list(profile.Keywords),
		NegativeKeywords: list(profile.NegativeKeywords),
		Competitors:      list(profile.Competitors),
		// The real size of this batch, not a figure written into the prompt.
		// The template used to say "a batch of twelve returns twelve objects"
		// while the payload carried fifty, and a model that believes the
		// sentence stops at twelve: the rest come back as the omissions
		// parseReply reports and the handler degrades to neutral.
		Count:    len(batch),
		Mentions: string(encoded),
	})
	if err != nil {
		return "", fmt.Errorf("render enricher prompt: %w", err)
	}
	return rendered.String(), nil
}

// list renders a string slice for the prompt. "(none)" rather than an empty
// line, because a blank after "Competitors:" reads to a model as a truncated
// prompt.
func list(values []string) string {
	if len(values) == 0 {
		return "(none)"
	}
	return strings.Join(values, ", ")
}

// parseReply decodes a batch response and splits the mentions asked about into
// the ones it answered usably and the ones it did not. A gap is per mention and
// never fatal for the batch: the caller joins on the id, so one dropped or
// invalid object costs that one mention and nothing else.
//
// The returned error is the whole report and not a sample of it: every object
// that decoded but failed Validate is named, and the ids a reply left out
// entirely are named together on one line. "The model sent sentiment 4.2" is a
// different problem from "the model said nothing" and an operator reading
// Errors has to be able to tell which happened to which id.
func parseReply(raw json.RawMessage, batch []models.Mention) ([]models.Enrichment, []models.Mention, error) {
	var reply enrichmentReply
	if err := json.Unmarshal(raw, &reply); err != nil {
		return nil, batch, fmt.Errorf("decode reply: %w", err)
	}

	byID := make(map[string]replyEnrichment, len(reply.Enrichments))
	for _, e := range reply.Enrichments {
		byID[e.MentionID] = e
	}

	enrichments := make([]models.Enrichment, 0, len(batch))
	var unanswered []models.Mention
	var omitted []string
	var problems []error
	for _, m := range batch {
		got, ok := byID[m.ID]
		if !ok {
			unanswered = append(unanswered, m)
			omitted = append(omitted, m.ID)
			continue
		}
		enrichment := models.Enrichment{
			MentionID:       m.ID,
			Sentiment:       got.Sentiment,
			SentimentLabel:  got.SentimentLabel,
			Emotion:         got.Emotion,
			Intent:          got.Intent,
			Aspects:         got.Aspects,
			IsAboutBrand:    got.IsAboutBrand,
			AboutCompetitor: got.AboutCompetitor,
		}
		if enrichment.Aspects == nil {
			enrichment.Aspects = []string{}
		}
		if err := enrichment.Validate(); err != nil {
			unanswered = append(unanswered, m)
			problems = append(problems, fmt.Errorf("mention %q: %w", m.ID, err))
			continue
		}
		enrichments = append(enrichments, enrichment)
	}

	// One line for all the omitted ids rather than one line each: a model that
	// drops forty of fifty should cost an operator one line to read, and the
	// ids are the point because they are how those mentions get found again.
	if len(omitted) > 0 {
		problems = append(problems, fmt.Errorf("reply omits %d mention(s): %s", len(omitted), list(omitted)))
	}
	return enrichments, unanswered, errors.Join(problems...)
}

// neutralEnrichments is what the mentions the model never answered degrade to.
// Neutral and not-about-brand, so a failed classification cannot manufacture
// sentiment or inflate share of voice; the Errors entry is what says it
// happened, and it names these ids.
func neutralEnrichments(batch []models.Mention) []models.Enrichment {
	enrichments := make([]models.Enrichment, 0, len(batch))
	for _, m := range batch {
		enrichments = append(enrichments, models.Enrichment{
			MentionID:      m.ID,
			Sentiment:      0,
			SentimentLabel: models.SentimentNeutral,
			Emotion:        models.EmotionNeutral,
			Intent:         models.IntentOther,
			Aspects:        []string{},
			IsAboutBrand:   false,
		})
	}
	return enrichments
}
