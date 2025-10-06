// Display formatting only. No business rules live here: a number that needed a
// rule computed in the browser would be a number the agents should have sent.

import type { SentimentLabel } from "./types";

export function signedPct(value: number): string {
  return `${value >= 0 ? "+" : ""}${value.toFixed(1)}%`;
}

export function clockTime(iso: string): string {
  return new Date(iso).toLocaleTimeString("en-IN", {
    hour: "2-digit",
    minute: "2-digit",
    timeZone: "Asia/Kolkata",
  });
}

export function minutesAndSeconds(totalSeconds: number): string {
  const minutes = Math.floor(totalSeconds / 60);
  return `${minutes}m ${totalSeconds % 60}s`;
}

export function rupees(paise: number): string {
  return `₹${(paise / 100).toFixed(2)}`;
}

export const SENTIMENT_COLOR: Record<SentimentLabel, string> = {
  negative: "var(--negative)",
  neutral: "var(--text-faint)",
  positive: "var(--positive)",
  mixed: "var(--warn)",
};
