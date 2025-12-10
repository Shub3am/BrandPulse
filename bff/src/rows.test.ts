// Proves the storage-to-wire reconstructions in CONTRACTS §3b, and proves the
// mappers invent nothing outside them.
//
// These run without Postgres on purpose. The mapping is where a field quietly
// goes missing or quietly gains a made-up value, and that is a pure function.

import assert from "node:assert/strict";
import { test } from "node:test";
import {
  toAlert,
  toEnrichedMention,
  toReplyDraft,
  toRunRecord,
  toTopic,
  type AlertRow,
  type EnrichedMentionRow,
  type MentionRow,
} from "./rows.js";

function mentionRow(overrides: Partial<MentionRow> = {}): MentionRow {
  return {
    id: "mn_01JFZ3P7QK8N2V5X9YB4C6D8EG",
    brand_id: "lumeo",
    source: "reddit",
    external_id: "t3_1abc234",
    url: "https://reddit.com/r/india/comments/1abc234",
    author: "u/chaiwithcode",
    author_followers: 0,
    text: "ordered the lumeo lamp, arrived cracked, support is not replying",
    lang: "en",
    posted_at: new Date("2026-09-20T14:02:11.000Z"),
    engagement: { likes: 214, replies: 37, shares: 4, views: 9100 },
    rating: null,
    matched_keyword: "lumeo",
    content_hash: "9f2c1e4b",
    raw: {},
    ...overrides,
  };
}

function enrichedRow(overrides: Partial<EnrichedMentionRow> = {}): EnrichedMentionRow {
  return {
    ...mentionRow(),
    sentiment: -0.71,
    sentiment_label: "negative",
    emotion: "anger",
    intent: "complaint",
    aspects: ["delivery", "support"],
    is_about_brand: true,
    about_competitor: null,
    model: "gpt-4o-mini",
    cost_paise: 0.42,
    ...overrides,
  };
}

test("a NULL column becomes an absent field, not a null on the wire", () => {
  const enriched = toEnrichedMention(
    enrichedRow({ ...mentionRow({ url: null, author: null, rating: null }) }),
  );

  assert.equal("url" in enriched.mention, true);
  assert.equal(enriched.mention.url, undefined);
  // omitempty means the key disappears once serialised, which is what Go emits.
  const serialised = JSON.parse(JSON.stringify(enriched.mention)) as Record<string, unknown>;
  assert.equal("url" in serialised, false);
  assert.equal("author" in serialised, false);
  assert.equal("rating" in serialised, false);
  assert.equal("about_competitor" in JSON.parse(JSON.stringify(enriched.enrichment)), false);
});

test("timestamptz becomes RFC 3339 with an explicit offset", () => {
  const enriched = toEnrichedMention(enrichedRow());
  assert.equal(enriched.mention.posted_at, "2026-09-20T14:02:11.000Z");
});

test("a rating of zero survives, because zero is a real one-star average", () => {
  const enriched = toEnrichedMention(enrichedRow({ ...mentionRow({ rating: 0 }) }));
  assert.equal(enriched.mention.rating, 0);
  assert.equal("rating" in JSON.parse(JSON.stringify(enriched.mention)), true);
});

test("engagement is forwarded exactly as stored, including an empty object", () => {
  const enriched = toEnrichedMention(
    enrichedRow({ ...mentionRow({ engagement: {} as MentionRow["engagement"] }) }),
  );
  // No four invented zeros. A row written without engagement has no counts, and
  // the dashboard is responsible for not printing a number it was not given.
  assert.deepEqual(enriched.mention.engagement, {});
});

test("an alert's sample mention ids become mention objects", () => {
  const row: AlertRow = {
    id: "al_01JFZ3P7QK8N2V5X9YB4C6D8EG",
    brand_id: "lumeo",
    kind: "crisis",
    severity: "critical",
    title: "Negative share tripled on reddit in one hour",
    why: "negative_share 0.61 against a 14-day mean of 0.19",
    evidence: [
      { metric: "negative_share", value: 0.61, threshold: 0.34, window: "1h", detail: "reddit" },
    ],
    sample_mention_ids: ["mn_01JFZ3P7QK8N2V5X9YB4C6D8EG", "mn_deleted_since"],
    status: "open",
    dedupe_key: "crisis:reddit:2026-09-20T14",
    created_at: new Date("2026-09-20T14:05:00.000Z"),
  };

  const known = new Map([["mn_01JFZ3P7QK8N2V5X9YB4C6D8EG", toEnrichedMention(enrichedRow()).mention]]);
  const alert = toAlert(row, known);

  assert.equal(alert.sample_mentions.length, 1);
  assert.equal(alert.sample_mentions[0]?.id, "mn_01JFZ3P7QK8N2V5X9YB4C6D8EG");
  // The second id resolved to nothing and is dropped. A placeholder mention on a
  // crisis alert would be a quote nobody wrote.
  assert.equal(alert.evidence[0]?.value, 0.61);
  assert.equal(alert.created_at, "2026-09-20T14:05:00.000Z");
});

test("a topic keeps its join-table membership and leaves top_examples empty", () => {
  const topic = toTopic({
    id: "tp_01JFZ3P7QK8N2V5X9YB4C6D8EG",
    brand_id: "lumeo",
    window_start: new Date("2026-09-20T00:00:00.000Z"),
    window_end: new Date("2026-09-20T14:00:00.000Z"),
    label: "cracked on arrival",
    summary: "packaging failures on the lamp",
    size: 46,
    sentiment_mix: { negative: 41, neutral: 4, positive: 1 },
    trend: 5.8,
    mention_ids: ["mn_a", "mn_b"],
  });

  assert.deepEqual(topic.mention_ids, ["mn_a", "mn_b"]);
  // "At most 3 mentions chosen by engagement" is bp-clusterer's rule and it has
  // no column. The BFF picking three would be the BFF inventing that output.
  assert.deepEqual(topic.top_examples, []);
  assert.equal(topic.size, 46);
  assert.equal(topic.trend, 5.8);
});

test("a reply draft carries requires_human_approval as a constant true", () => {
  const draft = toReplyDraft({
    id: "rd_01JFZ3P7QK8N2V5X9YB4C6D8EG",
    alert_id: "al_01JFZ3P7QK8N2V5X9YB4C6D8EG",
    mention_id: null,
    channel: "reddit",
    text: "We are sorry about the cracked lamp.",
    tone: "apologetic",
    do_not_say: ["guaranteed", "lifetime warranty"],
    status: "draft",
  });

  // There is no column and no Go struct field. MarshalJSON emits true
  // unconditionally, so every serialisation of this type says so.
  assert.equal(draft.requires_human_approval, true);
  assert.equal("mention_id" in JSON.parse(JSON.stringify(draft)), false);
});

test("a run still going has no finished_at rather than a made-up one", () => {
  const run = toRunRecord({
    id: "run_1",
    brand_id: "lumeo",
    kind: "on_demand",
    time_bucket: "2026-09-20T14",
    started_at: new Date("2026-09-20T14:02:11.000Z"),
    finished_at: null,
    status: "running",
    credits_used: 0,
    tokens_used: 0,
    cost_paise: 0,
    sources_attempted: ["reddit"],
    sources_skipped: [],
    degraded_reason: null,
    mentions_collected: 0,
    errors: [],
  });

  assert.equal(run.finished_at, undefined);
  const serialised = JSON.parse(JSON.stringify(run)) as Record<string, unknown>;
  assert.equal("finished_at" in serialised, false);
  assert.equal("degraded_reason" in serialised, false);
  assert.equal(serialised.credits_used, 0);
});

test("a timestamp that is not a timestamp throws instead of defaulting to now", () => {
  assert.throws(
    () => toEnrichedMention(enrichedRow({ ...mentionRow({ posted_at: null }) })),
    /mentions.posted_at is not a timestamp/,
  );
});
