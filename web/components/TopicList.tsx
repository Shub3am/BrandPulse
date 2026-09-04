// Clusters from bp-clusterer as ranked cards, with the sentiment split of each
// one rendered as a bar.
//
// The rank is the position the BFF returned, printed rather than decided: this
// component does not sort. Reordering clusters by size would be a ranking rule
// running in a browser, where `nasiko observe` cannot see it.
//
// Topic.trend uses 1.0 for both "flat" and "no prior window to compare", so the
// arrow is three-way. A two-way arrow would render "we have no comparison" as
// "declining", which is the kind of invented fact this product exists to avoid.

import type { SentimentLabel, Topic } from "@/lib/types";
import { count, sentimentColor } from "@/lib/format";
import { listOf, mapOf, numberOf, textOf } from "@/lib/wire";

const MIX_ORDER: SentimentLabel[] = ["positive", "neutral", "mixed", "negative"];

function trendArrow(trend: number): string {
  if (trend > 1) return "↑";
  if (trend < 1) return "↓";
  return "→";
}

function trendColor(trend: number): string {
  if (trend > 1) return "var(--warn)";
  if (trend < 1) return "var(--text-muted)";
  return "var(--text-faint)";
}

function TopicMix({ topic }: { topic: Topic }) {
  const size = numberOf(topic.size);
  const mix = mapOf<number>(topic.sentiment_mix);

  // size is the denominator, so an absent or zero one leaves the bar off
  // entirely. Dividing by it printed `width: NaN%`, and summing sentiment_mix
  // to get a denominator instead would be this component deciding what the
  // cluster's size is, which is bp-clusterer's number.
  if (size === null || size <= 0) {
    return <span className="mix-note">No size on this cluster, so its split is not drawn.</span>;
  }

  const segments = MIX_ORDER.map((label) => ({ label, value: numberOf(mix[label]) ?? 0 })).filter(
    (segment) => segment.value > 0,
  );

  if (segments.length === 0) {
    return <span className="mix-note">bp-clusterer recorded no sentiment split for this cluster.</span>;
  }

  return (
    <div className="mix">
      {segments.map((segment) => (
        <i
          key={segment.label}
          title={`${segment.label}: ${segment.value} of ${size}`}
          style={{
            width: `${Math.min((segment.value / size) * 100, 100)}%`,
            background: sentimentColor(segment.label),
          }}
        />
      ))}
    </div>
  );
}

function TopicCard({ topic, rank }: { topic: Topic; rank: number }) {
  const size = numberOf(topic.size);
  const trend = numberOf(topic.trend);
  const summary = textOf(topic.summary);

  return (
    <article className="topic-card">
      <div className="topic-top">
        <span className="topic-rank">{String(rank).padStart(2, "0")}</span>
        <span className="topic-label" title={textOf(topic.label) ?? undefined}>
          {textOf(topic.label) ?? "Unlabelled cluster"}
        </span>
      </div>

      <div className="topic-metrics">
        <span className="topic-metric">
          <b>{size === null ? <span className="absent">no size</span> : count(size)}</b>
          <span>mentions</span>
        </span>
        <span className="topic-metric">
          <b style={trend === null ? undefined : { color: trendColor(trend) }}>
            {trend === null ? (
              <span className="absent">no trend</span>
            ) : (
              `${trendArrow(trend)} ${trend.toFixed(1)}x`
            )}
          </b>
          <span>vs prior window</span>
        </span>
      </div>

      <TopicMix topic={topic} />

      <p className="topic-summary">
        {summary ?? "bp-clusterer wrote no summary for this cluster."}
      </p>
    </article>
  );
}

export function TopicList({ topics }: { topics: Topic[] }) {
  return (
    <div className="topic-grid">
      {listOf(topics).map((topic, index) => (
        <TopicCard key={textOf(topic.id) ?? `topic-${index}`} topic={topic} rank={index + 1} />
      ))}
    </div>
  );
}
