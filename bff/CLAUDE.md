# bff/: the dashboard's only backend

Fastify 5, TypeScript, `pg`. No ORM, no validation library, no GraphQL. Five
read routes and two write routes.

## What this module owns

Being the only thing `web/` talks to. It reads brand history out of Postgres,
starts an on-demand run through `bp-orchestrator`, onboards a new brand through
`bp-onboarder`, and hands the browser the agents' own artifact JSON without
reshaping it.

It also owns the secrets `web/` must never see: `DATABASE_URL` and both agent
URLs. All of them are read here, server-side, from the environment.

## What it must not know about

- **What a number means.** This module computes no figure. Every value it
  returns is a column Postgres already held or a field an agent already
  produced. Renaming a key, flattening a nested object, rounding a float or
  summing a list are all the same defect: a number on the screen that no agent
  can be held to. Unwrapping the A2A envelope is transport, and is allowed.
- **Writing anything an agent produces.** The agents own every write to the
  tables a run fills. `db.ts` opens its pool with
  `default_transaction_read_only=on`, so an `INSERT` added to a read path fails
  at the Postgres server rather than in review. The single exception is
  registering a brand that does not exist yet, which no agent can do because a
  run needs the `brands` row to already be there. That write lives in
  `registry.ts`, on its own writable pool, and it touches `brands` and
  `brand_profiles` and nothing else.
- **Posting anywhere.** There is no send path in this repo and there must not be
  one. `POST /api/runs` starts a collection run; it publishes nothing.
- **Which alert is the important one.** The dashboard renders what the `alerts`
  table holds, in `created_at` order. Severity is `bp-detector`'s output, not a
  BFF opinion.

## Entry points

| Path | Job |
|---|---|
| `src/server.ts` | Reads the env, opens the pool, listens. The only file that does. |
| `src/app.ts` | Builds the Fastify instance from collaborators it is handed. |
| `src/env.ts` | Config shape and required-variable failures. |
| `src/a2a.ts` | The A2A transport. One `callAgent`, shared by both agent clients. |
| `src/orchestrator.ts` | `bp-orchestrator`, its artifact name and its timeout. |
| `src/onboarder.ts` | `bp-onboarder`, same shape, reached only by `POST /api/brands`. |
| `src/db.ts` | The read-only pool and one method per query. |
| `src/registry.ts` | The one writable pool. Registers a brand and its profile. |
| `src/rows.ts` | Postgres row → wire shape, per CONTRACTS §3b. |
| `src/routes/*.ts` | One file per route, named for its path. |
| `src/contracts.ts` | The wire format, mirrored from `internal/models/models.go`. |
| `src/envelopes.ts` | The A2A artifact envelopes from CONTRACTS §2. |

## Invariants and gotchas

- **A2A is v1.0, not 0.x, and every difference fails silently.** The method is
  `SendMessage`, the role is `ROLE_USER`, the state is `TASK_STATE_COMPLETED`,
  the version travels as the `A2A-Version: 1.0` header, and `Part` is a
  flattened oneof with **no `kind` discriminator**. `orchestrator.test.ts` runs
  a real HTTP stub agent precisely so a guess here cannot pass.
- **A brand is still created when `bp-onboarder` is not.** `POST /api/brands`
  catches the agent's failure, writes the brand from the keywords the owner
  typed and returns them the id with `onboarder_error` set. A form that refuses
  because a crawler is down loses the one thing the person actually gave us.
- **A registered brand must be runnable the moment it is written.** That is
  three things, and each of them is a run that collects nothing if it is
  missing: `confirmed_at` is set, because `LoadProfile` in
  `agents/bp-orchestrator/store.go` requires a confirmed profile and refuses
  otherwise; `version` is 1, because the primary key is `(brand_id, version)`;
  and `sources` names the platforms, because `chooseSources` in
  `agents/bp-orchestrator/sources.go` ranks that list and nothing else.
  `bp-onboarder` returns `sources: []` every time, since `models.NewBrandProfile`
  initialises it empty, so the route supplies the same ten the seeded demo brand
  carries rather than persisting the agent's answer verbatim.
- **Every jsonb parameter is `JSON.stringify`d.** node-postgres encodes a JS
  array as a Postgres array literal, which is the wrong type for a `jsonb`
  column and fails at the server. `agents/bp-orchestrator/store.go` has the same
  helper for the same reason.
- **`src/contracts.ts` duplicates `web/lib/types.ts` on purpose.** Two Docker
  build contexts and two `rootDir`s mean neither can import the other.
  `web/scripts/checkTypesParity.mjs` checks both against `models.go`, so the
  duplication is build-enforced rather than review-enforced.
- **`internal/models/agentio.go` has landed, and `src/envelopes.ts` is still
  not checked against it.** These envelopes were transcribed from CONTRACTS §2
  while there was no Go source, and `checkTypesParity.mjs` mirrors `models.go`
  only, so this is the one wire shape here with no build-enforced parity. Where
  the two disagree, `agentio.go` wins and this file changes.
- **An absent optional field is an absent key, not a `null`.** `omitempty` means
  Go emits no key at all. `rows.ts` maps a NULL column to `undefined` so
  `JSON.stringify` drops it, and it never substitutes a zero: `rating: 0` is a
  real one-star average and `credits_used: 0` is a run that has not spent yet.
- **A populated `errors` array is not an HTTP 500.** `/pulse` gathers seven
  slots independently, so one failed query returns the other six with the
  failure named in `errors` and a 200. A 500 would blank a dashboard that still
  has a crisis alert to show.
- **`/mentions` and `/alerts` return bare arrays, not `MentionBatch` and
  `AlertSet`.** Those envelopes carry `credits_used`, `cache_hits`, `truncated`
  and `rules_evaluated`, which a Postgres read cannot know. Filling them in
  would be inventing numbers.
- **`/pulse` returns `share_of_voice: null` always.** There is no
  `share_of_voice` table and no SOV field on `RunRecord`. The figure the
  dashboard renders is `brief.numbers.share_of_voice`. Absence is not failure,
  so this adds no `errors` entry.
- **`?since=` on `/alerts` must be a `created_at` the caller received from that
  same route.** It is compared in Postgres and it is exclusive, so a browser
  clock running fast cannot skip an alert. Do not replace it with `Date.now()`.
- **CORS allows exactly one origin.** `DASHBOARD_ORIGIN`, never a list, never
  `*`. Widening it lets any page a customer has open read their mentions
  through their own browser.
- **Tests are compiled, not excluded.** `tsx` strips types without checking
  them, so leaving tests out of `tsconfig.json` means a broken test file is
  never typechecked. `Dockerfile` deletes `dist/**/*.test.*` instead.

## Who calls it

`web/` only, from server components and from the alert feed's poll. Nothing
else, and nothing calls it from the browser except that one origin.

## Run

```bash
npm install
npm run dev     # http://localhost:8080
npm run build   # tsc, no bundler
npm test        # node:test, zero network, zero Postgres, zero credits
```

Required environment: `DASHBOARD_ORIGIN`, `ORCHESTRATOR_URL`, `ONBOARDER_URL`,
`DATABASE_URL`. Optional: `PORT` (8080), `HOST`, `ORCHESTRATOR_TIMEOUT_MS`
(120000), `ONBOARDER_TIMEOUT_MS` (45000), `DATABASE_TIMEOUT_MS` (5000). Both
agent URLs are injected at deploy time and are not committed. `docker compose`
publishes only `bp-orchestrator` to the host, so a BFF running outside compose
reaches `bp-onboarder` only if that port is published too.
