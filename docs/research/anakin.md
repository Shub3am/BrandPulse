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
| `appstore` | Apple's **public review RSS**, `https://itunes.apple.com/in/rss/customerreviews/id=<app_id>/sortBy=mostRecent/json`, fetched via URL Scraper | **Good** — public, documented, no ToS problem. Validate the feed returns data for the demo app before relying on it. |
| `playstore` | URL Scraper with `useBrowser: true` on the app's Play listing | **Probe** — reviews are JS-rendered. B2 spends ≤ 3 credits testing this early. If it fails, the source is dropped, not faked. |
| `x` | Search API scoped `site:x.com`, snippets only | **Degraded** — no engagement metrics, no follower counts. Ships as a low-yield source or is dropped. |
| `instagram` | none | **Dropped** |
| `flipkart` | Wire product data only, no reviews | **Dropped as a mention source** |

Seven solid-to-good sources. The demo's "7 sources" claim survives; the
Play Store review-bomb story depends on the probe. See
[SOURCE-STRATEGY.md](../SOURCE-STRATEGY.md) for the decision and its
consequences for the pitch.

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

Costs: 1 credit basic (browser rendering included, no surcharge), 2 with AI
summary, 3 with AI JSON extraction. `useBrowser: true` is free — which is what
makes the Play Store probe cheap enough to try.

## 5. Crawl and Map

Both async, 202 + `jobId`, then poll.

`POST /v1/crawl` — `url*`, `maxPages` (default 10, max 100), `depth` (default 1,
max 5), `includePatterns`, `excludePatterns`, `useBrowser`. **1 credit per page
crawled.** Poll returns `results[]` with `url`, `html`, `markdown`, `status`.

`POST /v1/map` — `url*`, `includeSubdomains`, `includeExternalLinks`, `limit`
(default 100, max 5000), `depth` (default 2, max 5), `search`. **1 credit per
job.** Poll returns `links[]`, `totalLinks`, `externalLinks[]`.

Map is cheap and crawl is not. bp-onboarder maps first, filters the link list
to product/about pages, then crawls only those — 1 + N credits instead of
blanket-crawling a site. Cap `maxPages` at 20.

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
| **Task 1 schema reads (spent)** | 5 billed + 3 free 400s | **9** |
| Task 2 Play Store probe | 3 | 3 |
| Onboard: map ×3 + crawl 20pp ×1 | 4 | ~23 |
| Reddit `rt_search`, 3 queries × 3 brands | 9 | 18 |
| YouTube `yt_search` ×3 + `yt_comments` ×6 | 9 | 21 |
| News/web Search API, 10 queries | 10 | 30 |
| URL Scraper, only where a snippet came back short | ~5 | 5 |
| ~~Amazon search ×3 + reviews ×6~~ | 0 | **0** |
| App Store RSS ×3 | 3 | 3 |
| **Subtotal** | | **~112** |
| Retry/headroom | | ~60 |
| Reserved for the stage live call | | ~10 |
| **Ceiling** | | **~182 of 300** |

Roughly 118 credits spare, up from 95, because Amazon and the blanket scrape
came out. The client's `MaxCredits` ceiling enforces it (CONTRACTS §3,
`anakin.NewHTTPClient(cfg)`); B2 prints a dry-run estimate before spending
anything.

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
