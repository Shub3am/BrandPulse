# Track B1 — Shared core, database, CI

**Branch:** `track/b1-core`
**Owns:** `shared/bp_core/`, `db/`, `docker-compose.yml`, `.github/workflows/`
**Blocks:** every other track. Land Phase 1 fast, then stay on call as integrator.

Read [PLAN.md](../../PLAN.md) and [CONTRACTS.md](../CONTRACTS.md) first. You are
the owner of `CONTRACTS.md`; other tracks propose changes, you land them.

---

## What this track owns

The only code in the repo that talks to the network or the database. Nine
agents import from here and do no I/O of their own.

## What it must not know about

Any agent. `bp_core` never imports from `agents/`. If a helper needs to know
which agent is calling it, the helper is in the wrong place.

---

## Task 1 — Package skeleton and IDs

**Files:** `shared/pyproject.toml`, `shared/bp_core/__init__.py`,
`shared/bp_core/ids.py`, `shared/bp_core/hashing.py`,
`shared/tests/test_ids.py`

- [ ] `pyproject.toml` declaring package `bp_core`, requires-python `>=3.13`,
      deps: `pydantic>=2.9`, `httpx`, `asyncpg`, `openai`, `scikit-learn`,
      `numpy`, `python-ulid`. Dev extras: `pytest`, `pytest-asyncio`,
      `pytest-socket`.
- [ ] `ids.new_id(prefix: str) -> str` returning `f"{prefix}_{ULID()}"`.
      Prefixes in use: `brd`, `mnt`, `top`, `alr`, `drf`, `brf`, `run`.
- [ ] `hashing.content_hash(text: str, source: str) -> str` — lowercase, strip
      whitespace runs, strip URLs and `@handles`, then sha256 hex. Two copies
      of the same tweet with different trailing links must hash equal. Write
      that as the test.
- [ ] Test, run it, commit.

## Task 2 — PII redaction

**Files:** `shared/bp_core/redact.py`, `shared/tests/test_redact.py`

- [ ] `redact_pii(text: str) -> str` replacing emails with `[email]` and phone
      numbers with `[phone]`. Indian formats matter: `+91 98765 43210`,
      `9876543210`, `+91-98765-43210`.
- [ ] Must **not** redact public handles (`@brand`) or URLs — those are the
      data. Test both directions.
- [ ] Commit.

## Task 3 — Database layer

**Files:** `shared/bp_core/db.py`, `docker-compose.yml`,
`shared/tests/test_db.py`

- [ ] `docker-compose.yml` with `postgres:16` on 5432, volume, healthcheck, and
      a `DATABASE_URL` default of
      `postgresql://brandpulse:brandpulse@localhost:5432/brandpulse`.
- [ ] `db.py`: module-level asyncpg pool created lazily, `fetch`, `fetchrow`,
      `execute`, `executemany`, and `migrate()` that applies
      `db/migrations/*.sql` in filename order, tracked in a `schema_migrations`
      table so it is idempotent.
- [ ] Test against the compose Postgres: migrate twice, assert no error and one
      row per migration.
- [ ] Commit.

## Task 4 — Credit budget

**Files:** `shared/bp_core/budget.py`, `shared/tests/test_budget.py`

- [ ] `CreditBudget(brand_id, day, ceiling)` with `async def spend(n) -> None`
      raising `BudgetExceeded` when the day's total would pass the ceiling, and
      `async def remaining() -> int`.
- [ ] Backed by `runs.credits_used` summed for the brand-day, so it survives a
      process restart. Cache the running total in memory, re-read on
      `BudgetExceeded` before raising, so a stale cache cannot block a run
      that actually has headroom.
- [ ] Test: ceiling 10, spend 6, spend 6 → raises, `remaining()` == 4.
- [ ] Commit.

## Task 5 — Anakin client

**Files:** `shared/bp_core/anakin.py`, `shared/tests/test_anakin.py`,
`fixtures/_canned/*.json`

This is the highest-value file in the repo. One file knows Anakin's wire shape;
everything else consumes normalised dicts.

- [ ] Read `docs/research/anakin.md` (landed by the main session) for the real
      endpoints, auth header and response shapes. **Do not guess.** If the
      research marks something UNVERIFIED, implement it behind a small
      `_parse_*` function and leave a comment saying it is unverified, so the
      fix is one function.
- [ ] `AnakinClient(api_key, mode, budget, db)` with the five methods in
      CONTRACTS §3. Each method:
      1. builds a stable `query_hash` from its arguments,
      2. computes `time_bucket` (hour for wire/search, day for map/crawl),
      3. returns the cached `fetch_cache` payload on hit,
      4. on miss: `budget.spend(cost)`, HTTP call via `httpx.AsyncClient` with
         retry on 429/5xx (3 attempts, exponential backoff), write cache.
- [ ] Three modes per CONTRACTS §3. `replay` reads
      `fixtures/<source>/<query_hash>.json` and raises a clear error naming the
      missing fixture path when absent — a missing fixture must be obvious, not
      an empty list.
- [ ] `record` mode writes every response to `fixtures/` after the live call.
- [ ] Tests run in `replay` against `fixtures/_canned/` with `pytest-socket`
      disabling the network. Assert: cache hit costs zero, budget exceeded
      raises, missing fixture names the path.
- [ ] Commit.

## Task 6 — LLM router client

**Files:** `shared/bp_core/llm.py`, `shared/tests/test_llm.py`

- [ ] `chat_json(prompt, schema, *, model, max_tokens) -> tuple[dict, Usage]`
      using the `openai` SDK pointed at `OPENAI_BASE_URL`, with structured
      output constrained to `schema`. Returns parsed dict plus token usage.
- [ ] `embed(texts: list[str], *, model) -> list[list[float]]`, batched at 100.
- [ ] `Usage` carries `prompt_tokens`, `completion_tokens`, `cost_paise`.
      Cost table lives here, one dict, so the cost dashboard has one source.
- [ ] Both functions retry once on a JSON parse failure with a "return only
      valid JSON" nudge, then raise.
- [ ] Tests mock the OpenAI client. No live call in CI.
- [ ] Commit.

## Task 7 — Stats

**Files:** `shared/bp_core/stats.py`, `shared/tests/test_stats.py`

- [ ] `zscore(value, mean, std) -> float`, returning 0.0 when `std == 0` rather
      than dividing by zero. This guard is the difference between a working
      detector and a demo that throws on a quiet brand.
- [ ] `compute_baseline(brand_id, *, days=14, now) -> BaselineStats` producing
      exactly the shape in CONTRACTS §2 under bp-detector: per-source hourly
      mean and std, negative-share mean and std, mean rating.
- [ ] Baseline **excludes the current hour**, or a spike suppresses itself.
      Write that as an explicit test.
- [ ] `hour_bucket(dt) -> str` and `day_bucket(dt) -> str` helpers, UTC, used
      for cache keys, run idempotency and alert dedupe. One implementation.
- [ ] Commit.

## Task 8 — Prompts module and guardrails

**Files:** `shared/bp_core/prompts/__init__.py`,
`shared/bp_core/prompts/GUARDRAILS.md`

- [ ] `load(name: str) -> str` reading `prompts/<name>.md`, cached.
- [ ] `GUARDRAILS: list[str]` — phrases a drafted reply must never contain.
      Seed it with: no refunds promised, no admission of fault or liability,
      no medical or safety claims, no naming an individual employee, no
      commitment to a date, no legal characterisation ("we were negligent").
- [ ] Commit.

## Task 9 — CI

**Files:** `.github/workflows/ci.yml`

- [ ] Postgres service container, Python 3.13, `uv sync`, run migrations, then
      `BP_FIXTURE_MODE=replay pytest -q` with **no secrets configured**. The
      job must be green on a fork with no access to any key. That is the point.
- [ ] Add `pytest-socket`'s `--disable-socket --allow-unix-socket` so a stray
      network call fails loudly instead of silently costing credits.
- [ ] Commit.

---

## Integrator duties (after Phase 1)

- Review and merge every track PR into `main`.
- Own conflict resolution in `shared/` and `db/`.
- Land any agreed contract change on `main` yourself, then tell every track to
  rebase. Post it in `HACKATHON_NOTES.md`.
- Migrations after Phase 1 are **additive only** — new file, never an edit to
  `001_init.sql`.

## Definition of done

```bash
docker compose up -d postgres
BP_FIXTURE_MODE=replay pytest shared/ -q --disable-socket --allow-unix-socket
```

Green, with output pasted into the PR. Every other track can `import bp_core`
and get a working `AnakinClient` in replay mode.
