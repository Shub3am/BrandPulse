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
| ~~B5~~ | ~~you~~ | ~~No deploy pipeline~~ | **closed 2026-09-20**, `ci.yml` and `deploy.yml` exist, see below |
| ~~B5, B6~~ | ~~you~~ | ~~No hosting target for `web/` and `bff/`~~ | **closed 2026-09-20**, ruled: localhost for the demo, see below |
| ~~B1~~ | ~~B7~~ | ~~Fast-forward `main` to `track/b1-core`~~ | **closed 2026-09-20**, merged as `73ef873`, see below |
| ~~B6~~ | ~~B7~~ | ~~`track/b6-web` into `main`, `bff/` is new and `web/` drops `demoData`~~ | **closed 2026-09-20**, merged, see below |
| ~~B6 (Task 6)~~ | ~~you~~ | ~~A ruling on `.github/workflows/web.yml`~~ | **closed 2026-09-20**, `ci.yml` has a `web` job doing exactly the build-and-check-only run B6 recommended |
| B1 | B4 | **`a2a.Call` is unimplemented.** `internal/a2a` serves nine agents, but the client half that `bp-orchestrator` calls peers with returns `"a2a: Call is not implemented"` on every invocation. It is outside B1 Task 11's checklist and nothing imports it yet, so it is not stubbed into something that returns a plausible zero value. Whoever writes `bp-orchestrator` needs it, and it needs the Nasiko proxy address and routing header, which no track has verified yet. | open |
| B1 | B7 | **`ci.yml` passes when the tests fail.** `go test ./... \| tee test.log` reports `tee`'s exit status, and Actions runs steps under `bash -e`, which does not imply `pipefail`. Also missing: `-race`, a Postgres 16 service (`internal/db` silently `t.Skip()`s its whole suite without `DATABASE_URL`), and the `http.DefaultClient` grep. Detail and a fix in the B1 entry below. Your file, your call, I have not touched it. | open |
| B1 | B7 | **Second fast-forward of `main` to `track/b1-core`.** `73ef873` took the import surface; this is the eight commits after it: `anakin`, `llm`, `stats`, `prompts`, `obs`, `internal/a2a`, and the CONTRACTS §1 and §3 corrections. Rebased onto `main@631c9fc`, so `git merge --ff-only track/b1-core` again. | open, **first in the merge order** |
| ~~B2 (Tasks 4-8)~~ | ~~B1~~ | ~~Nothing in `internal/` exists except `models/`, so an adapter cannot compile~~ | **closed 2026-09-20**, `73ef873` merged into `track/b2-collect` |
| B2, B3, B4 | B1 | **`ids.New` and `hashing.ContentHash` still `panic("not implemented")`.** They are on the path every mention takes: `mentions.Stamp` calls both on every draft, so any adapter that actually returns a mention panics rather than failing. 23 stubs across `internal/` are in this state. Every adapter test in `internal/anakin/sources/` currently drives `Fetch` with a 2020 collection window so the filter empties the batch and `Stamp` loops zero times. That is a workaround, not coverage: the moment B1 implements these two, those windows move to real dates and the assertions get stronger. | open, blocks Task 7 recording |
| B2 | B1 | **CONTRACTS §4 specifies an impossible adapter layout.** It puts every adapter at a flat file in one package, each exporting `Fetch`, which is a redeclaration error the moment the second one lands. Detail and the proposed wording are in the resolved entry below. I am building against a directory per source meanwhile, so B1's ruling only has to confirm or rename, not reshape. | open, needs a ruling |
| B2 | B7 | `docs/SOURCE-STRATEGY.md` says seven sources and still lists `amazon` as a mention source. Amazon is dead (evidence below). The file lives on `main`, which I do not write to. | open |
| B2, B3 | B1 | **`anakin.NewHTTPClient` panics, so the Task 7 recording cannot run and the Task 8 labelling set cannot be sampled.** `demo/record` is written, tested and its dry run prints a 63-credit estimate, but `-confirm` panics at `internal/anakin/anakin.go:194`. B3 is waiting on `fixtures/labelled/mentions.jsonl` and I will not fabricate it: the repo rule is that a source which did not return data does not appear in a fixture, a count or a sentence. **The moment `NewHTTPClient` and `record` mode land I run the session and B3 has the corpus the same hour.** | open, blocks Tasks 7 and 8 |

In Go a missing package is a compile error for everyone downstream, not a
runtime `ImportError` in one test. That is why B1's signature commit is its own
blocker row and why it comes before B1's own implementation.

---

## Resolved unknowns

Each entry: the question, the answer, and how it was verified. An unverified
answer stays in "Open questions".

### 2026-09-20 — main — there is a pipeline now, and hosting is ruled

**CI is live.** `.github/workflows/ci.yml` runs on `main` and every `track/**`
push: `go mod tidy` drift check, `go build`, `go vet`, `go test` under
`BP_FIXTURE_MODE=replay`, a `web/` typecheck and build that skips itself while
`web/package.json` is absent, and an AgentCard check. It never needs a
credential.

It also prints how many packages have no test files, on every run. Today that
is all of them, so `go test` passes vacuously, and "CI is green" must not come
to mean "this is tested" while that is true.

**`scripts/check_agent_cards.py` is runnable on your laptop**, and it is the
half of `nasiko validate` that needs no cluster. Run it before you push a card.
Against the seven cards that exist today:

```
7 card(s) checked, all 7 failed
::error agents/bp-{briefer,clusterer,detector,enricher,orchestrator,responder,sov}/AgentCard.json
   no supportedInterfaces[], so an A2A 1.0 consumer cannot see this agent's transport
```

Nothing else was flagged, so `protocolVersion`, `skills` and the eight
top-level fields are already right in all seven. **B3 and B4: this is a
one-line addition per card**, and until you make it, CI is red on your branch.

**Deploy is live and deliberately hard to fire.** `deploy.yml` is
`workflow_dispatch` only, never on push, and asks you to type the agent name
twice. While `NASIKO_CONTROL_PLANE_URL` and `NASIKO_AUTH_TOKEN` are unset it
warns and skips instead of failing, because a pipeline that has been red all
day is a pipeline everyone ignores. It builds the CLI from source, since
`nasiko.md` §8 found no published binary.

`actionlint` is clean on both files.

**Hosting ruling: `web/` and `bff/` run on localhost for the demo**, against
the Docker Postgres on 5433 and the deployed Nasiko agent URLs. No Vercel, no
hosted Postgres, no new accounts. B6, build for that and stop waiting.

**Browser driver ruling: `clickr-runner`** for the DronaHQ console work in B5
Tasks 7 and 8, once the console login exists.

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

### 2026-09-20 — B2 Task 3 — the demo brand is boAt. Two traps came with it.

`demo/brand.json` is committed. It decodes into `models.BrandProfile` with
`DisallowUnknownFields` and passes `Validate()`; all six `Source` values return
`Valid() == true`. Spend for this task: **4 credits**, two `rt_search` calls.

| Field | Value |
|---|---|
| `brand_id` | `boat-lifestyle` |
| `name` | boAt (Imagine Marketing Limited) |
| `website` | https://www.boat-lifestyle.com/ |
| `competitors` | Noise, boult |
| `sources` | reddit, youtube, news, web, appstore, playstore |
| `source_handles.playstore` | `com.boAt.hearables` |
| `source_handles.appstore` | `6475390290` |
| `source_handles.amazon` | `B0CZ426LLT` |

Why boAt: the discourse is already proven, not assumed. Task 1's `rt_search`
returned 15 real posts, `yt_search` returned review videos with comment threads,
and the two ASINs carry 16 309 and 14 401 reviews. The Play listing has 2.7L
reviews and 1Cr+ downloads. Nothing here is a hope.

**Trap 1: `match: true` on `rt_search` is a phrase filter and it returns a
silent zero.** The query `"boAt vs Noise vs boult earbuds"` returned
`post_count: 0`, `posts: []`, `error: null` and **`posts_dropped_by_filter:
22`**. Reddit found 22 posts and the client-side filter discarded all of them,
because it wants the entire query string present in the post. **B2's own
adapter and anyone else touching Wire: one brand term per call, and read
`posts_dropped_by_filter`.** Zero posts with a non-zero drop count means the
query was too specific; zero with a zero drop count means Reddit had nothing.
Those are different incidents and the response already tells them apart.

**Trap 2: the brand names are polluted, and `NegativeKeywords` is now
load-bearing rather than decorative.** A single-token `rt_search` for `boult`
returned 19 posts of which most were cricket, because Trent Boult is a New
Zealand fast bowler: r/Cricket, r/CricketShitpost, r/RCB, "ETPL FINAL" threads.
Exactly one of the top twelve was about the audio brand. "boAt" has the same
problem with actual boats and "Noise" with the English word. So `keywords` are
all phrases (`boAt Airdopes`, `boAt Rockerz`, never bare `boAt`) and
`negative_keywords` carries 13 terms covering cricket and sailing. **B3: this
is a real precision problem in the labelled set, not a tidy demo of a feature.**

**`source_handles.appstore` is boAt Shopping, not boAt Hearables, and that is
deliberate.** Apple's review RSS returns **zero entries for boAt Hearables**,
for boAt Wearables, for NoiseFit and for both GOBOULT apps, while returning 36
for boAt Shopping and 50 for Zomato. Three identical attempts each, so it is
deterministic, not flaky, and it has nothing to do with popularity: NoiseFit
has 122 244 ratings and returns nothing. The table is in
[docs/research/anakin.md](docs/research/anakin.md) §1. **B1 and B5: validate
every App Store id by fetching it once before it goes into a profile**, because
the feed returns HTTP 200 and a well-formed envelope with the `entry` key
simply absent, which decodes to an empty slice and looks identical to "no
reviews this week".

The `amazon` ASIN is in `source_handles` because the brief asked for it and it
is a useful reference, but `amazon` is deliberately **not** in `sources`. See
the Task 1 entry for why.

### 2026-09-20 — B2 Task 4 — six adapters ship, and I have to correct my own Task 3 entry

All six sources are written, tested and on `track/b2-collect`: `reddit`,
`youtube`, `news`, `web`, `appstore`, `playstore`. `go build ./...`,
`go vet ./...` and `BP_FIXTURE_MODE=replay go test ./internal/anakin/sources/...`
are green. No source is faked and none is registered that was not probed.

**Correction to the Task 3 entry above. Do not act on it as written.** It says
"B1 and B5: validate every App Store id by fetching it once before it goes into
a profile", on the reasoning that an id that returns nothing today returns
nothing on stage. That reasoning is wrong. Re-measuring across four URL forms
with six repeated runs, **boAt Shopping went 36 to 0 and boAt Hearables went 0
to 50**, same ids, same storefront, same sort. Emptiness follows the URL form
and the day, not the app. A one-off validation proves nothing about stage. What
protects us instead is that the `appstore` adapter **errors on a zero-entry
feed** rather than returning an empty slice, so a bad fetch is loud. The
measurement matrix is in [docs/research/anakin.md](docs/research/anakin.md) §1
under "Correction, same day, Task 4". `demo/brand.json` now carries
`appstore: "1592550875"`, not `6475390290`.

**For B1, four things about `internal/anakin` that my adapters now depend on.**
None of them need a contract change; they need confirming or fixing in your
implementation:

1. **`Wire` must return the payload, not the envelope.** Every adapter
   unmarshals the `json.RawMessage` straight into its own response struct, so
   `Wire` has to have stripped both layers of the double nesting first
   (`.data.data`, per wire-schemas.md §1). Same for `Scrape`: fields arrive at
   the response's top level with no envelope at all, verified live.
2. **The registry shipped as `Adapters`, not `Registry`.** My Task 1 entry said
   `var Registry = map[models.Source]Adapter{...}`. It is
   `var Adapters = map[models.Source]FetchFunc{...}` plus
   `func For(source) (FetchFunc, bool)` in `internal/anakin/sources/registry.go`.
   `For` exists because a caller ranging over `BrandProfile.Sources` has to tell
   "not collected" from "collected nothing", and a nil out of the map does not.
   The per-source directory layout is unchanged and the §4 signature is exact.
3. **`WireOpt.Params` is load-bearing.** `yt_comments` needs
   `Params: {"video_id": ..., "include_replies": true}` and there is no field
   for either. Whatever `Params` does, it has to reach the action's own
   parameter object, not the top level.
4. **`ErrBudgetExceeded` has to survive wrapping.** Every adapter checks
   `errors.Is(err, anakin.ErrBudgetExceeded)` and treats it as stop-the-run
   rather than skip-this-keyword, returning the partial batch it already paid
   for. If the real client returns a fresh error instead of wrapping the
   sentinel, six adapters will keep spending after the ceiling.

**`go.mod` gained `golang.org/x/net v0.47.0`**, for `html.Parse` in the
`playstore` adapter. It is the only new dependency B2 adds. Google's listing is
obfuscated build output with rotating class names, and a regex over it would
fail silently on the next rotation; the parser fails loudly instead.

**`yt_comments` is no longer an open schema.** Read live for 3 credits, written
up in [wire-schemas.md](docs/research/wire-schemas.md) §4. Every open schema
from Task 1 is now closed. Watch out for three shapes if you touch YouTube:
`likes` is a string, `published` is relative prose and can carry an
` (edited)` suffix, and `reply_count` is `null` rather than `0`.

**Credits: 21 spent of 300.** Task 1 nine, Task 2 four, Task 3 four, Task 4
four. The Task 7 recording estimate is ~103 and the ceiling including headroom
is ~194 of 300.

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
### 2026-09-20 18:40 — B6 — the A2A v1.0 wire format, verified. B4 and B5 need this.

The BFF calls `bp-orchestrator` over A2A, so I had to pin the actual v1.0 wire
format rather than the 0.x shape most blog posts show. Verified against the
normative sources, not from memory: the `a2aproject/A2A` specification's
`a2a.proto`, `a2a-go` v2.5.0 `a2a/core.go`, and its `internal/jsonrpc`.

Three differences from 0.x, and **every one of them fails silently** if guessed:

1. The method is `SendMessage`, not `message/send`.
2. Role and state are **enum names**: `"ROLE_USER"`, `"TASK_STATE_COMPLETED"`,
   `"TASK_STATE_FAILED"`. Not `"user"` and not `"completed"`.
3. `Part` has **no `kind` discriminator**. It is a flattened oneof, so a JSON
   part is `{"data": {...}, "mediaType": "application/json"}` and adding
   `"kind": "data"` makes the peer reject the message.

The version travels as an HTTP header, `A2A-Version: 1.0`, not in the body.

Request:

```json
{"jsonrpc":"2.0","id":1,"method":"SendMessage","params":{"message":{
  "messageId":"<uuid>","role":"ROLE_USER",
  "parts":[{"data":{"brand_id":"lumeo","trigger":"on_demand"},
            "mediaType":"application/json"}]}}}
```

The artifact comes back at `result.task.artifacts[0].parts[0].data`.

Verified by `bff/src/orchestrator.test.ts`, which runs a real `node:http` stub
agent rather than a mocked `fetch`, because a mock would agree with whatever I
believed. Nine tests, no network, no credits.

### 2026-09-20 18:40 — B6 — `ShareOfVoice` has no producer and nowhere to live

`internal/models/models.go` defines `ShareOfVoice` (with a nested
`BySource map[Source]map[string]float64`), `001_init.sql` has no
`share_of_voice` table, `RunRecord` has no SOV field, and no agent brief names
it as an artifact. So the BFF has nothing to read. `GET /api/brands/:id/pulse`
returns `share_of_voice: null` and the dashboard renders
`brief.numbers.share_of_voice` out of `briefs.payload` instead, which is the
only place the figure actually exists. This is not a blocker for me; flagging it
so nobody later assumes the BFF dropped it.

### 2026-09-20 — B1 — the Anakin client works, and three of its rules will surprise you

`internal/anakin` is implemented: all five `Client` methods, the fetch cache,
the credit budget, fixture replay and record, and `anakin.NoNetwork()`. The
suite is green in replay with `-race`, with no key and no network. Build against
it now.

Three things are not in the brief and you will hit them on your first call.

**Replay needs `Config.MaxCredits`, or `NewHTTPClient` refuses to build.**
Replay is cut off from Postgres on purpose, so the budget has no `runs` rows and
no `brands.daily_credit_budget` to read a ceiling from. A zero would either mean
"unlimited", which is how 300 credits disappear, or fail on every call. It fails
at construction instead, with a message saying so. Pick any number.

**The `<source>` in `fixtures/<source>/<query_hash>.json` is the Postgres enum,
not the method.** `fetch_cache.source` is `source`, so the cache key and the
fixture path must be a member of it. `Wire`'s `platform` argument therefore has
to be a `models.Source` such as `reddit` or `youtube`, and it is rejected if it
is not. `Search`, `Scrape`, `Map` and `Crawl` carry no source in their
signatures, so they all file under `web`. That is a cache key, not a claim about
the mention: your adapter still sets the real `Mention.Source`.

**A cache hit is free and a repeat inside one run is a cache hit.** The cache
has two layers: `fetch_cache` in Postgres, and a per-client map in front of it
because a fan-out repeats the same query within a run and because a replay
client has no pool at all. Both count as `Stats().CacheHits` and neither spends.
So do not deduplicate queries before calling the client; it is already done, and
doing it yourself means doing it differently.

Two more worth knowing. A missing fixture is an error **naming the path it
wanted**, never an empty list, so if you see zero mentions that is a real zero.
And credits are held **before** the HTTP call and not refunded when it fails,
although Anakin does not bill a failed call: with 300 credits and no second
allocation, refusing one call too many is the right direction to be wrong in.

`fixtures/_canned/` is mine, five hand-written payloads that exist so the five
methods have a test. **Nothing in it is a recording** and no count or chart may
be fed from it; every file says so in a `_canned` key and in its own text. Real
recordings are B2's, in `fixtures/<source>/`.

Unverified and isolated, one function each, so B2's live run is a small fix and
not a rewrite: `parseJobID` (Wire documents `jobId`; the map and crawl 202
bodies are not documented, so `id` and `job_id` are accepted too),
`parseJobStatus` (the Wire poll envelope is assumed to cover map and crawl), and
`mapJobPath` / `crawlJobPath` (the poll paths follow the one documented example,
`/wire/jobs/{id}`). The Postgres cache path itself has no test: B1's suite runs
in replay, which never touches Postgres, so B2's first live run is the first
time those two queries execute.

CONTRACTS §3 gained a paragraph for the first two rules above. No signature
changed; `NoNetwork` was already in §3 from Task 1.

---

### 2026-09-20 — B1 — `llm.ChatJSON` works, and B3 owns the rupee it reports

`internal/llm` is implemented. `ChatJSON`, `Embed`, `Opt` and `Usage` are
exactly the §3 signatures, so nothing you wrote against the stub changes.

**Your schema struct must not use `omitempty`.** Strict mode requires every
property to appear in `required`, and the reflector only marks a field required
when it has no `omitempty`. A struct with `omitempty` compiles here and is
rejected by the provider, which is the worst place to find out. Exported
fields, `json` tags, no `omitempty`.

**A failed call still returns a non-zero `Usage`.** An unparseable reply is
retried once with a "return only valid JSON" nudge, and both attempts are
billed, so both are counted. If you are summing cost, sum it on the error path
too or the demo under-reports.

**The cost table is `internal/llm/cost.go`, one map, and B3's `eval/cost` is
its only consumer.** Do not put a second price anywhere. Three numbers in it
need a second pair of eyes before the pitch:

- The four rows are keyed on the **catalog name the router reports back**
  (`openai/gpt-4o` and so on), not on what an agent asked for, because Nasiko
  discards the request's `model` field. Lookup also tries the bare name, in
  case the proxy strips its own prefix on the way back. **B3: paste one real
  `resp.Model` string into this thread after the first live call.** If it is a
  fifth form, that is a one-line fix and better found now.
- An unknown model is charged at the **dearest** row, never at zero. A silent
  0.00 is how a cost dashboard lies. If `eval/cost` prints a number that looks
  too round and too high, look for a missing row before you look for a bug.
- `gemini/gemini-1.5-pro` is priced at $3.50/$10.50 per 1M from
  llmpricecheck.com, because **Google has retired 1.5 Pro from its own
  published price list** and there is no first-party figure to cite. It is the
  highest number in circulation, deliberately. The other three are
  first-party (OpenAI and Anthropic docs, read today). Rupees use 95.885/USD,
  the RBI reference rate for 2026-09-17. Re-read it on demo morning.

Nine tests, all on a swapped `*http.Client`, no live call, and CI needs no
`OPENAI_API_KEY`: the SDK does not error on a missing key, so "unset" is a
valid CI state rather than a thing to work around. `go mod tidy` promoted
`openai-go/v3` and `invopop/jsonschema` out of indirect and pulled their
transitive set; it also moved `golang.org/x/sync` to v0.22.0 and
`golang.org/x/text` to v0.40.0 in the shared `go.mod`. Nothing in the repo
pinned either.

---

### 2026-09-20 — B1 — `stats` is in, and B4 should read the two denominators

`internal/stats` is implemented. `ZScore`, `ComputeBaseline`, `HourBucket` and
`DayBucket` are the §3 signatures unchanged.

**B4, these two are judgement calls and they move your thresholds**, so argue
with them now rather than at 3am:

- **A source's hourly mean is divided by every hour in the window, not by the
  hours it posted in.** A source that posts 24 mentions in one hour a fortnight
  has a mean of 0.07, not 24. Averaging only the busy hours would make "quiet
  then loud" score z = 0, which is the entire spike rule. `padWithQuietHours`
  is where that happens.
- **Negative share is averaged only over hours that had an enriched mention.**
  Here the zeros are wrong: 333 empty hours counted as 0.0 negativity would put
  `NegativeShareMean` near zero and make any negativity at all look like a
  crisis. An hour with no data is not an hour with no negativity.

Std is population, not sample, because the window is the whole population.
`ZScore` returns 0.0 on std == 0, so a brand that posted the identical count
every hour for fourteen days is quiet, not +Inf on all five rules at once.

`MeanRating` is filled from `rating IS NOT NULL` alone rather than from a
second list of the three review sources. `Source.IsReviewSource` names them and
no other adapter writes a rating, so a list here would be a copy to forget.
**B2: if any non-review adapter ever sets `Mention.Rating`, tell me**, because
that silently widens this average.

`internal/anakin`'s `hourBucket`/`dayBucket` now call `stats.HourBucket` and
`stats.DayBucket` instead of carrying their own copy of the format. Same
strings, no fixture path changes, cache keys unaffected. Three consumers, one
implementation, which was the point of putting the buckets in `stats`.

The database tests create `brd_stats_test_<pid>_<nanos>` and delete it in
`t.Cleanup`. They touch no row they did not insert, so they are safe to run
while you are working.

---

### 2026-09-20 — B1 — `obs.Setup` is in, and B5 still owns the half that matters

`internal/obs` is implemented, one package for all nine agents rather than the
~150 lines per agent the research priced. First line of every `main()`:

```go
shutdown, err := obs.Setup("bp-collector")
if err != nil { return err }
defer shutdown(context.Background())
```

**With `OTEL_EXPORTER_OTLP_ENDPOINT` unset it returns a no-op shutdown and a
nil error.** CI has no collector, and an agent that refuses to start without one
is an agent that never runs in a test. The returned shutdown is safe to call
twice and safe under concurrent callers, which matters because you will defer
it and a signal handler will also call it. `obs` does not add that: the SDK's
`TracerProvider.Shutdown` already guards itself with a compare-and-swap and
returns nil after the first call. I had a `sync.Once` wrapper around it, then
mutation-tested it, found removing it changed no test, read
`sdk@v1.46.0/trace/provider.go:298`, and deleted it. The tests stayed, so an
SDK upgrade that withdraws the guarantee fails here rather than in your logs.

**B5: the deploy blocker is still yours.** A span that is emitted and never
received is worth nothing. `Setup` proves a provider was built, not that
`nasiko observe` sees anything, and no test here can prove that. Check one real
trace before the other eight agents deploy.

The exporter reads `OTEL_EXPORTER_OTLP_ENDPOINT` itself rather than being
handed it, so `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` and the rest of the
`OTEL_*` set behave exactly as the spec says. Transport is OTLP over HTTP.
`resource.Merge` is used rather than a replacement, so a schema-URL mismatch
after an SDK upgrade fails loudly instead of silently dropping `service.name`.

**No agent wires `otelhttp`.** `internal/a2a` does it once in Task 11.

**Stale line in my own brief, for the record:** B1-core.md Task 10 says the ADR
calls this `obs.Init` and assigns it to B5, and asks me to correct two lines.
`docs/decisions/001-go-for-agents.md` already says `obs.Setup` and already says
B1 writes it and B5 verifies it; that was fixed on `main` in 97b02f5, before
the tracks branched. Nothing to correct, and I am not editing a document to
make a checklist true.

`go get` added the OTel SDK and its transitive set to the shared `go.mod`:
`go.opentelemetry.io/otel` v1.46.0, `otel/sdk`, the `otlptracehttp` exporter,
`google.golang.org/grpc` v1.83.1 and `protobuf` v1.36.12. It also moved
`golang.org/x/net` to v0.58.0 and `golang.org/x/text` to v0.41.0.

### 2026-09-20 — B1 — `internal/a2a` is in, and it moves two fields the contract named

`a2a.Serve(card, handler)` works end to end: a real JSON-RPC request over
`httptest` comes back as one task in `TASK_STATE_COMPLETED` carrying one
artifact. Nineteen tests, `-race`, no network. Two things in `CONTRACTS.md` were
wrong, both verified against `a2a-go/v2@v2.5.0` source rather than against the
brief, and both are now corrected in §1 and §3 on this branch.

**B5, the card shape.** `protocolVersion` is not a top-level field. `AgentCard`
in v2.5.0 has no such field at all (`a2a/agent.go:136`) and the version lives on
each `supportedInterfaces[]` entry, next to `url` and `protocolBinding`. The
note above at "Nasiko AgentCard `protocolVersion` must be `1.0`" is still right
about the value and was never wrong; it just does not say where the field goes,
and the older A2A material puts it at the top level. That shape parses cleanly,
silently leaves `supportedInterfaces` empty, and the cluster answers `-32009`.
`LoadCard` now rejects it at startup with an error naming the right location, so
you read this once instead of debugging it nine times. §3 prints the exact JSON.
`JSONRPC` is the only binding `Serve` mounts; a `GRPC`-only card is refused too.

**B6, the envelope shape.** The artifact's mime type is `mediaType` **on the
part**, not `mimeType` on the artifact. `Artifact` has no mime field in v2.5.0.
The body is a `Data` part, so it arrives inline as real JSON and the BFF binds a
field directly; a `Raw` part would have arrived base64 and cost you a decode.
The verified wire envelope is pasted in `CONTRACTS.md` §1.

**B7, two things for the smoke tests.** The JSON-RPC method is `"SendMessage"`,
not the `"message/send"` older A2A material prints. v2.5.0 renamed them;
`internal/jsonrpc/jsonrpc.go:38` in the SDK is the list. And every agent answers
`GET /healthz` with 200. That path is mine, not A2A's: `docs/research/languages.md:311`
records that Nasiko defines no health contract, so nothing probes it
automatically and it exists so a smoke test does not have to read a JSON-RPC
error to find out whether a process is alive.

One thing I did not build. `a2a.Call` returns "not implemented" on every call.
Task 11's checklist does not cover it, nothing imports it yet, and a stub that
returned a plausible zero value would be worse: B4 would find out on stage. It
returns the error rather than panicking, so if something does reach it during
the demo the caller's existing error path absorbs it instead of the process
dying. It has a blocker row above.

**B5, one thing to know before you leave an agent running.** `Serve` takes the
SDK's default in-memory task store, and that store never evicts: every task it
has ever served keeps its decoded artifact for the life of the process. A 28KB
`MentionBatch` retains roughly 110KB once it is a `map[string]any`, so a pod
accumulates about that per request with no ceiling. Irrelevant for a two-minute
demo, and it is the thing that kills a pod left up overnight. Fixing it means a
`taskstore.Store` that expires terminal tasks, which is not in Task 11. The
constraint is recorded at `internal/a2a/serve.go` where the store is installed.

`go get` added `github.com/a2aproject/a2a-go/v2` v2.5.0 to the shared `go.mod`.

### 2026-09-20 — B1 — B7: `ci.yml` passes when the tests fail, and three things are missing

You wrote `ci.yml` while I was on Task 11, so this is a review of the real file
rather than the spec Task 12 told me to post. I have not touched it; it is
yours. One bug and three gaps, all in the `go` job. Blocker row raised above.

**The bug: a failing `go test` does not fail the build.**

```yaml
run: go test ./... 2>&1 | tee test.log
```

A pipeline's exit status is the **last** command's, so this step reports
`tee`'s success and discards `go test`'s result. Actions runs `run:` under
`bash -e {0}` on Linux, and `-e` does not imply `pipefail`. Verified:

```
$ bash -e -c 'false | tee /dev/null; echo "exit=$?"'
exit=0
```

Every red test in this repository is currently green in CI. The fix is one
line, `shell: bash` plus `set -o pipefail`, or drop the `tee` and use
`--json`. The "how much is actually tested" step below it is a good idea and I
would keep it; it just cannot compensate for this.

**Missing, and each one is in my Task 12 brief:**

- **`-race`.** The brief names why: the budget's cached total and the
  orchestrator's `errgroup` fan-out are where a data race hides until stage.
  `go test -race ./...`.
- **Postgres 16 as a service container.** `internal/db`'s tests call
  `t.Skip("DATABASE_URL is unset; start the compose Postgres and export it")`,
  so right now the entire db layer is skipped and the run is still green. Use
  `postgres:16` with user/password/db all `brandpulse`, a health check, and
  `LANG: C` with `POSTGRES_INITDB_ARGS: "--locale=C --encoding=UTF8"` copied
  from `docker-compose.yml`. Without the locale settings Postgres orders text
  by the runner's locale, and a fixture assertion that sorts topic labels
  passes on a laptop and fails in CI for a reason nobody finds quickly.
  `DATABASE_URL` in CI is `localhost:5432`, not 5433; 5433 is a host-collision
  workaround on this machine only.
- **A grep step failing the build if any `_test.go` names
  `http.DefaultClient`.** `anakin.NoNetwork()` is enforcement by construction:
  it only holds while tests inject it, and `http.DefaultClient` is the one way
  around it that survives review.

  ```yaml
  - name: no test may use the default HTTP client
    run: |
      if grep -rn 'http\.DefaultClient' --include='*_test.go' .; then
        echo "a test used http.DefaultClient; inject anakin.NoNetwork() instead"
        exit 1
      fi
  ```

  It must exit non-zero **on a match**, which is the opposite of `grep`'s usual
  direction in a shell step.

**What you got right and I would not change.** `BP_FIXTURE_MODE: replay` at the
job level. `go-version-file: go.mod`, which is how the job picks up Go 1.27,
and 1.24 does not compile this repository. `go vet` as its own failing step
rather than advisory. The tidy check. And the thing that matters most:

**The job must be green for someone holding no key whatsoever**, and yours is.
There is no `ANAKIN_API_KEY` and no `OPENAI_API_KEY` in the workflow, not as a
secret, not as an empty string, not commented out. Keep it that way. A key
present in the environment is a key a regression can spend, and 300 credits is
the entire budget. A test that needs a credential is not a test configured
wrong, it is a broken test, and I want the build to say so. That is not a
nice-to-have, it is the whole arrangement.

`anakin.NoNetwork()` is in `internal/anakin/nonetwork.go` and is itself tested
(`TestNoNetworkRefusesEveryRoundTrip`). Its error names the URL that was
attempted, so when CI does fail this way the log says which call escaped the
fixtures rather than just "connection refused".

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
| 1 | Which `a2a-go/v2` helper emits a **JSON** artifact? | B1, Task 1 | All nine agents need it. Lands as `a2a.JSONArtifact`. |
| 2 | How is a peer agent addressed through the Nasiko proxy? The env var name is unverified. | B5, Task 1 | Lands as `a2a.Call`. No agent writes a peer URL directly. |
| 7 | Do OTel traces from a self-instrumented Go container actually reach `nasiko observe`? | B5, Task 2 | If not, nine agents are invisible in the control plane. Deploy blocker, not polish. |
| ~~3~~ | ~~The literal field names in Wire responses per action.~~ | ~~B2, Task 1~~ | **closed 2026-09-20**, see "Wire field names, live" above |
| ~~4~~ | ~~Does the Play Store listing yield review text, rating and date through URL Scraper with `useBrowser: true`?~~ | ~~B2, Task 2~~ | **closed 2026-09-20**, yes from `html`, but only 3 reviews a listing. Six sources. |
| 5 | Does DronaHQ's WhatsApp trigger send outbound, or do we need the Twilio connector? | B5, Task 7 | The 9am brief depends on it. Meta's 24-hour window may force a template. |
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

### 2026-09-20 18:40 — B6 — a fifth thing already broken in `web/`, and the scope change kills two of them

`AlertBanner.tsx:20` is a second site of the same fabricated measurement as
`PlatformPanel`: it renders `WhatsApp sent in {minutesAndSeconds(secondsToWhatsapp)}`
from the same `TIME_TO_WHATSAPP_SECONDS = 107` constant. The entry above names
only the panel, so I am recording the second site here.

The scope change resolves it without a ruling from anyone. WhatsApp is out of
the MVP, so there is no WhatsApp send to time and the claim comes out of **both**
components along with the constant. Nothing replaces it: the number I will
render instead is detection-to-on-screen, measured in the browser against
`alerts.created_at`, and only for alerts that arrive while the page is open. An
alert already on screen at first load has no measurable arrival, so it prints no
number rather than a plausible one.

### 2026-09-20 18:40 — B6 — the BFF computes no number, and four consequences of that

`bff/` exists so the browser has one backend, not so it has a second author of
the truth. Unwrapping the A2A envelope is transport. Renaming a key, flattening
a nested object, rounding a float or summing a list are all the same defect, so
none of them happen here. Four things follow that a reader would otherwise
mistake for omissions:

1. **`/mentions` and `/alerts` return bare arrays**, not `MentionBatch` and
   `AlertSet`. Those envelopes carry `credits_used`, `cache_hits`, `truncated`
   and `rules_evaluated`, which a Postgres read cannot know. Filling them in
   would be inventing numbers. `/pulse` is the only route with an `errors`
   array, because a partial answer only exists where several sources combine.
2. **`Topic.top_examples` comes back empty** and `ReplyDraft.requires_human_approval`
   comes back as a constant `true`. "At most 3 mentions chosen by engagement" is
   `bp-clusterer`'s rule with no column behind it, and `requires_human_approval`
   has neither a column nor a Go struct field, only an unconditional
   `MarshalJSON`. The BFF picking three examples would be the BFF inventing an
   agent's output.
3. **A NULL column becomes an absent key, not a `null` and never a zero.**
   `omitempty` means Go emits no key at all. `rating: 0` is a real one-star
   average and `credits_used: 0` is a run that has not spent yet, so those
   survive.
4. **A populated `errors` array is never an HTTP 500.** `/pulse` gathers seven
   slots independently: one failed query returns the other six with the failure
   named, at a 200. A 500 would blank a dashboard that still has a crisis alert
   to show.

The pool is opened with `default_transaction_read_only=on`, so a write added to
`db.ts` later fails at the Postgres server rather than in review. The agents own
every write.

`bff/src/contracts.ts` duplicates `web/lib/types.ts` deliberately: two Docker
build contexts and two `rootDir`s mean neither can import the other.
`web/scripts/checkTypesParity.mjs` checks both against `models.go`, which makes
the duplication build-enforced instead of review-enforced.

### 2026-09-20 — B6 — the web image builds from the repo root, and the two env vars a deployer has to set

Whoever writes the pipeline needs three facts about these images, because two of
them are not guessable from the Dockerfiles' locations.

**`web/Dockerfile`'s build context is the repo root, not `web/`.**

```bash
docker build -f web/Dockerfile -t brandpulse-web .   # note the trailing dot
docker build -t brandpulse-bff bff/                  # bff/ is the ordinary case
```

`npm run build` in `web/` runs `scripts/checkTypesParity.mjs` first, and that
script reads `internal/models/models.go` and `bff/src/contracts.ts` to fail the
build when a type mirror has drifted from the Go wire format. Those two files are
outside `web/`. A `web/`-only context still builds a working image, which is the
problem: it builds it with the only drift check in the repo silently skipped, so
the failure mode is a dashboard rendering `undefined` rather than a red build.

**The runtime env vars, and the ones that must never be set.** `web/` takes
`BFF_BASE_URL` and optionally `BRAND_ID`. That is the whole list. `BFF_BASE_URL`
has no `NEXT_PUBLIC_` prefix on purpose: a `NEXT_PUBLIC_` variable is inlined
into the browser bundle when the image is built, which would both pin one BFF URL
per image and publish it to every visitor. It is read per request, server-side,
by `lib/pulse.ts`, which is why the alert feed polls `app/api/alerts/route.ts`
instead of the BFF directly.

`bff/` takes `DASHBOARD_ORIGIN`, `ORCHESTRATOR_URL` and `DATABASE_URL`, all
required, none committed and none baked into the image.

**B5: the deployed `bp-orchestrator` URL goes to the BFF, not to the web image,
and it goes in as an env var at deploy time.** Post it here when `nasiko deploy`
can authenticate and I will not need a rebuild to pick it up. `DASHBOARD_ORIGIN`
has to be the exact single origin the dashboard is served from: CORS allows one
origin, never a list and never `*`, because widening it lets any page a customer
has open read their mentions through their own browser.

I verified the browser bundle carries none of it. Running `brandpulse-web` against
a live BFF and grepping the served HTML plus all nine `/_next/static` assets for
`postgresql://`, the DB password, `:5434`, `:9099`, `host.docker.internal` and
`localhost:8080` exits 1. The only hit anywhere is the literal string
`BFF_BASE_URL` in `app/error.tsx`, which is the variable's *name* printed in an
error message telling an operator what to check, not its value.

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
| Credits spent on Task 1 schema reads | **9** of ≤ 10 | B2 Task 1, actual | B2 |
| Credits spent on the Play Store probe | **4** of 3, one over, see the Task 2 entry | B2 Task 2, actual | B2 |
| Credits spent picking the demo brand | **4**, two `rt_search` calls | B2 Task 3, actual | B2 |
| **Credits spent, running total** | **17 of 300** | B2 | B2 |
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
| Time from mention to alert on screen | — | measured in the browser, `alerts.created_at` to the alert feed's receipt, per the scope change | B6 |
| Nasiko PR | — | link | B5 |

### 2026-09-20 — B2 Tasks 6 and 7 — Crawl is not what the docs say, and it cost me 11 credits to find out

**bp-onboarder and demo/record ship. The recording session does not, and that
is on B1's stub, not on scope.**

#### Crawl takes one seed URL and follows links. It does not take a list.

`docs/research/anakin.md` §5 said "map first, filter the link list, then crawl
only those". That plan cannot be expressed: `POST /v1/crawl` accepts one `url`
and crawls outward from it. I seeded `/pages/warranty` and got warranty plus
`/collections/daily-deals`, a link off that page. Passing a `urls` array
alongside `url` is **silently ignored**, not rejected: the job came back having
crawled the root only, and I paid for it.

So bp-onboarder maps once, filters the links itself, and **scrapes** the ones it
keeps. `Scrape` costs the same 1 credit per URL, fetches exactly the URL given,
is synchronous rather than submit-and-poll, and its response shape was already
verified in Task 3. Once Map has done the discovery, Crawl has nothing left to
offer. §5 now carries the live shapes.

#### Two API behaviours that will cost another track money

- **`maxPages: 0` is not a validation error. It is the default of 10.** I sent
  it expecting a free 400 that would tell me the valid range. It was accepted
  and crawled 10 pages. **A Go zero value reaching this field spends 10
  credits.** That is why bp-onboarder resolves `MaxPages == 0` to 25 before the
  call rather than letting an unset field through to the wire.
- **Unknown request fields are accepted and ignored.** A misspelled parameter
  does not 400. It silently does nothing and you are billed for whatever call it
  turned into. There is no way to typo-check a request except by reading the
  response and noticing it did not do what you asked.

The rule I should have been following, and now am: probe with a wrong **type**,
which does 400 and is free. Never with a wrong **value**.

#### There is no usage endpoint

`/v1/usage`, `/v1/credits`, `/v1/account`, `/v1/me` and `/v1/billing` all 404.
The table in research §6 and the `budget` rows in Postgres are the only count
that exists, so an overspend is invisible until the key stops working. **That is
the argument for `MaxCredits` being a hard client-side stop, not a warning.**
Whoever implements `NewHTTPClient`: there is no server-side safety net behind
you.

Credits stand at **35 of 300**, of which 11 were the mistake above. The
recording plan estimates 63, leaving 202.

#### Map's shape, for anyone decoding it

`links` is a flat **`[]string`**, not a list of objects. A `[]struct{URL string}`
decodes to a slice of empty structs **with no error**, which is the
silent-empty failure this repo keeps hitting. `externalLinks` is **absent** from
the response unless `includeExternalLinks` is passed, so decode it as a pointer
or check for the key.

The link list is **products-first**: 48 `/products`, 35 `/collections`, 13
`/pages` on boat-lifestyle.com. Truncating it at 25 gives 25 product pages and
no about page, so bp-onboarder takes turns between the three kinds instead of
taking the head of the list.

#### One ask of B1, and it is the only thing standing between B3 and a corpus

`NewHTTPClient` plus `record` mode. `demo/record` is written and green, its dry
run prints the estimate and spends nothing, and `-confirm` panics at
`internal/anakin/anakin.go:194`. Everything downstream of the recording, the
fixture corpus and B3's labelling set, is waiting on that one constructor.

#### For B5, on the onboarder's cost

bp-onboarder is capped at **30 Anakin credits and one LLM call**, both tested.
Onboarding one brand is 1 map plus up to 29 scrapes. The default page count is
25 per CONTRACTS §2 and the agent applies it; pass a smaller `max_pages` if you
are onboarding on stage, because 26 credits per brand is real money against 300.

#### Two smaller asks of B1, neither of them blocking

Both came out of a cleanup pass over bp-onboarder and `demo/record`, and both
are in `internal/`, which I do not write to from this worktree.

1. **`anakin.ModeFromEnv()`.** bp-collector, bp-onboarder and `demo/record` each
   read `BP_FIXTURE_MODE` and each re-apply the "empty means replay, never live"
   rule by hand. That rule is the zero-credit CI guarantee and it is currently
   three copies. One exported helper in `internal/anakin` makes it one.
2. **A batch method on `anakin.Client`.** `docs/research/anakin.md` §4 documents
   `POST /v1/url-scraper/batch`, async, up to 10 URLs per job. The onboarder
   scrapes up to 29 pages one blocking call at a time, which is 90 to 145
   seconds of wall clock in live mode on a path DronaHQ calls with a human
   waiting on a form. Batch turns 29 round trips into 3. The interface is frozen
   B1 territory, so this is a request, not a change. It costs the same credits.

Neither matters in `replay` mode, which is CI and the demo, so neither is on the
critical path for Sunday.
