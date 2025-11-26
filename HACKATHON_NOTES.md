# Hackathon notes — shared decisions

The one place six parallel tracks talk to each other. Append, do not rewrite.
Newest entry at the bottom of its section.

**How to use this file**

- Anything another track needs to know goes here, not in a commit message.
- Format: `### YYYY-MM-DD HH:MM — <track> — <one line>` then the detail.
- If you are **blocked**, add a row to "Open blockers" and name the track that
  unblocks you.
- If you **answered an UNVERIFIED question**, put the answer under "Resolved
  unknowns" with how you verified it. Nine agents must not each rediscover it.
- Merge conflicts on this file are expected and always resolved by keeping
  both sides.

---

## Open blockers

| Raised by | Blocked on | What I need | Status |
|---|---|---|---|
| B2, B3, B4 | B1 | the `internal/` import surface as compiling signatures, on `main` | open, **B1 Task 1, twenty minutes** |
| B2, B3, B4 | B1 | `internal/anakin` with working `replay` mode | open |
| B3 (Task 4) | B2 | `fixtures/labelled/mentions.jsonl` sample | open |
| B5 (Tasks 4, 6, 7) | B2, B3, B4 | agents that run | open |
| B6 (`bff/`) | B4 | a deployed `bp-orchestrator` URL | open |
| B6 | B1 | `internal/models/agentio.go`. It is specified in CONTRACTS §2 and has no Go source, so every envelope shape the BFF returns is unverifiable today. | open, **B1 Task 1** |
| B5 | **you** | Nasiko CLI login. `ANAKIN_API_KEY` and `DRONAHQ_API_KEY` are now in `.env` in all seven checkouts, but there is still no Nasiko credential, so `nasiko deploy` cannot authenticate and not one of the nine agents can go live. | open, **hard blocker on deploy** |
| B5 | **you** | The **DronaHQ host URL** for our account. The only documented form is `https://<your-dronahq-host>/...`. Read it off the API Keys screen. Until then `DRONAHQ_API_KEY` cannot be used against anything. | open |
| ~~B5~~ | ~~you~~ | ~~Meta for Developers account, Twilio account, WhatsApp template approval~~ | **closed 2026-09-20**, WhatsApp is out of the MVP, see the scope decision below |
| B5 | **you** | No deploy pipeline. `.github/workflows/` is empty and the repo rule is "deploy through the automated pipeline". Either we build one in Phase 4 or we agree the hackathon deploys by CLI and say so. | open, needs a ruling |
| B5, B6 | **you** | No hosting target for `web/` and `bff/`, and no production Postgres. Nasiko hosts the nine agents; it does not host a Next.js app, a Fastify process or a database. Phase 4's gate says "`web/` renders a real run" against infrastructure nobody has named. | open |

In Go a missing package is a compile error for everyone downstream, not a
runtime `ImportError` in one test. That is why B1's signature commit is its own
blocker row and why it comes before B1's own implementation.

---

## Resolved unknowns

Each entry: the question, the answer, and how it was verified. An unverified
answer stays in "Open questions".

### 2026-09-20 — main — SCOPE CHANGE: WhatsApp is out of the MVP. Read this if you are mid-task.

**What we are building is observability over reviews and social data.** Read
everything said about a brand, cluster it, score it, alert on it. WhatsApp was
never the product, it is one delivery channel over the top, and it was dragging
three external accounts onto the critical path: a Meta for Developers app, a
WhatsApp Business number, and a template approval that takes days.

**The alert row in Postgres is now the source of truth, and every channel is a
reader of it.**

| Surface | MVP mechanism | External account |
|---|---|---|
| Alert delivery | `alerts` row, rendered live in `web/` and the DronaHQ dashboard | none |
| Conversational agent | DronaHQ Agent on the **Chat** trigger | none |
| `bp-detector` into DronaHQ | DronaHQ **Webhook** trigger, `api-key` header | none |
| 9am brief | DronaHQ **Scheduler** trigger | none |
| WhatsApp / Slack / email | post-MVP, read the same rows | Meta / Slack / Gmail |

**What changes for you:**

- **B4**: no change to `bp-detector`. It writes an alert row and POSTs to a
  webhook. It never knew what a channel was and it still does not.
- **B6**: you are now the **primary** alert surface, not a secondary one. The
  live alert feed in `web/` is the demo. Time-to-alert is measured to your UI.
- **B5**: build the DronaHQ agent on the **Chat** trigger. Do not configure a
  WhatsApp trigger, do not add a Twilio connector, do not touch Meta.
- **B1, B2, B3, B7**: nothing changes.

Nothing already built is wasted. Adding WhatsApp later is a connector plus a
phone number, because every channel reads the same row.

### 2026-09-20 — main — DronaHQ is researched, and the obvious WhatsApp block is a trap

`docs/research/dronahq.md` now exists, 409 lines, sourced. Three findings change
the build.

**1. The WhatsApp actionflow block cannot send anything.** It reads like an
outbound sender. It opens WhatsApp on the *viewer's own device* with a prefilled
message the viewer must press Send on. Its only two fields are Message Text and
Phone Number, and it has no credential field because nothing leaves the server.
Verified twice against
`docs.dronahq.com/reference/actionflow-blocks/whatsapp/`, once by the research
agent and once directly. Wiring the crisis alert to this is a demo where
nothing arrives.

**2. WhatsApp is two surfaces with two providers.** Inbound is DronaHQ's native
trigger on Meta's WhatsApp Business API via webhook plus verify token, no third
party. Outbound is the Twilio connector, `SendWhatsappTextMessage`, both numbers
prefixed `whatsapp:`. That means two accounts, two credentials, and possibly two
phone numbers.

**3. "Binds to typed JSON directly" holds only for flat arrays.** Table Grid
takes an array of objects natively, but DronaHQ's own docs say direct binding
*"will work only on JSON data which isn't too nested"* and push deeper shapes
through SQL-over-JSON or DQL. A struct-of-structs artifact reintroduces exactly
the translation layer CONTRACTS wanted to avoid. B5 and B1: check that each
artifact exposes a flat top-level array per intended table.

Also: the `sk_` key is an **Agentic platform** key, header `api-key`, never
`Authorization: Bearer`. Agent Starter caps at **10 tools per agent** and forces
"Powered by DronaHQ" branding, which with nine Go agents plus Twilio plus
Postgres is already over the line. Agent export is undocumented, so
`dronahq/whatsapp-agent.json` may have to become a runbook; that is B5's first
console check.

### 2026-09-20 — main — the Anakin key is live, and `/v1/search` wants `prompt`

`ANAKIN_API_KEY` and `DRONAHQ_API_KEY` are set in `.env` in all seven checkouts.
`.env` is gitignored in every one of them, the keys appear nowhere in tracked
files or in any commit reachable from any ref, and the files are `0600`. If you
rotate a key you rotate it in seven places.

The Anakin key was verified without spending a credit. Anakin does not bill
failed calls, so `POST /v1/search` with an empty body `{}` proves auth on its
own: the key returns `400 {"error":"invalid_request","message":"Prompt is
required"}`, which is a request that got past authentication and then failed
validation.

That error is itself a finding. `docs/research/anakin.md` §3 documents the
Search body as carrying a query, and the live API calls the required field
**`prompt`**. B2, confirm the exact field name in Task 1 before writing the
adapter, and correct the research doc in the same commit. This is the kind of
thing Task 1 exists to catch.

`BP_FIXTURE_MODE` stays `replay` everywhere. Holding a key is not a reason to
spend it. The 300 credits are still B2's to spend once, after a dry-run
estimate.

### 2026-09-20 — main — Anakin Wire does not carry four of the brief's sources

X/Twitter, Instagram, Google Play reviews and Apple App Store reviews are not in
Anakin's Wire catalogue. Verified by direct 404 on `anakin.io/catalog/<slug>`
for each, corroborated by Anakin's own blog post dated 2026-07-24. Flipkart has
product actions but no reviews action.

Replacements and drops are in [docs/SOURCE-STRATEGY.md](docs/SOURCE-STRATEGY.md).
The `Source` enum is unchanged, so this is a config change, not a schema change.

### 2026-09-20 — main — Anakin Search API returns a snippet, not full page content

The brief says "full page content". It returns `snippet`. Bodies require
chaining Search → URL Scraper at 3 + N credits. Budget accordingly.
Detail in [docs/research/anakin.md](docs/research/anakin.md) §2.

### 2026-09-20 — main — Nasiko AgentCard `protocolVersion` must be `"1.0"`

The `currency-agent` example ships `"0.2.9"`. A real cluster rejects that with
`-32009 VersionNotSupported`. Every BrandPulse card uses `"1.0"`.

### 2026-09-20 — main — the example Dockerfile does not build

`agents/currency-agent/Dockerfile` does `COPY pyproject.toml .` and no such file
exists anywhere in the Nasiko repo. Use ours from
[docs/research/nasiko.md](docs/research/nasiko.md) §4, which builds with the
repo root as context so `shared/` is available.

### 2026-09-20 — main — the Nasiko LLM router discards the request's `model` field

Per-agent model selection is configured with `nasiko llm-config`, not passed in
code. `BP_MODEL_SMALL` therefore does nothing once deployed. B5 owns this.
Detail in [docs/research/nasiko.md](docs/research/nasiko.md) §6 Finding A.

### 2026-09-20 — main — no verified embeddings endpoint on the Nasiko router

The catalogue lists chat models only. bp-clusterer therefore clusters on local
TF-IDF. Detail in §6 Finding B.

Amended 2026-09-20: the original entry put this behind a
`BP_VECTORISER=tfidf|embeddings` switch. There is no switch. One implementation
ships, because an alternate branch with no endpoint behind it is a branch
nobody can test. `research/nasiko.md` §6 still names the switch and is stale on
that one point.

### 2026-09-20 — B5 — Nasiko's deploy path read from source: no proxy env var, no OTel endpoint, and `nasiko upload` is as fatal as the dashboard

There is still no Nasiko credential, so instead of waiting I cloned
`Nasiko-Labs/nasiko`, built the CLI from source and read the deploy path.
Everything below is **source-read at commit `58cfe60`**, not confirmed against a
live cluster. Full working in [docs/DEPLOY-NOTES.md](docs/DEPLOY-NOTES.md).

**Answers open question #7, and B1 has nothing to change.** Nothing injects
`OTEL_EXPORTER_OTLP_ENDPOINT` into a deployed container, *in any language*.
`ServerState::agent_env` (`server/src/state.rs:462-472`) builds the entire
environment and it is: agent secrets, plus `OPENAI_API_KEY` /
`OPENAI_BASE_URL` / `OPENAI_MODEL`, plus `PORT` defaulted to 8000. That is the
list. Python's auto-instrumentation is patched in via `PYTHONSTARTUP`
(`upload.rs:858-920`) and then reads the same unset variable, so Python is no
better off than we are. `internal/obs` stays exactly as the ADR describes:
self-instrument, read `OTEL_EXPORTER_OTLP_ENDPOINT`. **B5 supplies it** as a
vault-wide `nasiko secrets set`. The collector's in-cluster address still needs
a live cluster.

**Answers open question #2, negatively, and B1 needs to know before writing
`a2a.Call`.** There is no `NASIKO_PROXY_URL`. `research/nasiko.md` §7 guessed
one; it does not exist. The server is the sole ingress, peers are reached at
`POST /api/agents/{agent_id}`, and the only agent-facing credential is
`x-nasiko-agent-token`, a delegation JWT the server mints and sends **inbound**
(`server/src/router/a2a_dispatch.rs:796-811`). All nine Nasiko-shipped example
agents were grepped: **none of them calls another agent**, so there is no
first-party example to copy.

So `a2a.Call` needs two things that do not arrive for free: a base URL, which
we set ourselves as a vault-wide secret (suggest `NASIKO_API_URL`, our name not
Nasiko's), and a credential, for which the only available shape is replaying
the inbound `x-nasiko-agent-token` off the request the caller is already
serving. That is a per-request value, so **it has to thread through
`a2a.Call`'s `ctx`, not sit in a package-level client**. B1: design for that
now, it is cheap today and a rewrite later. The replay itself is my inference
from how the server mints and consumes the token, not something the docs state,
and it gets confirmed on the first live two-agent call.

**Correction to the decisions log.** The Go ADR entry below says
`validate_agent_zip` guards "only the dashboard zip-upload path". It guards the
server route `/api/agents/upload`, and the **CLI's `nasiko upload` posts to
exactly that route** (`cli/src/commands/upload.rs:17`). So `nasiko upload` is
barred for Go agents too, not just the dashboard. `nasiko deploy` is unaffected:
it branches on `AgentCard.json`, builds locally and pushes to the OCI registry
(`cli/src/commands/deploy.rs:26-63`), and never touches the validator. The rule
is one word wider than we wrote it: **deploy, never upload.**

**Two things worth having before you hit them.** `version` in `AgentCard.json`
must be `x.y.z` or the server rejects it with no default applied
(`upload.rs:475-485`), so `"1.0"` fails and `"1.0.0"` passes. And `nasiko
validate` already accepts Go: it looks for `src/`, `cmd/` **or `main.go`**, and
a miss is a warning, not an error (`validate.rs:37-46`).

---

## Open questions

The things nobody has verified yet. Claim one by putting your track in the
"owner" column, and move it to "Resolved unknowns" when you have an answer.

| # | Question | Owner | Why it matters |
|---|---|---|---|
| 1 | Which `a2a-go/v2` helper emits a **JSON** artifact? | B1, Task 1 | All nine agents need it. Lands as `a2a.JSONArtifact`. |
| ~~2~~ | ~~How is a peer agent addressed through the Nasiko proxy?~~ | ~~B5, Task 1~~ | **answered 2026-09-20**, source-read. No proxy env var exists. See Resolved unknowns. B1 reads it before writing `a2a.Call`. |
| ~~7~~ | ~~Do OTel traces from a self-instrumented Go container reach `nasiko observe`?~~ | ~~B5, Task 2~~ | **half-answered 2026-09-20**, source-read. Nothing injects the endpoint, B5 sets it. The collector address still needs a live cluster. |
| 3 | The literal field names in Wire responses per action. | B2, Task 1 | A guessed field name is an empty dashboard on stage. |
| 4 | Does the Play Store listing yield review text, rating and date through URL Scraper with `useBrowser: true`? | B2, Task 2 | Decides six sources or seven. |
| ~~5~~ | ~~Does DronaHQ's WhatsApp trigger send outbound?~~ | ~~B5, Task 7~~ | **moot 2026-09-20**, WhatsApp is out of the MVP. The verified answer is kept in `research/dronahq.md` §1 for whoever switches it on later. |
| 6 | Does DronaHQ's Charts control expose the Plotly `hole` config for a donut? | B5, Task 6 | Cosmetic. Ship a pie if not. |
| 8 | What is the OTLP collector's address from inside an agent container, and does a Go span show up in `nasiko observe`? | B5, Task 2 | The remainder of #7. Needs a live cluster, so it needs the Nasiko credential first. |

---

## Decisions log

Things we chose, with the reason, so nobody relitigates them at 3am.

### 2026-09-20 — main — contracts frozen before any worktree branched

`docs/CONTRACTS.md`, `internal/models/` and `db/migrations/001_init.sql` are
frozen on `main`. A field rename is a breaking change for DronaHQ bindings and
every downstream agent. Changes go through B1 on `main`, never inside a
worktree.

### 2026-09-20 — main — we do not use the `anakin-sdk` package

It is alpha. We call the REST API from one package, `internal/anakin`, so a
wrong assumption about Anakin's shape is a one-package fix. Reasoning in
[docs/research/anakin.md](docs/research/anakin.md) §7.

### 2026-09-20 — main — the nine agents are Go, not Python

Full reasoning in [docs/decisions/001-go-for-agents.md](docs/decisions/001-go-for-agents.md).
The short version: six parallel tracks against a frozen contract, and in Go a
contract drift is a compile error instead of a runtime `KeyError` found on
stage. Verified before committing to it: `github.com/a2aproject/a2a-go/v2`
v2.5.0 exists, has server support in `a2asrv`, and Nasiko already runs Go
agents.

**The wire format did not change.** Field names, enum values, the five detector
rules and the divergence table are identical to the Python freeze. A fixture or
a DronaHQ binding made against the old contract still works. What changed is
the import surface (CONTRACTS §3), the naming rules (§4), and two costs that
pydantic and sklearn used to absorb:

- **Clustering is hand-rolled.** Go has no scikit-learn. `internal/cluster` is
  240 to 320 lines and it is B3's Task 1, before the agent that uses it.
- **OpenTelemetry is self-instrumented.** Nasiko auto-injects OTel for Python
  containers only. `internal/obs` is ~150 lines, written once by B1, called by
  every agent's `main()`.

A third finding that cost nothing but would have cost an hour live: Nasiko's
`validate_agent_zip` requires a `main.py`, but **only on the dashboard
zip-upload path**. `nasiko deploy` from the CLI does not run that gate. Never
use the dashboard uploader for these agents.

Amended 2026-09-20 by B5, from source: that gate guards the server route
`/api/agents/upload`, and the CLI's `nasiko upload` posts to that same route.
So it bars Go agents from the CLI too, not only from the dashboard. `nasiko
deploy` is still unaffected. **Deploy, never upload.**

### 2026-09-20 — main — zero values replace pydantic defaults, and Validate is the guard

Go has no field defaults. Three that pydantic supplied are now hazards:
`BrandProfile.Version` (was 1), `Topic.Trend` (was 1.0) and `Mention.Lang`
(was `"en"`) decode as `0`, `0.0` and `""`. The `New*` constructors carry the
intended value and `Validate()` catches the decoder path that bypasses them.

The one that could not be left to a convention is
`ReplyDraft.requires_human_approval`: a `bool` field would decode to `false`
from any payload that omitted it. So the field does not exist on the struct at
all, and `MarshalJSON` emits `true` unconditionally. Consequence: `ReplyDraft`
does not round-trip. Do not add the field back to "fix" that.

### 2026-09-20 — main — a sixth track, B6, owns the product front end

`web/` (Next.js 16, already built and rendering against demo data) and `bff/`
(Fastify). This is a different surface from B5's DronaHQ dashboard: DronaHQ is
the ops and analyst view, `web/` is the product view a customer sees. The
overlap invites duplicated work, so the split is stated here.

### 2026-09-20 — main — no source is faked

If a probe fails, that source does not appear in a fixture, a dashboard, a count
or a sentence. Synthetic data exists in exactly one file,
`demo/inject_crisis.go`, and is labelled as an injection in the UI. `web/` has
its own synthetic set in `web/lib/demoData.ts`, for a deliberately fictional
brand, labelled in the top bar and in the page footnote. Both labels are
load-bearing, not decoration.

### 2026-09-20 — main — Postgres is on host port 5433, and there is no `psql` on this machine

Two things every track would otherwise hit separately, both verified by running
it:

**Port 5433, not 5432.** Another project's container (`inboxready-postgres`)
already binds 5432 on this machine, and `docker compose up` does not degrade
gracefully, it fails outright with "port is already allocated". That container
is somebody else's running service and we do not stop it. Inside our container
the port is still 5432, so nothing on the compose network changes.
`DATABASE_URL` is `postgresql://brandpulse:brandpulse@localhost:5433/brandpulse`
and `.env.example` carries it.

**There is no `psql` binary on the host.** Use
`docker exec brandpulse-postgres psql -U brandpulse -d brandpulse -c '...'`.
The old root CLAUDE.md told you to run `psql "$DATABASE_URL" -f
db/migrations/001_init.sql` by hand; that command does not exist here and the
migration now applies itself through the compose mount anyway.

**The migration runs only on an empty volume.** `/docker-entrypoint-initdb.d`
is initdb-time only, so `docker compose down -v` is how you pick up a schema
change. Plain `up` will silently keep the old schema.

Verified from a cold volume: `(healthy)` in about 6 seconds, 12 tables,
7 enums, port reachable from the host.

### 2026-09-20 — main — one Postgres for all seven worktrees, and it is bound to loopback

`docker compose up -d postgres` from the B3 worktree failed:

```
Conflict. The container name "/brandpulse-postgres" is already in use by
container "fb0f8a04c62a...". You have to remove (or rename) that container
to be able to reuse that name.
```

Compose names the project after the directory. Seven checkouts meant seven
projects, each wanting its own volume but all colliding on the one fixed
`container_name`. It had already created a stray
`brandpulse-b3-intel_brandpulse_pgdata` before it died.

`docker-compose.yml` now pins `name: brandpulse` at the top level, so `up`
from any worktree resolves to the same project and attaches to the running
container instead of racing it. Consequences to know:

- **One database, shared.** That is what integration needs. It also means a
  track that writes junk rows writes them into everyone's database.
- **`docker compose down -v` from any worktree wipes it for everyone.** Post
  here before you run it. `down` without `-v` is harmless.
- **Always pass `--no-recreate`.** Plain `up` from a worktree that did not last
  start the container recreates it, because the compose project labels carry
  the absolute working directory and it differs per checkout. The data survives
  (the volume is separate) but Postgres restarts under whoever is mid-test.
  `docker compose up -d --no-recreate postgres` from three different worktrees
  printed `Container brandpulse-postgres Running` three times, no restart.

The published port is also `127.0.0.1:5433:5432` now, not `5433:5432`. The
password is `brandpulse`, and the bare form publishes on `0.0.0.0`, which
hands the database to everyone on the same wifi. Nothing off this laptop
needs it.

### 2026-09-20 — main — the freeze is `phase0-contracts-go` and all six worktrees sit on it

`phase0-contracts` is the superseded Python freeze. It stays in the repo as
history, because the deleted parity test is worth reading before porting it
(`git show phase0-contracts:shared/tests/test_contract_schema_parity.py`), but
nothing branches from it any more.

All six worktrees were fast-forwarded to `main` at the same commit. Verified
clean and zero commits ahead first, so no work was discarded. There is no
Python anywhere in the tree: the language set is Go, TypeScript and SQL.

### 2026-09-20 — main — four things already broken in `web/`, found before B6 started

Recorded here so B6 does not rediscover them and nobody calls them regressions.

1. **`web/lib/types.ts` is already drifted.** `ShareOfVoice` and `DailyBrief`
   exist in `models.go` and are missing from the mirror, and `/pulse` needs
   both. The drift the checker is for is present today, not hypothetical.
2. **The "demo data" pill sits in `app/layout.tsx`**, which receives no data.
   It cannot follow the data source from there, so "going live is one file" is
   not literally true until the badge moves out of the layout. The label is
   load-bearing under the no-faking rule, so this is correctness, not polish.
3. **`PlatformPanel.secondsToWhatsapp` has no wire field.** It renders `107`
   from a constant in `demoData.ts`. `RunRecord` has no such field and the
   number is still blank in the table below. Right now the UI shows a
   measurement nobody has taken. Either B4 puts it on the wire or the panel
   stops claiming it.
4. **`TopicList.tsx:39` divides by `topic.size`**, so a real topic with
   `size: 0` renders `NaN%` as a bar width.

Also: `.github/workflows/` is empty. There is no automated pipeline yet, so
"deploy through the pipeline" is a decision somebody has to make, not a step
somebody follows.

---

## Numbers for the pitch

Filled in as they are measured. **Real output only.** An estimate goes in
brackets and is replaced, never quietly promoted.

| Number | Value | Source | Owner |
|---|---|---|---|
| Credits spent recording fixtures | _(est. ~136)_ | B2 Task 7 actual | B2 |
| Mentions in the fixture corpus | — | `fixtures/` count | B2 |
| Sources shipped | _(7, or 6 if Play probe fails)_ | B2 Task 2 | B2 |
| Sentiment accuracy | — | `go run ./eval/accuracy` | B3 |
| Intent accuracy | — | `go run ./eval/accuracy` | B3 |
| Cost per brand-day, cold | _(target < ₹15)_ | `go run ./eval/cost` | B3 |
| Cost per brand-day, warm cache | — | `go run ./eval/cost` | B3 |
| Agent container image size | — | `docker images` after build | B5 |
| Agents deployed on Nasiko | 0 / 9 | `nasiko deploy` | B5 |
| Time from mention to WhatsApp | — | `demo/run_demo.sh` timing output | B4 |
| Nasiko PR | — | link | B5 |
