// Proves the routes against a fake history and a stub agent.
//
// The two behaviours worth a test here are the ones a reviewer cannot see by
// reading a handler: that a partial answer stays a 200 with its failures named,
// and that CORS answers exactly one origin.

import assert from "node:assert/strict";
import { test } from "node:test";
import { buildApp } from "./app.js";
import type { Alert, EnrichedMention, ReplyDraft, RunRecord, Topic } from "./contracts.js";
import type { History } from "./db.js";
import type { Onboarder } from "./onboarder.js";
import type { Orchestrator } from "./orchestrator.js";
import type { Registry } from "./registry.js";
import { AgentError } from "./a2a.js";

const DASHBOARD_ORIGIN = "http://localhost:3000";

const RUN: RunRecord = {
  id: "run_01JFZ3P7QK8N2V5X9YB4C6D8EG",
  brand_id: "lumeo",
  kind: "on_demand",
  time_bucket: "2026-09-20T14",
  started_at: "2026-09-20T14:02:11Z",
  finished_at: "2026-09-20T14:03:58Z",
  status: "partial",
  credits_used: 37,
  tokens_used: 18422,
  cost_paise: 214.5,
  sources_attempted: ["x", "reddit", "appstore"],
  sources_skipped: ["appstore"],
  degraded_reason: "Nasiko flow guard capped fan-out at 6 concurrent collectors",
  mentions_collected: 412,
  errors: ["appstore: probe returned 403"],
};

const ALERT: Alert = {
  id: "al_01JFZ3P7QK8N2V5X9YB4C6D8EG",
  brand_id: "lumeo",
  kind: "crisis",
  severity: "critical",
  title: "Negative share tripled on reddit in one hour",
  why: "negative_share 0.61 against a 14-day mean of 0.19",
  evidence: [{ metric: "negative_share", value: 0.61, threshold: 0.34, window: "1h" }],
  sample_mentions: [],
  status: "open",
  dedupe_key: "crisis:reddit:2026-09-20T14",
  created_at: "2026-09-20T14:05:00.000Z",
};

/** Every method resolves empty unless the test overrides it. */
function fakeHistory(overrides: Partial<History> = {}): History {
  return {
    listBrands: async () => [],
    latestBrandProfile: async () => null,
    latestRun: async () => RUN,
    runById: async () => null,
    latestBrief: async () => null,
    recentAlerts: async () => [ALERT],
    recentTopics: async () => [] as Topic[],
    recentMentions: async () => [] as EnrichedMention[],
    recentDrafts: async () => [] as ReplyDraft[],
    close: async () => {},
    ...overrides,
  };
}

function fakeOrchestrator(overrides: Partial<Orchestrator> = {}): Orchestrator {
  return { startRun: async () => RUN, ...overrides };
}

function appWith(history: History, orchestrator: Orchestrator = fakeOrchestrator()) {
  return buildApp({
    history,
    registry: refusingRegistry(),
    orchestrator,
    onboarder: refusingOnboarder(),
    dashboardOrigin: DASHBOARD_ORIGIN,
    logger: false,
  });
}

/**
 * The read tests in this file never create a brand, so their collaborators for
 * that path fail loudly rather than pretending. brands.test.ts supplies real
 * fakes; a no-op here would let a route start writing without a test noticing.
 */
function refusingRegistry(): Registry {
  return {
    createBrand: async () => {
      throw new Error("this test must not write a brand");
    },
    close: async () => {},
  };
}

function refusingOnboarder(): Onboarder {
  return {
    onboard: async () => {
      throw new Error("this test must not call bp-onboarder");
    },
  };
}

test("pulse returns a named slot per artifact with no errors when every query works", async () => {
  const app = appWith(fakeHistory());
  const response = await app.inject({ method: "GET", url: "/api/brands/lumeo/pulse" });

  assert.equal(response.statusCode, 200);
  const pulse = response.json();
  assert.deepEqual(Object.keys(pulse), [
    "brand_id",
    "profile",
    "run",
    "brief",
    "share_of_voice",
    "alerts",
    "topics",
    "mentions",
    "drafts",
    "errors",
  ]);
  assert.deepEqual(pulse.errors, []);
  assert.deepEqual(pulse.run, RUN);
  assert.equal(pulse.share_of_voice, null);
  await app.close();
});

test("a failing query leaves the other panels intact and stays a 200", async () => {
  const app = appWith(
    fakeHistory({
      recentTopics: async () => {
        throw new Error("relation topic_mentions does not exist");
      },
    }),
  );
  const response = await app.inject({ method: "GET", url: "/api/brands/lumeo/pulse" });

  // A 500 here would blank a dashboard that still has a crisis alert to show.
  assert.equal(response.statusCode, 200);
  const pulse = response.json();
  assert.deepEqual(pulse.topics, []);
  assert.deepEqual(pulse.alerts, [ALERT]);
  assert.deepEqual(pulse.errors, ["topics: relation topic_mentions does not exist"]);
  await app.close();
});

test("the alert feed passes since straight through to the query", async () => {
  let seenSince: string | undefined;
  let seenLimit = 0;
  const app = appWith(
    fakeHistory({
      recentAlerts: async (_brandId, limit, since) => {
        seenSince = since;
        seenLimit = limit;
        return [];
      },
    }),
  );

  const response = await app.inject({
    method: "GET",
    url: "/api/brands/lumeo/alerts?since=2026-09-20T14:05:00.000Z&limit=5",
  });

  assert.equal(response.statusCode, 200);
  assert.equal(seenSince, "2026-09-20T14:05:00.000Z");
  assert.equal(seenLimit, 5);
  await app.close();
});

test("a limit above the ceiling is capped rather than refused", async () => {
  let seenLimit = 0;
  const app = appWith(
    fakeHistory({
      recentMentions: async (_brandId, limit) => {
        seenLimit = limit;
        return [];
      },
    }),
  );

  await app.inject({ method: "GET", url: "/api/brands/lumeo/mentions?limit=100000" });
  assert.equal(seenLimit, 200);
  await app.close();
});

test("a limit that is not a positive integer is a 400", async () => {
  const app = appWith(fakeHistory());
  const response = await app.inject({ method: "GET", url: "/api/brands/lumeo/mentions?limit=-3" });

  assert.equal(response.statusCode, 400);
  assert.match(response.json().error, /limit must be a positive integer/);
  await app.close();
});

test("a since that is not a timestamp is a 400 and never reaches Postgres", async () => {
  let queried = false;
  const app = appWith(
    fakeHistory({
      recentAlerts: async () => {
        queried = true;
        return [];
      },
    }),
  );

  const response = await app.inject({ method: "GET", url: "/api/brands/lumeo/alerts?since=yesterday" });

  assert.equal(response.statusCode, 400);
  assert.equal(queried, false);
  await app.close();
});

test("POST /api/runs forwards the run record exactly as the agent produced it", async () => {
  const app = appWith(fakeHistory(), fakeOrchestrator());
  const response = await app.inject({
    method: "POST",
    url: "/api/runs",
    payload: { brand_id: "lumeo", trigger: "on_demand" },
  });

  assert.equal(response.statusCode, 200);
  assert.deepEqual(response.json(), RUN);
  await app.close();
});

test("a partial run is a 200, because it is a real run with real mentions", async () => {
  const app = appWith(fakeHistory(), fakeOrchestrator());
  const response = await app.inject({
    method: "POST",
    url: "/api/runs",
    payload: { brand_id: "lumeo", trigger: "on_demand" },
  });

  assert.equal(response.statusCode, 200);
  assert.equal(response.json().status, "partial");
  assert.deepEqual(response.json().errors, ["appstore: probe returned 403"]);
  await app.close();
});

test("an unknown trigger is a 400 and the agent is never called", async () => {
  let called = false;
  const app = appWith(
    fakeHistory(),
    fakeOrchestrator({
      startRun: async () => {
        called = true;
        return RUN;
      },
    }),
  );

  const response = await app.inject({
    method: "POST",
    url: "/api/runs",
    payload: { brand_id: "lumeo", trigger: "whenever" },
  });

  assert.equal(response.statusCode, 400);
  assert.equal(called, false);
  await app.close();
});

test("a missing brand_id is a 400", async () => {
  const app = appWith(fakeHistory());
  const response = await app.inject({
    method: "POST",
    url: "/api/runs",
    payload: { trigger: "on_demand" },
  });

  assert.equal(response.statusCode, 400);
  assert.match(response.json().error, /brand_id is required/);
  await app.close();
});

test("an agent timeout reaches the browser as a 504 with the reason", async () => {
  const app = appWith(
    fakeHistory(),
    fakeOrchestrator({
      startRun: async () => {
        throw new AgentError("bp-orchestrator did not answer within 120000ms", 504);
      },
    }),
  );

  const response = await app.inject({
    method: "POST",
    url: "/api/runs",
    payload: { brand_id: "lumeo", trigger: "on_demand" },
  });

  assert.equal(response.statusCode, 504);
  assert.match(response.json().error, /did not answer within/);
  await app.close();
});

test("an unknown run id is a 404, not an empty object", async () => {
  const app = appWith(fakeHistory());
  const response = await app.inject({ method: "GET", url: "/api/runs/run_nope" });

  assert.equal(response.statusCode, 404);
  await app.close();
});

test("CORS answers the dashboard origin and no other", async () => {
  const app = appWith(fakeHistory());

  const allowed = await app.inject({
    method: "GET",
    url: "/api/brands/lumeo/alerts",
    headers: { origin: DASHBOARD_ORIGIN },
  });
  assert.equal(allowed.headers["access-control-allow-origin"], DASHBOARD_ORIGIN);

  const refused = await app.inject({
    method: "GET",
    url: "/api/brands/lumeo/alerts",
    headers: { origin: "https://evil.example" },
  });
  assert.equal(refused.headers["access-control-allow-origin"], undefined);

  await app.close();
});
