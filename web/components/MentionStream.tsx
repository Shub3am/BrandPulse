// The live mention feed: each row is a Mention joined to its Enrichment.

import type { EnrichedMention } from "@/lib/types";
import { clockTime, SENTIMENT_COLOR } from "@/lib/format";

function SentimentBar({ score }: { score: number }) {
  // Centre is neutral, so the bar grows left for negative and right for positive.
  const width = Math.abs(score) * 50;
  const color = score < -0.15 ? "var(--negative)" : score > 0.15 ? "var(--positive)" : "var(--text-faint)";
  return (
    <span className="sentiment-bar" title={`sentiment ${score.toFixed(2)}`}>
      <i style={{ background: color, width: `${width}%`, left: score < 0 ? `${50 - width}%` : "50%" }} />
    </span>
  );
}

export function MentionStream({ items }: { items: EnrichedMention[] }) {
  return (
    <div className="card">
      {items.map(({ mention, enrichment }) => (
        <article className="mention" key={mention.id}>
          <div className="mention-head">
            <span className="pill">{mention.source}</span>
            <span
              className="pill"
              style={{ color: SENTIMENT_COLOR[enrichment.sentiment_label] }}
            >
              {enrichment.sentiment_label}
            </span>
            <SentimentBar score={enrichment.sentiment} />
            <span>{mention.author}</span>
            {mention.author_followers > 0 && (
              <span className="pill pill-accent">
                {(mention.author_followers / 1000).toFixed(0)}k followers
              </span>
            )}
            {mention.rating !== undefined && <span>{"★".repeat(mention.rating)}</span>}
            <span style={{ marginLeft: "auto" }}>{clockTime(mention.posted_at)}</span>
          </div>

          <p className="mention-text">{mention.text}</p>

          <div className="mention-foot">
            <span>{enrichment.intent.replace("_", " ")}</span>
            <span>{enrichment.aspects.join(", ")}</span>
            <span>{mention.engagement.likes + mention.engagement.replies + mention.engagement.shares} interactions</span>
          </div>
        </article>
      ))}
    </div>
  );
}
