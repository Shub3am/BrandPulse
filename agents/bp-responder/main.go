// bp-responder drafts one reply for one alert or one mention.
//
// It never posts. There is no posting code path in this repo, not behind a
// flag, and none is to be added: every draft leaves here as a ReplyDraft whose
// wire form says requires_human_approval: true, and a human sends it or does
// not.
//
// The prompt lives in internal/prompts/responder.md, not in this file. The
// guardrail check lives in guardrails.go so it is testable without a model.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"text/template"

	"brandpulse/internal/a2a"
	"brandpulse/internal/ids"
	"brandpulse/internal/llm"
	"brandpulse/internal/models"
	"brandpulse/internal/obs"
	"brandpulse/internal/prompts"
	"brandpulse/internal/redact"
)

const (
	// responderModel is what the cost table is keyed against. The Nasiko router
	// discards the model field in the request body and per-agent choice is set
	// with `nasiko llm-config`, so this is a label, not a routing instruction.
	responderModel = "bp-responder-default"

	// responderMaxTokens is generous for two or three sentences. A reply that
	// runs long is a reply a founder rewrites anyway.
	responderMaxTokens = 700

	// strippedToneNote is appended to Tone when a phrase had to be cut, so the
	// human editing the draft knows a sentence may read oddly and why.
	strippedToneNote = "guardrail phrase removed, re-read before sending"
)

// The three reply shapes. Which one applies is decided from the input, because
// RespondInput carries no Enrichment and therefore no models.Intent.
const (
	intentHoldingStatement  = "holding_statement"
	intentFactualCorrection = "factual_correction"
	intentAcknowledge       = "acknowledge_and_route"
)

// ResponderHandler holds the model call as a dependency rather than calling
// llm.ChatJSON directly, so a test can hand it a canned response instead of a
// live router.
type ResponderHandler struct {
	ChatJSON llm.ChatJSONFunc

	prompt *template.Template
}

// NewResponderHandler parses the embedded prompt once. A malformed prompt is a
// startup failure, not a per-request one.
func NewResponderHandler(chat llm.ChatJSONFunc) ResponderHandler {
	return ResponderHandler{
		ChatJSON: chat,
		prompt:   template.Must(template.New("responder").Parse(prompts.Load("responder"))),
	}
}

// draftResponse is the JSON shape the model is asked for.
type draftResponse struct {
	Text string `json:"text"`
	Tone string `json:"tone"`
}

// promptData is what responder.md is rendered against.
type promptData struct {
	Brand      string
	Tone       string
	Language   string
	Signature  string
	Channel    string
	Intent     string
	Subject    string
	Context    string
	DoNotSay   []string
	Guardrails []string
	Retry      bool
	Violations []string
}

// Handle drafts one reply.
//
// The error return is for malformed input only, which here means the
// Alert/Mention pair. Everything else that can go wrong, including the model
// failing, is a real error too: unlike a batch agent there is no partial draft
// worth returning, and half a reply to a crisis is worse than none.
func (h ResponderHandler) Handle(ctx context.Context, in models.RespondInput) (models.ReplyDraft, error) {
	if err := validateSubject(in); err != nil {
		return models.ReplyDraft{}, err
	}

	banned := bannedPhrases(in.Profile)
	data := h.buildPromptData(in, banned)

	draft, violations, err := h.draftWithinGuardrails(ctx, data, banned)
	if err != nil {
		return models.ReplyDraft{}, err
	}

	return h.buildReplyDraft(in, draft, violations, banned)
}

// validateSubject enforces the one input rule: exactly one of Alert and
// Mention. Both nil means there is nothing to reply to; both set means the
// caller has not decided what it is replying to, and guessing would produce a
// draft attached to the wrong row.
func validateSubject(in models.RespondInput) error {
	switch {
	case in.Alert == nil && in.Mention == nil:
		return fmt.Errorf("bp-responder: RespondInput has neither an alert nor a mention, there is nothing to reply to")
	case in.Alert != nil && in.Mention != nil:
		return fmt.Errorf("bp-responder: RespondInput has both an alert and a mention, exactly one is allowed")
	}
	return nil
}

// draftWithinGuardrails is the three-state contract: clean, regenerated clean,
// or stripped. It regenerates exactly once. A second failure is not a third
// attempt, because a model that has been told twice and complied neither time
// is not going to comply on the third, and each attempt costs tokens.
func (h ResponderHandler) draftWithinGuardrails(ctx context.Context, data promptData, banned []string) (draftResponse, []string, error) {
	draft, err := h.ask(ctx, data)
	if err != nil {
		return draftResponse{}, nil, err
	}

	violations := violationsIn(draft.Text, banned)
	if len(violations) == 0 {
		return draft, nil, nil
	}

	data.Retry = true
	data.Violations = violations
	retried, err := h.ask(ctx, data)
	if err != nil {
		return draftResponse{}, nil, fmt.Errorf("bp-responder: regenerating after %q: %w", violations[0], err)
	}

	stillViolating := violationsIn(retried.Text, banned)
	if len(stillViolating) == 0 {
		return retried, nil, nil
	}

	retried.Text = strip(retried.Text, stillViolating)
	return retried, stillViolating, nil
}

// ask renders the prompt and makes one model call.
func (h ResponderHandler) ask(ctx context.Context, data promptData) (draftResponse, error) {
	var rendered strings.Builder
	if err := h.prompt.Execute(&rendered, data); err != nil {
		return draftResponse{}, fmt.Errorf("bp-responder: rendering the prompt: %w", err)
	}

	body, _, err := h.ChatJSON(ctx, rendered.String(), draftResponse{}, llm.Opt{
		Model:     responderModel,
		MaxTokens: responderMaxTokens,
	})
	if err != nil {
		return draftResponse{}, fmt.Errorf("bp-responder: drafting: %w", err)
	}

	var draft draftResponse
	if err := json.Unmarshal(body, &draft); err != nil {
		return draftResponse{}, fmt.Errorf("bp-responder: the model did not return the draft shape: %w", err)
	}
	if strings.TrimSpace(draft.Text) == "" {
		return draftResponse{}, fmt.Errorf("bp-responder: the model returned an empty draft")
	}
	return draft, nil
}

// buildReplyDraft assembles the wire type. It goes through the constructor so
// Status and DoNotSay are not zero values, and through Validate so a draft
// referencing neither an alert nor a mention never reaches the caller.
func (h ResponderHandler) buildReplyDraft(in models.RespondInput, drafted draftResponse, stripped []string, banned []string) (models.ReplyDraft, error) {
	draft := models.NewReplyDraft(ids.New("drf"), in.Channel)
	draft.Text = strings.TrimSpace(drafted.Text)
	draft.Tone = drafted.Tone
	// DoNotSay is the audit trail: what was forbidden when this text was
	// written, not what the model happened to avoid.
	draft.DoNotSay = banned

	if len(stripped) > 0 {
		draft.Tone = strings.TrimSpace(draft.Tone + "; " + strippedToneNote + ": " + strings.Join(stripped, ", "))
	}

	if in.Alert != nil {
		draft.AlertID = in.Alert.ID
	} else {
		draft.MentionID = in.Mention.ID
	}

	if err := draft.Validate(); err != nil {
		return models.ReplyDraft{}, fmt.Errorf("bp-responder: %w", err)
	}
	return draft, nil
}

// bannedPhrases is the global guardrail list plus whatever this brand has
// added. Both are checked; the prompt carries both.
func bannedPhrases(profile models.BrandProfile) []string {
	banned := make([]string, 0, len(prompts.Guardrails)+len(profile.Voice.DoNotSay))
	banned = append(banned, prompts.Guardrails...)
	banned = append(banned, profile.Voice.DoNotSay...)
	return banned
}

func (h ResponderHandler) buildPromptData(in models.RespondInput, banned []string) promptData {
	subject, situation := subjectAndContext(in)
	return promptData{
		Brand:      in.Profile.Name,
		Tone:       in.Profile.Voice.Tone,
		Language:   in.Profile.Voice.Language,
		Signature:  in.Profile.Voice.Signature,
		Channel:    string(in.Channel),
		Intent:     replyIntentFor(in),
		Subject:    subject,
		Context:    situation,
		DoNotSay:   in.Profile.Voice.DoNotSay,
		Guardrails: prompts.Guardrails,
	}
}

// replyIntentFor picks the template. RespondInput carries no Enrichment, so
// intent is read from the alert kind: a crisis or a review bomb is unresolved
// and gets a holding statement, a competitor move gets a correction, and
// anything about one customer gets an acknowledgement and a route to support.
func replyIntentFor(in models.RespondInput) string {
	if in.Alert == nil {
		return intentAcknowledge
	}
	switch in.Alert.Kind {
	case models.AlertCrisis, models.AlertReviewBomb:
		return intentHoldingStatement
	case models.AlertCompetitorMove:
		return intentFactualCorrection
	default:
		return intentAcknowledge
	}
}

// subjectAndContext builds what the model reads. Every mention text passes
// through redact.PII first: a phone number in a complaint is not something to
// hand to a router, and it is certainly not something to echo back in a public
// reply.
func subjectAndContext(in models.RespondInput) (subject, situation string) {
	if in.Mention != nil {
		return fmt.Sprintf("A %s mention: %s", in.Mention.Source, redact.PII(in.Mention.Text)), ""
	}

	alert := in.Alert
	var b strings.Builder
	fmt.Fprintf(&b, "%s. %s", alert.Title, alert.Why)
	for _, e := range alert.Evidence {
		fmt.Fprintf(&b, "\n- %s: %v (fires at %v, over %s)", e.Metric, e.Value, e.Threshold, e.Window)
	}

	var samples strings.Builder
	samples.WriteString("What people are actually saying:\n")
	for _, m := range alert.SampleMentions {
		fmt.Fprintf(&samples, "- (%s) %s\n", m.Source, redact.PII(m.Text))
	}
	return strings.TrimSpace(samples.String()), b.String()
}

func main() {
	shutdown, err := obs.Setup("bp-responder")
	if err != nil {
		log.Fatalf("bp-responder: observability setup failed: %v", err)
	}
	defer func() { _ = shutdown(context.Background()) }()

	handler := NewResponderHandler(llm.ChatJSON)
	if err := a2a.Serve(a2a.Card{Name: "bp-responder"}, handler); err != nil {
		log.Fatalf("bp-responder: %v", err)
	}
}
