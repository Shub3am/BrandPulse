# db

## What this module owns

The Postgres schema, as ordered SQL migrations. Nothing else.

## What it must not know about

Application code. There is no ORM, no model layer and no seed data here —
seeding lives in `demo/`.

## Entry points

`migrations/001_init.sql` is the whole schema as of Phase 0. Apply with:

```bash
psql "$DATABASE_URL" -f db/migrations/001_init.sql
```

## Invariants and gotchas

- **Migrations are additive after Phase 1.** A column rename breaks every track
  at once. New migration files only, numbered in order, never an edit to a file
  that has already been applied.
- **The schema mirrors `shared/bp_core/models.py`.** They change together, in
  the same commit, on `main`, through B1.
- **`mentions` has `UNIQUE (brand_id, content_hash)`** — that constraint is the
  dedupe mechanism, not a safety net. Collectors rely on the conflict.
- **`alerts` has `UNIQUE (brand_id, dedupe_key)`** so a sustained crisis is one
  alert per hour, not one per run.
- **`runs` has `UNIQUE (brand_id, kind, time_bucket)`** — that is how the
  orchestrator is idempotent. Handle the conflict, do not race on it.
- **`fetch_cache` is keyed `(source, query_hash, time_bucket)`.** Widening the
  bucket is the cheapest lever we have on credit spend.
- **`source_yield` drives flow-guard source prioritisation.** An empty table
  means the orchestrator falls back to the profile's declared order.

## Who calls this

`bp_core.db` opens the pool; every agent queries through it. `demo/seed.py`
loads fixtures. `eval/cost.py` reads `runs` and `mention_enrichment`.
