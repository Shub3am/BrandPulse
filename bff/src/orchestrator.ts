// The bp-orchestrator client: one A2A call, one RunRecord back, unreshaped.
//
// Everything about how that call is made lives in a2a.ts. This file exists so
// that the routes depend on the verb they need, startRun, rather than on the
// transport, and so the test suite can hand them a stub that spends nothing.

import { callAgent } from "./a2a.js";
import type { RunRecord } from "./contracts.js";
import type { RunRequest } from "./envelopes.js";

const AGENT = "bp-orchestrator";
const ARTIFACT = "RunRecord";

/** What a route is allowed to depend on. The test stub implements this too. */
export interface Orchestrator {
  startRun(input: RunRequest): Promise<RunRecord>;
}

export function createOrchestrator(baseUrl: string, timeoutMs: number): Orchestrator {
  return {
    startRun: (input) => callAgent<RunRecord>(AGENT, baseUrl, input, ARTIFACT, timeoutMs),
  };
}
