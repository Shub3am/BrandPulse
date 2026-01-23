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
| B2 (Tasks 4-8) | B1 | **Nothing in `internal/` exists except `models/`.** No `anakin`, `ids`, `hashing`, `redact`, `llm`, `db`, `stats`, `prompts`, `obs`, `a2a`, and no `internal/models/agentio.go`, so no `CollectInput`/`MentionBatch`. An adapter cannot compile against packages that are not there, and the contracts rule forbids me writing them in a worktree. Tasks 1, 2 and 3 are doc-and-data only and proceed; 4 through 8 cannot start. | open, **hard blocker** |
| B2 | B1 | **CONTRACTS §4 specifies an impossible adapter layout.** It puts every adapter at a flat file in one package, each exporting `Fetch`, which is a redeclaration error the moment the second one lands. Detail and the proposed wording are in the resolved entry below. I am building against a directory per source meanwhile, so B1's ruling only has to confirm or rename, not reshape. | open, needs a ruling |
| B2 | B7 | `docs/SOURCE-STRATEGY.md` says seven sources and still lists `amazon` as a mention source. Amazon is dead (evidence below). The file lives on `main`, which I do not write to. | open |

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

### 2026-09-20 — B2 Task 1 — Wire field names, live. Open question #3 is closed.

Full schemas, with the real JSON, are in
[docs/research/wire-schemas.md](docs/research/wire-schemas.md). Commit that file
before you write an adapter; it is the thing that stops nine agents each
guessing. Spend: **9 of the ≤ 10 credit Task 1 budget**, 5 billed calls plus 3
deliberate 400s, which Anakin does not bill.

**The payload is two envelopes deep and the inner key changes per action.**
This is the finding that matters. `GET /v1/wire/jobs/{id}` returns
`{"job": {"data": {"data": {...}}}}`, and the array inside that second `data`
is named differently by every action:

| Action | Path to the array |
|---|---|
| `rt_search` | `job.data.data.posts` |
| `yt_search` | `job.data.data.data` |
| `am_search_products` | `job.data.data.products` |
| `am_product_reviews` | `job.data.data.reviews` |

Unmarshal one level short and Go gives you a struct of zero values **and a nil
error**. That is an empty dashboard on stage with nothing in the logs. Decode
the exact path or nothing.

**`amazon` is dead as a mention source. It is not a budget cut, it is a
correctness one.** `am_product_reviews` accepts only `asin`: no page, no limit,
no sort. On `B0CZ426LLT` it returned 4 reviews of a stated 16 309, on
`B0C7QHHT63` 6 of 14 401, and on all ten of them `title` and `text` were the
empty string. A `Mention` built from that has `Text: ""`, so every Amazon
mention hashes to the same `ContentHash` and dedupe collapses the lot to one
row. Separately the catalogue is `amazon.com`, not `amazon.in`: prices in USD,
reviewers in the UK, UAE, Italy and Australia. Wrong country and no text.
B3, this removes a source from your labelled set, not a few rows.

**Reddit carries no engagement numbers.** `score`, `upvote_ratio` and
`num_comments` were null on all 15 posts of a `rt_search`, and the `selftext`
tail shows why: the action reads Reddit's RSS. Recovering them means
`rt_post_details` at 2 credits a post, 30 credits for counters nothing reads
today. Decision: Reddit mentions ship with `Engagement` zeroed. **B4, the
`influencer_mention` rule cannot fire on Reddit.** Do not treat a zero there as
a measurement.

**`country`, `freshness` and `date_range` on `/v1/search` are accepted and
silently ignored.** HTTP 200 either way, so a green response proves nothing.
Proof: under `freshness: "week"` a result came back dated 2022-09-14. Date
filtering is therefore client-side on the `date` field, and `date` is sometimes
the empty string. **An empty date must drop the mention.** Defaulting it to
`time.Now()` is the quiet version of this bug: it would park old posts inside
the 14-day window and poison every baseline `bp-detector` computes.

**Amazon Wire actions cost 1 credit, not 2.** Reddit's seven actions cost 2,
Amazon's fifteen `am_*` cost 1. More usefully, `credits_per_call` is a live
field on the catalogue response, so `internal/anakin` should read the cost
rather than carry a table that goes stale. §6 of
[docs/research/anakin.md](docs/research/anakin.md) is corrected, and the
recording estimate drops from ~136 to **~112**, ceiling ~182 of 300.

**Correction to the `prompt` entry above.** That entry says
"`docs/research/anakin.md` §3 documents the Search body as carrying a query".
It does not, and never did: §3 already said `prompt`, at two places. `prompt`
is confirmed live, sending `query` returns
`400 {"error":"invalid_request","message":"Prompt is required"}`. Every other
`query` in that file is a **Wire action parameter**, where `query` is the
correct name, verified live on `rt_search`, `yt_search` and
`am_search_products`. So the field name differs between the two surfaces and
both docs were right. Nothing in `anakin.md` needed that fix; this note is the
thing that was stale.

Also settled cheaply: `/v1/search` `limit` is bounded 0 to 20, and the `snippet`
is bigger than "snippet" suggests. The one I measured was **2 966 characters**,
enough to classify without chaining the URL Scraper. That is why the blanket
20-page scrape came out of the budget.

### 2026-09-20 — B2 Task 1 — CONTRACTS §4 cannot compile as written

§4 puts each adapter at a flat file in one package, `reddit.go`, `youtube.go`
and so on, each exporting `Fetch`. The second one is a redeclaration error:
`Fetch redeclared in this block`. The B2 brief already flagged it and told me
to post the correction here.

What I am building against, so B1 can confirm or rename rather than redesign:
one directory per source, `internal/anakin/sources/reddit/`, each with
`func Fetch(...)`, and `internal/anakin/sources/registry.go` holding the
hand-written `var Registry = map[models.Source]Adapter{...}`. The `Adapter`
signature itself is unchanged from §4 and I am not touching it. If B1 prefers
one package, the fix is `FetchReddit`, `FetchYouTube` and so on, which is the
same amount of work; what cannot survive is the current text.

### 2026-09-20 — B2 Task 2 — the Play Store probe passes on fields and fails on volume. Six sources.

Closes open question #4. Evidence and the sample markup are in
[docs/research/playstore-probe.md](docs/research/playstore-probe.md). Spend
**4 credits**, one over budget; the mistake is owned at the bottom of that file
and in the numbers table below.

**The fields are all there, in `html`, not in `markdown`.** Review text, star
rating, review date, a stable UUID review id and the helpful count all extract
1:1 from a single `useBrowser: true` scrape of the listing. Ask for
`formats: ["markdown","html"]`, same 1 credit, parse the `html`, ignore the
`markdown`.

**Markdown alone would have shipped a bug that looked fine.** The star rating is
an empty div carrying an `aria-label`, so every converter drops it: `out of 5`
appears zero times in 11 925 characters of returned markdown. Worse, the review
date does not vanish, it **moves**: markdown flattens it to directly under the
developer's name and above the developer's reply, where any reader would call
it the reply date. In the DOM they are two different elements. `cleanedHtml` is
not a substitute either, it strips the rating and the reviewer name.

**Volume is 3 reviews per listing and nothing moved it.** Plain listing: 3.
`&showAllReviews=true`: 3, byte-identical markdown, that old trick is dead.
Click "See all reviews" plus scrolls via the `actions` array: 3, though the
click did fire (20 s to 40.5 s, HTML grew 14 KB). Play fills that dialog from an
internal RPC on the dialog's own scroll container. Two retries, brief's cap,
stop.

**So the source count is six, and the review-bomb story is off the table.**
`reddit`, `youtube`, `news`, `web`, `appstore`, `playstore`. **B5: the README
and the pitch say six sources**, and neither should imply we watch Play Store
review velocity, because at 3 reviews a run we do not. What `playstore` does
give us is a real star rating, which only `appstore` otherwise has, and
`Mention.Rating` is a `*float64` for exactly that reason.

**B1, two things for `internal/anakin`.** First, the `actions` DSL is now
known, free, from four deliberate 400s: `wait` (`milliseconds`, 1 to 15 000),
`wait_for` (`selector`), `click` (`selector`), `press` (`key`), `scroll` (no
field), `write` (unverified). Second, **`{"type":"scroll"}` is valid with no
required field**, returns 200 and bills a scrape. That is the credit I lost, and
the general lesson is that probing a schema with bad input is free only while
the input stays bad.

**The playstore adapter must error, never return an empty batch.** Its
selectors are Google's obfuscated build output (`h3YV2d`, `bp9Aid`, `X5PpBb`,
`iXRFPc`) and they rotate. A zero-review parse is indistinguishable from a quiet
week unless the adapter says so out loud. Recorded fixtures keep `replay` green
whatever Google does.

Note for B3's labelled set: the three reviews came back 3/5, 2/5 and 1/5 on an
app rated 4.6 overall. Play's default surface is "most helpful", which upvotes
complaints. Useful for listening, but it is a sampling bias, not a sentiment
collapse.

---

## Open questions

The things nobody has verified yet. Claim one by putting your track in the
"owner" column, and move it to "Resolved unknowns" when you have an answer.

| # | Question | Owner | Why it matters |
|---|---|---|---|
| 1 | Which `a2a-go/v2` helper emits a **JSON** artifact? | B1, Task 1 | All nine agents need it. Lands as `a2a.JSONArtifact`. |
| 2 | How is a peer agent addressed through the Nasiko proxy? The env var name is unverified. | B5, Task 1 | Lands as `a2a.Call`. No agent writes a peer URL directly. |
| 7 | Do OTel traces from a self-instrumented Go container actually reach `nasiko observe`? | B5, Task 2 | If not, nine agents are invisible in the control plane. Deploy blocker, not polish. |
| ~~3~~ | ~~The literal field names in Wire responses per action.~~ | ~~B2, Task 1~~ | **closed 2026-09-20**, see "Wire field names, live" above |
| ~~4~~ | ~~Does the Play Store listing yield review text, rating and date through URL Scraper with `useBrowser: true`?~~ | ~~B2, Task 2~~ | **closed 2026-09-20**, yes from `html`, but only 3 reviews a listing. Six sources. |
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

## B7 now owns `main`. Nobody else commits there.

From this commit on, the primary checkout
`/Users/shubhamvs/Desktop/anakin-hack/brandpulse` and the `main` branch belong
to B7 alone. Every scope change, doc fix and contract edit up to here was landed
on `main` from that same checkout, which was safe only because B7 had not
started. Two writers on one working tree is uncommitted work destroyed, not a
merge conflict, because git cannot help with edits it has never seen.

So: if you are B1 through B6 and you need something changed on `main`, you do
not go and change it. You add a row to "Open blockers" naming B7, or you open
the PR and let B7 merge it. B1 still owns `internal/models`, `001_init.sql` and
`CONTRACTS.md`, and still lands them through a PR that B7 merges. The contracts
rule did not move, the write path to `main` did.

Merge order is B1, B2, B3, B4, B6, B5, green between each.

---

## Numbers for the pitch

Filled in as they are measured. **Real output only.** An estimate goes in
brackets and is replaced, never quietly promoted.

| Number | Value | Source | Owner |
|---|---|---|---|
| Credits spent on Task 1 schema reads | **9** of ≤ 10 | B2 Task 1, actual | B2 |
| Credits spent on the Play Store probe | **4** of 3, one over, see the Task 2 entry | B2 Task 2, actual | B2 |
| Credits spent recording fixtures | _(est. ~103, was ~136 before Amazon came out)_ | B2 Task 7 actual | B2 |
| Mentions in the fixture corpus | — | `fixtures/` count | B2 |
| Sources shipped | **6** | B2 Task 2, settled | B2 |
| Play Store reviews per listing per run | **3** | B2 Task 2, actual | B2 |
| Sentiment accuracy | — | `go run ./eval/accuracy` | B3 |
| Intent accuracy | — | `go run ./eval/accuracy` | B3 |
| Cost per brand-day, cold | _(target < ₹15)_ | `go run ./eval/cost` | B3 |
| Cost per brand-day, warm cache | — | `go run ./eval/cost` | B3 |
| Agent container image size | — | `docker images` after build | B5 |
| Agents deployed on Nasiko | 0 / 9 | `nasiko deploy` | B5 |
| Time from mention to alert on screen | — | `demo/run_demo.sh` timing output | B4 |
| Nasiko PR | — | link | B5 |
