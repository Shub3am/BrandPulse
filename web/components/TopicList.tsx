// Clusters from bp-clusterer, with the sentiment split rendered as one bar.

import type { SentimentLabel, Topic } from "@/lib/types";
import { SENTIMENT_COLOR } from "@/lib/format";

const MIX_ORDER: SentimentLabel[] = ["negative", "neutral", "positive", "mixed"];

export function TopicList({ topics }: { topics: Topic[] }) {
  return (
    <div className="card">
      {topics.map((topic) => (
        <section className="topic" key={topic.id}>
          <div className="topic-head">
            <b>{topic.label}</b>
            <span className={`pill ${topic.trend > 1.5 ? "pill-negative" : ""}`}>
              {topic.trend > 1 ? "↑" : "↓"} {topic.trend.toFixed(1)}x
            </span>
            <span className="pill" style={{ marginLeft: "auto" }}>{topic.size}</span>
          </div>
          <p>{topic.summary}</p>
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
        </section>
      ))}
    </div>
  );
}
