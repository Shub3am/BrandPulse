// Reads and validates the BFF's configuration once, at startup.
//
// It exists so that a missing variable is a refusal to boot with a named
// variable in the message, rather than a 500 on the first request or, worse, a
// fetch against the string "undefined".
//
// This file must not read a variable that belongs in the browser bundle. Every
// value here is server-side: the orchestrator URL, the database credentials and
// the one origin allowed to call us. None of them are ever sent to a client.

export interface Config {
  port: number;
  host: string;
  /** The single browser origin CORS permits. Not a list, and never "*". */
  dashboardOrigin: string;
  /** bp-orchestrator's A2A base URL. Injected at deploy time, never committed. */
  orchestratorUrl: string;
  /** bp-onboarder's A2A base URL. Same deal, and reached only by POST /api/brands. */
  onboarderUrl: string;
  databaseUrl: string;
  orchestratorTimeoutMs: number;
  onboarderTimeoutMs: number;
  databaseTimeoutMs: number;
}

function required(name: string): string {
  const value = process.env[name];
  if (value === undefined || value.trim() === "") {
    throw new Error(
      `${name} is not set. The BFF needs it to start; see bff/CLAUDE.md for the full list.`,
    );
  }
  return value.trim();
}

function positiveInt(name: string, fallback: number): number {
  const raw = process.env[name];
  if (raw === undefined || raw.trim() === "") return fallback;
  const parsed = Number(raw);
  if (!Number.isInteger(parsed) || parsed <= 0) {
    throw new Error(`${name} must be a positive integer, got ${JSON.stringify(raw)}.`);
  }
  return parsed;
}

export function loadConfig(): Config {
  return {
    port: positiveInt("PORT", 8080),
    host: process.env.HOST?.trim() || "0.0.0.0",
    dashboardOrigin: required("DASHBOARD_ORIGIN"),
    orchestratorUrl: required("ORCHESTRATOR_URL").replace(/\/+$/, ""),
    onboarderUrl: required("ONBOARDER_URL").replace(/\/+$/, ""),
    databaseUrl: required("DATABASE_URL"),
    // A run fans out nine agents behind one call, so the ceiling is generous.
    // It is still a ceiling: without it a hung agent hangs the dashboard.
    orchestratorTimeoutMs: positiveInt("ORCHESTRATOR_TIMEOUT_MS", 120_000),
    // One agent, one site crawl, one LLM call. A person is watching a form
    // submit spin, and waiting two minutes for it is worse than falling back
    // to the keywords they typed, which POST /api/brands does on a timeout.
    onboarderTimeoutMs: positiveInt("ONBOARDER_TIMEOUT_MS", 45_000),
    databaseTimeoutMs: positiveInt("DATABASE_TIMEOUT_MS", 5_000),
  };
}
