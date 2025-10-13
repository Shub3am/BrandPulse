// The wire format, mirrored from internal/models/models.go.
//
// These are the JSON shapes the agents emit. Field names here must match the
// Go json tags exactly, because the Fastify BFF passes agent artifacts through
// without reshaping them. When models.go changes, this file changes in the same
// commit or the dashboard silently renders undefined.
//
// This file must not contain mock values, fetch calls or React.

export type Source =
  | "x" | "reddit" | "youtube" | "news" | "playstore"
  | "appstore" | "amazon" | "flipkart" | "instagram" | "web";

export type SentimentLabel = "negative" | "neutral" | "positive" | "mixed";

export type Emotion =
  | "anger" | "joy" | "sadness" | "fear" | "disgust" | "surprise" | "neutral";

export type Intent =
  | "complaint" | "praise" | "question" | "purchase_intent"
  | "comparison" | "spam" | "news" | "other";

/** Either a Source value or one of the three reply-only channels. */
export type Channel = Source | "whatsapp" | "email" | "statement";

export type RunKind = "onboard" | "scheduled" | "on_demand" | "crisis_replay";

export type AlertKind =
  | "spike" | "crisis" | "influencer_mention" | "competitor_move" | "review_bomb";

export type Severity = "low" | "medium" | "high" | "critical";

export type AlertStatus = "open" | "acked" | "snoozed" | "resolved";

export type DraftStatus = "draft" | "approved" | "rejected" | "sent";

export type RunStatus = "running" | "ok" | "partial" | "failed";

export interface BrandVoice {
  tone: string;
  language: string;
  signature?: string;
  do_not_say: string[];
}

export interface BrandProfile {
  brand_id: string;
  name: string;
  website?: string;
  keywords: string[];
  hashtags: string[];
  products: string[];
  competitors: string[];
  sources: Source[];
  negative_keywords: string[];
  source_handles: Record<string, string>;
  voice: BrandVoice;
  version: number;
}

export interface Engagement {
  likes: number;
  replies: number;
  shares: number;
  views: number;
}

export interface Mention {
  id: string;
  brand_id: string;
  source: Source;
  external_id: string;
  url?: string;
  author?: string;
  author_followers: number;
  text: string;
  lang: string;
  posted_at: string;
  engagement: Engagement;
  rating?: number;
  matched_keyword?: string;
  content_hash: string;
  raw?: Record<string, unknown>;
}

export interface Enrichment {
  mention_id: string;
  sentiment: number;
  sentiment_label: SentimentLabel;
  emotion: Emotion;
  intent: Intent;
  aspects: string[];
  is_about_brand: boolean;
  about_competitor?: string;
  model: string;
  cost_paise: number;
}

export interface EnrichedMention {
  mention: Mention;
  enrichment: Enrichment;
}

/** One statistical fact that fired an alert. Rendered verbatim, never summarised. */
export interface AlertEvidence {
  metric: string;
  value: number;
  threshold: number;
  window: string;
  detail?: string;
}

export interface Alert {
  id: string;
  brand_id: string;
  kind: AlertKind;
  severity: Severity;
  title: string;
  why: string;
  evidence: AlertEvidence[];
  sample_mentions: Mention[];
  status: AlertStatus;
  dedupe_key: string;
  created_at: string;
}

export interface ReplyDraft {
  id: string;
  alert_id?: string;
  mention_id?: string;
  channel: Channel;
  text: string;
  tone: string;
  do_not_say: string[];
  status: DraftStatus;
  /**
   * Always true. The Go type has no such field: MarshalJSON emits it
   * unconditionally, so it is present on the wire and absent from the struct.
   * That asymmetry is why ReplyDraft cannot be machine-translated from Go.
   */
  requires_human_approval: boolean;
}

export interface Topic {
  id: string;
  brand_id: string;
  window_start: string;
  window_end: string;
  label: string;
  summary: string;
  size: number;
  sentiment_mix: Partial<Record<SentimentLabel, number>>;
  top_examples: Mention[];
  trend: number;
  mention_ids: string[];
}

export interface BriefNumbers {
  mentions: number;
  mentions_delta_pct: number;
  sentiment_avg: number;
  negative_share: number;
  share_of_voice: number;
}

export interface RunRecord {
  id: string;
  brand_id: string;
  kind: RunKind;
  time_bucket: string;
  started_at: string;
  finished_at?: string;
  status: RunStatus;
  credits_used: number;
  tokens_used: number;
  cost_paise: number;
  sources_attempted: Source[];
  sources_skipped: Source[];
  degraded_reason?: string;
  mentions_collected: number;
  errors: string[];
}
