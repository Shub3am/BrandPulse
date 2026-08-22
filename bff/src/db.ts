// Reads brand history out of Postgres. Nothing here writes.
//
// The agents own every write in this system. The BFF exists so the browser has
// one backend, not so it has a second author of the truth, and the read-only
// guard below is enforced by Postgres rather than by reviewers remembering:
// the session sets default_transaction_read_only, so an INSERT added here later
// fails at the server even though the SQL is valid.
//
// It must not call an agent, and it must not compute a figure. Every number a
// route returns is a column Postgres already held. `LIMIT` is the one thing
// this file decides, because a request asking for 50 mentions is pagination,
// not a derived statistic.

import { Pool, type QueryResultRow } from "pg";
import type { Alert, DailyBrief, EnrichedMention, ReplyDraft, RunRecord, Topic } from "./contracts.js";
import type { BrandProfile } from "./contracts.js";
import {
  toAlert,
  toBrandProfile,
  toBrandSummary,
  toEnrichedMention,
  toMention,
  toReplyDraft,
  toRunRecord,
  toTopic,
  type AlertRow,
  type BrandProfileRow,
  type BrandSummary,
  type BrandSummaryRow,
  type EnrichedMentionRow,
  type MentionRow,
  type ReplyDraftRow,
  type RunRow,
  type TopicRow,
} from "./rows.js";

/** What a route is allowed to depend on. The test fake implements this too. */
export interface History {
  listBrands(): Promise<BrandSummary[]>;
  latestBrandProfile(brandId: string): Promise<BrandProfile | null>;
  latestRun(brandId: string): Promise<RunRecord | null>;
  runById(runId: string): Promise<RunRecord | null>;
  latestBrief(brandId: string): Promise<DailyBrief | null>;
  recentAlerts(brandId: string, limit: number, since?: string): Promise<Alert[]>;
  recentTopics(brandId: string, limit: number): Promise<Topic[]>;
  recentMentions(brandId: string, limit: number): Promise<EnrichedMention[]>;
  recentDrafts(brandId: string, limit: number): Promise<ReplyDraft[]>;
  close(): Promise<void>;
}

const MENTION_COLUMNS = `
  m.id, m.brand_id, m.source, m.external_id, m.url, m.author, m.author_followers,
  m.text, m.lang, m.posted_at, m.engagement, m.rating, m.matched_keyword,
  m.content_hash, m.raw`;

export function createHistory(databaseUrl: string, timeoutMs: number): History {
  const pool = new Pool({
    connectionString: databaseUrl,
    max: 8,
    connectionTimeoutMillis: timeoutMs,
    // Two separate ceilings. statement_timeout stops a slow query holding a
    // request open; the read-only default makes a write impossible rather than
    // merely absent.
    statement_timeout: timeoutMs,
    options: "-c default_transaction_read_only=on",
  });

  // pg emits "error" on the pool when Postgres drops a client that is sitting
  // idle, which happens on a restart or an idle timeout and is not a request
  // failing. An EventEmitter with no "error" listener rethrows, and node turns
  // that into an unhandled exception, so without this line one recycled
  // connection takes the whole dashboard backend down with it. Swallowing it
  // is correct: pg discards the dead client itself and the next query checks
  // out a fresh one.
  pool.on("error", (cause) => {
    console.error("bff: postgres dropped an idle connection:", cause.message);
  });

  async function query<Row extends QueryResultRow>(sql: string, params: unknown[]): Promise<Row[]> {
    const result = await pool.query<Row>(sql, params);
    return result.rows;
  }

  /** Resolves alert sample ids in one round trip instead of one per alert. */
  async function mentionsByIds(ids: string[]): Promise<Map<string, EnrichedMention["mention"]>> {
    if (ids.length === 0) return new Map();
    const rows = await query<MentionRow>(
      `SELECT ${MENTION_COLUMNS} FROM mentions m WHERE m.id = ANY($1::text[])`,
      [ids],
    );
    return new Map(rows.map((row) => [row.id, toMention(row)]));
  }

  return {
    async listBrands() {
      // Every brand, unpaginated on purpose: this feeds a picker in a top bar,
      // and a picker that hides the brand you just created is worse than a
      // query that returns a few more rows than it needs to.
      const rows = await query<BrandSummaryRow>(
        `SELECT id, name, website FROM brands ORDER BY name ASC`,
        [],
      );
      return rows.map(toBrandSummary);
    },

    async latestBrandProfile(brandId) {
      const rows = await query<BrandProfileRow>(
        `SELECT b.id, b.name, b.website, p.version, p.keywords, p.hashtags,
                p.products, p.competitors, p.sources, p.negative_keywords,
                p.source_handles, p.voice
           FROM brand_profiles p
           JOIN brands b ON b.id = p.brand_id
          WHERE p.brand_id = $1
          ORDER BY p.version DESC
          LIMIT 1`,
        [brandId],
      );
      return rows[0] ? toBrandProfile(rows[0]) : null;
    },

    async latestRun(brandId) {
      const rows = await query<RunRow>(
        `SELECT id, brand_id, kind, time_bucket, started_at, finished_at, status,
                credits_used, tokens_used, cost_paise, sources_attempted,
                sources_skipped, degraded_reason, mentions_collected, errors
           FROM runs
          WHERE brand_id = $1
          ORDER BY started_at DESC
          LIMIT 1`,
        [brandId],
      );
      return rows[0] ? toRunRecord(rows[0]) : null;
    },

    async runById(runId) {
      const rows = await query<RunRow>(
        `SELECT id, brand_id, kind, time_bucket, started_at, finished_at, status,
                credits_used, tokens_used, cost_paise, sources_attempted,
                sources_skipped, degraded_reason, mentions_collected, errors
           FROM runs
          WHERE id = $1`,
        [runId],
      );
      return rows[0] ? toRunRecord(rows[0]) : null;
    },

    async latestBrief(brandId) {
      // briefs.payload is the whole DailyBrief as bp-briefer produced it, so
      // this is the one query that needs no reconstruction at all.
      const rows = await query<{ payload: DailyBrief }>(
        `SELECT payload FROM briefs
          WHERE brand_id = $1
          ORDER BY period_start DESC
          LIMIT 1`,
        [brandId],
      );
      return rows[0]?.payload ?? null;
    },

    async recentAlerts(brandId, limit, since) {
      // `since` is exclusive so a poll that passes back the newest created_at it
      // already has does not receive that alert a second time.
      const rows = await query<AlertRow>(
        `SELECT id, brand_id, kind, severity, title, why, evidence,
                sample_mention_ids, status, dedupe_key, created_at
           FROM alerts
          WHERE brand_id = $1
            AND ($2::timestamptz IS NULL OR created_at > $2::timestamptz)
          ORDER BY created_at DESC
          LIMIT $3`,
        [brandId, since ?? null, limit],
      );
      const sampleIds = [...new Set(rows.flatMap((row) => row.sample_mention_ids))];
      const mentions = await mentionsByIds(sampleIds);
      return rows.map((row) => toAlert(row, mentions));
    },

    async recentTopics(brandId, limit) {
      const rows = await query<TopicRow>(
        `SELECT t.id, t.brand_id, t.window_start, t.window_end, t.label, t.summary,
                t.size, t.sentiment_mix, t.trend,
                COALESCE(
                  ARRAY_AGG(tm.mention_id) FILTER (WHERE tm.mention_id IS NOT NULL),
                  '{}'
                ) AS mention_ids
           FROM topics t
           LEFT JOIN topic_mentions tm ON tm.topic_id = t.id
          WHERE t.brand_id = $1
          GROUP BY t.id
          ORDER BY t.window_end DESC, t.size DESC
          LIMIT $2`,
        [brandId, limit],
      );
      return rows.map(toTopic);
    },

    async recentMentions(brandId, limit) {
      // An inner join, not a left join: an EnrichedMention is both halves, and
      // standing in a neutral enrichment for a mention nobody has scored would
      // put a sentiment on the screen that no model produced. Mentions awaiting
      // enrichment therefore do not appear in the stream yet.
      const rows = await query<EnrichedMentionRow>(
        `SELECT ${MENTION_COLUMNS},
                e.sentiment, e.sentiment_label, e.emotion, e.intent, e.aspects,
                e.is_about_brand, e.about_competitor, e.model, e.cost_paise
           FROM mentions m
           JOIN mention_enrichment e ON e.mention_id = m.id
          WHERE m.brand_id = $1
          ORDER BY m.posted_at DESC
          LIMIT $2`,
        [brandId, limit],
      );
      return rows.map(toEnrichedMention);
    },

    async recentDrafts(brandId, limit) {
      // reply_drafts has no brand_id. It reaches a brand through whichever of
      // alert_id or mention_id the CHECK constraint guarantees is set.
      const rows = await query<ReplyDraftRow>(
        `SELECT d.id, d.alert_id, d.mention_id, d.channel, d.text, d.tone,
                d.do_not_say, d.status
           FROM reply_drafts d
           LEFT JOIN alerts a ON a.id = d.alert_id
           LEFT JOIN mentions m ON m.id = d.mention_id
          WHERE COALESCE(a.brand_id, m.brand_id) = $1
          ORDER BY d.created_at DESC
          LIMIT $2`,
        [brandId, limit],
      );
      return rows.map(toReplyDraft);
    },

    async close() {
      await pool.end();
    },
  };
}
