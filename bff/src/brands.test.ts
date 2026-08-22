// Proves POST /api/brands against a fake registry and a stub onboarder.
//
// The behaviours worth a test here are the ones a reviewer cannot see by
// reading the handler: that a brand is still created when bp-onboarder is down,
// that the profile written is confirmed and carries sources the orchestrator
// can collect from, and that bad input is refused before anything is written.

import assert from "node:assert/strict";
import { test } from "node:test";
import { AgentError } from "./a2a.js";
import { buildApp } from "./app.js";
import type { BrandProfile, EnrichedMention, ReplyDraft, Topic } from "./contracts.js";
import type { History } from "./db.js";
import type { Onboarder } from "./onboarder.js";
import type { Orchestrator } from "./orchestrator.js";
import type { Registry } from "./registry.js";
import type { BrandSummary } from "./rows.js";

const DASHBOARD_ORIGIN = "http://localhost:3000";

const DRAFTED: BrandProfile = {
  brand_id: "replaced_by_the_route",
  name: "Lumeo",
  website: "https://lumeo.example.in",
  keywords: ["lumeo", "lumeo serum"],
  hashtags: ["#lumeo"],
  products: ["Lumeo Vitamin C Serum"],
  competitors: ["minimalist"],
  sources: [],
  negative_keywords: ["lumeo motors"],
  source_handles: { x: "@lumeo" },
  voice: { tone: "warm", language: "English", do_not_say: ["we guarantee"] },
  version: 1,
};

function fakeHistory(brands: BrandSummary[] = []): History {
  return {
    listBrands: async () => brands,
    latestBrandProfile: async () => null,
    latestRun: async () => null,
    runById: async () => null,
    latestBrief: async () => null,
    recentAlerts: async () => [],
    recentTopics: async () => [] as Topic[],
    recentMentions: async () => [] as EnrichedMention[],
    recentDrafts: async () => [] as ReplyDraft[],
    close: async () => {},
  };
}

/** Records what the route tried to write so a test can assert on the row. */
function recordingRegistry(): Registry & { written: BrandProfile[] } {
  const written: BrandProfile[] = [];
  return {
    written,
    createBrand: async (profile) => {
      written.push(profile);
    },
    close: async () => {},
  };
}

function stubOnboarder(overrides: Partial<Onboarder> = {}): Onboarder {
  return { onboard: async () => DRAFTED, ...overrides };
}

const silentOrchestrator: Orchestrator = {
  startRun: async () => {
    throw new Error("this test must not start a run");
  },
};

function appWith(
  history: History,
  registry: Registry,
  onboarder: Onboarder = stubOnboarder(),
) {
  return buildApp({
    history,
    registry,
    orchestrator: silentOrchestrator,
    onboarder,
    dashboardOrigin: DASHBOARD_ORIGIN,
    logger: false,
  });
}

test("GET /api/brands returns the brands the picker needs and nothing else", async () => {
  const brands: BrandSummary[] = [
    { id: "brd_demo", name: "Suncoast", website: "https://suncoast.example.in" },
    { id: "brd_lumeo_ab12", name: "Lumeo" },
  ];
  const app = appWith(fakeHistory(brands), recordingRegistry());

  const response = await app.inject({ method: "GET", url: "/api/brands" });

  assert.equal(response.statusCode, 200);
  assert.deepEqual(response.json(), brands);
  await app.close();
});

test("a created brand merges the owner's keywords with the onboarder's", async () => {
  const registry = recordingRegistry();
  const app = appWith(fakeHistory(), registry);

  const response = await app.inject({
    method: "POST",
    url: "/api/brands",
    payload: {
      name: "Lumeo",
      website: "https://lumeo.example.in",
      keywords: ["Lumeo", "lumeo india"],
      competitors: ["Minimalist"],
    },
  });

  assert.equal(response.statusCode, 201);
  const created = response.json();
  assert.match(created.brand_id, /^brd_lumeo_[0-9a-f]{4}$/);
  assert.equal(created.onboarder_error, undefined);

  assert.equal(registry.written.length, 1);
  const stored = registry.written[0]!;
  assert.equal(stored.brand_id, created.brand_id);
  // The owner typed these, so they lead, and "Lumeo" is not stored twice
  // because the agent happened to lowercase it.
  assert.deepEqual(stored.keywords, ["Lumeo", "lumeo india", "lumeo serum"]);
  assert.deepEqual(stored.competitors, ["Minimalist"]);
  assert.deepEqual(stored.products, ["Lumeo Vitamin C Serum"]);
  assert.equal(stored.version, 1);
  await app.close();
});

test("the stored profile names sources, or the orchestrator would collect nothing", async () => {
  const registry = recordingRegistry();
  const app = appWith(fakeHistory(), registry);

  await app.inject({
    method: "POST",
    url: "/api/brands",
    payload: { name: "Lumeo", website: "https://lumeo.example.in" },
  });

  // bp-onboarder returns an empty sources list and the orchestrator collects
  // only what the profile names, so an empty list here is a brand that can
  // never produce a mention.
  assert.deepEqual(registry.written[0]!.sources, [
    "x", "reddit", "youtube", "news", "playstore",
    "appstore", "amazon", "flipkart", "instagram", "web",
  ]);
  await app.close();
});

test("an onboarder outage still creates the brand, and says that it did", async () => {
  const registry = recordingRegistry();
  const app = appWith(
    fakeHistory(),
    registry,
    stubOnboarder({
      onboard: async () => {
        throw new AgentError("bp-onboarder is unreachable: ECONNREFUSED 127.0.0.1:8001", 502);
      },
    }),
  );

  const response = await app.inject({
    method: "POST",
    url: "/api/brands",
    payload: { name: "Lumeo", website: "https://lumeo.example.in", keywords: ["lumeo"] },
  });

  assert.equal(response.statusCode, 201);
  assert.match(response.json().onboarder_error, /bp-onboarder is unreachable/);
  assert.deepEqual(registry.written[0]!.keywords, ["lumeo"]);
  // Nothing invented in place of what the agent did not say.
  assert.deepEqual(registry.written[0]!.products, []);
  assert.deepEqual(registry.written[0]!.voice, { tone: "", language: "", do_not_say: [] });
  await app.close();
});

test("a missing name is a 400 and nothing is written", async () => {
  const registry = recordingRegistry();
  const app = appWith(fakeHistory(), registry);

  const response = await app.inject({
    method: "POST",
    url: "/api/brands",
    payload: { website: "https://lumeo.example.in" },
  });

  assert.equal(response.statusCode, 400);
  assert.match(response.json().error, /name is required/);
  assert.equal(registry.written.length, 0);
  await app.close();
});

test("a missing website is a 400, because there is nothing for the agent to map", async () => {
  const app = appWith(fakeHistory(), recordingRegistry());

  const response = await app.inject({
    method: "POST",
    url: "/api/brands",
    payload: { name: "Lumeo" },
  });

  assert.equal(response.statusCode, 400);
  assert.match(response.json().error, /website is required/);
  await app.close();
});

test("a website without a scheme is a 400 here rather than a 502 from the agent", async () => {
  const app = appWith(fakeHistory(), recordingRegistry());

  const response = await app.inject({
    method: "POST",
    url: "/api/brands",
    payload: { name: "Lumeo", website: "lumeo.example.in" },
  });

  assert.equal(response.statusCode, 400);
  assert.match(response.json().error, /website must be a URL including the scheme/);
  await app.close();
});

test("keywords that are not a list of strings are a 400", async () => {
  const app = appWith(fakeHistory(), recordingRegistry());

  const response = await app.inject({
    method: "POST",
    url: "/api/brands",
    payload: { name: "Lumeo", website: "https://lumeo.example.in", keywords: [1, 2] },
  });

  assert.equal(response.statusCode, 400);
  assert.match(response.json().error, /keywords must be an array of strings/);
  await app.close();
});

test("the onboarder is asked for the brand id the route just minted", async () => {
  let seenBrandId = "";
  const registry = recordingRegistry();
  const app = appWith(
    fakeHistory(),
    registry,
    stubOnboarder({
      onboard: async (input) => {
        seenBrandId = input.brand_id;
        return DRAFTED;
      },
    }),
  );

  const response = await app.inject({
    method: "POST",
    url: "/api/brands",
    payload: { name: "Lumeo", website: "https://lumeo.example.in" },
  });

  // The agent's own brand_id is never trusted back: the row is keyed on the id
  // this route minted, and DRAFTED deliberately carries a different one.
  assert.equal(seenBrandId, response.json().brand_id);
  assert.equal(registry.written[0]!.brand_id, response.json().brand_id);
  await app.close();
});
