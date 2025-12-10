// GET /api/brands/:id/pulse — everything the dashboard's first paint reads.
//
// One request, seven independent queries, and a named slot for each artifact.
// Naming the slots is composition, not reshaping: every value under a key is the
// artifact exactly as contracts.ts declares it, and no key holds a figure this
// route worked out.
//
// The route must never turn a partial answer into a failure. Queries run through
// allSettled, so a brand with a broken topics table still renders its alerts and
// its mention stream, with the failure named in `errors` for the UI to show. A
// 500 here is a blank dashboard, which tells a customer less than four panels
// and a note saying the fifth is down.

import type { FastifyInstance, FastifyRequest } from "fastify";
import type { History } from "../db.js";
import type { Alert, BrandProfile, DailyBrief, EnrichedMention, ReplyDraft, RunRecord, ShareOfVoice, Topic } from "../contracts.js";
import { limitFrom } from "./params.js";

const ALERT_LIMIT = 20;
const TOPIC_LIMIT = 12;
const MENTION_LIMIT = 50;
const MENTION_CEILING = 200;
const DRAFT_LIMIT = 20;

export interface Pulse {
  brand_id: string;
  profile: BrandProfile | null;
  run: RunRecord | null;
  brief: DailyBrief | null;
  /**
   * Always null today. There is no share_of_voice table and no SOV field on
   * RunRecord, so no agent output for this type is persisted anywhere for the
   * BFF to read. The slot is here because CONTRACTS declares the type and the
   * dashboard will want it; computing one from the mentions table would be the
   * BFF inventing a statistic. The figure the dashboard renders today comes from
   * brief.numbers.share_of_voice, which bp-briefer produced. Raised in
   * HACKATHON_NOTES.md.
   */
  share_of_voice: ShareOfVoice | null;
  alerts: Alert[];
  topics: Topic[];
  mentions: EnrichedMention[];
  drafts: ReplyDraft[];
  /** One entry per query that failed. Empty means every panel below is complete. */
  errors: string[];
}

type PulseRequest = FastifyRequest<{
  Params: { id: string };
  Querystring: { limit?: string };
}>;

export function registerPulseRoute(app: FastifyInstance, history: History): void {
  app.get("/api/brands/:id/pulse", async (request: PulseRequest) => {
    const brandId = request.params.id;
    const mentionLimit = limitFrom(request.query.limit, MENTION_LIMIT, MENTION_CEILING);
    const errors: string[] = [];

    const [profile, run, brief, alerts, topics, mentions, drafts] = await Promise.all([
      settle(errors, "profile", history.latestBrandProfile(brandId), null),
      settle(errors, "run", history.latestRun(brandId), null),
      settle(errors, "brief", history.latestBrief(brandId), null),
      settle(errors, "alerts", history.recentAlerts(brandId, ALERT_LIMIT), []),
      settle(errors, "topics", history.recentTopics(brandId, TOPIC_LIMIT), []),
      settle(errors, "mentions", history.recentMentions(brandId, mentionLimit), []),
      settle(errors, "drafts", history.recentDrafts(brandId, DRAFT_LIMIT), []),
    ]);

    const pulse: Pulse = {
      brand_id: brandId,
      profile,
      run,
      brief,
      share_of_voice: null,
      alerts,
      topics,
      mentions,
      drafts,
      errors,
    };
    return pulse;
  });
}

/**
 * Records a failed query in `errors` and falls back to the empty value for that
 * slot. Empty, not invented: an alerts query that failed yields no alerts, which
 * the UI reports as a failure rather than as a quiet day.
 */
async function settle<T>(
  errors: string[],
  slot: string,
  work: Promise<T>,
  empty: T,
): Promise<T> {
  try {
    return await work;
  } catch (cause) {
    errors.push(`${slot}: ${cause instanceof Error ? cause.message : String(cause)}`);
    return empty;
  }
}
