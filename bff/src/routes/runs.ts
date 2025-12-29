// GET /api/runs/:id and POST /api/runs.
//
// POST /api/runs is the only write path in the whole BFF, and it writes nothing
// itself: it asks bp-orchestrator to run, and bp-orchestrator owns every row
// that follows. The RunRecord it returns is forwarded byte-for-byte as the agent
// produced it.
//
// window_hours and force are forwarded only when the caller sent them. Sending
// window_hours: 0 on the caller's behalf is not a neutral default in Go, it is a
// window of zero hours, and choosing 24 instead would be the BFF inventing a
// policy that belongs to the orchestrator.
//
// A run that comes back status "partial" with a populated errors array is a
// success as far as HTTP is concerned. It is a real run that got through some of
// its sources, the dashboard is built to say so, and a 500 would throw away the
// mentions it did collect.

import type { FastifyInstance, FastifyReply, FastifyRequest } from "fastify";
import type { RunKind } from "../contracts.js";
import type { History } from "../db.js";
import type { RunRequest } from "../envelopes.js";
import type { Orchestrator } from "../orchestrator.js";
import { BadRequestError } from "./params.js";

const RUN_KINDS: readonly RunKind[] = ["onboard", "scheduled", "on_demand", "crisis_replay"];

type RunByIdRequest = FastifyRequest<{ Params: { id: string } }>;
type StartRunRequest = FastifyRequest<{ Body: unknown }>;

export function registerRunRoutes(
  app: FastifyInstance,
  history: History,
  orchestrator: Orchestrator,
): void {
  app.get("/api/runs/:id", async (request: RunByIdRequest, reply: FastifyReply) => {
    const run = await history.runById(request.params.id);
    if (!run) {
      return reply.code(404).send({ error: `no run with id ${request.params.id}` });
    }
    return run;
  });

  app.post("/api/runs", async (request: StartRunRequest) => {
    return orchestrator.startRun(runInputFrom(request.body));
  });
}

function runInputFrom(body: unknown): RunRequest {
  if (typeof body !== "object" || body === null) {
    throw new BadRequestError("body must be a JSON object");
  }
  const fields = body as Record<string, unknown>;

  const brandId = fields.brand_id;
  if (typeof brandId !== "string" || brandId === "") {
    throw new BadRequestError("brand_id is required and must be a non-empty string");
  }

  const trigger = fields.trigger;
  if (typeof trigger !== "string" || !RUN_KINDS.includes(trigger as RunKind)) {
    throw new BadRequestError(`trigger must be one of: ${RUN_KINDS.join(", ")}`);
  }

  const input: RunRequest = { brand_id: brandId, trigger: trigger as RunKind };

  const windowHours = fields.window_hours;
  if (windowHours !== undefined) {
    if (!Number.isInteger(windowHours) || (windowHours as number) <= 0) {
      throw new BadRequestError("window_hours must be a positive integer when present");
    }
    input.window_hours = windowHours as number;
  }

  const force = fields.force;
  if (force !== undefined) {
    if (typeof force !== "boolean") {
      throw new BadRequestError("force must be a boolean when present");
    }
    input.force = force;
  }
  return input;
}
