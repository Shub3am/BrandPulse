// Display aggregates over artifacts that are already on the page.
//
// Counting a list by a label an agent already assigned is display, the same
// class of work as counting how many alerts came back. Deciding what the label
// should have been is not, and belongs nowhere in web/: sentiment is
// bp-enricher's output and severity is bp-detector's.
//
// Every function here reports absence rather than returning a zero that reads
// as a measurement. A split with no classified mentions has no net sentiment,
// and the caller prints a sentence instead of a figure.

import type { Alert, EnrichedMention, SentimentLabel, Severity } from "./types";
import { listOf } from "./wire";

export interface SentimentSplit {
  negative: number;
  neutral: number;
  positive: number;
  mixed: number;
  /** Mentions carrying one of the four labels. The denominator, never assumed. */
  classified: number;
}

const SENTIMENT_LABELS: SentimentLabel[] = ["negative", "neutral", "positive", "mixed"];

const SEVERITIES: Severity[] = ["low", "medium", "high", "critical"];

export function sentimentSplit(mentions: EnrichedMention[] | null | undefined): SentimentSplit {
  const split: SentimentSplit = { negative: 0, neutral: 0, positive: 0, mixed: 0, classified: 0 };
  for (const item of listOf(mentions)) {
    const label = item.enrichment?.sentiment_label;
    if (label && SENTIMENT_LABELS.includes(label)) {
      split[label] += 1;
      split.classified += 1;
    }
  }
  return split;
}

/**
 * Positive minus negative, as a share of the mentions that carry a label.
 * Null when nothing on screen is classified, because a net of zero and no
 * measurement at all are different answers.
 */
export function netSentimentPct(split: SentimentSplit): number | null {
  if (split.classified === 0) return null;
  return ((split.positive - split.negative) / split.classified) * 100;
}

export function countBySeverity(alerts: Alert[] | null | undefined): Record<Severity, number> {
  const counts: Record<Severity, number> = { low: 0, medium: 0, high: 0, critical: 0 };
  for (const alert of listOf(alerts)) {
    if (alert.severity && SEVERITIES.includes(alert.severity)) counts[alert.severity] += 1;
  }
  return counts;
}
