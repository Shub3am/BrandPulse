// The BFF fetches. One function per route, and the only file in web/ that knows
// where the backend is.
//
// No React, no formatting, no derived value. What a function returns is the
// BFF's JSON plus which backend answered it, because the badge in the header has
// to follow the payload rather than a build-time constant.
//
// This file is server-only. BFF_BASE_URL has no NEXT_PUBLIC_ prefix, so it never
// reaches the browser bundle, which is also why the alert feed polls through
// app/api/alerts/route.ts instead of calling the BFF itself: a NEXT_PUBLIC_ value
// is inlined when the image is built, and the backend's URL is not known until it
// is deployed.
//
// A failed fetch throws. It must never fall back to lib/demoData: a dashboard
// that quietly swaps in synthetic numbers when the backend is down is the exact
// failure this product is pitched against.

import type {
  Alert,
  BrandProfile,
  DailyBrief,
  EnrichedMention,
  ReplyDraft,
  RunRecord,
  ShareOfVoice,
  Topic,
} from "./types";

/** Whatever answered the fetch. There is one backend and a failure throws. */
export type AnsweredBy = "bff";

export interface Fetched<T> {
  answeredBy: AnsweredBy;
  data: T;
}

/**
 * GET /api/brands/:id/pulse. Named slots, one per artifact, and an `errors` array
 * that is populated when a panel below is incomplete. This shape is the BFF's,
 * not models.go's, so it is declared here and not in the mirrored types.ts.
 */
export interface Pulse {
  brand_id: string;
  profile: BrandProfile | null;
  run: RunRecord | null;
  brief: DailyBrief | null;
  share_of_voice: ShareOfVoice | null;
  alerts: Alert[];
  topics: Topic[];
  mentions: EnrichedMention[];
  drafts: ReplyDraft[];
  errors: string[];
}

/** A first paint that hangs is worse than one that says the backend is down. */
const REQUEST_TIMEOUT_MS = 8_000;

export async function fetchPulse(brandId: string): Promise<Fetched<Pulse>> {
  return answered(await getJson<Pulse>(`/api/brands/${encode(brandId)}/pulse`));
}

/**
 * `since` is exclusive and must be a `created_at` this route already returned.
 * The comparison happens in Postgres, so a browser clock running fast cannot
 * skip an alert.
 */
export async function fetchAlerts(brandId: string, since?: string): Promise<Fetched<Alert[]>> {
  const query = since ? `?since=${encodeURIComponent(since)}` : "";
  return answered(await getJson<Alert[]>(`/api/brands/${encode(brandId)}/alerts${query}`));
}

export async function fetchMentions(brandId: string, limit?: number): Promise<Fetched<EnrichedMention[]>> {
  const query = limit === undefined ? "" : `?limit=${limit}`;
  return answered(await getJson<EnrichedMention[]>(`/api/brands/${encode(brandId)}/mentions${query}`));
}

export async function fetchRun(runId: string): Promise<Fetched<RunRecord>> {
  return answered(await getJson<RunRecord>(`/api/runs/${encode(runId)}`));
}

function answered<T>(data: T): Fetched<T> {
  return { answeredBy: "bff", data };
}

function encode(pathSegment: string): string {
  return encodeURIComponent(pathSegment);
}

async function getJson<T>(path: string): Promise<T> {
  const response = await fetch(`${baseUrl()}${path}`, {
    // Never cached. A dashboard whose first paint is a cached crisis from an hour
    // ago is worse than one that takes another 200ms.
    cache: "no-store",
    signal: AbortSignal.timeout(REQUEST_TIMEOUT_MS),
  });

  if (!response.ok) {
    // The BFF's own body carries the reason, including the named `errors` of a
    // partial run, so it is passed through rather than replaced with a status.
    throw new Error(`BFF ${path} answered ${response.status}: ${(await response.text()).slice(0, 300)}`);
  }

  return (await response.json()) as T;
}

function baseUrl(): string {
  const configured = process.env.BFF_BASE_URL;
  if (!configured) {
    throw new Error("BFF_BASE_URL is not set, so the dashboard has no backend to read.");
  }
  return configured.replace(/\/$/, "");
}
