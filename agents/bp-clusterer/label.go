// Naming a cluster: the one part of this agent that needs a model.
//
// It must not decide cluster membership. The grouping was settled by
// internal/cluster before anything here runs, and a label that disagreed with
// the grouping would still be a label for that grouping.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"brandpulse/internal/llm"
	"brandpulse/internal/models"
	"brandpulse/internal/prompts"
)

// clustererPrompt is parsed once, at process start, so a renamed prompt fails
// before the first window rather than during it.
var clustererPrompt = template.Must(template.New("clusterer").Parse(prompts.Load("clusterer")))

// labelReply is the schema handed to llm.ChatJSON and the shape read back.
type labelReply struct {
	Label   string `json:"label"`
	Summary string `json:"summary"`
}

// examplesInPrompt caps how many mentions are shown to the labeller. The
// cluster can hold hundreds; a label is a two-to-five word noun phrase and
// does not get better for reading all of them, and the prompt is billed.
const examplesInPrompt = 12

// labelCluster asks the model to name one cluster.
//
// A failed call is the caller's problem to report, not this function's to
// paper over: a topic labelled "unknown" would flow into PriorWindowCounts as
// a real label and collide with every other failure next window.
func labelCluster(ctx context.Context, chat chatJSON, members []models.EnrichedMention) (labelReply, llm.Usage, error) {
	prompt, err := buildLabelPrompt(members)
	if err != nil {
		return labelReply{}, llm.Usage{}, err
	}

	raw, usage, err := chat(ctx, prompt, labelReply{}, llm.Opt{})
	if err != nil {
		return labelReply{}, usage, fmt.Errorf("label call: %w", err)
	}

	var reply labelReply
	if err := json.Unmarshal(raw, &reply); err != nil {
		return labelReply{}, usage, fmt.Errorf("decode label reply: %w", err)
	}
	reply.Label = strings.TrimSpace(reply.Label)
	if reply.Label == "" {
		return labelReply{}, usage, fmt.Errorf("label reply has an empty label")
	}
	reply.Summary = strings.TrimSpace(reply.Summary)
	return reply, usage, nil
}

// buildLabelPrompt renders the embedded template over the cluster's most
// engaged mentions, so what the model reads is what a human would have read
// first.
func buildLabelPrompt(members []models.EnrichedMention) (string, error) {
	shown := byEngagement(members)
	if len(shown) > examplesInPrompt {
		shown = shown[:examplesInPrompt]
	}

	var mentions strings.Builder
	for _, m := range shown {
		fmt.Fprintf(&mentions, "- %s\n", strings.ReplaceAll(m.Mention.Text, "\n", " "))
	}

	var rendered strings.Builder
	if err := clustererPrompt.Execute(&rendered, struct{ Mentions string }{mentions.String()}); err != nil {
		return "", fmt.Errorf("render clusterer prompt: %w", err)
	}
	return rendered.String(), nil
}
