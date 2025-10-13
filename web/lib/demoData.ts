// Demo data for the dashboard shell, shaped exactly like the agent artifacts.
//
// Everything here is synthetic and the UI labels it as such, because the repo
// rule is that no source is ever faked silently. The brand is fictional on
// purpose: a mockup must not put invented complaints next to a real company's
// name. Swapping this file for a fetch against the Fastify BFF is the whole of
// the "make it live" step.
//
// This file must not import React or call anything.

import type {
  Alert, BrandProfile, BriefNumbers, EnrichedMention, ReplyDraft, RunRecord, Topic,
} from "./types";

export const BRAND: BrandProfile = {
  brand_id: "lumeo",
  name: "Lumeo",
  website: "https://lumeo.example",
  keywords: ["Lumeo", "Lumeo serum", "Lumeo skincare"],
  hashtags: ["#lumeo"],
  products: ["Vitamin C serum", "Mineral sunscreen", "Ceramide moisturiser"],
  competitors: ["Minimalist", "The Ordinary", "Plum", "Dot & Key"],
  sources: ["reddit", "amazon", "youtube", "news", "playstore", "web"],
  negative_keywords: ["lumeo lighting", "lumeo camera"],
  source_handles: { playstore: "com.lumeo.shop", amazon: "B0C7X9LUM" },
  voice: {
    tone: "warm, direct, no corporate filler",
    language: "English with light Hinglish if the mention is Hinglish",
    do_not_say: ["clinically proven", "dermatologically tested", "we apologise for any inconvenience"],
  },
  version: 3,
};

export const NUMBERS: BriefNumbers = {
  mentions: 412,
  mentions_delta_pct: 38.4,
  sentiment_avg: -0.21,
  negative_share: 0.34,
  share_of_voice: 22.6,
};

export const CRISIS_ALERT: Alert = {
  id: "alert_7c21",
  brand_id: "lumeo",
  kind: "crisis",
  severity: "critical",
  title: "Negative spike on Reddit about the Vitamin C serum batch",
  why:
    "Negative mentions in the last hour are 4.1 standard deviations above the " +
    "14-day hourly baseline for this source, and 61% of them share the aspect " +
    "'skin reaction'.",
  evidence: [
    { metric: "negative_mentions_z", value: 4.1, threshold: 3.0, window: "1h", detail: "baseline 2.3/h over 14d" },
    { metric: "negative_share", value: 0.61, threshold: 0.4, window: "1h", detail: "28 of 46 mentions" },
    { metric: "aspect_concentration", value: 0.61, threshold: 0.5, window: "1h", detail: "aspect: skin reaction" },
  ],
  sample_mentions: [],
  status: "open",
  dedupe_key: "crisis:reddit:2026-09-20T14",
  created_at: "2026-09-20T14:06:00Z",
};

/** Every open alert. Components count this rather than hardcoding a total. */
export const ALERTS: Alert[] = [CRISIS_ALERT];

export const DRAFT: ReplyDraft = {
  id: "draft_7c21",
  alert_id: "alert_7c21",
  channel: "reddit",
  tone: BRAND.voice.tone,
  text:
    "This is Ananya from Lumeo. We're reading every one of these and we're not " +
    "going to hand-wave it. If you have a reaction, stop using the serum now. " +
    "DM me your order ID and we'll refund it today, no return needed. We're " +
    "pulling batch L-2411 while we test it, and I'll post what we find here by " +
    "Monday, whatever it says.",
  do_not_say: BRAND.voice.do_not_say,
  status: "draft",
  requires_human_approval: true,
};

/** Every draft awaiting a human. The panel counts this rather than a literal. */
export const DRAFTS: ReplyDraft[] = [DRAFT];

export const MENTIONS: EnrichedMention[] = [
  {
    mention: {
      id: "m_01", brand_id: "lumeo", source: "reddit", external_id: "t3_1abcd",
      url: "https://reddit.com/r/IndianSkincareAddicts/comments/1abcd",
      author: "u/serum_skeptic", author_followers: 0,
      text: "Anyone else break out badly from the new Lumeo vitamin C batch? Third person in this sub this week.",
      lang: "en", posted_at: "2026-09-20T13:58:00Z",
      engagement: { likes: 214, replies: 61, shares: 12, views: 8900 },
      matched_keyword: "Lumeo", content_hash: "a1f0…",
    },
    enrichment: {
      mention_id: "m_01", sentiment: -0.72, sentiment_label: "negative", emotion: "anger",
      intent: "complaint", aspects: ["skin reaction", "product quality"],
      is_about_brand: true, model: "gpt-4o-mini", cost_paise: 1.8,
    },
  },
  {
    mention: {
      id: "m_02", brand_id: "lumeo", source: "amazon", external_id: "R2XYZ",
      author: "Priya K.", author_followers: 0,
      text: "Burning sensation within 10 minutes. Returned it. Was fine with the old bottle.",
      lang: "en", posted_at: "2026-09-20T13:41:00Z",
      engagement: { likes: 33, replies: 2, shares: 0, views: 0 },
      rating: 1, matched_keyword: "Lumeo serum", content_hash: "b7c2…",
    },
    enrichment: {
      mention_id: "m_02", sentiment: -0.81, sentiment_label: "negative", emotion: "disgust",
      intent: "complaint", aspects: ["skin reaction"],
      is_about_brand: true, model: "gpt-4o-mini", cost_paise: 1.6,
    },
  },
  {
    mention: {
      id: "m_03", brand_id: "lumeo", source: "youtube", external_id: "Ugx9",
      author: "@skinwithsneha", author_followers: 184000,
      text: "Doing a batch test on the Lumeo serum tonight since my comments are full of it.",
      lang: "en", posted_at: "2026-09-20T13:20:00Z",
      engagement: { likes: 1420, replies: 308, shares: 96, views: 44000 },
      matched_keyword: "Lumeo", content_hash: "c9d4…",
    },
    enrichment: {
      mention_id: "m_03", sentiment: -0.15, sentiment_label: "neutral", emotion: "surprise",
      intent: "news", aspects: ["product quality"],
      is_about_brand: true, model: "gpt-4o-mini", cost_paise: 1.9,
    },
  },
  {
    mention: {
      id: "m_04", brand_id: "lumeo", source: "playstore", external_id: "gp_5512",
      author: "Rahul M.", author_followers: 0,
      text: "App is smooth, delivery was next day in Pune. Sunscreen is genuinely good.",
      lang: "en", posted_at: "2026-09-20T11:02:00Z",
      engagement: { likes: 4, replies: 0, shares: 0, views: 0 },
      rating: 5, matched_keyword: "Lumeo", content_hash: "d1e8…",
    },
    enrichment: {
      mention_id: "m_04", sentiment: 0.78, sentiment_label: "positive", emotion: "joy",
      intent: "praise", aspects: ["delivery", "app experience"],
      is_about_brand: true, model: "gpt-4o-mini", cost_paise: 1.5,
    },
  },
  {
    mention: {
      id: "m_05", brand_id: "lumeo", source: "news", external_id: "yst_771",
      author: "YourStory", author_followers: 0,
      text: "D2C skincare funding slows in Q3; Lumeo and two peers reportedly extending runway.",
      lang: "en", posted_at: "2026-09-20T09:15:00Z",
      engagement: { likes: 61, replies: 7, shares: 24, views: 0 },
      matched_keyword: "Lumeo", content_hash: "e4a1…",
    },
    enrichment: {
      mention_id: "m_05", sentiment: -0.08, sentiment_label: "neutral", emotion: "neutral",
      intent: "news", aspects: ["funding"],
      is_about_brand: true, model: "gpt-4o-mini", cost_paise: 2.1,
    },
  },
];

export const TOPICS: Topic[] = [
  {
    id: "t_01", brand_id: "lumeo",
    window_start: "2026-09-19T14:00:00Z", window_end: "2026-09-20T14:00:00Z",
    label: "Vitamin C serum skin reactions",
    summary: "Users on Reddit and Amazon report burning and breakouts, several naming the same batch code.",
    size: 46, sentiment_mix: { negative: 38, neutral: 6, positive: 2 },
    top_examples: [], trend: 5.8, mention_ids: [],
  },
  {
    id: "t_02", brand_id: "lumeo",
    window_start: "2026-09-19T14:00:00Z", window_end: "2026-09-20T14:00:00Z",
    label: "Fast delivery in metros",
    summary: "Repeat praise for next-day delivery in Pune, Bengaluru and Mumbai.",
    size: 31, sentiment_mix: { positive: 27, neutral: 4 },
    top_examples: [], trend: 1.1, mention_ids: [],
  },
  {
    id: "t_03", brand_id: "lumeo",
    window_start: "2026-09-19T14:00:00Z", window_end: "2026-09-20T14:00:00Z",
    label: "Price versus Minimalist",
    summary: "Shoppers comparing the 10% serum against a cheaper competitor SKU.",
    size: 22, sentiment_mix: { neutral: 14, negative: 6, positive: 2 },
    top_examples: [], trend: 0.8, mention_ids: [],
  },
];

export const RUN: RunRecord = {
  id: "run_0920_14",
  brand_id: "lumeo",
  kind: "scheduled",
  time_bucket: "2026-09-20T14",
  started_at: "2026-09-20T14:00:00Z",
  finished_at: "2026-09-20T14:01:47Z",
  status: "partial",
  credits_used: 11,
  tokens_used: 18420,
  cost_paise: 63.4,
  sources_attempted: ["reddit", "amazon", "youtube", "news", "playstore", "web"],
  sources_skipped: ["appstore"],
  degraded_reason: "Nasiko flow guard capped fan-out at 6 concurrent collectors",
  mentions_collected: NUMBERS.mentions,
  errors: [],
};

/** Time from the first negative mention to the WhatsApp send, in seconds. */
export const TIME_TO_WHATSAPP_SECONDS = 107;
