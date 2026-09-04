// Display formatting only. No business rules live here: a number that needed a
// rule computed in the browser would be a number the agents should have sent.
//
// A label-to-colour map and a duration rendered from two timestamps the record
// already carries are formatting. A threshold, a baseline or a classification
// is not, and does not belong in this file or in a component.
//
// Every function takes what the wire can actually hold, which includes null,
// and answers null rather than "Invalid Date" or "NaN%". The caller prints a
// sentence where the figure would have gone.

import type { SentimentLabel, Severity } from "./types";

// Built once. Passing options to toLocaleTimeString defeats the engine's
// formatter cache, which costs ~20x per call once the real 400-mention feed
// replaces the demo data.
const IST_CLOCK = new Intl.DateTimeFormat("en-IN", {
  hour: "2-digit",
  minute: "2-digit",
  timeZone: "Asia/Kolkata",
});

const IST_DAY = new Intl.DateTimeFormat("en-IN", {
  day: "2-digit",
  month: "short",
  timeZone: "Asia/Kolkata",
});

const INDIAN_NUMBER = new Intl.NumberFormat("en-IN");

/** Milliseconds since the epoch, or null when the wire value is not a date. */
function parsed(iso: string | null | undefined): number | null {
  if (typeof iso !== "string") return null;
  const ms = Date.parse(iso);
  return Number.isNaN(ms) ? null : ms;
}

export function clockTime(iso: string | null | undefined): string | null {
  const ms = parsed(iso);
  return ms === null ? null : IST_CLOCK.format(ms);
}

export function dayAndTime(iso: string | null | undefined): string | null {
  const ms = parsed(iso);
  return ms === null ? null : `${IST_DAY.format(ms)}, ${IST_CLOCK.format(ms)}`;
}

export function count(value: number): string {
  return INDIAN_NUMBER.format(value);
}

export function pct(fraction01: number, digits = 0): string {
  return `${(fraction01 * 100).toFixed(digits)}%`;
}

export function signedPct(alreadyPercent: number, digits = 1): string {
  return `${alreadyPercent >= 0 ? "+" : ""}${alreadyPercent.toFixed(digits)}%`;
}

export function minutesAndSeconds(totalSeconds: number): string {
  const minutes = Math.floor(totalSeconds / 60);
  return `${minutes}m ${totalSeconds % 60}s`;
}

/** Elapsed time between two wire timestamps. Null while still running. */
export function durationBetween(
  startIso: string | null | undefined,
  endIso?: string | null,
): string | null {
  const start = parsed(startIso);
  const end = parsed(endIso);
  if (start === null || end === null) return null;
  return minutesAndSeconds(Math.round((end - start) / 1000));
}

/** Paise to rupees, with the paise kept: a run costs less than one rupee. */
export function rupees(paise: number): string {
  return `₹${(paise / 100).toFixed(2)}`;
}

/** Whole rupees, grouped the Indian way. For prices, never for measurements. */
export function wholeRupees(amount: number): string {
  return `₹${INDIAN_NUMBER.format(Math.round(amount))}`;
}

/** Wire enums are snake_case. This is spelling, not interpretation. */
export function humanLabel(wireValue: string | null | undefined): string | null {
  return typeof wireValue === "string" && wireValue !== "" ? wireValue.replaceAll("_", " ") : null;
}

export const SENTIMENT_COLOR: Record<SentimentLabel, string> = {
  negative: "var(--negative)",
  neutral: "var(--neutral)",
  positive: "var(--positive)",
  mixed: "var(--warn)",
};

/** Severity to colour. The detector assigns the severity; this only paints it. */
export const SEVERITY_COLOR: Record<Severity, string> = {
  low: "var(--neutral)",
  medium: "var(--warn)",
  high: "var(--orange)",
  critical: "var(--negative)",
};

export function severityColor(severity: Severity | null | undefined): string {
  return severity && severity in SEVERITY_COLOR
    ? SEVERITY_COLOR[severity]
    : "var(--border-strong)";
}

export function sentimentColor(label: SentimentLabel | null | undefined): string {
  return label && label in SENTIMENT_COLOR ? SENTIMENT_COLOR[label] : "var(--neutral)";
}
