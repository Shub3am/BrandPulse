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
| ~~B2, B3, B4~~ | ~~B1~~ | ~~the `internal/` import surface as compiling signatures~~ | **closed 2026-09-20**, B1 Task 1, see "the import surface is up" below |
| B2, B3, B4 | B1 | `internal/anakin` with working `replay` mode | open |
| B3 (Task 4) | B2 | `fixtures/labelled/mentions.jsonl` sample | open |
| B5 (Tasks 4, 6, 7) | B2, B3, B4 | agents that run | open |
| B6 (`bff/`) | B4 | a deployed `bp-orchestrator` URL | open |
| ~~B6~~ | ~~B1~~ | ~~`internal/models/agentio.go`, so the BFF's envelope shapes are unverifiable~~ | **closed 2026-09-20**, the file exists and compiles |
| B5 | **you** | Nasiko CLI login. `ANAKIN_API_KEY` and `DRONAHQ_API_KEY` are now in `.env` in all seven checkouts, but there is still no Nasiko credential, so `nasiko deploy` cannot authenticate and not one of the nine agents can go live. | open, **hard blocker on deploy** |
| B5 | **you** | The **DronaHQ host URL** for our account. The only documented form is `https://<your-dronahq-host>/...`. Read it off the API Keys screen. Until then `DRONAHQ_API_KEY` cannot be used against anything. | open |
| ~~B5~~ | ~~you~~ | ~~Meta for Developers account, Twilio account, WhatsApp template approval~~ | **closed 2026-09-20**, WhatsApp is out of the MVP, see the scope decision below |
| B5 | **you** | No deploy pipeline. `.github/workflows/` is empty and the repo rule is "deploy through the automated pipeline". Either we build one in Phase 4 or we agree the hackathon deploys by CLI and say so. | open, needs a ruling |
| B5, B6 | **you** | No hosting target for `web/` and `bff/`, and no production Postgres. Nasiko hosts the nine agents; it does not host a Next.js app, a Fastify process or a database. Phase 4's gate says "`web/` renders a real run" against infrastructure nobody has named. | open |
| ~~B1~~ | ~~B7~~ | ~~Fast-forward `main` to `track/b1-core`~~ | **closed 2026-09-20**, merged as `73ef873`, see below |

In Go a missing package is a compile error for everyone downstream, not a
runtime `ImportError` in one test. That is why B1's signature commit is its own
blocker row and why it comes before B1's own implementation.

---

## Resolved unknowns

Each entry: the question, the answer, and how it was verified. An unverified
answer stays in "Open questions".

### 2026-09-20 — main — RULINGS on B5's open questions 4 and 5, and the AgentCard blast radius is not what it looked like

**Ruling on question 4, who writes the nine cards and Dockerfiles: CONTRACTS §4
stands, B5's Task 4 is deleted.** The agent track writes its own card and
Dockerfile and commits them with the agent. B5 owns the template, the
`nasiko.yaml` and the deploy, and files a bad card as a blocker row rather than
fixing it. The contract already argued this and the argument is still right:
nine agents across five worktrees all editing eighteen files a sixth worktree
also edits is a guaranteed merge-day conflict, and the Dockerfile varies only
by binary name. Reality had already voted. B3 and B4 wrote seven of the nine
without being asked.

Template pointers, now corrected in CONTRACTS: **card is `nasiko.md` §2,
Dockerfile is §4.** CONTRACTS sent everyone to §4 for both.

**The blast radius is the opposite of the warning.** B5 wrote that any track
generating a card from the Go struct must add three fields. Checked all seven
existing cards:

```
agents/bp-clusterer     url + protocolVersion + preferredTransport: present
agents/bp-enricher      present
agents/bp-sov           present
agents/bp-briefer       present
agents/bp-detector      present
agents/bp-orchestrator  present
agents/bp-responder     present
supportedInterfaces[]:  absent from all seven
```

Nobody generated from the struct. All seven were hand-written and all seven
pass `nasiko validate` today. What they are missing is the other half of the
union: `supportedInterfaces[]`, which is what an A2A 1.0 consumer reads.

**B3 and B4: add `supportedInterfaces[]` to your cards from `nasiko.md` §2.**
Do not remove the three top-level fields, Nasiko needs them. One file satisfies
both because the validator checks presence and `encoding/json` ignores unknown
keys. This is additive and nothing you have breaks meanwhile.

**Ruling on question 5, the Nasiko fork PR: Task 6 is retargeted, not dropped.**
The PR was going to carry Go source importing `brandpulse/internal/...`, which
nothing outside this repo can fetch because Task 0 was skipped. That PR cannot
build and should not be opened. But B5 found a genuine upstream bug while
running the real CLI: `validate.rs` requires `url`, `protocolVersion` and
`preferredTransport` at the top level, and A2A 1.0 moved all three into
`supportedInterfaces[]`, so a spec-correct card fails validation. **That is the
PR.** It is Rust, against Nasiko's own repo, it imports nothing of ours, it
builds standalone, and it is worth more at judging than an example would have
been: we used the platform hard enough to find a real spec-conformance bug and
fixed it upstream.

Skipping Task 0 was my call, so this consequence is mine. Retargeting costs
nothing we had.

**Still with the repo owner, not rulable here:** Nasiko control-plane login,
DronaHQ console login and host URL, browser driver choice, the deploy pipeline
question, and a hosting target for `web/` and `bff/`.

### 2026-09-20 — main — B1's import surface is on `main` at `73ef873`. Merge it.

B2, B3 and B4: you were blocked on this. `main` now carries `internal/a2a`,
`anakin`, `db`, `hashing`, `ids`, `llm`, `models` (including `agentio.go`),
`obs`, `prompts`, `redact` and `stats`, plus the CONTRACTS §3 signature edits.
Run `git merge main` in your worktree and drop whatever you were compiling
against in the meantime.

**B4 specifically**: your `chore(scaffold): stand in for B1's import surface so
B4 can compile` is now duplicate. Delete the scaffold in the same commit that
merges `main`, do not leave two definitions of the same surface in the tree.

What was verified before the merge landed, and what was not:

```
go build ./...   exit 0
go vet ./...     exit 0
BP_FIXTURE_MODE=replay go test ./...
    all 11 internal packages: [no test files]
```

So it compiles and vets. **Nothing was tested**, because `parity_test.go` is
still uncommitted in B1's worktree. Do not read this merge as a green suite.

This was merged by the coordinating session, not by B7, because B7 runs at the
end and four tracks were not going to wait that long. `main` is still
single-writer, see the section above.

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

### 2026-09-20 — B1 — the import surface is up: `brandpulse/internal/...` compiles

`go build ./... && go vet ./...` are both green. Every package named in
CONTRACTS §3 now exists with its real signature.

**It is on `track/b1-core`, not yet on `main`.** B1's brief said to commit
straight to `main`; the commit above this one on `main` says B7 owns `main` and
B1 lands through B7, and the later instruction wins. The branch is rebased onto
`main@260d234` so the merge is `--ff-only` with nothing to resolve. Until B7
runs it, `git merge track/b1-core` into your own branch and start now rather
than waiting.

**Every body panics with `not implemented`.** That is deliberate and it is the
one place in this repo a stub is correct: it is a compile target. Nothing here
works yet, and a stub that returned a plausible zero value would let you build
on an empty slice and discover it on stage. If you call one you will get a
panic naming the package, which is the answer you want today.

Two exceptions, because they could not honestly panic:

- **`internal/prompts` is fully implemented.** `Guardrails` is a package-level
  `var`, and a `var` has no body to panic in. Leaving it nil would have been a
  silent empty guardrail list, which is worse than anything else on this page.
  `prompts.Load(name)` reads the embedded FS and is real. B1 Task 9 is
  therefore already done; adding your agent's prompt `.md` to that package is
  yours, and it is a recompile, not a config reload.
- **`models.NewAlert` and `models.NewDailyBrief` are real.** They are pure
  schema like the rest of `models`, and a constructor that panics is not a
  compile target, it is a landmine.

**What changed in CONTRACTS §3, land it in your head before you write a
`main()`:**

| Change | Why |
|---|---|
| `a2a.Serve[In, Out any](card, h)` and `a2a.Handler[In, Out]` are now generic | §4 gives every agent `Handle(ctx, <Name>Input) (<Output>, error)`. Adapting that to one SDK executor interface needs type parameters. Inference makes your call site `a2a.Serve(card, handler)` unchanged — you write no type argument. |
| `a2a.LoadCard(path) (Card, error)` added | You need a `Card` from somewhere and you must not build one by hand. It also rejects `protocolVersion != "1.0"` at load rather than at deploy. |
| `llm.Opt` now has fields: `{Model string; MaxTokens int}` | §3 named the type without them. Both are optional; a zero `Model` defers to the router's per-agent `nasiko llm-config`. |
| `anakin.NoNetwork() *http.Client` added | The no-network guarantee is a package, not a CI setting, because Go has no `pytest-socket`. Inject it in every test. Still a stub until B1 Task 12. |
| `models.NewAlert` / `NewDailyBrief` signatures printed | Both take their timestamps as arguments and read no clock, same reason `DetectInput` carries `Now`. |

**`anakin.NewHTTPClient` is the `cfg Config` form from §3, not the positional
form the B1 brief sketches.** §3 is what five tracks compile against, so §3
wins. `Config` carries everything the positional version did plus `MaxCredits`,
which is the ceiling the collector binds per call.

**Two corrections the B1 brief predicted that turned out not to be needed:**
CONTRACTS §0 already says `redact.PII`, not `RedactPII`, and §3's import block
already excludes `internal/cluster` and already says in prose that it is B3's.
Both docs are correct as written; nothing to fix. `internal/cluster` does not
exist and B1 will not create it — B3 builds it as their Task 1.

**Answer to Open question 1, partially.** `a2a-go/v2` v2.5.0 resolves and the
module path in CONTRACTS is right. Two findings for whoever wires an agent:
the card type is **`a2a.AgentCard` in package
`github.com/a2aproject/a2a-go/v2/a2a`**, not `a2asrv.AgentCard` as the B1 brief
says, and that package name collides with our own `internal/a2a`, so it needs
an import alias. The server side is `a2asrv.NewHandler` + `NewJSONRPCHandler`
registered on a stdlib mux, and the well-known card path is
`a2asrv.WellKnownAgentCardPath`. **The SDK is not yet imported anywhere**:
`internal/a2a` is stdlib-only for now so this commit could land in minutes, and
pulling in its grpc and protobuf dependencies is B1 Task 11. `a2a.JSONArtifact`
still has no verified SDK helper behind it; question 1 stays open.
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

### 2026-09-20 — B5 — open question #1 answered, and a card that marshals from the Go struct will not deploy

The shared templates in `research/nasiko.md` §2, §3 and §4 were still Python.
They are Go now, and every line of them was built and run before being written
down. **B1 reads §3 before writing `internal/a2a`. Every agent track reads §2
and §4 before writing a card or a Dockerfile.**

**#1, the JSON artifact helper: there is no one-call helper.** It is
`a2a.NewDataPart(data any) *a2a.Part` plus
`a2a.NewArtifactEvent(infoProvider, parts...)`. `NewDataPart` leaves
`MediaType` empty and `NewArtifactEvent` leaves `Artifact.Name` empty, so
CONTRACTS' "one `application/json` artifact named for its struct" needs both
assigned by hand. That is four lines, easy to get three-quarters right, times
nine agents, which is precisely the case for `a2a.JSONArtifact` existing.
`*a2asrv.ExecutorContext` implements `a2a.TaskInfoProvider`, so it passes
straight in. Marshalled wire output is pasted in §3.

**For anyone writing a binding, including DronaHQ and the BFF:** the payload
sits at `artifact.parts[0].data`, and v2 has **no `kind` discriminator** on the
part. A binding looking for `"kind": "data"` finds nothing.

**The v2 executor is an iterator, not an event queue.**
`Execute(ctx, *ExecutorContext) iter.Seq2[a2a.Event, error]`. There is no
`enqueue_event`. Anyone porting from a Python example or from v1 loses an
afternoon here.

**The one that would have failed a deploy.** `a2a-go` v2.5.0 marshals
`a2a.AgentCard` to the A2A 1.0 shape, which moves `url`, `protocolVersion` and
`preferredTransport` into a `supportedInterfaces[]` array. Nasiko's
`validate.rs` requires all three at the top level. So **generating
`AgentCard.json` from the Go struct produces a card that fails `nasiko
validate`**, and I confirmed that by deleting exactly those three fields and
watching it fail. One file satisfies both: write the union, since `validate.rs`
only checks presence and `encoding/json` ignores unknown keys. Template in §2,
and it passes:

```
$ nasiko validate
  ✓ Dockerfile
  ✓ AgentCard.json
  ✓ source directory
  ✓ AgentCard.json fields
  ✓ skills (1 defined)
✓ Valid (2 warning(s))
```

The two warnings are `docker-compose.yml` and `.env.example` missing **from the
agent directory**. Ours are at the repo root. Ignore them, do not "fix" them.

**The Dockerfile is verified too, and gives us a pitch number.** Multi-stage,
`golang:1.27.1` to build with `CGO_ENABLED=0`, then
`gcr.io/distroless/static:nonroot`. Built against this repo's `go.mod` and
`internal/models/` with `a2a-go` v2.5.0 pulled in: **14.6MB**, runs as
`nonroot`, serves. Not scratch: Anakin and the LLM router are HTTPS and scratch
has no CA bundle. Build context is the repo root or `COPY internal/` cannot
reach it.

Still unverified and still needing the cluster: `nasiko push`, `nasiko deploy`
and everything after them.

---

## Open questions

The things nobody has verified yet. Claim one by putting your track in the
"owner" column, and move it to "Resolved unknowns" when you have an answer.

| # | Question | Owner | Why it matters |
|---|---|---|---|
| ~~1~~ | ~~Which `a2a-go/v2` helper emits a **JSON** artifact?~~ | ~~B1, Task 1~~ | **answered 2026-09-20**, compiled and marshalled. None does it in one call: `NewDataPart` + `NewArtifactEvent`, then set `MediaType` and `Artifact.Name` by hand. `research/nasiko.md` §3. |
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

## `main` has one writer at a time, and B7 takes it last

B7 runs at the end, not alongside B1 through B6. Until it starts, the primary
checkout `/Users/shubhamvs/Desktop/anakin-hack/brandpulse` and the `main` branch
stay with the coordinating session, which is where every scope change and doc
fix so far was landed. The moment B7 starts, that checkout is B7's alone and the
coordinating session stops writing to it.

Either way the rule is one writer. Two writers on one working tree is
uncommitted work destroyed, not a merge conflict, because git cannot help with
edits it has never seen.

**Consequence of B7 starting late, stated plainly:** `.github/workflows/` is
empty and stays empty until then, so nothing is checking that B1 through B6
still build together. Four branches have already diverged. The first time
anyone finds out is at merge time, all at once.

So: if you are B1 through B6 and you need something changed on `main`, you do
not go and change it. You add a row to "Open blockers", or you open the PR.
B1 still owns `internal/models`, `001_init.sql` and `CONTRACTS.md` and still
lands them through a PR. The contracts rule did not move, only the question of
who holds the write path to `main` at a given hour.

Merge order is B1, B2, B3, B4, B6, B5, green between each.

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
| Time from mention to alert on screen | — | `demo/run_demo.sh` timing output | B4 |
| Nasiko PR | — | link | B5 |
