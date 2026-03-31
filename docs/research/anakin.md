# Anakin — verified findings

Researched 2026-09-20 against **anakin.io** (not anakin.ai, which is an
unrelated product). Base URL `https://api.anakin.io/v1`, auth header
`X-API-Key: <key>` on every request.

**Read §1 before planning any source work. It changes the brief.**

---

## 1. The blocker: Wire does not cover four of our sources

The brief assumes Wire gives us X, Reddit, YouTube, Instagram, Play Store,
App Store, Amazon and Flipkart reviews. Direct fetches of
`anakin.io/catalog/<slug>` say otherwise. A 404 below is a confirmed absence,
not a failed lookup.

| Brief's source | Reality | Evidence |
|---|---|---|
| Reddit | **Available**, 7 actions | `anakin.io/catalog/reddit` 200 |
| YouTube | **Available**, 6 actions incl. comments | `anakin.io/catalog/youtube` 200 |
| Amazon reviews | **Available** (`am_product_reviews`) | `anakin.io/catalog/amazon` 200, 15 actions |
| Flipkart | Products only, **no reviews action** | `anakin.io/catalog/flipkart` 200, 8 actions, none for reviews |
| **X / Twitter** | **Absent** | `anakin.io/catalog/twitter` → 404 |
| **Instagram** | **Absent** | `anakin.io/catalog/instagram` → 404 |
| **Google Play reviews** | **Absent** | `anakin.io/catalog/google-play-store` → 404. AppBrain exists but returns rankings and aggregate stats, no per-review text |
| **Apple App Store reviews** | **Absent** | `anakin.io/catalog/apple-app-store` → 404, no alternative slug found |

Corroborated by Anakin's own blog, 2026-07-24: *"LinkedIn, Google Maps,
Instagram, Facebook, Pinterest, Twitter/X, Glassdoor, and Crunchbase aren't in
Wire's catalogue yet"* — https://anakin.io/blog/anakin-vs-apify

### The adapted source list

Wire is not the only surface. Search API, URL Scraper and a public RSS feed
cover most of the gap. This is what BrandPulse actually ships:

| Source | Path | Confidence |
|---|---|---|
| `reddit` | Wire `rt_search` + `rt_subreddit_posts` + `rt_post_details` | **Solid** |
| `youtube` | Wire `yt_search` + `yt_comments` | **Solid** |
| `amazon` | Wire `am_search_products` + `am_product_reviews` | **Dead as a mention source** — see below |
| `news` | Search API, `prompt` scoped to brand + news terms | **Solid** |
| `web` | Search API for blogs/forums, then URL Scraper for full text | **Solid** |
| `appstore` | Apple's **public review RSS**, `https://itunes.apple.com/in/rss/customerreviews/id=<app_id>/sortBy=mostRecent/json`, fetched via URL Scraper, parsing **`html`** | **Good, but the feed comes back with no `entry` key at all for many real ids, and which ids depends on the exact URL form as much as on the app.** Measured below, and corrected below that. |
| `playstore` | URL Scraper with `useBrowser: true` on the app's Play listing, parsing **`html`**, not `markdown` | **Low-yield, confirmed live** — text, star rating, date, review id and helpful count all extract cleanly, but a listing renders only **3 reviews** and no method moved that. [playstore-probe.md](playstore-probe.md) |
| `x` | Search API scoped `site:x.com`, snippets only | **Degraded** — no engagement metrics, no follower counts. Ships as a low-yield source or is dropped. |
| `instagram` | none | **Dropped** |
| `flipkart` | Wire product data only, no reviews | **Dropped as a mention source** |

**Six sources, not seven, and the count is settled** (updated 2026-09-20 after
B2 Tasks 1 and 2): `reddit`, `youtube`, `news`, `web`, `appstore`, `playstore`.
Amazon is out because its review text comes back empty, not because of budget.
The Play Store **review-bomb story is dead** either way: the probe passed on
field extraction and returned 3 reviews per listing, which is coverage, not a
velocity signal. Do not build a demo beat on it.
[SOURCE-STRATEGY.md](../SOURCE-STRATEGY.md) still says seven and is stale;
it lives on `main` and B7 owns the fix.

### The Apple review RSS is per-app, and a healthy-looking response can be empty

Measured 2026-09-20 by B2 Task 3, `in` storefront, `sortBy=mostRecent`, three
identical attempts each, deterministic. All HTTP 200, all a well-formed feed
envelope with `author`, `updated`, `title`, `link`, `id`:

| App | id | entries |
|---|---|---|
| boAt Hearables | 1592550875 | **0** |
| boAt Wearables | 1542443145 | **0** |
| boAt Shopping | 6475390290 | 36 |
| NoiseFit | 1498457147 | **0** |
| NoiseFit Track | 1573689962 | 50 |
| GOBOULT Amp | 6476545014 | **0** |
| GOBOULT Fit | 1629163626 | **0** |
| Zomato, control | 434613896 | 50 |
| Nykaa, control | 1479127399 | **0** |

There is no pattern by popularity: NoiseFit has 122 244 ratings and returns
nothing, NoiseFit Track has 948 and returns 50. Nykaa returns nothing.
`sortBy=mostHelpful` made no difference, and `us` on a busy app was also 0
where `in` was 50.

#### Correction, same day, Task 4: emptiness follows the URL form, not the app

The table above is a true record of what those nine URLs returned. The
conclusion drawn under it was wrong, and this correction is the finding.

Re-measuring with a 4-form × 9-app matrix and 6 repeated runs: **boAt Shopping
went 36 → 0 and boAt Hearables went 0 → 50**, on the same ids, the same
storefront and the same sort. Within one session a given URL is deterministic,
which is what made the first measurement look stable; across forms and across
days it flips.

So strike the sentence "an id that returns nothing today returns nothing on
stage". It does not hold. What replaces it:

- **Validating an app id once proves nothing about stage.** The check that
  matters is the one the adapter already makes on every run.
- `demo/brand.json` uses **1592550875** (boAt Hearables), which returned 50 on
  the form the adapter builds, and which matches the `playstore` package
  already listed. It does not use 6475390290.
- The `appstore` adapter returns an error on a zero-entry feed, the same rule
  as `playstore`. A source that quietly contributes nothing is worse than one
  that fails loudly. Given this correction that rule is load-bearing, not
  belt-and-braces.

#### Read `html`, not `markdown`, when the URL serves JSON

Measured 2026-09-20, 1 credit, scraping the feed URL through
`POST /v1/url-scraper/scrape`:

| Format | 40 KB feed | `json.Unmarshal` |
|---|---|---|
| `html` | the document verbatim | **parses, 50 entries** |
| `markdown` | `[` escaped to `\[` | fails at char 112 |
| `cleanedHtml` | every `"` turned into `&#34;` | fails at char 1 |

The scraper treats a JSON body as a document to render, and both derived
formats are lossy in ways that are invisible until `Unmarshal` refuses. This is
the same trap as the Play Store rating, one layer down.

The same call also settled the envelope question: the response fields
(`id`, `status`, `url`, `jobType`, `country`, `html`, `cleanedHtml`,
`markdown`, `cached`, `createdAt`, `completedAt`, `durationMs`) arrive at the
**top level, with no envelope**.

Where it works it is the best-shaped source we have: `im:rating`, `im:version`,
`updated` (RFC 3339), `author.name`, a numeric `id`, `title`, `content`,
`im:voteSum` and `im:voteCount`. That is a complete `Mention` including
`Rating` and `Engagement`, from a free public endpoint.

**Nobody fabricates a source.** If the Play Store probe fails, the README and
the pitch say six sources. Fixtures are recorded from real calls or the source
does not exist.

---

## 2. Search API

`POST /v1/search`, synchronous, **3 credits**.

```json
{"prompt": "Minimalist face serum reviews", "limit": 5}
```

`limit` defaults to 5, max 20. Response:

```json
{"id": "...", "results": [
  {"url": "...", "title": "...", "snippet": "...",
   "date": "...", "last_updated": "..."}]}
```

**Important correction to the brief:** the brief says Search API returns "full
page content". It does not — it returns a `snippet`. Full text requires
chaining Search → URL Scraper (1 credit per URL), or possibly the separate
`POST /v1/agentic-search` (10 credits + 1/URL). Budget accordingly: a news
sweep that needs article bodies costs 3 + N credits, not 3.

**RESOLVED 2026-09-20 by B2 Task 1, live.** There are no country/locale or
freshness parameters, and the API does **not** tell you so: `country`,
`freshness` and `date_range` sent together returned **HTTP 200** and were
silently ignored. The proof is in the results — with `freshness: "week"` set,
the three results were dated 2025-01-29, empty, and 2022-09-14. So recency is
filtered **client-side on `date`** and we pay for results we discard, which is
the expensive branch this note feared. Two further consequences: a 200 never
proves a parameter was honoured, and **`date` can be an empty string**, so a
result with no parseable date is dropped rather than defaulted. Full evidence
in [wire-schemas.md](wire-schemas.md) §6.

**`limit` bounds confirmed live:** `limit: 999` returns
`400 {"error":"invalid_request","message":"limit must be between 0 and 20"}`.

**The `snippet` is bigger than this section implies.** One measured result was
2 966 characters of cleaned page text, enough to classify sentiment, intent and
aspects without chaining a scrape. The "3 + N credits" rule still applies where
a body is genuinely needed, but the `web` adapter does not need it by default.

## 3. Wire

Per-site prebuilt actions, ~940 sites, async job pattern.

```
GET  /v1/wire/catalog              list sites
GET  /v1/wire/catalog/{slug}       actions + live param schema for one site
GET  /v1/wire/resolve?q=...        find action_id by intent
POST /v1/wire/task                 {action_id, params, credential_id?, webhook_url?}
GET  /v1/wire/jobs/{id}            poll
```

Rate limits: `POST /wire/task` **20 req/min/user**; `/wire/resolve` 120/min/IP.
The 20/min cap matters for fan-out — a run hitting three Wire sources with
multiple keyword variants must pace itself. `bp_core.anakin` owns that
throttle; adapters do not.

Job responses:

```json
{"status": "processing", "retry_after_ms": 2000}
{"status": "completed", "data": {...}, "credits_used": 2, "execution_ms": 8420}
{"status": "failed", "error": {"code": "EXECUTION_FAILED", "message": "..."},
 "credits_used": 0}
```

Failed calls are **not billed**.

### Reddit actions

| action_id | Params | Credits |
|---|---|---|
| `rt_search` | `query*`, `sort`, `time`, `limit`, `after`, `match` | 2 |
| `rt_subreddit_posts` | `subreddit*`, `sort`, `time`, `limit`, `after` | 2 |
| `rt_post_details` | `post_id*`, `subreddit`, `comment_limit` | 2 |
| `rt_user_profile` | `username*`, `include_posts`, ... | 2 |
| `rt_search_subreddits` | `query*`, `limit`, `after` | 2 |

Pagination is Reddit's `after` cursor.

### YouTube actions

| action_id | Params | Credits |
|---|---|---|
| `yt_search` | `query*`, `limit` | 1 |
| `yt_comments` | `video_id*`, `limit`, `include_replies`, `sort` | **3** |
| `yt_video` | `video_id*` | 1 |
| `yt_channel` | `channel_id*` | 1 |

Anonymous, no `credential_id`. Takes a **video id, not a URL** — the adapter
extracts the id.

### Amazon actions

`am_search_products` (`query*`, `page`, `limit`, `sort`),
`am_product_details` (`asin*`), `am_product_reviews` (`asin*`) — "top product
reviews with ratings, authors, dates".

**RESOLVED 2026-09-20 by B2 Task 1**, live, for `rt_search`, `yt_search`,
`am_search_products` and `am_product_reviews`. Schemas are in
[wire-schemas.md](wire-schemas.md). `yt_comments` is the one action still
unread, because Task 1's 10-credit budget ran out at 9. Two findings that
change the build:

- **The payload is two envelopes deep, at `job.data.data`**, and the array key
  differs per action (`posts`, `data`, `products`, `reviews`). Unmarshalling one
  level short yields zero values and no error.
- **`am_product_reviews` returns 4–6 reviews out of ~15 000, every one with an
  empty `text` and `title`.** Confirmed on two ASINs. A mention built from it
  has no text to classify and no distinct content hash, so `amazon` cannot ship
  as a mention source. The action also has no page, limit or sort parameter, so
  there is no way to ask for more. Decision and fallout in `HACKATHON_NOTES.md`.

**Mitigation, and B2's first task:** before writing any adapter, call
`GET /v1/wire/catalog/{reddit,youtube,amazon}` with the real key — these are
catalog reads and cost nothing or near nothing — and paste the live schemas
into `docs/research/wire-schemas.md`. Then write adapters against real field
names. Do not guess a field name, ever; a wrong guess surfaces as an empty
dashboard at demo time.

## 4. URL Scraper

| Mode | Call |
|---|---|
| Inline, blocking ≤90s | `POST /v1/url-scraper/scrape` |
| Async single | `POST /v1/url-scraper` → 202 `{jobId, status}` |
| Async batch, ≤10 URLs | `POST /v1/url-scraper/batch` |
| Poll | `GET /v1/url-scraper/{id}` |

```json
{"url": "https://example.com",
 "formats": ["markdown", "html", "links"],
 "country": "us", "useBrowser": false,
 "generateJson": false, "outputSchema": {},
 "webhook_url": "...", "actions": []}
```

`formats`: `markdown`, `html`, `links`, `images`, `summary`, `screenshot`,
`json`. Plain `text` is UNVERIFIED — use `markdown`.

Response carries `html`, `cleanedHtml`, `markdown`, `generatedJson`, `cached`,
`durationMs`.

**`actions` schema, confirmed live 2026-09-20 by B2 Task 2** at a cost of zero
credits, from four deliberate 400s:

| Type | Required field | Bounds |
|---|---|---|
| `wait` | `milliseconds` | 1 to 15 000 |
| `wait_for` | `selector` | |
| `click` | `selector` | |
| `press` | `key` | |
| `scroll` | none | |
| `write` | UNVERIFIED, presumably `text` + `selector` | |

`scroll` takes no required field, so `{"type":"scroll"}` is **valid** and bills
a full scrape. That is how B2 spent a credit expecting a 400.

**`cleanedHtml` is lossy in a way that matters.** On a Play Store listing it
dropped the per-review star rating (an empty div carrying only an `aria-label`)
and the reviewer name, while keeping the review bodies. Markdown dropped the
rating too and reordered the date next to the developer's reply, where it reads
as the reply date. Any adapter that needs attributes rather than text must take
the full `html`. Detail in [playstore-probe.md](playstore-probe.md).

Costs: 1 credit basic (browser rendering included, no surcharge), 2 with AI
summary, 3 with AI JSON extraction. `useBrowser: true` is free — which is what
makes the Play Store probe cheap enough to try.

## 5. Crawl and Map

Both async, 202 + `jobId`, then poll.

Everything below the request lines was read live on 2026-09-20 in Task 6, on
`boat-lifestyle.com`, before writing bp-onboarder. The previous version of this
section was copied from the documentation and got three things wrong. The URL
Scraper probe in Task 3 had already shown doc-derived shapes are not reliable
here, so this one is measured.

`POST /v1/crawl` — `url*`, `maxPages` (default 10, max 100), `depth` (default 1,
max 5), `includePatterns`, `excludePatterns`, `useBrowser`. **1 credit per page
crawled.**

`POST /v1/map` — `url*`, `includeSubdomains`, `includeExternalLinks`, `limit`
(default 100, max 5000), `depth` (default 2, max 5), `search`. **1 credit per
job.**

### What Map actually returns

Job `345b9ad4`, `limit: 100`. Top level is
`completedAt, createdAt, durationMs, id, links, status, totalLinks, url`.

- **`links` is `[]string`, a flat list of URLs.** It is not a list of objects.
  A `[]struct{ URL string }` decodes to a slice of empty structs with no error,
  which is the silent-empty failure this repo keeps hitting.
- **`externalLinks` is absent from the response**, not empty, when
  `includeExternalLinks` is not passed. The old text listed it as always
  present. Decode it as a pointer or check for the key.
- `totalLinks` was 100, `len(links)` was 100, and the `limit` passed was 100.
  On this call `totalLinks` is not distinguishable from "how many came back",
  so do not read it as "how many the site has".

Path distribution of those 100 links: 48 `/products`, 35 `/collections`,
13 `/pages`, 2 `/account`, 1 `/cart`, 1 root. **The list is products-first**, so
truncating it at 20 gives 20 product pages and no about page. The page filter
needs a per-kind quota, not a head-20.

### What Crawl actually returns

Job `c6bb9d31`, seeded at `/pages/warranty`, `maxPages: 2`. Top level is
`completedAt, completedPages, createdAt, durationMs, id, results, status,
totalPages, url` — `completedPages` and `totalPages` are both undocumented.
Each `results[]` element is `durationMs, html, markdown, status, url`.
**There is no `cleanedHtml`.** Neither response is wrapped in an envelope.

Crawl `markdown` escapes list numbers: `"1\\. Copyright Notice"`. Same trap as
the App Store `markdown` finding in §1.

### Crawl takes one seed URL and follows links from it

This is the finding that changed bp-onboarder's design. Crawl is not "fetch
these pages". Seeded at `/pages/warranty` it returned warranty **and**
`/collections/daily-deals`, a link off that page.

Three request probes, 2026-09-20:

| Body | Result |
|---|---|
| `includePatterns: 123` and `[123]` | 400 `invalid_request`, "Invalid JSON body", free |
| `maxPages: 0` | **202 accepted, crawled 10 pages, 10 credits** |
| `urls: ["…/pages/warranty"]` alongside `url` | 202, crawled the root only, the `urls` key was ignored |

Two consequences, and both cost money:

- **`maxPages: 0` is not a validation error, it is the default of 10.** A Go
  zero value reaching this field silently spends 10 credits. bp-onboarder
  applies its own default before the call rather than letting an unset field
  through.
- **Unknown request fields are accepted and ignored.** A misspelled parameter
  does not 400, it silently does nothing and you are billed for the call it
  turned into. There is no way to typo-check a request except by reading the
  response.

`includePatterns` only ever produced the generic "Invalid JSON body" on a type
error, so the **pattern syntax is UNVERIFIED** — glob, regex or prefix is
unknown. Nothing here should depend on it.

### So the onboarder maps, then scrapes

Crawling from the site root is wasteful and unsteerable: the 10-page probe
spent 3 of its 10 credits on `/account`, `/account/login` and `/cart`. Steering
it away needs `excludePatterns`, whose syntax is unverified.

Once Map has the link list, Crawl has nothing left to offer. `Scrape` costs the
same 1 credit per URL, fetches exactly the URL given, is synchronous instead of
submit-and-poll, and its response shape is already verified in §4. bp-onboarder
therefore maps once, filters the links, and scrapes each kept page: 1 + N
credits, N pages chosen by us.

This departs from the B2 brief's "then `Client.Crawl` only those", which assumed
Crawl accepts a list. It does not, and the `urls` probe above shows a list is
silently ignored rather than rejected. The brief's intent, do not blanket-crawl,
is what the filter preserves.

## 6. Credits

Free tier **300 credits, no card, no expiry**. Failed calls are not billed.

| Call | Credits |
|---|---|
| Search API | 3 |
| URL scrape (basic, incl. browser) | 1 |
| URL scrape + AI summary / + AI JSON | 2 / 3 |
| Crawl | 1 per page |
| Map | 1 per job |
| Wire Reddit actions (all 7) | 2 |
| Wire Amazon `am_*` actions | **1** |
| Wire `yt_comments` | 3 |
| Wire `yt_search`, `yt_video`, `yt_channel`, `yt_related`, `yt_suggestions` | 1 |
| Agentic Search | 10 + 1 per URL |

Corrected 2026-09-20 by B2 Task 1: Amazon actions cost **1**, not 2. Every
catalogue action carries a live `credits_per_call` field, so `internal/anakin`
should read it rather than keep this table in code. Live action counts are
Reddit 7, YouTube 6, Amazon 15.

### The recording budget

B2's one live session, demo brand + 2 competitors:

Revised 2026-09-20 after B2 Task 1. Three lines changed: Amazon is gone, the
blanket URL-Scraper pass is gone because the Search `snippet` is already large
enough to classify, and Task 1's own spend is now a real number.

| Item | Calls | Credits |
|---|---|---|
| **Task 1 schema reads, spent** | 5 billed + 3 free 400s | **9** |
| **Task 2 Play Store probe, spent** | 3 on the listing + 1 mistake | **4** |
| **Task 3 App Store id survey, spent** | 4 | **4** |
| **Task 4 `yt_comments` schema read, spent** | 1 | **3** |
| **Task 4 scrape-format check on a JSON url, spent** | 1 | **1** |
| **Task 6 Map and Crawl shape reads, spent** | 2 | **3** |
| **Task 6 crawl parameter probes, spent in error** | 2 | **11** |
| Onboard: map ×3 + scrape 20 pages ×1 | 23 | ~23 |
| Reddit `rt_search`, 3 queries × 3 brands | 9 | 18 |
| YouTube `yt_search` ×3 + `yt_comments` ×6 | 9 | 21 |
| News/web Search API, 10 queries | 10 | 30 |
| URL Scraper, only where a snippet came back short | ~5 | 5 |
| App Store RSS ×3 | 3 | 3 |
| Play Store listings ×3, `markdown`+`html` | 3 | 3 |
| ~~Amazon search ×3 + reviews ×6~~ | 0 | **0** |
| **Task 7 recording subtotal** | | **~103** |
| Spent already (Tasks 1 to 6) | | 35 |
| Retry/headroom | | ~60 |
| Reserved for the stage live call | | ~10 |
| **Ceiling** | | **~208 of 300** |

Roughly 92 credits spare. The client's `MaxCredits` ceiling enforces it
(CONTRACTS §3, `anakin.NewHTTPClient(cfg)`); B2 prints a dry-run estimate
before spending anything.

**The 11-credit line is a mistake of mine, not a planned read.** I sent
`maxPages: 0` to `/v1/crawl` expecting a 400 that would tell me the valid
range, because failed calls are free. It was accepted as the default of 10 and
crawled 10 pages. The rule that follows: an out-of-range probe against a
parameter is only free if the parameter rejects out-of-range values, and on
this API most do not. Probe with a wrong **type**, which does 400, never with a
wrong **value**.

**There is no usage or balance endpoint.** `/v1/usage`, `/v1/credits`,
`/v1/account`, `/v1/me` and `/v1/billing` all 404. This table and the `budget`
rows in Postgres are the only count that exists, so an overspend is invisible
until the key stops working. That is the argument for `MaxCredits` being a hard
client-side stop rather than a warning.

Corpus feasibility without Amazon: Reddit 9 searches × ~15 posts = ~135,
YouTube 6 × `yt_comments` at `limit: 50` = up to 300, Search 10 × 20 = up to
200. The Phase 2 gate of ≥ 300 mentions clears on Reddit and YouTube alone.

Rate limits: most `POST` submits 60/min/user, `wire/task` 20/min/user, polling
`GET`s unlimited but back off at ~1/sec/job. **UNVERIFIED at full-text level** —
these came from a search summary of the rate-limits page, not a direct fetch.

## 7. Python SDK

`pip install anakin-sdk`, import `anakin`, **alpha (0.1.x)**, Python 3.10+,
Apache-2.0, repo `github.com/Anakin-Inc/anakin-py`.

```python
from anakin import Anakin
client = Anakin(api_key="ak-...")          # or env ANAKIN_API_KEY
client.search("best web scraping libraries 2025")
client.wire("rt_search", {"query": "python scraping", "limit": 10})
client.scrape("https://example.com", formats=["markdown"])
client.crawl("https://example.com", max_pages=20)
client.map("https://example.com", limit=200)
```

It polls long-running jobs internally (1s → 10s backoff, 300s timeout, 4
retries on 429/5xx), so no manual job loop.

**Decision: we do not use the SDK.** It is alpha, its exact signatures are
UNVERIFIED, and we need cache/budget/fixture interception on every call
anyway. `bp_core.anakin` calls the REST endpoints with `httpx` directly. The
endpoints are documented and stable; an alpha SDK is a dependency risk for one
day of work and buys us only the polling loop, which is twenty lines.

## 8. Errors

```json
{"error": "rate_limit_exceeded", "message": "Too many requests..."}
```

Parse `error`, never `message`.

| Code | Retry |
|---|---|
| 400, 401, 403, 404 | No |
| 402 insufficient credits | No — surfaces as `BudgetExceeded` |
| 409, 422 | After resolving the cause |
| 429 | Yes, honour `Retry-After` |
| 500/502/503 | Yes, exponential backoff with jitter, cap 3–5 attempts |

Async job failures come back as `{"status": "failed", "error": "..."}` with
common substrings: "Blocked by website", "CAPTCHA", "timeout", "DNS resolution
failed", "TLS/SSL". `bp_core.anakin` maps these to a typed
`AnakinFetchError(kind=...)` so the collector records the reason in
`MentionBatch.errors` instead of dying.

---

## Confidence

**Solid:** base URL and auth; Map/Crawl/URL-Scraper schemas and the async
poll pattern; Search request/response; error format and codes; credit costs;
the 300-credit free tier; Reddit/YouTube/Amazon/Flipkart action lists with ids,
params and costs; **the absence of X, Instagram, Play Store and App Store
reviews from Wire**.

**Shaky, resolve before coding the adapters:** literal response field names for
every Wire action (B2's first task, via a live catalog read); Search API
locale/freshness params; whether a `text` format exists on URL Scraper; the
full rate-limit table; the Python SDK signatures (moot, we are not using it).

Open: https://anakin.io/docs/api-reference/wire/get-catalog,
https://anakin.io/docs/api-reference/search/search,
https://anakin.io/docs/documentation/pricing,
https://anakin.io/docs/documentation/rate-limits,
https://anakin.io/catalog
