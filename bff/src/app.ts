// Builds the Fastify instance from collaborators it is handed.
//
// It takes the database and the agent client as arguments rather than
// constructing them, so the tests can run the real routes against a fake
// history and a stub agent. That keeps the test suite at zero network calls,
// zero credits and zero Postgres, which is the repo's CI rule.
//
// It must not read process.env, open a pool or know an agent URL. server.ts does
// all three. It must not listen either: a test that binds a port is a test that
// fails when two of them run at once.

import cors from "@fastify/cors";
import Fastify, { type FastifyInstance } from "fastify";
import type { History } from "./db.js";
import { OrchestratorError, type Orchestrator } from "./orchestrator.js";
import { BadRequestError } from "./routes/params.js";
import { registerAlertsRoute } from "./routes/alerts.js";
import { registerMentionsRoute } from "./routes/mentions.js";
import { registerPulseRoute } from "./routes/pulse.js";
import { registerRunRoutes } from "./routes/runs.js";

export interface AppDependencies {
  history: History;
  orchestrator: Orchestrator;
  /** The one browser origin allowed to call these routes. Never a list, never "*". */
  dashboardOrigin: string;
  /** Off in tests only. A request log is how a degraded run is diagnosed in production. */
  logger?: boolean;
}

export function buildApp({
  history,
  orchestrator,
  dashboardOrigin,
  logger = true,
}: AppDependencies): FastifyInstance {
  const app = Fastify({ logger });

  // A single origin, because this API answers exactly one dashboard. Widening
  // this to "*" would let any page a customer has open read their brand's
  // mentions through their own browser.
  app.register(cors, { origin: [dashboardOrigin], methods: ["GET", "POST"] });

  // The deploy target needs something cheap to poll that does not touch
  // Postgres, so a failing database shows up as unhealthy requests rather than
  // as a container restart loop.
  app.get("/health", async () => ({ ok: true }));

  registerPulseRoute(app, history);
  registerMentionsRoute(app, history);
  registerAlertsRoute(app, history);
  registerRunRoutes(app, history, orchestrator);

  app.setErrorHandler((error: unknown, request, reply) => {
    if (error instanceof BadRequestError) {
      return reply.code(error.httpStatus).send({ error: error.message });
    }
    if (error instanceof OrchestratorError) {
      request.log.error({ err: error }, "agent call failed");
      return reply.code(error.httpStatus).send({ error: error.message });
    }
    // Fastify's own 4xx (a malformed JSON body, a missing route) already carry a
    // usable status, and replacing it with 500 would tell the dashboard the
    // backend broke when the request did.
    const status = fastifyStatus(error) ?? 500;
    const message = error instanceof Error ? error.message : String(error);
    if (status >= 500) request.log.error({ err: error }, "request failed");
    return reply.code(status).send({ error: message });
  });

  return app;
}

/** Fastify attaches statusCode to the errors it raises itself; nothing else does. */
function fastifyStatus(error: unknown): number | undefined {
  if (typeof error !== "object" || error === null) return undefined;
  const status = (error as { statusCode?: unknown }).statusCode;
  return typeof status === "number" && status >= 400 ? status : undefined;
}
