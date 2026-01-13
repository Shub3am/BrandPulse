# db

## What this module owns

The Postgres schema, as ordered SQL migrations. Nothing else.

## What it must not know about

Application code. There is no ORM, no model layer and no seed data here —
seeding lives in `demo/`.

## Entry points

`migrations/001_init.sql` is the whole schema as of Phase 0. You do not apply it
by hand and there is no `psql` on the host. `docker-compose.yml` mounts this
directory at `/docker-entrypoint-initdb.d`, so Postgres applies it itself, once,
on an empty volume:

```bash
docker compose up -d --no-recreate postgres
docker exec brandpulse-postgres psql -U brandpulse -d brandpulse -c '\dt'
```

Changing the schema therefore means `docker compose down -v` and back up. That
volume is shared by all seven worktrees, so announce it in `HACKATHON_NOTES.md`
before you wipe it. `db.Migrate` covers the deployed path, where no compose runs.

## Invariants and gotchas

- **The two schema paths do not mix, and `db.Migrate` fails loudly rather than
  pretend otherwise.** A database seeded by the compose initdb mount has the
  schema and no `schema_migrations` rows, so `db.Migrate` against your local
  `brandpulse` database fails on `type "source" already exists`. That is the
  intended behaviour: swallowing the conflict would hide a schema that has
  genuinely diverged. `internal/db/db_test.go` therefore tests the runner in a
  scratch database it creates and drops, and never touches yours.
- **Migrations are additive after Phase 1.** A column rename breaks every track
  at once. New migration files only, numbered in order, never an edit to a file
  that has already been applied.
- **The schema mirrors `internal/models/`.** They change together, in the same
  commit, on `main`, through B1. `internal/models/parity_test.go` parses this
  file and compares its columns to the `json` tags, so a field added on one
  side without the other is a test failure rather than a discovery on stage.
  The intended divergences are listed in CONTRACTS §3b and the test knows them.
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

`internal/db` opens the pool; every agent queries through it. `demo/seed.go`
loads fixtures. `eval/cost` reads `runs` and `mention_enrichment`.
