// Turns Postgres rows into the artifact shapes in contracts.ts.
//
// This is reconstruction, not derivation. Storage and the wire disagree in the
// ways docs/CONTRACTS.md §3b lists, and every difference here is one of them:
// a brand's name lives on `brands` rather than `brand_profiles`, an alert stores
// mention ids where the wire carries mention objects, a topic stores its
// membership in a join table. Putting those back is the BFF's job.
//
// It must not compute a figure. No sum, no average, no percentage, no rounding,
// no count that Postgres did not already return. If a field has no column and
// no agent produced it, this file leaves it empty and bff/CLAUDE.md says why.

import type {
  Alert,
  BrandProfile,
  EnrichedMention,
  ReplyDraft,
  RunRecord,
  Topic,
} from "./contracts.js";

/**
 * TIMESTAMPTZ arrives as a JS Date from node-postgres. CONTRACTS §1 wants
 * RFC 3339 with an explicit offset, which toISOString gives as a Z suffix.
 * A missing or unexpected value throws rather than defaulting, because a
 * timestamp that is quietly wrong is worse than a request that fails.
 */
function isoTimestamp(value: unknown, column: string): string {
  if (value instanceof Date) return value.toISOString();
  if (typeof value === "string" && value !== "") return value;
  throw new TypeError(`${column} is not a timestamp: ${JSON.stringify(value)}`);
}

function optionalIsoTimestamp(value: unknown, column: string): string | undefined {
  return value === null || value === undefined ? undefined : isoTimestamp(value, column);
}

/** A NULL text column maps to an absent field, which is what omitempty means. */
function optional<T>(value: T | null | undefined): T | undefined {
  return value === null ? undefined : value;
}

export interface BrandProfileRow {
  id: string;
  name: string;
  website: string | null;
  version: number;
  keywords: BrandProfile["keywords"];
  hashtags: BrandProfile["hashtags"];
  products: BrandProfile["products"];
  competitors: BrandProfile["competitors"];
  sources: BrandProfile["sources"];
  negative_keywords: BrandProfile["negative_keywords"];
  source_handles: BrandProfile["source_handles"];
  voice: BrandProfile["voice"];
}

export function toBrandProfile(row: BrandProfileRow): BrandProfile {
  return {
    brand_id: row.id,
    name: row.name,
    website: optional(row.website),
    keywords: row.keywords,
    hashtags: row.hashtags,
    products: row.products,
    competitors: row.competitors,
    sources: row.sources,
    negative_keywords: row.negative_keywords,
    source_handles: row.source_handles,
    voice: row.voice,
    version: row.version,
  };
}

export interface MentionRow {
  id: string;
  brand_id: string;
  source: EnrichedMention["mention"]["source"];
  external_id: string;
  url: string | null;
  author: string | null;
  author_followers: number;
  text: string;
  lang: string;
  posted_at: unknown;
  engagement: EnrichedMention["mention"]["engagement"];
  rating: number | null;
  matched_keyword: string | null;
  content_hash: string;
  raw: Record<string, unknown>;
}

export function toMention(row: MentionRow): EnrichedMention["mention"] {
  return {
    id: row.id,
    brand_id: row.brand_id,
    source: row.source,
    external_id: row.external_id,
    url: optional(row.url),
    author: optional(row.author),
    author_followers: row.author_followers,
    text: row.text,
    lang: row.lang,
    posted_at: isoTimestamp(row.posted_at, "mentions.posted_at"),
    // Forwarded exactly as stored. The column defaults to '{}', so a row written
    // without engagement yields an object with no counts rather than four zeros
    // this file made up.
    engagement: row.engagement,
    rating: optional(row.rating),
    matched_keyword: optional(row.matched_keyword),
    content_hash: row.content_hash,
    raw: row.raw,
  };
}

export interface EnrichedMentionRow extends MentionRow {
  sentiment: number;
  sentiment_label: EnrichedMention["enrichment"]["sentiment_label"];
  emotion: EnrichedMention["enrichment"]["emotion"];
  intent: EnrichedMention["enrichment"]["intent"];
  aspects: string[];
  is_about_brand: boolean;
  about_competitor: string | null;
  model: string;
  cost_paise: number;
}

export function toEnrichedMention(row: EnrichedMentionRow): EnrichedMention {
  return {
    mention: toMention(row),
    enrichment: {
      mention_id: row.id,
      sentiment: row.sentiment,
      sentiment_label: row.sentiment_label,
      emotion: row.emotion,
      intent: row.intent,
      aspects: row.aspects,
      is_about_brand: row.is_about_brand,
      about_competitor: optional(row.about_competitor),
      model: row.model,
      cost_paise: row.cost_paise,
    },
  };
}

export interface AlertRow {
  id: string;
  brand_id: string;
  kind: Alert["kind"];
  severity: Alert["severity"];
  title: string;
  why: string;
  evidence: Alert["evidence"];
  sample_mention_ids: string[];
  status: Alert["status"];
  dedupe_key: string;
  created_at: unknown;
}

/**
 * §3b: the wire carries whole mentions, storage carries their ids. The caller
 * resolves the ids and passes what it found; an id with no surviving mention is
 * dropped rather than stood in for, because a placeholder mention on a crisis
 * alert is a fabricated quote.
 */
export function toAlert(row: AlertRow, mentionsById: Map<string, EnrichedMention["mention"]>): Alert {
  const samples = row.sample_mention_ids
    .map((id) => mentionsById.get(id))
    .filter((mention): mention is EnrichedMention["mention"] => mention !== undefined);

  return {
    id: row.id,
    brand_id: row.brand_id,
    kind: row.kind,
    severity: row.severity,
    title: row.title,
    why: row.why,
    evidence: row.evidence,
    sample_mentions: samples,
    status: row.status,
    dedupe_key: row.dedupe_key,
    created_at: isoTimestamp(row.created_at, "alerts.created_at"),
  };
}

export interface TopicRow {
  id: string;
  brand_id: string;
  window_start: unknown;
  window_end: unknown;
  label: string;
  summary: string;
  size: number;
  sentiment_mix: Topic["sentiment_mix"];
  trend: number;
  mention_ids: string[];
}

export function toTopic(row: TopicRow): Topic {
  return {
    id: row.id,
    brand_id: row.brand_id,
    window_start: isoTimestamp(row.window_start, "topics.window_start"),
    window_end: isoTimestamp(row.window_end, "topics.window_end"),
    label: row.label,
    summary: row.summary,
    size: row.size,
    sentiment_mix: row.sentiment_mix,
    // Empty on purpose. "At most 3 mentions, chosen by engagement" is
    // bp-clusterer's rule and it has no column, so the BFF picking three would
    // be the BFF inventing the clusterer's output.
    top_examples: [],
    trend: row.trend,
    mention_ids: row.mention_ids,
  };
}

export interface ReplyDraftRow {
  id: string;
  alert_id: string | null;
  mention_id: string | null;
  channel: ReplyDraft["channel"];
  text: string;
  tone: string;
  do_not_say: string[];
  status: ReplyDraft["status"];
}

export function toReplyDraft(row: ReplyDraftRow): ReplyDraft {
  return {
    id: row.id,
    alert_id: optional(row.alert_id),
    mention_id: optional(row.mention_id),
    channel: row.channel,
    text: row.text,
    tone: row.tone,
    do_not_say: row.do_not_say,
    status: row.status,
    // Constant, not a column. ReplyDraft.MarshalJSON emits true unconditionally,
    // so any serialisation of this type carries it. A column would imply it
    // could be false, and nothing in this product may post without a human.
    requires_human_approval: true,
  };
}

export interface RunRow {
  id: string;
  brand_id: string;
  kind: RunRecord["kind"];
  time_bucket: string;
  started_at: unknown;
  finished_at: unknown;
  status: RunRecord["status"];
  credits_used: number;
  tokens_used: number;
  cost_paise: number;
  sources_attempted: RunRecord["sources_attempted"];
  sources_skipped: RunRecord["sources_skipped"];
  degraded_reason: string | null;
  mentions_collected: number;
  errors: string[];
}

export function toRunRecord(row: RunRow): RunRecord {
  return {
    id: row.id,
    brand_id: row.brand_id,
    kind: row.kind,
    time_bucket: row.time_bucket,
    started_at: isoTimestamp(row.started_at, "runs.started_at"),
    finished_at: optionalIsoTimestamp(row.finished_at, "runs.finished_at"),
    status: row.status,
    credits_used: row.credits_used,
    tokens_used: row.tokens_used,
    cost_paise: row.cost_paise,
    sources_attempted: row.sources_attempted,
    sources_skipped: row.sources_skipped,
    degraded_reason: optional(row.degraded_reason),
    mentions_collected: row.mentions_collected,
    errors: row.errors,
  };
}
