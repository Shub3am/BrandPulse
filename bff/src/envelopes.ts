// Agent input and output envelopes, transcribed from docs/CONTRACTS.md §2.
//
// CONTRACTS §2 names internal/models/agentio.go as the Go home for these
// types. That file does not exist on this branch, so §2 is the only source and
// nothing here is verifiable against a compiler until B1 lands it. Raised in
// HACKATHON_NOTES.md "Open blockers".
//
// These are transcriptions, not designs. If a shape here turns out to disagree
// with agentio.go, agentio.go wins and this file changes. Do not invent a field
// to make a route tidier, and do not reshape one to fit the dashboard.
//
// checkTypesParity.mjs deliberately does not check this file: there is no Go
// struct to diff it against yet.

import type { Alert, Enrichment, Mention, RunKind, Source, Topic } from "./contracts.js";

export interface MentionBatch {
  mentions: Mention[];
  credits_used: number;
  cache_hits: number;
  source: Source;
  truncated: boolean;
  errors: string[];
}

export interface EnrichmentBatch {
  enrichments: Enrichment[];
  tokens_used: number;
  cost_paise: number;
  cache_hits: number;
  errors: string[];
}

export interface TopicSet {
  topics: Topic[];
  unclustered: string[];
  tokens_used: number;
  cost_paise: number;
  errors: string[];
}

export interface AlertSet {
  alerts: Alert[];
  rules_evaluated: number;
  errors: string[];
}

export interface BaselineStats {
  per_source_hourly_mean: Partial<Record<Source, number>>;
  per_source_hourly_std: Partial<Record<Source, number>>;
  negative_share_mean: number;
  negative_share_std: number;
  mean_rating: number | null;
  days: number;
}

/** bp-orchestrator's input, as §2 declares it. */
export interface RunInput {
  brand_id: string;
  trigger: RunKind;
  window_hours: number;
  force: boolean;
}

/**
 * The only thing the BFF ever sends to an agent.
 *
 * window_hours and force are optional here although §2 has them required,
 * because a field the caller did not send is omitted rather than filled. Go
 * decodes an absent window_hours as 0, and 0 is a zero-hour window, not a
 * request for the orchestrator's default. Picking 24 instead would put a
 * collection policy in the BFF.
 */
export type RunRequest =
  & Pick<RunInput, "brand_id" | "trigger">
  & Partial<Pick<RunInput, "window_hours" | "force">>;
