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
  RunKind,
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

/**
 * Starting a run is not a paint, so it gets its own budget. It sits just above
 * the BFF's own ORCHESTRATOR_TIMEOUT_MS default of 120000 so that a slow
 * orchestrator produces the BFF's named timeout rather than a bare abort here.
 */
const START_RUN_TIMEOUT_MS = 125_000;

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

/**
 * POST /api/runs, the one write path the dashboard has. The body is the two
 * fields `runInputFrom` in bff/src/routes/runs.ts requires. `window_hours` and
 * `force` are deliberately not sent: the orchestrator owns how far back a run
 * looks, and a number chosen here would be the dashboard setting that policy.
 */
export async function startRun(brandId: string, trigger: RunKind): Promise<Fetched<RunRecord>> {
  const body = { brand_id: brandId, trigger };
  return answered(await postJson<RunRecord>("/api/runs", body, START_RUN_TIMEOUT_MS));
}

function answered<T>(data: T): Fetched<T> {
  return { answeredBy: "bff", data };
}

function encode(pathSegment: string): string {
  return encodeURIComponent(pathSegment);
}

async function getJson<T>(path: string): Promise<T> {
  return sendJson<T>(path, {}, REQUEST_TIMEOUT_MS);
}

async function postJson<T>(path: string, body: unknown, timeoutMs: number): Promise<T> {
  return sendJson<T>(
    path,
    { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify(body) },
    timeoutMs,
  );
}

async function sendJson<T>(path: string, init: RequestInit, timeoutMs: number): Promise<T> {
  const response = await fetch(`${baseUrl()}${path}`, {
    ...init,
    // Never cached. A dashboard whose first paint is a cached crisis from an hour
    // ago is worse than one that takes another 200ms.
    cache: "no-store",
    signal: AbortSignal.timeout(timeoutMs),
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
