// The Run now button's target. It exists for the same reason the alert poll's
// route does: the browser must never need the BFF's URL, which is injected when
// the backend is deployed and therefore cannot be inlined into a bundle at image
// build time.
//
// It forwards three fields and returns the BFF's RunRecord unchanged. It
// validates nothing beyond their presence, because `runInputFrom` in
// bff/src/routes/runs.ts is the one place that decides what a run request is
// allowed to say.

import { startRun } from "@/lib/pulse";
import type { RunKind } from "@/lib/types";

/** Every request hits the BFF. There is nothing here worth caching. */
export const dynamic = "force-dynamic";

export async function POST(request: Request): Promise<Response> {
  let body: unknown;
  try {
    body = await request.json();
  } catch {
    return Response.json({ error: "body must be JSON" }, { status: 400 });
  }

  const fields = (body ?? {}) as {
    brand_id?: unknown;
    trigger?: unknown;
    window_hours?: unknown;
  };
  if (typeof fields.brand_id !== "string" || fields.brand_id === "") {
    return Response.json({ error: "brand_id is required" }, { status: 400 });
  }
  if (typeof fields.trigger !== "string") {
    return Response.json({ error: "trigger is required" }, { status: 400 });
  }

  // Absent stays absent. Coercing a missing window_hours to a number here would
  // send the orchestrator a window this route chose.
  const windowHours = typeof fields.window_hours === "number" ? fields.window_hours : undefined;

  try {
    const answered = await startRun(fields.brand_id, fields.trigger as RunKind, windowHours);
    return Response.json(answered.data);
  } catch (cause) {
    // 502, not 500: the dashboard is up and the backend behind it is not, and the
    // button says exactly that rather than going quiet.
    const reason = cause instanceof Error ? cause.message : String(cause);
    return Response.json({ error: reason }, { status: 502 });
  }
}
