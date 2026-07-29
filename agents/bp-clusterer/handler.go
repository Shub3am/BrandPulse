// ClustererHandler turns one window of enriched mentions into labelled topics.
//
// The grouping is free and deterministic: internal/cluster does it locally
// with TF-IDF and no network. Only the naming costs tokens, one call per
// surviving cluster, and that is the whole reason the cluster cap exists.
//
// It must not call llm.Embed. It exists, and at 400 mentions TF-IDF is enough,
// costs nothing and runs in CI. research/nasiko.md §6 Finding B is that no
// embeddings endpoint on the Nasiko router has been verified at all, so
// swapping to one would be trading a measured path for an unmeasured one.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"brandpulse/internal/cluster"
	"brandpulse/internal/ids"
	"brandpulse/internal/llm"
	"brandpulse/internal/models"
)

const (
	// distanceCutoff is the average-linkage cosine distance at which two
	// clusters stop merging. Measured on internal/cluster's toy corpus, where
	// every value from 0.80 to 0.98 gives the same answer: within-topic
	// distances run 0.44 to 0.76 and cross-topic distances are 1.0. 0.85 sits
	// mid-band. Note the range is not [0, 1]: see internal/cluster/CLAUDE.md.
	distanceCutoff = 0.85

	// maxLabelledClusters caps LLM calls at one per cluster per window, so a
	// noisy day cannot fan out into forty. Clusters past the cap become
	// Unclustered with a note in Errors.
	maxLabelledClusters = 8

	// defaultMinClusterSize is what ClusterInput.MinClusterSize == 0 means,
	// per CONTRACTS §2.
	defaultMinClusterSize = 3

	// topExamplesPerTopic is fixed by models.Topic's contract.
	topExamplesPerTopic = 3
)

// chatJSON is llm.ChatJSON's signature, held as a field so tests can stub it.
type chatJSON func(ctx context.Context, prompt string, schema any, opt llm.Opt) (json.RawMessage, llm.Usage, error)

type ClustererHandler struct {
	chat chatJSON
}

func NewClustererHandler() ClustererHandler {
	return ClustererHandler{chat: llm.ChatJSON}
}

// Handle clusters in.Enriched and names each surviving cluster.
//
// Mentions the enricher judged not about this brand are dropped before
// clustering rather than sent to Unclustered: they are not noise this brand
// failed to group, they are somebody else's conversation.
//
// A window that yields no topics always says why in Errors. Zero topics is a
// legitimate answer here, so it is not an error return, but an empty TopicSet
// and an empty Errors are indistinguishable from a crash to everything
// downstream, and the three ways to reach zero have three different fixes:
// nothing collected, nothing on-brand, or nothing similar enough to group.
func (h ClustererHandler) Handle(ctx context.Context, in models.ClusterInput) (models.TopicSet, error) {
	out := models.TopicSet{Topics: []models.Topic{}, Unclustered: []string{}, Errors: []string{}}

	if len(in.Enriched) == 0 {
		out.Errors = append(out.Errors, "no topics: the window carried no enriched mentions")
		return out, nil
	}

	onBrand := onBrandMentions(in.Enriched)
	if len(onBrand) == 0 {
		out.Errors = append(out.Errors, fmt.Sprintf(
			"no topics: the enricher judged all %d mentions in this window not about this brand, so nothing reached clustering; check the brand's keyword set",
			len(in.Enriched)))
		return out, nil
	}

	texts := make([]string, 0, len(onBrand))
	for _, m := range onBrand {
		texts = append(texts, m.Mention.Text)
	}
	minSize := minClusterSize(in.MinClusterSize)
	groups := cluster.Agglomerative(cluster.TFIDF(texts), distanceCutoff, minSize)
	if len(groups) == 0 {
		out.Errors = append(out.Errors, fmt.Sprintf(
			"no topics: %d of %d mentions were about this brand and none of them formed a group of at least %d at cosine distance %.2f",
			len(onBrand), len(in.Enriched), minSize, distanceCutoff))
	}

	// Order is cluster.Agglomerative's: size descending with a defined
	// tie-break. Re-sorting here would replace a deterministic order with a
	// map walk.
	labelled := groups
	if len(labelled) > maxLabelledClusters {
		labelled = groups[:maxLabelledClusters]
		out.Errors = append(out.Errors, fmt.Sprintf(
			"labelled %d of %d clusters; the remaining %d went to unclustered at the %d-cluster cap",
			maxLabelledClusters, len(groups), len(groups)-maxLabelledClusters, maxLabelledClusters))
	}

	// Indices are dense over onBrand, so a slice says that and a map does not.
	grouped := make([]bool, len(onBrand))
	for _, group := range labelled {
		// Ranked once per cluster and used twice: the prompt shows the top 12
		// and the Topic keeps the top 3, and both want the same order.
		members := membersOf(onBrand, group)
		ranked := byEngagement(members)
		reply, usage, err := labelCluster(ctx, h.chat, ranked)
		out.TokensUsed += usage.PromptTokens + usage.CompletionTokens
		out.CostPaise += usage.CostPaise
		if err != nil {
			// An unnamed cluster cannot become a Topic, because Label is the
			// key PriorWindowCounts is joined on. Its mentions fall through to
			// Unclustered with the rest.
			out.Errors = append(out.Errors, err.Error())
			continue
		}

		topic, err := buildTopic(in, reply, members, ranked)
		if err != nil {
			out.Errors = append(out.Errors, err.Error())
			continue
		}
		out.Topics = append(out.Topics, topic)
		for _, index := range group {
			grouped[index] = true
		}
	}

	for index, m := range onBrand {
		if !grouped[index] {
			out.Unclustered = append(out.Unclustered, m.Mention.ID)
		}
	}
	return out, nil
}

// buildTopic assembles one Topic from a named cluster. members is in window
// order and fixes MentionIDs; ranked is the same mentions most-engaged first
// and fixes TopExamples. Built through NewTopic and validated, because a Topic
// literal gives Trend 0 and a nil SentimentMix that panics on the first write.
func buildTopic(in models.ClusterInput, reply labelReply, members, ranked []models.EnrichedMention) (models.Topic, error) {
	topic := models.NewTopic(ids.New("top"), in.BrandID)
	topic.WindowStart = in.WindowStart
	topic.WindowEnd = in.WindowEnd
	topic.Label = reply.Label
	topic.Summary = reply.Summary
	topic.Size = len(members)
	topic.Trend = trendAgainst(in.PriorWindowCounts, reply.Label, len(members))

	for _, m := range members {
		topic.MentionIDs = append(topic.MentionIDs, m.Mention.ID)
		topic.SentimentMix[m.Enrichment.SentimentLabel]++
	}

	examples := ranked
	if len(examples) > topExamplesPerTopic {
		examples = examples[:topExamplesPerTopic]
	}
	for _, m := range examples {
		topic.TopExamples = append(topic.TopExamples, m.Mention)
	}

	if err := topic.Validate(); err != nil {
		return models.Topic{}, fmt.Errorf("topic %q: %w", reply.Label, err)
	}
	return topic, nil
}

// trendAgainst is this window's size over the prior window's, and 1.0 when
// there is no prior window.
//
// The comma-ok form is load-bearing: a missing map key reads as 0 in Go, so
// the naive division yields +Inf, which does not fail here. It fails at
// json.Marshal as "unsupported value", after the run has already cost money.
func trendAgainst(priorCounts map[string]int, label string, size int) float64 {
	if prior, ok := priorCounts[label]; ok && prior > 0 {
		return float64(size) / float64(prior)
	}
	return 1.0
}

// onBrandMentions drops what the enricher judged irrelevant, keeping input
// order so the indices cluster.Agglomerative returns stay meaningful.
func onBrandMentions(enriched []models.EnrichedMention) []models.EnrichedMention {
	onBrand := make([]models.EnrichedMention, 0, len(enriched))
	for _, m := range enriched {
		if m.Enrichment.IsAboutBrand {
			onBrand = append(onBrand, m)
		}
	}
	return onBrand
}

func membersOf(onBrand []models.EnrichedMention, group []int) []models.EnrichedMention {
	members := make([]models.EnrichedMention, 0, len(group))
	for _, index := range group {
		members = append(members, onBrand[index])
	}
	return members
}

// byEngagement ranks a cluster's mentions for display. Engagement.Total()
// rather than a hand-rolled sum: Total() excludes Views on purpose, and a
// two-million-view video would otherwise outrank every real complaint.
//
// Ties break on PostedAt and then on ID, so the choice is the same on every
// run over the same input.
func byEngagement(members []models.EnrichedMention) []models.EnrichedMention {
	ranked := make([]models.EnrichedMention, len(members))
	copy(ranked, members)
	sort.SliceStable(ranked, func(i, j int) bool {
		left, right := ranked[i].Mention, ranked[j].Mention
		leftTotal, rightTotal := left.Engagement.Total(), right.Engagement.Total()
		if leftTotal != rightTotal {
			return leftTotal > rightTotal
		}
		if !left.PostedAt.Equal(right.PostedAt) {
			return left.PostedAt.After(right.PostedAt)
		}
		return left.ID < right.ID
	})
	return ranked
}

func minClusterSize(requested int) int {
	if requested <= 0 {
		return defaultMinClusterSize
	}
	return requested
}
