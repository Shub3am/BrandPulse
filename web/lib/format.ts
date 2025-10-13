// Display formatting only. No business rules live here: a number that needed a
// rule computed in the browser would be a number the agents should have sent.
//
// A label-to-colour map and a duration rendered from two timestamps the record
// already carries are formatting. A threshold, a baseline or a classification
// is not, and does not belong in this file or in a component.

import type { SentimentLabel } from "./types";

// Built once. Passing options to toLocaleTimeString defeats the engine's
// formatter cache, which costs ~20x per call once the real 400-mention feed
// replaces the demo data.
const IST_CLOCK = new Intl.DateTimeFormat("en-IN", {
  hour: "2-digit",
  minute: "2-digit",
  timeZone: "Asia/Kolkata",
});

export function clockTime(iso: string): string {
  return IST_CLOCK.format(new Date(iso));
}

export function pct(fraction01: number, digits = 0): string {
  return `${(fraction01 * 100).toFixed(digits)}%`;
}

export function signedPct(alreadyPercent: number): string {
  return `${alreadyPercent >= 0 ? "+" : ""}${alreadyPercent.toFixed(1)}%`;
}

export function minutesAndSeconds(totalSeconds: number): string {
  const minutes = Math.floor(totalSeconds / 60);
  return `${minutes}m ${totalSeconds % 60}s`;
}

/** Elapsed time between two wire timestamps. Returns null while still running. */
export function durationBetween(startIso: string, endIso?: string): string | null {
  if (!endIso) return null;
  return minutesAndSeconds(Math.round((Date.parse(endIso) - Date.parse(startIso)) / 1000));
}

export function rupees(paise: number): string {
  return `₹${(paise / 100).toFixed(2)}`;
}

/** Wire enums are snake_case. This is spelling, not interpretation. */
export function humanLabel(wireValue: string): string {
  return wireValue.replaceAll("_", " ");
}

export const SENTIMENT_COLOR: Record<SentimentLabel, string> = {
  negative: "var(--negative)",
  neutral: "var(--text-faint)",
  positive: "var(--positive)",
  mixed: "var(--warn)",
};
