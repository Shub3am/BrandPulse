// The live mention feed: each row is a Mention joined to its Enrichment.
//
// The bar is coloured from the label bp-enricher assigned, not from a cutoff
// applied here. A second classifier in the browser could paint a red bar beside
// a "neutral" pill, and would be a rule `nasiko observe` cannot see.

import type { EnrichedMention, SentimentLabel } from "@/lib/types";
import { clockTime, humanLabel, SENTIMENT_COLOR } from "@/lib/format";

function SentimentBar({ score, label }: { score: number; label: SentimentLabel }) {
  // Centre is neutral, so the bar grows left for negative and right for positive.
  const width = Math.abs(score) * 50;
  return (
    <span className="sentiment-bar" title={`${label} ${score.toFixed(2)}`}>
      <i
        style={{
          background: SENTIMENT_COLOR[label],
          width: `${width}%`,
          left: score < 0 ? `${50 - width}%` : "50%",
        }}
      />
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
            <span className="pill" style={{ color: SENTIMENT_COLOR[enrichment.sentiment_label] }}>
              {enrichment.sentiment_label}
            </span>
            <SentimentBar score={enrichment.sentiment} label={enrichment.sentiment_label} />
            <span>{mention.author}</span>
            {mention.author_followers > 0 && (
              <span className="pill pill-accent">
                {(mention.author_followers / 1000).toFixed(0)}k followers
              </span>
            )}
            {mention.rating !== undefined && <span>{"★".repeat(mention.rating)}</span>}
            <span className="push-right">{clockTime(mention.posted_at)}</span>
          </div>

          <p className="mention-text">{mention.text}</p>

          <div className="mention-foot">
            <span>{humanLabel(enrichment.intent)}</span>
            <span>{enrichment.aspects.join(", ")}</span>
            {/* Component counts, not a total: "which engagement counts" is a
                clusterer rule and lives in models.Engagement.Total. */}
            <span>
              {mention.engagement.likes} likes &middot; {mention.engagement.replies} replies
            </span>
          </div>
        </article>
      ))}
    </div>
  );
}
