// The sentiment split across the mentions on screen, as one segmented bar.
//
// Each segment is counted from the label bp-enricher assigned, and the
// denominator is the number of mentions that carry a label rather than the
// number of rows. A mention the enricher never reached is missing from both
// sides of the fraction instead of being counted as neutral.
//
// A segment is sized in proportion and never rounded up to be visible, so the
// bar cannot overstate a slice. The legend beside it carries the exact count,
// which is the figure a reader should be taking away anyway.

import type { SentimentLabel, EnrichedMention } from "@/lib/types";
import { netSentimentPct, sentimentSplit } from "@/lib/derive";
import { count, pct, sentimentColor, signedPct } from "@/lib/format";

const ORDER: SentimentLabel[] = ["positive", "neutral", "mixed", "negative"];

export function SentimentMeter({ mentions }: { mentions: EnrichedMention[] }) {
  const split = sentimentSplit(mentions);
  const net = netSentimentPct(split);

  if (net === null) {
    return (
      <p className="empty">
        <b>No sentiment to plot.</b>
        None of the mentions on this page carries a label from bp-enricher, so there is no
        split to draw. The bar is absent rather than flat: a flat bar would claim the
        conversation is evenly balanced, and nothing measured that.
      </p>
    );
  }

  return (
    <div className="card">
      <div className="meter-head">
        <span className="meter-net" style={{ color: net < 0 ? "var(--negative)" : "var(--positive)" }}>
          {signedPct(net, 0)}
        </span>
        <span className="note">
          net sentiment across {count(split.classified)} classified mentions, positive minus
          negative
        </span>
      </div>

      <div className="meter-track">
        {ORDER.map((label) => {
          const segment = split[label];
          if (segment === 0) return null;
          return (
            <i
              className="meter-seg"
              key={label}
              title={`${label}: ${segment} of ${split.classified}`}
              style={{
                width: `${(segment / split.classified) * 100}%`,
                background: sentimentColor(label),
              }}
            />
          );
        })}
      </div>

      <div className="meter-legend">
        {ORDER.map((label) => (
          <span className="legend-item" key={label}>
            <i className="legend-swatch" style={{ background: sentimentColor(label) }} />
            {label}
            <b className="legend-count">{count(split[label])}</b>
            <span className="legend-share">{pct(split[label] / split.classified)}</span>
          </span>
        ))}
      </div>
    </div>
  );
}
