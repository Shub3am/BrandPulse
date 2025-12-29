// Clusters from bp-clusterer, with the sentiment split rendered as one bar.
//
// Topic.trend uses 1.0 for both "flat" and "no prior window to compare", so the
// arrow is three-way. A two-way arrow would render "we have no comparison" as
// "declining", which is the kind of invented fact this product exists to avoid.

import type { SentimentLabel, Topic } from "@/lib/types";
import { SENTIMENT_COLOR } from "@/lib/format";

const MIX_ORDER: SentimentLabel[] = ["negative", "neutral", "positive", "mixed"];

function trendArrow(trend: number): string {
  if (trend > 1) return "↑";
  if (trend < 1) return "↓";
  return "—";
}

export function TopicList({ topics }: { topics: Topic[] }) {
  return (
    <div className="card">
      {topics.map((topic) => (
        <section className="topic" key={topic.id}>
          <div className="topic-head">
            <b>{topic.label}</b>
            <span className="pill">
              {trendArrow(topic.trend)} {topic.trend.toFixed(1)}x
            </span>
            <span className="pill push-right">{topic.size}</span>
          </div>
          <p>{topic.summary}</p>
          {/* size is the denominator, so a zero leaves the bar off entirely.
              Dividing by it printed `width: NaN%`, and summing sentiment_mix to
              get a denominator instead would be this component deciding what the
              cluster's size is, which is bp-clusterer's number. */}
          {topic.size > 0 && (
            <div className="mix">
              {MIX_ORDER.map((label) => {
                const count = topic.sentiment_mix[label] ?? 0;
                if (count === 0) return null;
                return (
                  <i
                    key={label}
                    title={`${label}: ${count}`}
                    style={{ width: `${(count / topic.size) * 100}%`, background: SENTIMENT_COLOR[label] }}
                  />
                );
              })}
            </div>
          )}
        </section>
      ))}
    </div>
  );
}
