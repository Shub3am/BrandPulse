# BrandPulse

Social listening for Indian D2C brands. Nine A2A agents in Go on Nasiko, Anakin
as the whole data layer, DronaHQ as the chat agent and the ops dashboard.

Start with [PLAN.md](PLAN.md), then your track brief in [docs/tracks/](docs/tracks/),
then [docs/CONTRACTS.md](docs/CONTRACTS.md). Spec: [docs/BRIEF.md](docs/BRIEF.md).

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
| `dronahq/` | Exported chat agent and dashboard app. [CLAUDE.md](dronahq/CLAUDE.md) |
| `web/` | Next.js product dashboard. [CLAUDE.md](web/CLAUDE.md) |
| `bff/` | Fastify backend-for-frontend, the dashboard's only backend. |
| `docs/` | Contracts, research, track briefs, decisions, pricing. Why Go: [001](docs/decisions/001-go-for-agents.md). |
| `scripts/` | Card check, and B7's end-to-end and deployed smoke tests. |

## Run, test, deploy

```bash
cp .env.example .env                # DATABASE_URL points at port 5433, not 5432
docker compose up -d --no-recreate postgres   # one shared DB, see rules below
docker compose ps                   # must show (healthy) before anything else
go build ./... && go vet ./... && BP_FIXTURE_MODE=replay go test ./...
python3 scripts/check_agent_cards.py  # the half of nasiko validate needing no cluster
./demo/run_demo.sh                 # the 2-minute flow
```

Deploy is the `deploy` workflow, by hand, never `nasiko deploy` from a laptop.
CI runs all of the above on every push.

## Repo-wide rules

- **Zero-credit CI.** `BP_FIXTURE_MODE=replay`. A test needing a key is broken.
- **`record` mode is spent once**, by B2, dry-run estimate first. 300 credits.
- **No LLM in `bp-detector`.** Alerts ship the numbers that fired them.
- **AgentCards carry both shapes**, top-level fields and `supportedInterfaces[]`.
- **Nothing auto-posts.** There is no posting code path here. Do not add one.
- **Redact before the prompt.** `redact.PII` runs on every mention text.
- **LLM traffic only via `OPENAI_BASE_URL`.** No provider SDK config in an agent.
- **Typed JSON artifacts.** One `application/json` `internal/models` struct per
  agent call. DronaHQ binds to it directly.
- **Contracts are frozen.** `internal/models/`, `001_init.sql` and
  `CONTRACTS.md` change only on `main`, through B1. Never inside a worktree.
- **Zero values are the hazard.** Go has no field defaults. Use `New*`, then
  `Validate()` before persisting.
- **No source is faked.** Synthetic data is labelled in the UI.
- **One Postgres for all seven worktrees.** `down -v` wipes every track, so
  announce it. Plain `up` elsewhere restarts it, hence `--no-recreate` above.
- **Coordinate in [HACKATHON_NOTES.md](HACKATHON_NOTES.md).** Never read
  another track's code.
- Commit small, one logical change each, conventional-commit prefix.
