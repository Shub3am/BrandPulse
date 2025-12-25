// Makes the three platforms visible in the product rather than only in the
// architecture diagram: what Nasiko orchestrated, what Anakin cost, what
// DronaHQ delivered.
//
// Every row reads from the RunRecord the orchestrator wrote or from a list
// whose length is the answer. A literal here would be a fabricated measurement
// sitting inside the panel whose whole job is honest degradation reporting,
// which is why the agent count is absent: RunRecord does not carry it yet.

import type { ReplyDraft, RunRecord } from "@/lib/types";
import { durationBetween, rupees } from "@/lib/format";

interface PlatformCard {
  name: string;
  blurb: string;
  rows: [label: string, value: string][];
  note?: string;
}

function buildCards(run: RunRecord, drafts: ReplyDraft[]): PlatformCard[] {
  return [
    {
      name: "Nasiko",
      blurb: "A2A agents, routed and metered.",
      rows: [
        ["Run status", run.status],
        ["Wall clock", durationBetween(run.started_at, run.finished_at) ?? "running"],
        ["LLM spend", rupees(run.cost_paise)],
        ["Tokens", run.tokens_used.toLocaleString("en-IN")],
      ],
      note: run.degraded_reason,
    },
    {
      name: "Anakin",
      blurb: "Every mention on this page was fetched here.",
      rows: [
        ["Credits this run", String(run.credits_used)],
        ["Mentions collected", String(run.mentions_collected)],
        ["Sources hit", run.sources_attempted.join(", ")],
        ["Skipped", run.sources_skipped.join(", ") || "none"],
      ],
    },
    {
      name: "DronaHQ",
      blurb: "Where a human reads the draft and decides.",
      rows: [
        ["Reply drafts queued", String(drafts.length)],
        ["Awaiting approval", String(drafts.filter((draft) => draft.status === "draft").length)],
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
              <span>{value}</span>
            </div>
          ))}
          {card.note && <p className="platform-note">{card.note}</p>}
        </div>
      ))}
    </div>
  );
}
