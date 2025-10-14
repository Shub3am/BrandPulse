# BrandPulse

Social listening for Indian D2C brands. Nine A2A agents in Go on Nasiko, Anakin
as the whole data layer, DronaHQ as the WhatsApp agent and the ops dashboard.

Start with [PLAN.md](PLAN.md), then your track brief in [docs/tracks/](docs/tracks/),
then [docs/CONTRACTS.md](docs/CONTRACTS.md). Spec: [docs/BRIEF.md](docs/BRIEF.md).
Why Go: [docs/decisions/001-go-for-agents.md](docs/decisions/001-go-for-agents.md).

## Modules

| Path | What it is |
|---|---|
| `internal/models/` | The frozen wire format. [CLAUDE.md](internal/models/CLAUDE.md) |
| `internal/` | Shared library: anakin, llm, db, stats, cluster, obs, a2a. B1 owns it. |
| `agents/` | Nine A2A agents, one directory each. [CLAUDE.md](agents/CLAUDE.md) |
| `db/` | Postgres schema. [CLAUDE.md](db/CLAUDE.md) |
| `fixtures/` | Recorded Anakin responses and the labelled eval set. |
| `demo/` | Seed, replay, crisis injection, the 2-minute run. [CLAUDE.md](demo/CLAUDE.md) |
| `eval/` | Classifier accuracy and real cost per brand-day. [CLAUDE.md](eval/CLAUDE.md) |
| `dronahq/` | Exported WhatsApp agent and dashboard app. [CLAUDE.md](dronahq/CLAUDE.md) |
| `web/` | Next.js product dashboard. [CLAUDE.md](web/CLAUDE.md) |
| `bff/` | Fastify backend-for-frontend, the dashboard's only backend. |
| `docs/` | Contracts, research, track briefs, decisions, pricing. |

## Run, test, deploy

```bash
docker compose up -d postgres
psql "$DATABASE_URL" -f db/migrations/001_init.sql
go build ./... && go vet ./... && BP_FIXTURE_MODE=replay go test ./...
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
- **Redact before the prompt.** `redact.PII` runs on every mention text before
  it reaches a model.
- **LLM traffic only via `OPENAI_BASE_URL`.** No provider SDK config in an agent.
- **Typed JSON artifacts.** Every agent returns one `application/json` artifact
  that is an `internal/models` struct. DronaHQ binds to it directly.
- **Contracts are frozen.** `internal/models/`, `001_init.sql` and
  `CONTRACTS.md` change only on `main`, through B1. Never inside a worktree.
- **Zero values are the hazard.** Use the `New*` constructors, call `Validate()`
  before persisting. Go has no field defaults and pydantic used to supply them.
- **No source is faked.** If a probe fails, that source does not appear in a
  fixture, a count or a sentence. Synthetic data is labelled in the UI.
- **Coordinate in [HACKATHON_NOTES.md](HACKATHON_NOTES.md)**, not in commit
  messages. Do not read another track's code.
- Commit small, one logical change each, conventional-commit prefix.
