// GET /api/brands/:id/mentions — the mention stream, newest first.
//
// Returns a bare EnrichedMention array and not a MentionBatch. MentionBatch is
// bp-collector's output envelope and carries credits_used, cache_hits and
// truncated, none of which a read out of Postgres knows. Wrapping the rows in
// that shape would mean filling those three fields with numbers nobody counted.
//
// A failure here has no partial answer to give, so it becomes an HTTP error
// rather than an empty array. An empty array means the brand has no enriched
// mentions, and the dashboard says exactly that.

import type { FastifyInstance, FastifyRequest } from "fastify";
import type { History } from "../db.js";
import { limitFrom } from "./params.js";

const DEFAULT_LIMIT = 50;
const CEILING = 200;

type MentionsRequest = FastifyRequest<{
  Params: { id: string };
  Querystring: { limit?: string };
}>;

export function registerMentionsRoute(app: FastifyInstance, history: History): void {
  app.get("/api/brands/:id/mentions", async (request: MentionsRequest) => {
    const limit = limitFrom(request.query.limit, DEFAULT_LIMIT, CEILING);
    return history.recentMentions(request.params.id, limit);
  });
}
