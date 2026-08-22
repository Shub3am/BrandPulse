// GET /api/brands and POST /api/brands.
//
// POST is the second write path in the BFF and the only one that writes a row
// itself. It asks bp-onboarder to turn a website into a keyword set, then
// persists the brand and a confirmed profile, because submitting this form is
// the confirmation step CONTRACTS §2 describes.
//
// An onboarder outage must not block a brand. bp-onboarder crawls a site with
// Anakin and asks an LLM for keywords, both of which can be down, slow or out
// of credits on the day someone is trying to use this product. When the call
// fails the brand is still created from the keywords its owner typed, and the
// reply names the failure so the screen can say where the profile came from.
// A 502 here would mean a brand cannot be added while one of nine agents is
// unwell, and the keywords the owner already typed were good enough to run.

import type { FastifyInstance, FastifyReply, FastifyRequest } from "fastify";
import type { BrandProfile, Source } from "../contracts.js";
import type { History } from "../db.js";
import type { OnboardRequest } from "../envelopes.js";
import type { Onboarder } from "../onboarder.js";
import type { Registry } from "../registry.js";
import { BadRequestError } from "./params.js";

/**
 * The ten sources 001_init.sql declares, written onto every new profile.
 *
 * Not a preference. bp-onboarder returns a profile with an empty sources list
 * and the orchestrator collects only what the profile names, so a profile
 * stored with the agent's own empty list produces a run that reaches nothing.
 * The orchestrator ranks these by yesterday's yield and drops the worst, so
 * enabling all ten is a starting point it narrows, not a spending decision.
 */
const ALL_SOURCES: Source[] = [
  "x", "reddit", "youtube", "news", "playstore",
  "appstore", "amazon", "flipkart", "instagram", "web",
];

/** A first profile is version 1. A second version is an edit, which has no route. */
const FIRST_VERSION = 1;

/** Enough random suffix that two brands called Acme never collide. */
const ID_SUFFIX_CHARS = 4;
const ID_SLUG_CHARS = 24;

interface NewBrandInput {
  name: string;
  website: string;
  keywords: string[];
  competitors: string[];
}

export interface CreatedBrand {
  brand_id: string;
  /** Present only when bp-onboarder could not be reached or refused the site. */
  onboarder_error?: string;
}

type CreateBrandRequest = FastifyRequest<{ Body: unknown }>;

export function registerBrandRoutes(
  app: FastifyInstance,
  history: History,
  registry: Registry,
  onboarder: Onboarder,
): void {
  app.get("/api/brands", async () => history.listBrands());

  app.post("/api/brands", async (request: CreateBrandRequest, reply: FastifyReply) => {
    const input = newBrandFrom(request.body);
    const brandId = brandIdFor(input.name);

    const drafted = await draftProfile(onboarder, brandId, input);
    await registry.createBrand(profileToStore(brandId, input, drafted.profile));

    const created: CreatedBrand = { brand_id: brandId };
    if (drafted.error) created.onboarder_error = drafted.error;
    return reply.code(201).send(created);
  });
}

/**
 * Asks bp-onboarder for a profile and reports rather than throws when it
 * cannot. The caller needs both halves: the profile if there is one, and the
 * sentence to show the owner if there is not.
 */
async function draftProfile(
  onboarder: Onboarder,
  brandId: string,
  input: NewBrandInput,
): Promise<{ profile: BrandProfile | null; error?: string }> {
  const request: OnboardRequest = {
    brand_id: brandId,
    name: input.name,
    website: input.website,
    competitors: input.competitors,
  };
  try {
    return { profile: await onboarder.onboard(request) };
  } catch (cause) {
    return { profile: null, error: cause instanceof Error ? cause.message : String(cause) };
  }
}

/**
 * Composes the row that gets stored out of what the owner typed and what the
 * agent found. The owner's own keywords are kept either way: they typed them
 * on a form that asked for them, and dropping them because an agent also had
 * an opinion is the form lying about what it does.
 */
function profileToStore(
  brandId: string,
  input: NewBrandInput,
  drafted: BrandProfile | null,
): BrandProfile {
  return {
    brand_id: brandId,
    name: input.name,
    website: input.website,
    keywords: mergeTerms(input.keywords, drafted?.keywords ?? []),
    hashtags: drafted?.hashtags ?? [],
    products: drafted?.products ?? [],
    competitors: mergeTerms(input.competitors, drafted?.competitors ?? []),
    sources: ALL_SOURCES,
    negative_keywords: drafted?.negative_keywords ?? [],
    source_handles: drafted?.source_handles ?? {},
    // Empty strings, not a tone this file made up. bp-onboarder returns the
    // product's own default voice, and when it could not answer, the honest
    // record is that nobody has said how this brand sounds yet.
    voice: drafted?.voice ?? { tone: "", language: "", do_not_say: [] },
    version: FIRST_VERSION,
  };
}

/** Case-insensitive union, typed terms first, because every term is a paid query. */
function mergeTerms(typed: string[], found: string[]): string[] {
  const seen = new Set<string>();
  const merged: string[] = [];
  for (const term of [...typed, ...found]) {
    const key = term.toLowerCase();
    if (seen.has(key)) continue;
    seen.add(key);
    merged.push(term);
  }
  return merged;
}

/**
 * A readable id with a random tail. Readable because it shows on the dashboard
 * whenever a brand has no profile to name it, and random because two brands
 * called Acme must not race each other into one primary key.
 */
function brandIdFor(name: string): string {
  const slug = name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "_")
    .replace(/^_+|_+$/g, "")
    .slice(0, ID_SLUG_CHARS);
  const suffix = crypto.randomUUID().replace(/-/g, "").slice(0, ID_SUFFIX_CHARS);
  return slug ? `brd_${slug}_${suffix}` : `brd_${suffix}`;
}

function newBrandFrom(body: unknown): NewBrandInput {
  if (typeof body !== "object" || body === null) {
    throw new BadRequestError("body must be a JSON object");
  }
  const fields = body as Record<string, unknown>;

  const name = fields.name;
  if (typeof name !== "string" || name.trim() === "") {
    throw new BadRequestError("name is required and must be a non-empty string");
  }

  const website = fields.website;
  if (typeof website !== "string" || website.trim() === "") {
    throw new BadRequestError("website is required and must be a non-empty string");
  }

  return {
    name: name.trim(),
    website: httpUrl(website.trim()),
    keywords: termList(fields.keywords, "keywords"),
    competitors: termList(fields.competitors, "competitors"),
  };
}

/**
 * bp-onboarder hands this straight to Anakin's crawler, so a string that is not
 * an http URL is a 400 here rather than a 502 from an agent thirty seconds
 * later. Only http and https: a file: or a data: url is an agent being asked to
 * read something that is not a website.
 */
function httpUrl(raw: string): string {
  let parsed: URL;
  try {
    parsed = new URL(raw);
  } catch {
    throw new BadRequestError(`website must be a URL including the scheme, got ${JSON.stringify(raw)}`);
  }
  if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
    throw new BadRequestError(`website must be http or https, got ${JSON.stringify(parsed.protocol)}`);
  }
  return raw;
}

function termList(value: unknown, field: string): string[] {
  if (value === undefined || value === null) return [];
  if (!Array.isArray(value)) {
    throw new BadRequestError(`${field} must be an array of strings when present`);
  }
  const terms: string[] = [];
  for (const entry of value) {
    if (typeof entry !== "string") {
      throw new BadRequestError(`${field} must be an array of strings when present`);
    }
    const trimmed = entry.trim();
    if (trimmed !== "") terms.push(trimmed);
  }
  return terms;
}
