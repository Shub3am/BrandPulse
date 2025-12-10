// GET /api/brands/:id/alerts — the alert feed, newest first.
//
// Since the 2026-09-20 scope change this is the primary alert surface's source:
// web/ renders these rows live, so this route is polled rather than read once.
// `since` is what makes polling cheap, and it takes a created_at the caller
// received from this route rather than a clock reading of its own. A browser
// clock that runs slow against the database would otherwise skip an alert, and
// the one thing this feed may never do is silently miss a crisis.
//
// It returns a bare Alert array and not an AlertSet: AlertSet carries
// rules_evaluated, which is bp-detector's count of the rules it ran, and a read
// out of Postgres has no idea what that number was.

import type { FastifyInstance, FastifyRequest } from "fastify";
import type { History } from "../db.js";
import { limitFrom, sinceFrom } from "./params.js";

const DEFAULT_LIMIT = 20;
const CEILING = 100;

type AlertsRequest = FastifyRequest<{
  Params: { id: string };
  Querystring: { limit?: string; since?: string };
}>;

export function registerAlertsRoute(app: FastifyInstance, history: History): void {
  app.get("/api/brands/:id/alerts", async (request: AlertsRequest) => {
    const limit = limitFrom(request.query.limit, DEFAULT_LIMIT, CEILING);
    const since = sinceFrom(request.query.since);
    return history.recentAlerts(request.params.id, limit, since);
  });
}
