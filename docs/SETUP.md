# Setup

Written so that somebody cloning this at 2am does not have to ask anyone
anything. Every command in section 2 and section 3 has been run against this
checkout and its real output is what the document describes.

---

## 1. What you need installed

| Tool | Version | Why that version |
|---|---|---|
| Go | **1.27** or later | `go.mod` declares `go 1.27`. Verified on `go1.27.1 darwin/arm64`. |
| Docker with Compose v2 | any current | Postgres runs in a container. There is no `psql` on the host and you do not need one. |
| `jq` | any | Only for the deploy and smoke-test commands. Nothing in the build needs it. |

Nothing else. No Python, no Node, unless you are working in `web/` or `bff/`,
which have their own instructions in their own directories.

The module path is **`brandpulse`**. Imports read `brandpulse/internal/models`,
not a GitHub path. It was never renamed, so if an import in your editor says
otherwise, your editor is stale.

---

## 2. From cold to a passing test suite

```bash
cp .env.example .env
docker compose up -d --no-recreate postgres
docker compose ps                     # wait until postgres says (healthy)
go build ./... && go vet ./...
BP_FIXTURE_MODE=replay go test ./...
```

That is the whole thing. **No API key is required.** `BP_FIXTURE_MODE=replay`
reads recorded fixtures from `fixtures/` and makes zero network calls, which is
the default everywhere including CI. A test that needs a credential to pass is
a broken test, not a test you are missing a key for.

A successful run ends with the twelve tables listed in section 3 and a build
and vet that print nothing. Packages without tests report `[no test files]`,
which is not a failure.

Then the two checks that go beyond compiling:

```bash
python3 scripts/check_agent_cards.py   # the half of `nasiko validate` that needs no cluster
./demo/run_demo.sh                     # the 2-minute run
```

The card check prints one line per `agents/*/AgentCard.json` it finds, or
`no AgentCard.json under agents/ yet` when none exist. `run_demo.sh` needs a
seeded database and the fixtures; see [../demo/CLAUDE.md](../demo/CLAUDE.md).

---

## 3. The database

Postgres is on **host port 5433, not 5432**. `docker-compose.yml` maps it that
way deliberately: another project's Postgres commonly holds 5432, and when it
does, compose fails to start at all rather than failing usefully.
`DATABASE_URL` in `.env.example` already points at 5433.

**You do not apply the migration by hand.** `docker-compose.yml` mounts `db/`
at `/docker-entrypoint-initdb.d`, so Postgres applies `001_init.sql` itself,
once, on an empty volume. Confirm it landed:

```bash
docker exec brandpulse-postgres psql -U brandpulse -d brandpulse -c '\dt'
```

If that lists tables, you are done. If it lists nothing, the volume was not
empty when the container first started, and the fix is section 5.

`db.Migrate` exists for the deployed path, where no compose file runs. It is
not part of local setup.

---

## 4. About `--no-recreate`

If you cloned this repository normally, **`--no-recreate` does nothing for you
and you can ignore it.** It is harmless, so the command in every document
carries it and you do not have to think about which case you are in.

It is there for the way this project was built: seven git worktrees sharing one
Postgres container. In that setup a plain `docker compose up -d postgres` from
any one worktree restarts the container out from under the other six.
`--no-recreate` makes it a no-op when the container is already running.

The same concern makes `docker compose down -v` a shared-state operation: it
wipes the volume for every worktree at once, so during the hackathon it had to
be announced in `HACKATHON_NOTES.md` first. On a fresh single clone it just
resets your own database.

---

## 5. When it does not work

**`docker compose ps` never reaches `(healthy)`.** Check the logs with
`docker compose logs postgres`. The usual cause is port 5433 already taken by
something else; change the host side of the mapping in `docker-compose.yml` and
the port in `DATABASE_URL` to match.

**Tables are missing after startup.** The init script only runs on an empty
volume, so a container that started once before the mount was right will never
apply it. Reset the volume, remembering that this wipes the database:

```bash
docker compose down -v && docker compose up -d postgres
```

**A test fails wanting a network call.** Check that `BP_FIXTURE_MODE=replay` is
actually set. If it is set and the test still reaches out, the test is wrong,
not your environment.

**A test fails on a zero value.** Go has no field defaults, which is the single
most common source of surprise in this codebase. Domain types are built with
their `New*` constructor and checked with `Validate()` before persisting. See
[../internal/models/CLAUDE.md](../internal/models/CLAUDE.md).

**An import does not resolve.** Run `go mod download`. The module path is
`brandpulse` with no host prefix, which means `go get` on it cannot work and
was never meant to.

---

## 6. The credentials, and what each one unlocks

`.env.example` splits into two groups for a reason. Everything in the first
group works out of the box. Everything in the second is blank until someone
supplies it, and **only two workflows ever need them**.

| Variable | Needed for | Without it |
|---|---|---|
| `DATABASE_URL` | Everything | Supplied, points at 5433 |
| `BP_FIXTURE_MODE` | Everything | Supplied, `replay` |
| `ANAKIN_API_KEY` | Recording fresh fixtures, and live collection | Replay mode works fully. 300 credits total for the project, so it is spent once, deliberately, with a dry-run estimate printed first. Sent as `X-API-Key`, not a Bearer token. |
| `DRONAHQ_API_KEY` | Building the two DronaHQ front ends | Everything else runs. Header is `api-key`. |
| `OPENAI_BASE_URL`, `OPENAI_API_KEY` | Live LLM calls | Replay mode works fully |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Traces reaching a collector | Agents run, spans go nowhere |

Two things about the LLM configuration that will otherwise cost you an hour.
All model traffic goes through `OPENAI_BASE_URL`; there is no provider SDK
configured in any agent. And the Nasiko LLM router **ignores the `model` field
in the request body**, so per-agent model choice is set with `nasiko llm-config`
and not in code.

`.env` is never committed. `.env.example` is the only version in git.

---

## 7. Deploying

Local setup is above and is enough to build, test and demo. Deployment is a
separate thing with its own rules:

Deploy through the `deploy` GitHub Actions workflow, by hand. **Never**
`nasiko deploy` from a laptop, and never the dashboard zip uploader or
`nasiko upload`. Both of those post to `/api/agents/upload`, whose
`validate_agent_zip` check requires a Python entrypoint and rejects a Go agent
with a `400`. `nasiko deploy` does not go near that route: it branches on
`AgentCard.json`, builds locally and pushes the image. Deploy, never upload.
One letter apart, one fatal.

Everything else we learned about the platform, including the parts that are not
in its documentation, is in [DEPLOY-NOTES.md](DEPLOY-NOTES.md).
