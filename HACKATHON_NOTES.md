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
| Time from mention to alert on screen | — | `demo/run_demo.sh` timing output | B4 |
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
