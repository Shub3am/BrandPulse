# BrandPulse

Social listening for Indian D2C brands. Nine A2A agents on Nasiko, Anakin as the
whole data layer, DronaHQ as the WhatsApp agent and the dashboard.

Start with [PLAN.md](PLAN.md), then your track brief in [docs/tracks/](docs/tracks/),
then [docs/CONTRACTS.md](docs/CONTRACTS.md). Spec: [docs/BRIEF.md](docs/BRIEF.md).

## Modules

| Path | What it is |
|---|---|
| `shared/bp_core/` | Models, Anakin client, LLM client, DB, stats, redaction. [CLAUDE.md](shared/bp_core/CLAUDE.md) |
| `shared/bp_core/sources/` | One adapter per source, Anakin response to `Mention`. [CLAUDE.md](shared/bp_core/sources/CLAUDE.md) |
| `agents/` | Nine A2A agents, one directory each. [CLAUDE.md](agents/CLAUDE.md) |
| `db/` | Postgres schema. [CLAUDE.md](db/CLAUDE.md) |
| `fixtures/` | Recorded Anakin responses and the labelled eval set. |
| `demo/` | Seed, replay, crisis injection, the 2-minute run. [CLAUDE.md](demo/CLAUDE.md) |
| `eval/` | Classifier accuracy and real cost per brand-day. [CLAUDE.md](eval/CLAUDE.md) |
| `dronahq/` | Exported WhatsApp agent and dashboard app. [CLAUDE.md](dronahq/CLAUDE.md) |
| `web/` | Next.js product dashboard. [CLAUDE.md](web/CLAUDE.md) |
| `docs/` | Contracts, research, track briefs, architecture, pricing. |

## Run, test, deploy

```bash
docker compose up -d postgres
psql "$DATABASE_URL" -f db/migrations/001_init.sql
BP_FIXTURE_MODE=replay pytest -q --disable-socket --allow-unix-socket
./demo/run_demo.sh                 # the 2-minute flow
nasiko validate && nasiko deploy   # from agents/bp-<name>/
```

## Repo-wide rules

- **Zero-credit, zero-key CI.** `BP_FIXTURE_MODE=replay` is the default
  everywhere. A test that needs `ANAKIN_API_KEY` is a broken test.
- **`record` mode is spent once**, by B2, with a dry-run estimate printed first.
  300 free credits total and no more.
- **No LLM in `bp-detector`.** Alerting is statistics, and every alert carries
  the numbers and thresholds that fired it.
- **Nothing auto-posts.** There is no posting code path here. Do not add one.
- **Redact before the prompt.** `bp_core.redact.redact_pii` runs on every
  mention text before it reaches a model.
- **LLM traffic only via `OPENAI_BASE_URL`.** No provider SDK config in an agent.
- **Typed JSON artifacts.** Every agent returns one `application/json` artifact
  that is a `bp_core.models` dump. DronaHQ binds to it directly.
- **Contracts are frozen.** `models.py`, `001_init.sql` and `CONTRACTS.md`
  change only on `main`, through B1. Never inside a worktree.
- **No source is faked.** If a probe fails, that source does not appear in a
  fixture, a count or a sentence. Synthetic data lives only in
  `demo/inject_crisis.py` and is labelled in the UI.
- **Coordinate in [HACKATHON_NOTES.md](HACKATHON_NOTES.md)**, not in commit
  messages. Do not read another track's code.
- Commit small, one logical change each, conventional-commit prefix.
