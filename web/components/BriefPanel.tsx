// The daily brief bp-briefer wrote, as prose with its figures lifted out.
//
// The five figures across the top are BriefNumbers exactly as they arrived. The
// prose below is the agent's own markdown, rendered as paragraphs, headings and
// bullets and nothing else: there is no markdown library here and there must
// not be one, because a brief is a few hundred words of plain text and a parser
// that renders raw HTML would let an agent's output write into this page.
//
// A figure that did not arrive prints what is missing instead of a digit.

import type { DailyBrief } from "@/lib/types";
import { count, dayAndTime, pct, signedPct } from "@/lib/format";
import { listOf, numberOf, textOf } from "@/lib/wire";

interface Figure {
  label: string;
  value: string | null;
}

function figuresOf(brief: DailyBrief): Figure[] {
  const numbers = brief.numbers;
  const mentions = numberOf(numbers?.mentions);
  const delta = numberOf(numbers?.mentions_delta_pct);
  const sentiment = numberOf(numbers?.sentiment_avg);
  const negative = numberOf(numbers?.negative_share);
  const share = numberOf(numbers?.share_of_voice);

  return [
    { label: "Mentions", value: mentions === null ? null : count(mentions) },
    { label: "vs prior period", value: delta === null ? null : signedPct(delta) },
    { label: "Mean sentiment", value: sentiment === null ? null : sentiment.toFixed(2) },
    { label: "Negative share", value: negative === null ? null : pct(negative) },
    { label: "Share of voice", value: share === null ? null : `${share.toFixed(1)}%` },
  ];
}

/**
 * The agent's markdown as blocks. It recognises a heading, a bullet and a
 * paragraph, which is everything bp-briefer emits, and it renders every other
 * line as text: an unrecognised construct shows up as the characters the agent
 * actually wrote rather than disappearing.
 */
type Block =
  | { kind: "heading"; text: string }
  | { kind: "bullets"; items: string[] }
  | { kind: "paragraph"; text: string };

function blocksOf(markdown: string): Block[] {
  const blocks: Block[] = [];
  let paragraph: string[] = [];
  let bullets: string[] = [];

  function flush(): void {
    if (bullets.length > 0) {
      blocks.push({ kind: "bullets", items: bullets });
      bullets = [];
    }
    if (paragraph.length > 0) {
      blocks.push({ kind: "paragraph", text: paragraph.join(" ") });
      paragraph = [];
    }
  }

  for (const rawLine of markdown.split("\n")) {
    const line = rawLine.trim();
    if (line === "") {
      flush();
      continue;
    }
    if (line.startsWith("#")) {
      flush();
      blocks.push({ kind: "heading", text: line.replace(/^#+\s*/, "") });
      continue;
    }
    if (line.startsWith("- ") || line.startsWith("* ")) {
      if (paragraph.length > 0) flush();
      bullets.push(line.slice(2).trim());
      continue;
    }
    if (bullets.length > 0) flush();
    paragraph.push(line);
  }
  flush();
  return blocks;
}

function BriefProse({ markdown }: { markdown: string }) {
  const blocks = blocksOf(markdown);
  if (blocks.length === 0) return null;

  return (
    <div className="brief-prose">
      {blocks.map((block, index) => {
        if (block.kind === "heading") return <h4 key={index}>{block.text}</h4>;
        if (block.kind === "bullets") {
          return (
            <ul key={index}>
              {block.items.map((item, itemIndex) => (
                <li key={itemIndex}>{item}</li>
              ))}
            </ul>
          );
        }
        return <p key={index}>{block.text}</p>;
      })}
    </div>
  );
}

function NamedList({ title, items }: { title: string; items: string[] }) {
  if (items.length === 0) return null;
  return (
    <div className="brief-list">
      <div className="guard-title">{title}</div>
      <ol>
        {items.map((item, index) => (
          <li key={`${index}-${item}`}>{item}</li>
        ))}
      </ol>
    </div>
  );
}

export function BriefPanel({ brief }: { brief: DailyBrief }) {
  const headline = textOf(brief.headline);
  const markdown = textOf(brief.markdown);
  const period = [dayAndTime(brief.period_start), dayAndTime(brief.period_end)].filter(
    (edge): edge is string => edge !== null,
  );

  const actions = listOf(brief.suggested_actions)
    .map((action) => textOf(action))
    .filter((action): action is string => action !== null);
  const competitors = listOf(brief.competitor_watch)
    .map((name) => textOf(name))
    .filter((name): name is string => name !== null);

  return (
    <div className="card">
      <h3 className="brief-headline">
        {headline ?? "bp-briefer wrote this brief without a headline."}
      </h3>
      {period.length === 2 && (
        <p className="note" style={{ marginTop: 8 }}>
          Period {period[0]} to {period[1]} IST
        </p>
      )}

      <div className="brief-figures">
        {figuresOf(brief).map((figure) => (
          <div className="figure" key={figure.label}>
            <div className="figure-label">{figure.label}</div>
            {figure.value === null ? (
              <div className="figure-value absent">not sent</div>
            ) : (
              <div className="figure-value">{figure.value}</div>
            )}
          </div>
        ))}
      </div>

      <div className="brief-cols">
        <div>
          {markdown ? (
            <BriefProse markdown={markdown} />
          ) : (
            <p className="note">
              This brief carried its numbers but no written body, so there is no prose to
              render.
            </p>
          )}
        </div>
        <div>
          <NamedList title="Suggested actions" items={actions} />
          <NamedList title="Competitor watch" items={competitors} />
        </div>
      </div>
    </div>
  );
}
