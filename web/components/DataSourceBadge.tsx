// Says what the data on this page actually is, from the payload rather than from
// a flag someone has to remember to flip.
//
// Two facts decide it: which backend answered the fetch, and whether any mention
// being rendered carries `raw.synthetic === true`. demo/inject_crisis marks every
// mention it injects that way, so a live run holding an injected crisis is still
// labelled. Repeating what the producer said about its own row is not a business
// rule: this file classifies nothing and decides nothing.
//
// It must never claim the brand is fictional. That is a different claim from "a
// synthetic mention was injected into a real run", nothing on the wire carries
// it, and inferring it from a `.example` website would be exactly the invented
// fact this product exists to avoid. So the two never share a sentence, and only
// the derivable one is ever printed.

import type { AnsweredBy } from "@/lib/pulse";
import type { EnrichedMention } from "@/lib/types";

interface DataSourceProps {
  answeredBy: AnsweredBy;
  /** The mentions actually on screen, not everything stored for the brand. */
  mentions: EnrichedMention[];
}

/** `raw` is the collector's untouched payload, so this reads the producer's own
    marker. A missing `raw` is not synthetic: it is a row nobody marked. */
function countSynthetic(mentions: EnrichedMention[]): number {
  return mentions.filter(({ mention }) => mention.raw?.synthetic === true).length;
}

export function DataSourceBadge({ answeredBy, mentions }: DataSourceProps) {
  const synthetic = countSynthetic(mentions);

  if (synthetic === 0) {
    return (
      <span className="pill pill-positive">
        <i className="dot" />
        live via {answeredBy}
      </span>
    );
  }

  if (synthetic === mentions.length) {
    return (
      <span className="pill" style={{ color: "var(--warn)" }}>
        <i className="dot" />
        synthetic data
      </span>
    );
  }

  return (
    <span className="pill" style={{ color: "var(--warn)" }}>
      <i className="dot" />
      {synthetic} injected mention{synthetic === 1 ? "" : "s"}
    </span>
  );
}

export function DataSourceFootnote({ answeredBy, mentions }: DataSourceProps) {
  const synthetic = countSynthetic(mentions);

  // Live data gets no footnote. A standing "some of this may be synthetic"
  // disclaimer is the kind of label a reader learns to stop seeing.
  if (synthetic === 0) return null;

  if (synthetic === mentions.length) {
    return (
      <p className="footnote">
        Every mention on this page is synthetic. All {mentions.length} of them are marked{" "}
        <span className="mono">raw.synthetic</span> by whatever produced them, and they
        reached this page through {answeredBy} like any other row. No figure here was
        measured from a real conversation.
      </p>
    );
  }

  return (
    <p className="footnote">
      This is a real run with synthetic mentions injected into it:{" "}
      {synthetic} of the {mentions.length} mentions on this page are marked{" "}
      <span className="mono">raw.synthetic</span> by whatever injected them. The other{" "}
      {mentions.length - synthetic} were collected, and the alerts above were fired by
      rules over both.
    </p>
  );
}
