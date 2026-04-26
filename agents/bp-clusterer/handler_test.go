// Tests for filtering, clustering, the cluster cap and the Trend trap.
//
// The corpus is internal/cluster's toy corpus lifted into EnrichedMentions:
// fifteen mentions, three topics a reader can separate by eye, three of them
// Hinglish. Labelling is stubbed, so nothing here touches a network.
//
// ids.New is still B1's compile-target stub, so these skip until it lands; see
// requireIDs.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"brandpulse/internal/ids"
	"brandpulse/internal/llm"
	"brandpulse/internal/models"
)

// requireIDs skips while ids.New panics. A skip reports as a skip: nothing
// here claims to pass until B1 Task 2 lands, and then it enforces with no edit.
func requireIDs(t *testing.T) {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Skipf("ids.New is still B1's stub (%v); this suite enforces once B1 Task 2 lands", recovered)
		}
	}()
	ids.New("top")
}

var (
	windowStart = time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	windowEnd   = time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
)

// toyTexts mirrors internal/cluster's toy corpus: delivery complaints (0-4),
// price praise (5-9), packaging damage (10-14).
var toyTexts = []string{
	"delivery delayed again order stuck with courier",
	"order delivery late courier never arrived",
	"delivery bahut late hai courier ne order nahi diya",
	"courier delayed my order delivery three days",
	"late delivery order courier problem again",

	"price is great value for money totally worth",
	"great price worth the money value product",
	"price kaafi accha hai value for money worth",
	"value for money price great worth buying",
	"worth the price great value money saved",

	"packaging damaged bottle leaked box crushed",
	"box arrived damaged packaging leaked bottle",
	"packaging bekaar hai bottle leaked box damaged",
	"damaged packaging bottle broken leaked box",
	"box crushed packaging damaged bottle leaked",
}

func toyMentions() []models.EnrichedMention {
	mentions := make([]models.EnrichedMention, 0, len(toyTexts))
	for i, text := range toyTexts {
		mentions = append(mentions, models.EnrichedMention{
			Mention: models.Mention{
				ID:       fmt.Sprintf("mn_%03d", i),
				BrandID:  "brd_01J",
				Source:   models.SourceX,
				Text:     text,
				PostedAt: windowStart.Add(time.Duration(i) * time.Minute),
				// Engagement rises with index so the top examples of each
				// cluster are predictable.
				Engagement: models.Engagement{Likes: i},
			},
			Enrichment: models.Enrichment{
				MentionID:      fmt.Sprintf("mn_%03d", i),
				SentimentLabel: models.SentimentNegative,
				IsAboutBrand:   true,
			},
		})
	}
	return mentions
}

// stubLabeller names each cluster from the first distinctive word in the
// mentions, so the labels are stable across runs and readable in a failure
// message.
type stubLabeller struct {
	calls int
	fail  bool
}

func (s *stubLabeller) chat(_ context.Context, prompt string, _ any, _ llm.Opt) (json.RawMessage, llm.Usage, error) {
	s.calls++
	if s.fail {
		return nil, llm.Usage{}, fmt.Errorf("upstream 503")
	}
	label := "other chatter"
	for _, candidate := range []string{"delivery", "price", "packaging"} {
		if strings.Contains(mentionsIn(prompt), candidate) {
			label = candidate + " complaints"
			break
		}
	}
	raw, err := json.Marshal(labelReply{Label: label, Summary: "stubbed summary"})
	return raw, llm.Usage{PromptTokens: 200, CompletionTokens: 20, CostPaise: 5}, err
}

// mentionsIn returns only the cluster's own text. Matching against the whole
// prompt matches the template's instructions instead, which name every aspect
// vocabulary word and so labels every cluster identically.
func mentionsIn(prompt string) string {
	_, mentions, found := strings.Cut(prompt, "# Mentions in this cluster")
	if !found {
		panic("the clusterer prompt no longer has a mentions section; this stub reads it")
	}
	return mentions
}

func newHandler(stub *stubLabeller) ClustererHandler {
	return ClustererHandler{chat: stub.chat}
}

func toyInput() models.ClusterInput {
	return models.ClusterInput{
		Enriched:    toyMentions(),
		BrandID:     "brd_01J",
		WindowStart: windowStart,
		WindowEnd:   windowEnd,
	}
}

func handle(t *testing.T, h ClustererHandler, in models.ClusterInput) models.TopicSet {
	t.Helper()
	out, err := h.Handle(context.Background(), in)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	return out
}

func TestHandleFindsTheThreeToyTopics(t *testing.T) {
	requireIDs(t)
	stub := &stubLabeller{}

	out := handle(t, newHandler(stub), toyInput())

	if len(out.Topics) != 3 {
		t.Fatalf("got %d topics, want 3: %v", len(out.Topics), labelsOf(out))
	}
	if stub.calls != 3 {
		t.Errorf("made %d label calls, want 3: one per cluster", stub.calls)
	}
	for _, topic := range out.Topics {
		if topic.Size != 5 {
			t.Errorf("topic %q has size %d, want 5", topic.Label, topic.Size)
		}
		if len(topic.MentionIDs) != 5 {
			t.Errorf("topic %q has %d mention ids, want 5", topic.Label, len(topic.MentionIDs))
		}
		if topic.SentimentMix[models.SentimentNegative] != 5 {
			t.Errorf("topic %q sentiment mix = %v, want 5 negative", topic.Label, topic.SentimentMix)
		}
		if len(topic.TopExamples) != topExamplesPerTopic {
			t.Errorf("topic %q has %d top examples, want %d", topic.Label, len(topic.TopExamples), topExamplesPerTopic)
		}
		if !topic.WindowStart.Equal(windowStart) || !topic.WindowEnd.Equal(windowEnd) {
			t.Errorf("topic %q window = %v..%v, want the input's", topic.Label, topic.WindowStart, topic.WindowEnd)
		}
	}
	if len(out.Unclustered) != 0 {
		t.Errorf("Unclustered = %v, want empty", out.Unclustered)
	}
	if out.TokensUsed != 660 {
		t.Errorf("TokensUsed = %d, want 660", out.TokensUsed)
	}
	if out.CostPaise != 15 {
		t.Errorf("CostPaise = %v, want 15", out.CostPaise)
	}
}

// The zero-value trap: a missing map key is 0 in Go, so the naive division
// gives +Inf and fails at json.Marshal rather than here.
func TestHandleTrendIsOneWithoutAPriorWindow(t *testing.T) {
	requireIDs(t)

	out := handle(t, newHandler(&stubLabeller{}), toyInput())

	for _, topic := range out.Topics {
		if topic.Trend != 1.0 {
			t.Errorf("topic %q has trend %v, want 1.0 when there is no prior window", topic.Label, topic.Trend)
		}
	}
	if _, err := json.Marshal(out); err != nil {
		t.Fatalf("marshal failed, which is how an +Inf trend actually surfaces: %v", err)
	}
}

func TestHandleTrendAgainstThePriorWindow(t *testing.T) {
	requireIDs(t)
	in := toyInput()
	in.PriorWindowCounts = map[string]int{
		"delivery complaints": 10, // shrank: 5/10
		"price complaints":    0,  // present but zero, which is still no baseline
	}

	out := handle(t, newHandler(&stubLabeller{}), in)

	want := map[string]float64{
		"delivery complaints":  0.5,
		"price complaints":     1.0,
		"packaging complaints": 1.0,
	}
	for _, topic := range out.Topics {
		if got := topic.Trend; got != want[topic.Label] {
			t.Errorf("topic %q has trend %v, want %v", topic.Label, got, want[topic.Label])
		}
	}
}

// Off-brand mentions are not this brand's noise, so they are dropped before
// clustering rather than reported as Unclustered.
func TestHandleDropsOffBrandMentionsBeforeClustering(t *testing.T) {
	requireIDs(t)
	in := toyInput()
	for i := range in.Enriched {
		if i >= 10 {
			in.Enriched[i].Enrichment.IsAboutBrand = false
		}
	}

	out := handle(t, newHandler(&stubLabeller{}), in)

	if len(out.Topics) != 2 {
		t.Fatalf("got %d topics, want 2: %v", len(out.Topics), labelsOf(out))
	}
	for _, id := range out.Unclustered {
		if id >= "mn_010" {
			t.Errorf("%s is off-brand and must not appear in Unclustered", id)
		}
	}
}

func TestHandleMinClusterSizeSendsSmallGroupsToUnclustered(t *testing.T) {
	requireIDs(t)
	in := toyInput()
	// Six mentions is two topics of three, below a min size of 4.
	in.Enriched = append(toyMentions()[0:3], toyMentions()[5:8]...)
	in.MinClusterSize = 4

	out := handle(t, newHandler(&stubLabeller{}), in)

	if len(out.Topics) != 0 {
		t.Errorf("got %d topics, want 0: every group is below MinClusterSize", len(out.Topics))
	}
	if len(out.Unclustered) != 6 {
		t.Errorf("Unclustered has %d ids, want 6: nothing is lost, it is just not a topic", len(out.Unclustered))
	}
}

func TestHandleCapsLabelCallsAtEightClusters(t *testing.T) {
	requireIDs(t)
	stub := &stubLabeller{}
	in := toyInput()
	in.MinClusterSize = 1
	// Twelve mutually dissimilar mentions, so clustering yields twelve
	// singletons and the cap has something to bite on.
	in.Enriched = nil
	for i := range 12 {
		id := fmt.Sprintf("mn_x%02d", i)
		in.Enriched = append(in.Enriched, models.EnrichedMention{
			Mention:    models.Mention{ID: id, Text: fmt.Sprintf("alpha%d beta%d gamma%d", i, i, i)},
			Enrichment: models.Enrichment{MentionID: id, IsAboutBrand: true},
		})
	}

	out := handle(t, newHandler(stub), in)

	if stub.calls != maxLabelledClusters {
		t.Errorf("made %d label calls, want %d: the cap is what stops a noisy day fanning out", stub.calls, maxLabelledClusters)
	}
	if len(out.Topics) != maxLabelledClusters {
		t.Errorf("got %d topics, want %d", len(out.Topics), maxLabelledClusters)
	}
	if len(out.Unclustered) != 4 {
		t.Errorf("Unclustered has %d ids, want 4: the clusters past the cap", len(out.Unclustered))
	}
	if len(out.Errors) != 1 {
		t.Errorf("Errors = %v, want one entry naming the cap", out.Errors)
	}
}

// A cluster the model could not name cannot become a Topic: Label is the key
// PriorWindowCounts joins on, so an unnamed topic would poison next window.
func TestHandleUnlabelledClustersFallThroughToUnclustered(t *testing.T) {
	requireIDs(t)
	stub := &stubLabeller{fail: true}

	out := handle(t, newHandler(stub), toyInput())

	if len(out.Topics) != 0 {
		t.Errorf("got %d topics, want 0 when every label call failed", len(out.Topics))
	}
	if len(out.Unclustered) != 15 {
		t.Errorf("Unclustered has %d ids, want 15", len(out.Unclustered))
	}
	if len(out.Errors) != 3 {
		t.Errorf("Errors = %v, want one per failed cluster", out.Errors)
	}
}

// Total() excludes Views on purpose. A viral video must not displace the
// mentions people actually engaged with.
func TestTopExamplesRankByEngagementNotViews(t *testing.T) {
	requireIDs(t)
	in := toyInput()
	in.Enriched[0].Mention.Engagement = models.Engagement{Views: 2_000_000}
	in.Enriched[1].Mention.Engagement = models.Engagement{Likes: 3, Replies: 1}

	out := handle(t, newHandler(&stubLabeller{}), in)

	var delivery models.Topic
	for _, topic := range out.Topics {
		if topic.Label == "delivery complaints" {
			delivery = topic
		}
	}
	if delivery.ID == "" {
		t.Fatalf("no delivery topic in %v", labelsOf(out))
	}
	for _, example := range delivery.TopExamples {
		if example.ID == "mn_000" {
			t.Error("the two-million-view mention with no interactions made the top examples")
		}
	}
	if delivery.TopExamples[0].ID != "mn_004" {
		t.Errorf("top example is %s, want mn_004 (4 likes)", delivery.TopExamples[0].ID)
	}
}

func TestHandleIsDeterministic(t *testing.T) {
	requireIDs(t)
	first := handle(t, newHandler(&stubLabeller{}), toyInput())

	for run := range 5 {
		next := handle(t, newHandler(&stubLabeller{}), toyInput())
		if len(next.Topics) != len(first.Topics) {
			t.Fatalf("run %d returned %d topics, first returned %d", run, len(next.Topics), len(first.Topics))
		}
		for i := range next.Topics {
			if next.Topics[i].Label != first.Topics[i].Label {
				t.Errorf("run %d topic %d is %q, first run had %q", run, i, next.Topics[i].Label, first.Topics[i].Label)
			}
			if strings.Join(next.Topics[i].MentionIDs, ",") != strings.Join(first.Topics[i].MentionIDs, ",") {
				t.Errorf("run %d topic %d has different members", run, i)
			}
		}
	}
}

// Topic ids must be fresh per topic. NewTopic is handed ids.New("top"), and a
// reused id would collide on insert.
func TestTopicIDsAreDistinctAndPrefixed(t *testing.T) {
	requireIDs(t)
	out := handle(t, newHandler(&stubLabeller{}), toyInput())

	seen := map[string]bool{}
	for _, topic := range out.Topics {
		if !strings.HasPrefix(topic.ID, "top_") {
			t.Errorf("topic id %q is not prefixed top_", topic.ID)
		}
		if seen[topic.ID] {
			t.Errorf("topic id %q was reused", topic.ID)
		}
		seen[topic.ID] = true
	}
}

func TestHandleEmptyInput(t *testing.T) {
	requireIDs(t)
	stub := &stubLabeller{}

	out := handle(t, newHandler(stub), models.ClusterInput{BrandID: "brd_01J"})

	if stub.calls != 0 {
		t.Errorf("made %d label calls on empty input, want 0", stub.calls)
	}
	if out.Topics == nil || out.Unclustered == nil || out.Errors == nil {
		t.Error("nil slices marshal as null; the contract's zero value is an empty list")
	}
}

func labelsOf(out models.TopicSet) []string {
	labels := make([]string, 0, len(out.Topics))
	for _, topic := range out.Topics {
		labels = append(labels, topic.Label)
	}
	return labels
}
