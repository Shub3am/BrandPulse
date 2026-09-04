// The mention feed: each row is a Mention joined to its Enrichment.
//
// The sentiment pill is coloured from the label bp-enricher assigned, not from
// a cutoff applied here. A second classifier in the browser could paint a red
// pill beside a "neutral" label, and would be a rule `nasiko observe` cannot
// see.
//
// Every field on a row is optional at runtime even where the mirrored type says
// otherwise: `mentions.engagement` is JSONB DEFAULT '{}', an author can be
// absent on a review, and `aspects` arrives as null when the enricher found
// none. An absent count is not zero, so the part is dropped rather than printed
// as a figure nobody measured.

import type { CSSProperties } from "react";
import type { Engagement, EnrichedMention, Enrichment, Mention } from "@/lib/types";
import { clockTime, count, humanLabel, sentimentColor } from "@/lib/format";
import { listOf, numberOf, textOf } from "@/lib/wire";

function EngagementCounts({ engagement }: { engagement: Engagement | null | undefined }) {
  const parts: [label: string, value: number][] = [];
  for (const [label, key] of [
    ["likes", "likes"],
    ["replies", "replies"],
    ["shares", "shares"],
    ["views", "views"],
  ] as const) {
    const value = numberOf(engagement?.[key]);
    if (value !== null) parts.push([label, value]);
  }

  if (parts.length === 0) return null;

  return (
    <>
      {parts.map(([label, value]) => (
        <span key={label}>
          <b>{count(value)}</b> {label}
        </span>
      ))}
    </>
  );
}

function SentimentPill({ enrichment }: { enrichment: Enrichment | null | undefined }) {
  const label = enrichment?.sentiment_label;
  if (!label) return null;
  const score = numberOf(enrichment?.sentiment);
  return (
    <span className="sentiment-pill" style={{ "--tone": sentimentColor(label) } as CSSProperties}>
      {label}
      {score !== null && <span className="sentiment-score">{score.toFixed(2)}</span>}
    </span>
  );
}

/** Aspects are chips, one per aspect. A joined string is a list pretending to
    be a sentence, and joining a null one is how this page last crashed. */
function AspectChips({ aspects }: { aspects: string[] | null | undefined }) {
  const named = listOf(aspects)
    .map((aspect) => textOf(aspect))
    .filter((aspect): aspect is string => aspect !== null);

  if (named.length === 0) return null;

  return (
    <>
      <span className="chips-label">aspects</span>
      {named.map((aspect) => (
        <span className="chip chip-aspect" key={aspect}>
          {aspect}
        </span>
      ))}
    </>
  );
}

function MentionRow({ mention, enrichment }: { mention: Mention; enrichment: Enrichment | null }) {
  const author = textOf(mention.author);
  const followers = numberOf(mention.author_followers);
  const rating = numberOf(mention.rating);
  const posted = clockTime(mention.posted_at);
  const intent = humanLabel(enrichment?.intent);
  const emotion = humanLabel(enrichment?.emotion);
  const matched = textOf(mention.matched_keyword);
  const url = textOf(mention.url);

  return (
    <article className="mention">
      <div className="mention-head">
        <span className="source-badge">
          <i />
          {textOf(mention.source) ?? "source not recorded"}
        </span>
        <SentimentPill enrichment={enrichment} />
        {author && <span className="mention-author">{author}</span>}
        {followers !== null && followers > 0 && (
          <span className="chip">{count(followers)} followers</span>
        )}
        {rating !== null && rating > 0 && rating <= 5 && (
          <span className="chip" style={{ color: "var(--warn)" }}>
            {"★".repeat(Math.round(rating))}
          </span>
        )}
        {posted && <span className="push-right">{posted} IST</span>}
      </div>

      <p className="mention-text">{textOf(mention.text) ?? "This mention arrived with no text."}</p>

      <div className="chips">
        {intent && <span className="chip">{intent}</span>}
        {emotion && <span className="chip">{emotion}</span>}
        <AspectChips aspects={enrichment?.aspects} />
      </div>

      <div className="mention-foot">
        {/* Component counts, not a total: "which engagement counts" is a
            clusterer rule and lives in models.Engagement.Total. */}
        <EngagementCounts engagement={mention.engagement} />
        {matched && <span>matched on &ldquo;{matched}&rdquo;</span>}
        {url && (
          <a className="link push-right" href={url} target="_blank" rel="noreferrer">
            open source
          </a>
        )}
      </div>
    </article>
  );
}

export function MentionStream({ items }: { items: EnrichedMention[] }) {
  const rows = listOf(items).filter((item) => item.mention);

  return (
    <div className="card card-flush">
      {rows.map((item, index) => (
        <MentionRow
          key={textOf(item.mention.id) ?? `mention-${index}`}
          mention={item.mention}
          enrichment={item.enrichment ?? null}
        />
      ))}
    </div>
  );
}
