# BrandPulse Implementation Plan

> **For agentic workers:** read this file, then your own brief in
> [docs/tracks/](docs/tracks/), then [docs/CONTRACTS.md](docs/CONTRACTS.md).
> Do not read another track's code. Coordinate only through
> [HACKATHON_NOTES.md](HACKATHON_NOTES.md).

**Goal:** Ship a live, deployed social-listening product for Indian D2C brands —
continuous public-web monitoring, topic clustering, sentiment, deterministic
crisis detection, WhatsApp brief, live dashboard — running as nine A2A agents on
Nasiko, with Anakin as the entire data layer and DronaHQ as the two front ends.

**Architecture:** Nine single-purpose A2A agents in containers on Nasiko. One
orchestrator fans out to source collectors under a flow guard, then runs a
linear enrich → cluster/score/detect → respond → brief pipeline. All external
data comes from Anakin, wrapped in a caching, deduping, budget-capped client so
CI needs zero credits. All state is Postgres. All LLM traffic goes through the
Nasiko router so spend is metered per brand. Alerting is pure statistics, never
an LLM, so every alert can show the rule that fired it.

**Tech Stack:** Go 1.27 for all nine agents (`github.com/a2aproject/a2a-go/v2`
v2.5.0, `net/http`, `pgx/v5`, `errgroup`, stdlib `testing`), Postgres 16,
TypeScript for the front end (Next.js 16 dashboard, Fastify BFF), Docker,
`nasiko deploy`, DronaHQ. Clustering and OpenTelemetry setup are hand-rolled in
`internal/cluster` and `internal/obs`; see
[docs/decisions/001-go-for-agents.md](docs/decisions/001-go-for-agents.md) for
why that trade was taken and what it costs.

**Spec:** the kickoff brief, preserved verbatim at [docs/BRIEF.md](docs/BRIEF.md).
Where verified research contradicts the brief, the correction lives in
[docs/SOURCE-STRATEGY.md](docs/SOURCE-STRATEGY.md) and
[docs/research/](docs/research/), and those win. The brief is never edited.

---

## Global Constraints

Copied from the brief. Every task inherits these.

- **Public data only.** No login-walled scraping, no ToS circumvention. Anakin
  is the only fetch path.
- **Zero-credit CI.** Every external call is fixture-backed. A test that needs
  `ANAKIN_API_KEY` is a broken test.
- **No LLM in the detector.** Alerting is deterministic and explainable, and
  every alert carries the numbers and thresholds that fired it.
- **Nothing auto-posts.** `ReplyDraft.requires_human_approval` is always true.
  There is no posting code path in this repo.
- **PII redacted before the LLM.** Public handles and URLs are stored; emails
  and phone numbers found in mention text are stripped by `redact.PII` before
  any prompt.
- **LLM calls only via `OPENAI_BASE_URL`** (Nasiko router). No direct provider
  SDK configuration in any agent.
- **No raw API keys in containers.** Secrets are injected by Nasiko at runtime.
- **Typed JSON artifacts only.** Every agent returns one
  `application/json` artifact that is an `internal/models` struct, so DronaHQ
  binds to it without a translation layer.
- **Zero values are the hazard.** Go has no field defaults. Construct domain
  types through their `New*` constructor and call `Validate()` before
  persisting. See [internal/models/CLAUDE.md](internal/models/CLAUDE.md).
- **Anakin free credits: 300 total.** They are spent once, in Phase 2, on the
  demo brand + two competitors, in `record` mode. Everything after that replays
  fixtures. One live call is reserved for the stage demo.
- **Cost target:** under ₹15 per brand-day at 8 sources. `eval/` must report the
  real number.
- **Commit small.** One logical change per commit, conventional-commit prefix,
  no Claude co-author line.

---

## File Structure

| Path | Responsibility | Owner |
|---|---|---|
| `internal/models/` | Wire format for every agent. Pure schema, no I/O. | B1 |
| `internal/anakin/` | Anakin HTTP client: cache, dedupe, budget, fixture modes. | B1 |
| `internal/llm/` | Nasiko LLM router client: `ChatJSON`, `Embed`. | B1 |
| `internal/db/` | `pgxpool` and query helpers, migrations runner. | B1 |
| `internal/stats/` | Baselines, z-scores, negative-share windows, time buckets. | B1 |
| `internal/redact/` | PII stripping before prompts. | B1 |
| `internal/cluster/` | TF-IDF + agglomerative clustering, hand-rolled. | B3 |
| `internal/obs/` | OpenTelemetry self-instrumentation for Go containers. | B1 |
| `internal/a2a/` | A2A server wiring and the one artifact envelope. | B1 |
| `internal/anakin/sources/*.go` | One adapter per source; Anakin response → `Mention`. | B2 |
| `internal/prompts/*.md` | One prompt per LLM-using agent, `//go:embed`ed. | owner of that agent |
| `agents/bp-collector/` | Fan-in for one source. No LLM. | B2 |
| `agents/bp-onboarder/` | Site crawl → keyword set. | B2 |
| `agents/bp-enricher/` | Batched sentiment/intent/aspect classifier. | B3 |
| `agents/bp-clusterer/` | Embed + cluster + label topics. | B3 |
| `agents/bp-sov/` | Share-of-voice counting. | B3 |
| `agents/bp-detector/` | Five deterministic alert rules. | B4 |
| `agents/bp-responder/` | Guardrailed reply drafting. | B4 |
| `agents/bp-briefer/` | Daily/weekly brief, markdown + WhatsApp short. | B4 |
| `agents/bp-orchestrator/` | Pipeline, idempotency, flow-guard-aware fan-out. | B4 |
| `db/migrations/*.sql` | Schema. Additive only after Phase 1. | B1 |
| `fixtures/` | Recorded Anakin responses + 300 labelled mentions. | B2 records, B3 labels |
| `demo/` | Seed, replay script, crisis injection. | B4 |
| `eval/` | Classifier accuracy + real cost per brand-day. | B3 |
| `dronahq/` | Exported WhatsApp agent + dashboard app, screenshots. | B5 |
| `web/`, `bff/` | Next.js product dashboard and the Fastify BFF it calls. | B6 |
| `docs/` | Architecture, setup, pricing, contracts, track briefs. | B5 (B1 owns CONTRACTS) |

---

## Phases

Tracks run in parallel **within** a phase. A phase gate is a real check, not a
vibe: the listed command must pass on `main` before the next phase opens.

### Phase 0 — Contracts (main session, before any worktree exists)

Locks the interfaces so six agents can build without talking. **Done when**
`docs/CONTRACTS.md`, `internal/models/` and `db/migrations/001_init.sql` are
committed to `main` and `go build ./... && go vet ./...` succeeds.

This phase is already complete when you read this. The tag is
`phase0-contracts-go`; the earlier `phase0-contracts` tag is the superseded
Python freeze and no track branches from it.

### Phase 1 — Foundation (B1 alone, ~60 min)

B1 finishes `internal/` and `docker-compose.yml`. Every other track is blocked
on the client stubs, so B1 lands the `anakin`, `llm` and `db` packages with
**working `replay` mode and passing unit tests** before anything else.

Because Go will not compile against a package that does not exist, B1's first
commit is the **whole import surface as signatures with `panic("not
implemented")` bodies**, pushed to `main` inside the first twenty minutes.
That unblocks five tracks before any of it works. This is the one place a stub
is correct rather than lazy: it is a compile target, not a fake test pass.

**Gate:** `docker compose up -d postgres && go test ./internal/... -v` green,
and `BP_FIXTURE_MODE=replay` returns a canned response for every one of the
five Anakin methods.

### Phase 2 — Data acquisition and the credit spend (B2 alone for the spend)

B2 builds the source adapters and the collector, then runs the **one and only**
live recording session against the demo brand and two competitors. This is the
irreversible step: 300 credits, spent once.

**Before spending:** the adapter for a source must pass against a hand-written
fixture first. We do not discover a parsing bug with live credits.

**Gate:** `fixtures/` contains ≥ 300 mentions across ≥ 7 sources, and
`go test ./agents/bp-collector/...` passes in replay mode with zero network
calls, enforced by injecting an `*http.Client` whose `Transport` errors.

### Phase 3 — Intelligence and pipeline (B3 + B4 in parallel)

B3 turns mentions into enriched mentions, topics and share-of-voice, builds
`internal/cluster` and `eval/`. B4 builds the detector, responder, briefer and
orchestrator against the contract, using B2's fixtures.

**Gate:** `go test ./...` green across the repo; `eval/` prints real sentiment
accuracy on the labelled set and a real ₹/brand-day figure.

### Phase 4 — Deploy and front ends (B5 + B6 in parallel, with B1 on call)

B5: nine `AgentCard.json` + `Dockerfile` + `nasiko deploy`. Flow guards
configured. DronaHQ WhatsApp agent and ops dashboard built against the deployed
URLs. Copy `agents/bp-*` into the Nasiko fork and open the PR.

B6: the Fastify BFF that fronts `bp-orchestrator` and Postgres, then swaps
`web/lib/demoData` for real fetches. B6 is unblocked from Phase 0 because the
dashboard already renders against demo data; only the BFF needs Phase 3.

**Gate:** all nine agents respond to a real A2A call at their deployed URLs;
the DronaHQ dashboard renders a live run; the WhatsApp agent delivers a brief;
`web/` renders a real run with no `demoData` import left in `app/page.tsx`.

### Phase 5 — Demo hardening (all tracks, converging)

Run the full 2-minute demo end to end, three times, on the real deployment.
Fix what breaks. Record the fallback video.

**Gate:** three consecutive clean runs of `demo/run_demo.sh`.

---

## Track assignments

Each track gets a git worktree and a branch. Full briefs in `docs/tracks/`.

| Track | Branch | Owns | Blocked by |
|---|---|---|---|
| B1 | `track/b1-core` | `internal/` (except `cluster`), `db/`, `docker-compose.yml`, CI | — |
| B2 | `track/b2-collect` | `internal/anakin/sources/`, `bp-collector`, `bp-onboarder`, fixtures | B1 signature commit |
| B3 | `track/b3-intel` | `internal/cluster/`, `bp-enricher`, `bp-clusterer`, `bp-sov`, `eval/` | B1 signature commit; B2 fixtures |
| B4 | `track/b4-pipeline` | `bp-detector`, `bp-responder`, `bp-briefer`, `bp-orchestrator`, `demo/` | B1 signature commit |
| B5 | `track/b5-deploy` | AgentCards, Dockerfiles, Nasiko deploy, DronaHQ, docs, pitch | B2–B4 |
| B6 | `track/b6-web` | `web/`, `bff/` | nothing for `web/`; B4 for `bff/` |

**Integration:** each track opens a PR into `main` at its phase gate. B1 is the
integrator and resolves conflicts. Nobody merges their own track into another's.
---

## Worktrees

Six worktrees, all branched from the `phase0-contracts-go` tag so every track
starts on identical frozen interfaces.

| Track | Branch | Worktree |
|---|---|---|
| B1 | `track/b1-core` | `../brandpulse-b1-core` |
| B2 | `track/b2-collect` | `../brandpulse-b2-collect` |
| B3 | `track/b3-intel` | `../brandpulse-b3-intel` |
| B4 | `track/b4-pipeline` | `../brandpulse-b4-pipeline` |
| B5 | `track/b5-deploy` | `../brandpulse-b5-deploy` |
| B6 | `track/b6-web` | `../brandpulse-b6-web` |

Work only inside your own worktree. `git worktree list` shows them all.

**The stash stack is shared across worktrees.** Never use bare `git stash` or
`git stash pop`. Set work aside with a WIP commit instead.

Rebase onto `main` at your phase gate, then open a PR. B1 is the integrator.


---

## Risk register

The things most likely to sink this, and what we do about each.

| Risk | Mitigation |
|---|---|
| Anakin's real API shape differs from what we assumed | **Already happened.** Wire does not carry X, Instagram, Play Store or App Store reviews, and Search returns a snippet not a page body. Recorded in [docs/SOURCE-STRATEGY.md](docs/SOURCE-STRATEGY.md); seven sources ship instead of eight. Remaining unknown is the literal Wire field names, which B2 Task 1 reads live. `anakin.Client` returns `json.RawMessage` for exactly this reason, so the typed shape lives only in `internal/anakin/sources/` and a further surprise is a one-file fix. |
| Go has no sklearn, so clustering is hand-rolled | Priced in the ADR at 240–320 lines. B3 Task 1 builds `internal/cluster` **before** `bp-clusterer` and tests it on a fixed toy corpus, so the algorithm is proven separately from the agent. Fallback if quality is poor is keyword grouping; the contract does not change. |
| Nasiko auto-injects OpenTelemetry for Python only | Go agents self-instrument. B1 writes `internal/obs` once, ~150 lines, and every agent's `main()` calls `obs.Setup`. Recorded in the ADR. |
| 300 credits burn out during development | `replay` is the default mode everywhere including local dev. `record` requires an explicit env flag and is run once, by B2, with a dry-run credit estimate printed first. |
| Nasiko deploy fails late and we have nothing to show | Phase 4 starts with **one** agent deployed end to end (`bp-sov`, the simplest) before the other eight. `docker-compose.yml` is a working local fallback for the demo. |
| Live demo call fails on stage (wifi, rate limit) | Demo replays fixtures by default; the live call is one clearly-labelled extra step that can be skipped without breaking the flow. Fallback video recorded in Phase 5. |
| Parallel tracks drift on interfaces | `docs/CONTRACTS.md` is frozen at Phase 0. Changes go through B1 on `main`, never inside a worktree. In Go the drift is also a compile error, which is most of why Go was chosen. |
| Five tracks blocked waiting for B1 to finish `internal/` | B1's first commit is the full import surface as signatures with `panic("not implemented")` bodies, on `main` inside twenty minutes. Everyone compiles against it immediately. |
| DronaHQ cannot do WhatsApp natively | It has a native WhatsApp trigger over the Meta Business API. Outbound send mechanics are unverified and B5 confirms them on day one; the fallback is the Twilio connector as a REST connector. The demo needs message delivery, not a specific vendor. |
| Clustering has no embeddings endpoint to use | **Already happened.** The Nasiko router lists chat models only, so bp-clusterer clusters on local TF-IDF. Deterministic, free, and runs in CI. No `BP_VECTORISER` switch: one implementation ships, and an unused branch is a branch nobody tests. |

---

## What we are deliberately not building

Stated up front so no track wastes time on it:

- No auto-posting to any platform, ever.
- No login-walled or authenticated scraping.
- No multi-tenant auth, billing, or signup flow. One seeded demo brand plus the
  onboarding conversation is the whole account story.
- No historical backfill beyond the 14-day baseline window.
- No mobile app. DronaHQ's published app is the mobile story.
- No real-time streaming. The pipeline is scheduled plus on-demand.
- No fine-tuned models. Prompted classification only.

---

## Verification

Nothing is "done" without pasted output. Per repo rule: test suite for
correctness, benchmark for anything on a hot path, real numbers in `eval/`.

```bash
docker compose up -d postgres
psql "$DATABASE_URL" -f db/migrations/001_init.sql
go build ./... && go vet ./...              # vet is part of done, not optional
BP_FIXTURE_MODE=replay go test ./...        # must pass with no network, no keys
go run ./eval/accuracy                      # sentiment/intent accuracy, real
go run ./eval/cost                          # ₹ per brand-day, real
cd web && npm run build                     # typechecks the dashboard
./demo/run_demo.sh                          # the 2-minute flow, end to end
```
