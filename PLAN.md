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

**Tech Stack:** Python 3.13, `a2a` SDK, Starlette, pydantic v2, asyncpg,
Postgres 16, scikit-learn, pytest + pytest-asyncio, Docker, `nasiko deploy`,
DronaHQ.

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
  and phone numbers found in mention text are stripped by
  `bp_core.redact.redact_pii` before any prompt.
- **LLM calls only via `OPENAI_BASE_URL`** (Nasiko router). No direct provider
  SDK configuration in any agent.
- **No raw API keys in containers.** Secrets are injected by Nasiko at runtime.
- **Typed JSON artifacts only.** Every agent returns one
  `application/json` artifact that is a `bp_core.models` dump, so DronaHQ binds
  to it without a translation layer.
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
| `shared/bp_core/models.py` | Wire format for every agent. Pure schema, no I/O. | B1 |
| `shared/bp_core/anakin.py` | Anakin HTTP client: cache, dedupe, budget, fixture modes. | B1 |
| `shared/bp_core/llm.py` | Nasiko LLM router client: `chat_json`, `embed`. | B1 |
| `shared/bp_core/db.py` | asyncpg pool and query helpers. | B1 |
| `shared/bp_core/stats.py` | Baselines, z-scores, negative-share windows. | B1 |
| `shared/bp_core/redact.py` | PII stripping before prompts. | B1 |
| `shared/bp_core/budget.py` | Per-brand-day credit ceiling. | B1 |
| `shared/bp_core/sources/*.py` | One adapter per source; Anakin response → `Mention`. | B2 |
| `shared/bp_core/prompts/*.md` | One prompt per LLM-using agent. | owner of that agent |
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
| `docs/` | Architecture, setup, pricing, contracts, track briefs. | B5 (B1 owns CONTRACTS) |

---

## Phases

Tracks run in parallel **within** a phase. A phase gate is a real check, not a
vibe: the listed command must pass on `main` before the next phase opens.

### Phase 0 — Contracts (main session, before any worktree exists)

Locks the interfaces so five agents can build without talking. **Done when**
`docs/CONTRACTS.md`, `shared/bp_core/models.py` and `db/migrations/001_init.sql`
are committed to `main` and `python -c "import bp_core.models"` succeeds.

This phase is already complete when you read this.

### Phase 1 — Foundation (B1 alone, ~45 min)

B1 finishes `bp_core` and `docker-compose.yml`. Every other track is blocked on
the client stubs, so B1 lands `anakin.py`, `llm.py`, `db.py` with **working
`replay` mode and passing unit tests** before anything else.

**Gate:** `docker compose up -d postgres && pytest shared/ -q` green, and
`BP_FIXTURE_MODE=replay` returns a canned response for every one of the five
Anakin methods.

### Phase 2 — Data acquisition and the credit spend (B2 alone for the spend)

B2 builds the source adapters and the collector, then runs the **one and only**
live recording session against the demo brand and two competitors. This is the
irreversible step: 300 credits, spent once.

**Before spending:** the adapter for a source must pass against a hand-written
fixture first. We do not discover a parsing bug with live credits.

**Gate:** `fixtures/` contains ≥ 300 mentions across ≥ 7 sources, and
`pytest agents/bp-collector -q` passes in replay mode with zero network calls
(enforced by a `pytest` socket-blocking fixture).

### Phase 3 — Intelligence and pipeline (B3 + B4 in parallel)

B3 turns mentions into enriched mentions, topics and share-of-voice, and builds
`eval/`. B4 builds the detector, responder, briefer and orchestrator against the
contract, using B2's fixtures.

**Gate:** `pytest -q` green across the repo; `eval/` prints real sentiment
accuracy on the labelled set and a real ₹/brand-day figure.

### Phase 4 — Deploy and front ends (B5, with B1 on call)

Nine `AgentCard.json` + `Dockerfile` + `nasiko deploy`. Flow guards configured.
DronaHQ WhatsApp agent and dashboard built against the deployed URLs. Copy
`agents/bp-*` into the Nasiko fork and open the PR.

**Gate:** all nine agents respond to a real A2A call at their deployed URLs;
the DronaHQ dashboard renders a live run; the WhatsApp agent delivers a brief.

### Phase 5 — Demo hardening (all tracks, converging)

Run the full 2-minute demo end to end, three times, on the real deployment.
Fix what breaks. Record the fallback video.

**Gate:** three consecutive clean runs of `demo/run_demo.sh`.

---

## Track assignments

Each track gets a git worktree and a branch. Full briefs in `docs/tracks/`.

| Track | Branch | Owns | Blocked by |
|---|---|---|---|
| B1 | `track/b1-core` | `shared/bp_core`, `db/`, `docker-compose.yml`, CI | — |
| B2 | `track/b2-collect` | source adapters, `bp-collector`, `bp-onboarder`, fixtures | B1 Phase 1 gate |
| B3 | `track/b3-intel` | `bp-enricher`, `bp-clusterer`, `bp-sov`, `eval/` | B1 gate; B2 fixtures |
| B4 | `track/b4-pipeline` | `bp-detector`, `bp-responder`, `bp-briefer`, `bp-orchestrator`, `demo/` | B1 gate (contracts only for B2/B3) |
| B5 | `track/b5-deploy` | AgentCards, Dockerfiles, Nasiko deploy, DronaHQ, docs, pitch | B2–B4 |

**Integration:** each track opens a PR into `main` at its phase gate. B1 is the
integrator and resolves conflicts. Nobody merges their own track into another's.

---

## Risk register

The things most likely to sink this, and what we do about each.

| Risk | Mitigation |
|---|---|
| Anakin's real API shape differs from what we assumed | **Already happened.** Wire does not carry X, Instagram, Play Store or App Store reviews, and Search returns a snippet not a page body. Recorded in [docs/SOURCE-STRATEGY.md](docs/SOURCE-STRATEGY.md); seven sources ship instead of eight. Remaining unknown is the literal Wire field names, which B2 Task 1 reads live. `bp_core.anakin` is the only file that knows Anakin's shape, so a further surprise is a one-file fix. |
| 300 credits burn out during development | `replay` is the default mode everywhere including local dev. `record` requires an explicit env flag and is run once, by B2, with a dry-run credit estimate printed first. |
| Nasiko deploy fails late and we have nothing to show | Phase 4 starts with **one** agent deployed end to end (`bp-sov`, the simplest) before the other eight. `docker-compose.yml` is a working local fallback for the demo. |
| Live demo call fails on stage (wifi, rate limit) | Demo replays fixtures by default; the live call is one clearly-labelled extra step that can be skipped without breaking the flow. Fallback video recorded in Phase 5. |
| Clustering quality is poor on 300 mentions | `min_cluster_size=3` and a hand-checked topic list on the demo brand. If agglomerative clustering looks bad, fall back to keyword-grouping — the contract does not change. |
| Parallel tracks drift on interfaces | `docs/CONTRACTS.md` is frozen at Phase 0. Changes go through B1 on `main`, never inside a worktree. |
| DronaHQ cannot do WhatsApp natively | It has a native WhatsApp trigger over the Meta Business API. Outbound send mechanics are unverified and B5 confirms them on day one; the fallback is the Twilio connector as a REST connector. The demo needs message delivery, not a specific vendor. |
| Clustering has no embeddings endpoint to use | **Already happened.** The Nasiko router lists chat models only, so bp-clusterer defaults to local TF-IDF behind `BP_VECTORISER=tfidf\|embeddings`. Deterministic, free, and runs in CI. |

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
BP_FIXTURE_MODE=replay pytest -q            # must pass with no network, no keys
python -m eval.accuracy                     # sentiment/intent accuracy, real
python -m eval.cost                         # ₹ per brand-day, real
./demo/run_demo.sh                          # the 2-minute flow, end to end
```
