// The headline numbers from a DailyBrief.
//
// Every value here arrives computed. Counting a list is display; comparing one
// against a baseline is a rule, and a rule belongs in bp-detector where
// `nasiko observe` can see it fire.

import type { Alert, BriefNumbers } from "@/lib/types";
import { pct, signedPct } from "@/lib/format";

interface Stat {
  label: string;
  value: string;
  note: string;
  /** Only set where the wire carries a direction; never inferred here. */
  tone?: "up" | "down";
  color?: string;
}

function buildStats(numbers: BriefNumbers, alerts: Alert[]): Stat[] {
  const critical = alerts.filter((a) => a.severity === "critical").length;
  return [
    {
      label: "Mentions 24h",
      value: String(numbers.mentions),
      note: `${signedPct(numbers.mentions_delta_pct)} vs yesterday`,
      tone: numbers.mentions_delta_pct >= 0 ? "up" : "down",
    },
    {
      label: "Sentiment",
      value: numbers.sentiment_avg.toFixed(2),
      note: "mean, range -1 to 1",
      color: numbers.sentiment_avg < 0 ? "var(--negative)" : "var(--positive)",
    },
    {
      label: "Negative share",
      value: pct(numbers.negative_share),
      note: "of all mentions",
    },
    {
      label: "Share of voice",
      value: `${numbers.share_of_voice.toFixed(1)}%`,
      note: "brand vs competitors",
    },
    {
      label: "Open alerts",
      value: String(alerts.length),
      note: `${critical} critical`,
      color: alerts.length > 0 ? "var(--negative)" : undefined,
    },
  ];
}

export function StatRow({ numbers, alerts }: { numbers: BriefNumbers; alerts: Alert[] }) {
  return (
    <div className="stats">
      {buildStats(numbers, alerts).map((stat) => (
        <div className="card stat" key={stat.label}>
          <div className="stat-label">{stat.label}</div>
          <div className="stat-value" style={stat.color ? { color: stat.color } : undefined}>
            {stat.value}
          </div>
          <div className={`stat-delta${stat.tone ? ` ${stat.tone}` : ""}`}>{stat.note}</div>
        </div>
      ))}
    </div>
  );
}
