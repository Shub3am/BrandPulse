# Track B6: the product dashboard and the BFF it talks to

**Branch:** `track/b6-web`
**Owns:** `web/`, `bff/`
**Blocked by:** nothing for `web/`. `bff/` needs B4's `bp-orchestrator` and a
deployed URL from B5. Everything else you build against the contract, not
against another track's code.

Read [PLAN.md](../../PLAN.md), [docs/CONTRACTS.md](../CONTRACTS.md) and
[web/CLAUDE.md](../../web/CLAUDE.md). The module doc's boundaries are binding
on every task below.

You own the screen a customer sees, as opposed to B5's DronaHQ ops view. Two
things matter more than the rest: the dashboard must never show a number no
agent produced, and it must never present synthetic data as real.

---

## State of play

`web/` already exists and already builds. It renders the full dashboard against
`web/lib/demoData.ts`: hero, stat row, live alert with its evidence table,
mention stream, topic list, drafted reply, and a run report showing what Nasiko,
Anakin and DronaHQ each did. Verified working: `npx next build` compiles and the
page serves.

**B6 does not rebuild any of that.** B6 makes it real and adds the backend.
`bff/` does not exist yet; Task 1 creates it.

---

## What this track owns

`web/`, the Next.js 16 App Router dashboard (React 19, TypeScript, plain CSS,
anakin.io theme), and `bff/`, the Fastify backend-for-frontend that is the
dashboard's only backend.

## What it must not know about

- **Business rules.** Nothing in `web/` decides whether an alert should fire,
  what a sentiment score means, or how share of voice is computed. A rule that
  runs in the browser is a rule invisible to `nasiko observe`. `lib/format.ts`
  is display formatting and nothing else.
- **Posting.** Approving a draft is a handoff to DronaHQ. There is no send path
  in this repo and there must not be one, not even behind a flag.
- **Raw colour.** Components reference CSS custom properties from
  `app/globals.css`, never a hex literal, so a re-theme stays one file.
- **Any agent, any database, any Anakin call.** `web/` talks to the BFF and to
  nothing else. The BFF is the only thing in this track that opens a socket to
  Postgres or speaks A2A.
- **Deriving a figure in the BFF.** The BFF passes agent artifacts through
  unreshaped. If the dashboard needs a number, an agent produces it.

---

## Task 1: the `bff/` skeleton

**Files:** `bff/package.json`, `bff/tsconfig.json`, `bff/src/server.ts`,
`bff/src/env.ts`, `bff/src/orchestrator.ts`, `bff/src/db.ts`,
`bff/src/routes/pulse.ts`, `bff/src/routes/mentions.ts`,
`bff/src/routes/alerts.ts`, `bff/src/routes/runs.ts`, `bff/CLAUDE.md`

The BFF has exactly three jobs: call `bp-orchestrator` over A2A, read Postgres
for history, and hand the artifacts to the browser unchanged.

- [ ] Fastify with TypeScript, one route file per resource, registered as
      plugins from `server.ts`. `server.ts` wires and listens, it does not hold
      a query or a fetch.
- [ ] **Pass agent artifacts through unreshaped.** Unwrapping the A2A envelope
      to get at the `application/json` artifact body is transport, and is
      allowed. Renaming a field, flattening a nested object, rounding a float or
      summing a list is not. The browser receives the same JSON the agent
      emitted.
- [ ] **The BFF computes no derived number.** Not a total, not a percentage,
      not a delta, not a count that the payload does not already carry. If the
      dashboard needs a figure and no agent produces it, that is a contract
      question for B1 in `HACKATHON_NOTES.md`, not a line of TypeScript here.
- [ ] Routes:
      `GET /api/brands/:id/pulse`, `GET /api/brands/:id/mentions`,
      `GET /api/brands/:id/alerts`, `GET /api/runs/:id`, `POST /api/runs`.
- [ ] `POST /api/runs` is the only write. It sends `RunInput`
      (`{brand_id, trigger: "on_demand"}`, per CONTRACTS §2) to `bp-orchestrator`
      and returns the `RunRecord` artifact. It is what the "Run now" button in
      `app/layout.tsx` calls.
- [ ] `orchestrator.ts` is the only file that knows the agent's URL, read from
      an env var. No route file contains a hostname.
- [ ] `db.ts` opens a read-only pool and holds the history queries. Reads only:
      the agents own every write to Postgres and the BFF must not insert, update
      or delete a row.
- [ ] **`errors` survives the trip.** Every agent output carries an `errors`
      array and a partial result. The BFF forwards both. It never drops the
      array to make a response look clean, and it never turns a populated
      `errors` into an HTTP 500 when there is partial data to show.
- [ ] Set a request timeout on the A2A call and on the pool. A hung
      orchestrator must surface as a timed-out route, not a hung page.
- [ ] CORS allows the dashboard origin and nothing else.
- [ ] `bff/CLAUDE.md` covering what the module owns, what it must not know
      about, its routes as entry points, and the pass-through invariant.
- [ ] Commit.

**Wrinkle you will hit:** `internal/models/agentio.go` does not exist on this
branch. `MentionBatch`, `AlertSet`, `TopicSet` and `BaselineStats` are specified
in CONTRACTS §2 but have no Go source yet, so the envelope shapes are not
machine-checkable today. Raise it in `HACKATHON_NOTES.md` and type the BFF
against CONTRACTS §2. Do not invent a shape and do not reshape one to fit.

## Task 2: stop `web/lib/types.ts` drifting from `models.go`

**Files:** `web/lib/types.ts`, `web/scripts/checkTypesParity.mjs`,
`web/package.json`

`web/lib/types.ts` is a hand-maintained mirror of `internal/models/models.go`.
TypeScript cannot catch a drift, because the data arrives as JSON at runtime: a
renamed `json` tag renders `undefined` and nothing fails. The choice is to
generate the file from the Go structs, or to keep it hand-maintained and add a
check that fails the build on drift.

**Recommendation: keep it hand-maintained, add the check.** A generator would
have to translate `time.Time` to `string`, `*float64` to `number | undefined`,
`map[SentimentLabel]int` to a partial record, and then be hand-patched for
`ReplyDraft`, which is the one type whose invariant actually matters. It also
has nothing to read for the envelope types until `agentio.go` lands. A generator
you have to correct by hand for the most important type is a generator you
cannot trust, and it buys a build step and a codegen dependency in a repo whose
only other TypeScript is the BFF. The file is 183 lines and changes only on
`main`, through B1. A drift check is cheaper, and it names the field that
drifted, which a generator never does.

- [ ] `web/scripts/checkTypesParity.mjs`: extract every `json:"..."` tag per
      struct from `internal/models/models.go`, extract every field name per
      interface from `web/lib/types.ts`, and diff the two sets. Exit non-zero
      naming the struct and the field on any mismatch.
- [ ] **Special-case `ReplyDraft`.** The Go struct has no
      `RequiresHumanApproval` field at all; `MarshalJSON` emits
      `requires_human_approval: true` unconditionally, so the field exists on
      the wire and not in the struct. The checker carries exactly one named
      divergence entry for it, with the reason in a comment. It is not an
      allowlist that grows: a second entry is a conversation with B1, not an
      edit to this file.
- [ ] Strip the `,omitempty` suffix before comparing, and treat an
      `omitempty` Go field as the optional `?` form in TypeScript.
- [ ] Wire it as `npm run check:types` and make `npm run build` depend on it,
      so a drift fails the same command that typechecks.
- [ ] **Add the two missing types while you are here.** `ShareOfVoice` and
      `DailyBrief` exist in `models.go` and are absent from `types.ts`. The
      `/pulse` route returns both. This is the drift the checker exists to
      catch, and it is already live.
- [ ] Commit.

## Task 3: swap demo data for live fetches

**Files:** `web/app/page.tsx`, `web/lib/pulse.ts`

Per `web/CLAUDE.md`, going live is one file: replace the `@/lib/demoData`
imports in `app/page.tsx` with fetches against the BFF. Nothing else changes,
which is the whole reason no component computes a derived value. Do not take
this task as licence to restructure a component.

- [ ] `web/lib/pulse.ts` holds the fetch calls against the BFF, one function per
      route, each returning the artifact typed against `lib/types.ts`. No React,
      no formatting, no derived values.
- [ ] `app/page.tsx` awaits those functions and passes the results to the same
      six components with the same props. The component tree does not change
      shape.
- [ ] The BFF base URL comes from an env var read in `lib/pulse.ts`. No
      component and no other file contains a URL.
- [ ] `grep -r demoData web/app/` returns nothing when this task is done. That
      is the definition, not a nice-to-have.
- [ ] **`PlatformPanel` has a prop with no wire field behind it.**
      `secondsToWhatsapp` is fed today by `TIME_TO_WHATSAPP_SECONDS = 107` in
      `demoData.ts`, and `RunRecord` carries no such field. "Time from mention
      to WhatsApp" is also still blank in the `HACKATHON_NOTES.md` numbers
      table, so it is a figure nobody has measured. Either B4 measures it and B1
      adds the field to `RunRecord` on `main`, or the row comes out of the
      panel. Do not keep rendering the literal against live data: that is a
      fabricated measurement sitting inside the panel whose job is honest
      reporting.
- [ ] `demoData.ts` stays in the tree, unimported by `app/`. Task 5 uses it and
      the pitch needs a fallback if the stage network dies.
- [ ] Commit.

## Task 4: empty, loading and error states

**Files:** `web/app/page.tsx`, `web/app/loading.tsx`, `web/app/error.tsx`,
`web/components/RunNotice.tsx`, `web/components/TopicList.tsx`,
`web/app/globals.css`

The current page assumes the data is there. A real run returns zero alerts, or
zero topics, or fails partway with `errors` populated. Every agent output
carries an `errors` array and the UI ignores all of them today.

- [ ] **Partial failure renders as partial data plus a visible note.** Never a
      blank panel, and never a fabricated number standing in for a missing one.
      `RunRecord.status === "partial"`, a non-empty `errors`, a non-empty
      `sources_skipped` or a set `degraded_reason` each produce a note the user
      can read.
- [ ] `RunNotice.tsx` renders that note. It reads the fields and prints them. It
      does not decide what counts as bad: severity is the agent's word, not the
      component's.
- [ ] Zero alerts is a success state, not an empty one. Say that no rule fired
      in this window, and say which window. Do not render an empty card.
- [ ] Zero topics and zero mentions get their own sentences. A run that
      collected nothing is a real outcome the run report should explain.
- [ ] Loading shows the panel frames and no values. A skeleton that renders a
      placeholder digit is a fabricated number for as long as it is on screen.
- [ ] `error.tsx` covers the BFF being unreachable: say the dashboard could not
      reach its backend, and say it plainly. Do not fall back to demo data
      silently, ever.
- [ ] **Fix the divide-by-zero in `TopicList`.** Line 39 computes
      `(count / topic.size) * 100` for the mix bar. A topic with `size: 0`
      yields `NaN%`, which the demo data never produces and a real run can.
- [ ] Commit.

## Task 5: keep the synthetic-data labelling honest

**Files:** `web/app/layout.tsx`, `web/app/page.tsx`, `web/lib/pulse.ts`,
`web/components/DataSourceBadge.tsx`

The "demo data" pill in the top bar and the footnote on the page are
load-bearing under the repo rule that no figure is ever presented as real when
it is not. They have to disappear when the data is real and reappear when it is
not, and that has to follow the data, not a flag someone forgets to flip.

- [ ] The badge state is **derived from the payload**, from two facts: which
      source answered the fetch, and whether any rendered mention carries
      `raw.synthetic === true`. B4's `demo/inject_crisis` marks every injected
      mention that way, so a live run holding an injected crisis is still
      labelled. Reading a label off the payload is not a business rule: it
      decides nothing, it repeats what the producer said.
- [ ] `lib/pulse.ts` returns the source alongside the data. The badge reads
      that value. There is no `IS_DEMO` constant anywhere in the tree when this
      task is done.
- [ ] **The pill currently lives in `app/layout.tsx`, which receives no data,**
      so it cannot know the source. Move it into `DataSourceBadge.tsx` rendered
      from the page, and leave `layout.tsx` holding chrome only, as its own
      header comment says it does.
- [ ] The footnote follows the same value. Live data means no footnote.
      Synthetic or injected data means the footnote names which it is: a
      fictional demo brand and an injection into a real run are different
      claims and must not share a sentence.
- [ ] Commit.

## Task 6: deploy `web/` and `bff/`

**Files:** `.github/workflows/web.yml`, `bff/Dockerfile`, `web/Dockerfile`

- [ ] **`.github/workflows/` is empty.** There is no pipeline to deploy through
      yet. Decide with the repo owner before writing one: this is the only task
      in the track that creates repo-wide infrastructure, and it is not B6's to
      assume.
- [ ] Build both images. `npm run build` typechecks `web/`, so a failing
      typecheck is a failing build, not a warning.
- [ ] `npm run check:types` from Task 2 runs in the pipeline. A drift between
      `models.go` and `types.ts` fails CI, which is the entire point of writing
      the checker.
- [ ] Coordinate the deployed agent URLs with B5 through
      `HACKATHON_NOTES.md`. The orchestrator URL is a BFF env var, injected at
      deploy time, and it is not committed to the repo.
- [ ] No API key, no `DATABASE_URL` and no agent URL in the browser bundle.
      Every one of them is read server-side, in the BFF or in a server
      component.
- [ ] Commit.

---

## Definition of done

```bash
cd web && npm run check:types && npm run build
cd ../bff && npm run build && npm test
grep -r demoData web/app/ ; echo "grep exit: $?"
curl -s localhost:8080/api/brands/lumeo/pulse | head -40
curl -s -X POST localhost:8080/api/runs -H 'content-type: application/json' \
  -d '{"brand_id":"lumeo","trigger":"on_demand"}' | head -40
```

Green, the `grep` exiting 1 with no output, and both `curl` responses carrying
artifact field names that match `internal/models/models.go`. Paste all of it
into the PR, including the `grep` exit code and the raw JSON. A screenshot of
the dashboard is not evidence that the fetch is live: the JSON is.
