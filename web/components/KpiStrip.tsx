// The six figures at the top of the dashboard: volume, sentiment, alerts,
// topics, what this run cost, and what that costs against an incumbent suite.
//
// Five of the six are read or counted from the payload. The sixth is a price
// comparison, and it is the only place in web/ that holds a number the backend
// did not send: LIST_PRICE_* below are published subscription prices, not
// measurements of anything, and the subtext on that cell says so on screen.
//
// A cell whose figure is absent prints a sentence in place of the digit. A
// greyed zero in a monitoring product reads as a measurement that came back
// zero, and that is the one thing this strip must never do.

import type { Alert, DailyBrief, EnrichedMention, RunRecord, Topic } from "@/lib/types";
import { countBySeverity, netSentimentPct, sentimentSplit } from "@/lib/derive";
import { count, rupees, signedPct, wholeRupees } from "@/lib/format";
import { listOf, numberOf } from "@/lib/wire";

/** BrandPulse's own monthly price. Copy, not a measurement. */
const LIST_PRICE_BRANDPULSE = 2_999;

/** What a mid-size D2C brand pays an incumbent listening suite each month. */
const LIST_PRICE_INCUMBENT = 77_000;

interface Kpi {
  label: string;
  /** The figure, or null when nothing measured one. */
  value: string | null;
  /** What the cell says instead of a digit when `value` is null. */
  absent: string;
  sub: string;
  tone?: "accent" | "positive" | "negative" | "warn";
  headline?: boolean;
}

interface KpiStripProps {
  mentions: EnrichedMention[];
  alerts: Alert[];
  topics: Topic[];
  brief: DailyBrief | null;
  run: RunRecord | null;
}

function volumeKpi(mentions: EnrichedMention[], brief: DailyBrief | null): Kpi {
  const briefed = numberOf(brief?.numbers?.mentions);
  const delta = numberOf(brief?.numbers?.mentions_delta_pct);

  if (briefed !== null) {
    return {
      label: "Mentions in window",
      value: count(briefed),
      absent: "",
      sub: delta === null
        ? "counted by bp-briefer over its period"
        : `${signedPct(delta)} against the period before it`,
    };
  }

  return {
    label: "Mentions in window",
    value: mentions.length > 0 ? count(mentions.length) : null,
    absent: "nothing stored",
    sub: mentions.length > 0
      ? "classified rows on this page, no brief written yet"
      : "no mention has been collected for this brand",
  };
}

function sentimentKpi(mentions: EnrichedMention[]): Kpi {
  const split = sentimentSplit(mentions);
  const net = netSentimentPct(split);

  if (net === null) {
    return {
      label: "Net sentiment",
      value: null,
      absent: "unclassified",
      sub: "no mention on this page carries a sentiment label",
    };
  }

  return {
    label: "Net sentiment",
    value: signedPct(net, 0),
    absent: "",
    sub: `positive minus negative across ${count(split.classified)} classified mentions`,
    tone: net < 0 ? "negative" : "positive",
  };
}

function alertsKpi(alerts: Alert[]): Kpi {
  const bySeverity = countBySeverity(alerts);
  const urgent = bySeverity.critical + bySeverity.high;
  return {
    label: "Alerts firing",
    value: count(alerts.length),
    absent: "",
    sub: alerts.length === 0
      ? "nothing has crossed a threshold, which is the good outcome"
      : `${count(bySeverity.critical)} critical, ${count(bySeverity.high)} high, fired by rules not by a model`,
    tone: urgent > 0 ? "negative" : undefined,
  };
}

function topicsKpi(topics: Topic[]): Kpi {
  return {
    label: "Topics tracked",
    value: count(topics.length),
    absent: "",
    sub: topics.length === 0
      ? "bp-clusterer needs enough mentions in one window to form a cluster"
      : "TF-IDF clusters bp-clusterer formed over the run window",
  };
}

function costKpi(run: RunRecord | null): Kpi {
  const paise = numberOf(run?.cost_paise);
  const credits = numberOf(run?.credits_used);
  const tokens = numberOf(run?.tokens_used);

  if (paise === null) {
    return {
      label: "Cost of this run",
      value: null,
      absent: "not metered",
      sub: "no run record carries a spend figure for this brand yet",
    };
  }

  const spent = [
    credits === null ? null : `${count(credits)} Anakin credits`,
    tokens === null ? null : `${count(tokens)} tokens`,
  ].filter((part): part is string => part !== null);

  return {
    label: "Cost of this run",
    value: rupees(paise),
    absent: "",
    sub: spent.length > 0 ? `LLM spend for ${spent.join(" and ")}` : "LLM spend for this run",
    tone: "accent",
  };
}

/** The price argument. Both figures are list prices and the subtext says so. */
function savingKpi(): Kpi {
  return {
    label: "Saved vs incumbent",
    value: `${wholeRupees(LIST_PRICE_INCUMBENT - LIST_PRICE_BRANDPULSE)}/mo`,
    absent: "",
    sub: `${wholeRupees(LIST_PRICE_BRANDPULSE)} a month against ${wholeRupees(LIST_PRICE_INCUMBENT)} list price. Prices, not measurements.`,
    tone: "accent",
    headline: true,
  };
}

export function KpiStrip({ mentions, alerts, topics, brief, run }: KpiStripProps) {
  const safeMentions = listOf(mentions);
  const safeAlerts = listOf(alerts);
  const safeTopics = listOf(topics);

  const kpis: Kpi[] = [
    volumeKpi(safeMentions, brief),
    sentimentKpi(safeMentions),
    alertsKpi(safeAlerts),
    topicsKpi(safeTopics),
    costKpi(run),
    savingKpi(),
  ];

  return (
    <div className="kpis">
      {kpis.map((kpi) => (
        <div
          className={`kpi${kpi.tone ? ` kpi-${kpi.tone}` : ""}${kpi.headline ? " kpi-headline" : ""}`}
          key={kpi.label}
        >
          <div className="kpi-label">{kpi.label}</div>
          {kpi.value === null ? (
            <div className="kpi-value absent">{kpi.absent}</div>
          ) : (
            <div className="kpi-value">{kpi.value}</div>
          )}
          <div className="kpi-sub">{kpi.sub}</div>
        </div>
      ))}
    </div>
  );
}
