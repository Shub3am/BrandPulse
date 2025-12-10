# bff/ — the dashboard's only backend

Fastify 5, TypeScript, `pg`. No ORM, no validation library, no GraphQL. Four
read routes and one write route, and the write is a single A2A call.

## What this module owns

Being the only thing `web/` talks to. It reads brand history out of Postgres,
starts an on-demand run through `bp-orchestrator`, and hands the browser the
agents' own artifact JSON without reshaping it.

It also owns the two secrets `web/` must never see: `DATABASE_URL` and the
orchestrator's URL. Both are read here, server-side, from the environment.

## What it must not know about

- **What a number means.** This module computes no figure. Every value it
  returns is a column Postgres already held or a field an agent already
  produced. Renaming a key, flattening a nested object, rounding a float or
  summing a list are all the same defect: a number on the screen that no agent
  can be held to. Unwrapping the A2A envelope is transport, and is allowed.
- **Writing.** The agents own every write in this system. `db.ts` opens the pool
  with `default_transaction_read_only=on`, so an `INSERT` added here later fails
  at the Postgres server rather than in review.
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
| `src/orchestrator.ts` | The A2A client. The only file that knows an agent URL. |
| `src/db.ts` | The read-only pool and one method per query. |
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
- **`src/contracts.ts` duplicates `web/lib/types.ts` on purpose.** Two Docker
  build contexts and two `rootDir`s mean neither can import the other.
  `web/scripts/checkTypesParity.mjs` checks both against `models.go`, so the
  duplication is build-enforced rather than review-enforced.
- **`internal/models/agentio.go` does not exist.** CONTRACTS §2 specifies the
  envelopes and there is no Go source for them, so `src/envelopes.ts` is typed
  from the document, not from code, and the parity check deliberately skips it.
  See the blocker row in `HACKATHON_NOTES.md`.
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

Required environment: `DASHBOARD_ORIGIN`, `ORCHESTRATOR_URL`, `DATABASE_URL`.
Optional: `PORT` (8080), `HOST`, `ORCHESTRATOR_TIMEOUT_MS` (120000),
`DATABASE_TIMEOUT_MS` (5000). The orchestrator URL is injected at deploy time
and is not committed.
