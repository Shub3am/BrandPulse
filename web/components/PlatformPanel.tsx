// Makes the three platforms visible in the product rather than only in the
// architecture diagram: what Nasiko orchestrated, what Anakin cost, what
// DronaHQ delivered.
//
// Every row reads from the RunRecord the orchestrator wrote or from a list
// whose length is the answer. A literal here would be a fabricated measurement
// sitting inside the panel whose whole job is honest degradation reporting,
// which is why the agent count is absent: RunRecord does not carry it yet.
//
// A field that did not arrive prints what is missing. "0 credits" and "no
// credit figure was recorded" are different claims, and only one of them is
// true of a run that never reported.

import type { ReplyDraft, RunRecord } from "@/lib/types";
import { count, durationBetween, rupees } from "@/lib/format";
import { listOf, numberOf, textOf } from "@/lib/wire";

interface PlatformCard {
  name: string;
  blurb: string;
  rows: [label: string, value: string][];
  note?: string;
}

/** A count from the wire, or the sentence that says none was recorded. */
function counted(value: number | null | undefined): string {
  const real = numberOf(value);
  return real === null ? "not recorded" : count(real);
}

function joined(values: string[] | null | undefined, whenEmpty: string): string {
  const named = listOf(values)
    .map((value) => textOf(value))
    .filter((value): value is string => value !== null);
  return named.length > 0 ? named.join(", ") : whenEmpty;
}

function buildCards(run: RunRecord, drafts: ReplyDraft[]): PlatformCard[] {
  const paise = numberOf(run.cost_paise);
  const rows = listOf(drafts);

  return [
    {
      name: "Nasiko",
      blurb: "A2A agents, routed and metered.",
      rows: [
        ["Run status", textOf(run.status) ?? "not recorded"],
        ["Kind", textOf(run.kind) ?? "not recorded"],
        ["Wall clock", durationBetween(run.started_at, run.finished_at) ?? "still running"],
        ["LLM spend", paise === null ? "not metered" : rupees(paise)],
        ["Tokens", counted(run.tokens_used)],
      ],
      note: textOf(run.degraded_reason) ?? undefined,
    },
    {
      name: "Anakin",
      blurb: "Every mention on this page was fetched here.",
      rows: [
        ["Credits this run", counted(run.credits_used)],
        ["Mentions collected", counted(run.mentions_collected)],
        ["Sources hit", joined(run.sources_attempted, "none recorded")],
        ["Skipped", joined(run.sources_skipped, "none")],
      ],
    },
    {
      name: "DronaHQ",
      blurb: "Where a human reads the draft and decides.",
      rows: [
        ["Reply drafts queued", count(rows.length)],
        ["Awaiting approval", count(rows.filter((draft) => draft.status === "draft").length)],
        ["Auto-posted", "0, by design"],
      ],
    },
  ];
}

export function PlatformPanel({ run, drafts }: { run: RunRecord; drafts: ReplyDraft[] }) {
  return (
    <div className="platforms">
      {buildCards(run, drafts).map((card) => (
        <div className="card platform" key={card.name}>
          <h4>
            <i className="dot" />
            {card.name}
          </h4>
          <p>{card.blurb}</p>
          {card.rows.map(([label, value]) => (
            <div className="kv" key={label}>
              <span>{label}</span>
              <span title={value}>{value}</span>
            </div>
          ))}
          {card.note && <p className="platform-note">{card.note}</p>}
        </div>
      ))}
    </div>
  );
}
