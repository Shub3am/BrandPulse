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
| ~~B3, and therefore B5's first deploy~~ | ~~B1~~ | ~~`models.SOVInput`, `obs.Setup`, `a2a.Serve`~~ | **closed 2026-09-20**, B1's surface merged. **bp-sov is written, tested and green.** B5's first deploy target is ready. |
| B3 (bp-enricher tests) | B1 | `redact.PII` with a real body, B1 Task 3. The handler calls it for real and must: a guardrail tested against a stub of itself is not a guardrail. All nine bp-enricher tests therefore **skip**, not pass. They enforce with no edit the moment B1 lands. | open, **B1 Task 3** |
| B3 (bp-clusterer tests) | B1 | `ids.New` with a real body, B1 Task 2. Same shape: all eleven bp-clusterer tests skip until it lands. | open, **B1 Task 2** |
| B3 (Task 6, 7) | B1 | `llm.Usage` does not report **which model the router actually used**, so `Enrichment.Model` cannot be filled with anything true and bp-enricher leaves it empty. The router ignores the requested model, so `Opt.Model` is not the answer either. `eval/cost` needs it to price per model. Smallest fix: add `Model string` to `llm.Usage`, read off the response body. | open, **B1** |
| B3 (Tasks 5, 6, 7) | B2 | `fixtures/` does not exist yet. Task 5 is hand-labelling that set, Task 6 scores the enricher against it, Task 7 needs real `RunRecord` values from a full brand-day. All three read real numbers or they are worthless, so none of them can start early. | open |
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
### 2026-09-20 — B3 — internal/cluster is done and measured, and the cutoff is 0.85

`internal/cluster` has landed on `track/b3-intel`: TF-IDF, average-linkage
agglomerative merging, cosine distance, English + Hinglish stopwords. Standard
library only, no `models` import, no context, no I/O. 339 lines of
implementation against the ADR's 240–320 estimate; the overshoot is
`stopwords.go`, which is a word list rather than algorithm.

**Measured, Apple M5, 400 documents** (`go test -bench=. -benchmem`):

```
BenchmarkTFIDF400-10            5092     236604 ns/op    401363 B/op   1216 allocs/op
BenchmarkAgglomerative400-10     100   10774839 ns/op   1311606 B/op    858 allocs/op
```

0.24 ms to vectorise and 10.8 ms to cluster a full brand-day. The ADR's claim
that an O(n²) hand-roll is affordable at this corpus size is now a number
rather than an assumption, and there is no case for optimising it.

**The distance cutoff is 0.85** and it is not on a knife edge. On the toy
corpus, within-topic distances run 0.44–0.76 and cross-topic distances are
exactly 1.0, so every cutoff from 0.80 to 0.98 yields the identical three
clusters. A test pins that band. The constant itself lives in `bp-clusterer`,
not in the package.

Two findings worth other tracks' attention:

**1. `CosineDistance` can exceed 1.0.** The specified `idf = log(N/(1+df))`
goes negative for a term in every document, so two rows can be opposed and the
distance can reach 2. A cutoff of 1.0 therefore merges the whole corpus into
one cluster. Nobody outside `bp-clusterer` should be picking a cutoff, but if
you are, do not assume the range is [0, 1].

**2. A zero vector is reachable and it was a NaN waiting to happen.** That same
IDF is exactly 0 for a term appearing in N-1 documents, so a document whose
every term is corpus-wide vectorises to all zeros. Normalising it would produce
NaN, which does not fail in the clusterer: it fails wherever the artifact is
marshalled, as `json: unsupported value`, several agents downstream. Guarded
and tested. Flagging it because the same shape of trap exists anywhere we
divide by a computed norm or a count.

### 2026-09-20 — B3 — `research/nasiko.md` §4 is stale on language, not on build context

§4's Dockerfile is `FROM python:3.13-slim` with a `pip install`. The repo is Go.
The rule it states is still right and still load-bearing — **build context is
the repo root**, because no agent directory can see `internal/` on its own —
but the body is superseded by `agents/CLAUDE.md`, which specifies multi-stage
`golang:1.27` with `CGO_ENABLED=0` onto a distroless static base. B3's three
Dockerfiles follow `agents/CLAUDE.md`. §6 is stale in the same way, naming
`sklearn.feature_extraction.text.TfidfVectorizer` for a vectoriser that is now
339 lines of Go. B5 owns that doc; flagging rather than editing it.

One detail other agent owners will hit: the Dockerfile copies `go.*`, not
`go.mod` and `go.sum` separately. There is no `go.sum` until B1 adds the first
third-party dependency, and Docker's `COPY` fails on a glob that matches
nothing.

### 2026-09-20 — B3 — three agents are written; bp-sov is ready for B5 to deploy

`bp-sov`, `bp-enricher` and `bp-clusterer` are on `track/b3-intel`, rebased
onto `main` after B1's surface merged. `go build ./... && go vet ./...` clean.

**B5: bp-sov is your first deploy target and it is done.** No LLM, no network,
no database. Its twenty tests pass today, not once someone else lands
something. The other two compile and are fully written, but their suites
**skip** rather than pass, because `redact.PII` and `ids.New` are still B1's
panicking stubs and both are called for real. Skips are in the blocker table.
I verified both suites green against throwaway local implementations of those
two functions and reverted them; the code is right, the gate is B1's.

**Every agent owner needs this Dockerfile fix.** `a2a.LoadCard` reads
`AgentCard.json` from disk at startup, and the multi-stage build's run stage
copies only the binary, so a card that builds fine gives you a container that
dies on boot. Add to the distroless stage:

```dockerfile
COPY --from=build /src/agents/bp-<name>/AgentCard.json /AgentCard.json
```

and load it from `/AgentCard.json`, absolutely, because distroless has no
shell to set a working directory from. All three of B3's Dockerfiles do this.

**Two contract gaps found by writing against the surface rather than reading
it.** Neither is a blocker for me; both are wrong numbers for somebody else.

1. `llm.Usage` does not say which model answered. The router ignores the
   requested model by design, so `Opt.Model` is not it either, and
   `Enrichment.Model` has nothing true to hold. bp-enricher leaves it empty
   rather than writing the model it asked for and did not get. `eval/cost`
   cannot price per model until `Usage` carries it.
2. `ClusterInput` has a `BrandID` and no `BrandProfile`, so bp-clusterer
   cannot tell the labelling model the brand's name or products. I cut that
   section out of `clusterer.md` rather than ask B1 to widen a frozen struct:
   a label is named from the mentions and a bare `brd_01J` would have told the
   model less than the text already does.

**The label-drift limitation the brief names is real and is not fixed here.**
`PriorWindowCounts` is keyed by the label the LLM wrote, so a label that
drifts between windows reads as a brand new topic with a flat trend of 1.0.
`clusterer.md` keeps labels to a two-to-five word noun phrase with no dates,
counts or sentiment adjectives, which is the mitigation the contract allows.
Anyone reading a trend of exactly 1.0 on a topic that clearly existed
yesterday should suspect drift before believing the number.

**Tasks 5, 6 and 7 have not started and cannot.** There is no `fixtures/`
directory. Task 5 is hand-labelling that set, Task 6 scores the enricher
against it, and Task 7 needs real `RunRecord` values from a brand-day that has
not been run. The cost-per-brand-day number B5 needs for `PRICING.md` is
therefore still unknown, and I would rather hand over "unknown" than a number
derived from a constant.

---

## Open questions

The things nobody has verified yet. Claim one by putting your track in the
"owner" column, and move it to "Resolved unknowns" when you have an answer.

| # | Question | Owner | Why it matters |
|---|---|---|---|
| 1 | Which `a2a-go/v2` helper emits a **JSON** artifact? | B1, Task 1 | All nine agents need it. Lands as `a2a.JSONArtifact`. |
| 2 | How is a peer agent addressed through the Nasiko proxy? The env var name is unverified. | B5, Task 1 | Lands as `a2a.Call`. No agent writes a peer URL directly. |
| 7 | Do OTel traces from a self-instrumented Go container actually reach `nasiko observe`? | B5, Task 2 | If not, nine agents are invisible in the control plane. Deploy blocker, not polish. |
| 3 | The literal field names in Wire responses per action. | B2, Task 1 | A guessed field name is an empty dashboard on stage. |
| 4 | Does the Play Store listing yield review text, rating and date through URL Scraper with `useBrowser: true`? | B2, Task 2 | Decides six sources or seven. |
| 5 | Does DronaHQ's WhatsApp trigger send outbound, or do we need the Twilio connector? | B5, Task 7 | The 9am brief depends on it. Meta's 24-hour window may force a template. |
| 6 | Does DronaHQ's Charts control expose the Plotly `hole` config for a donut? | B5, Task 6 | Cosmetic. Ship a pie if not. |

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
