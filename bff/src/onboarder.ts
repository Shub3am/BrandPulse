// The bp-onboarder client: a website in, a draft BrandProfile back, unreshaped.
//
// The profile that comes back is unconfirmed and carries Version 1, per
// CONTRACTS §2. Confirming it is a write, and writes live in registry.ts.
//
// This file makes no decision about what happens when the agent is down. That
// belongs to routes/brands.ts, which is the only place that knows a brand can
// still be created from the words its owner typed.

import { callAgent } from "./a2a.js";
import type { BrandProfile } from "./contracts.js";
import type { OnboardRequest } from "./envelopes.js";

const AGENT = "bp-onboarder";
const ARTIFACT = "BrandProfile";

/** What a route is allowed to depend on. The test stub implements this too. */
export interface Onboarder {
  onboard(input: OnboardRequest): Promise<BrandProfile>;
}

export function createOnboarder(baseUrl: string, timeoutMs: number): Onboarder {
  return {
    onboard: (input) => callAgent<BrandProfile>(AGENT, baseUrl, input, ARTIFACT, timeoutMs),
  };
}
