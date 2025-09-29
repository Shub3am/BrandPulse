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
| `amazon` | Wire `am_search_products` + `am_product_reviews` | **Solid** |
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

**UNVERIFIED:** country/locale and freshness/date-range parameters. Not found
in the docs. If they do not exist, recency is filtered client-side on the
`date` field, which means we pay for results we then discard. B2 checks this
against a live call before designing the news adapter.

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

**UNVERIFIED across all three platforms:** the literal response field names.
The catalog pages render prose descriptions, not example JSON. This is the
single biggest unknown left.

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
| Wire Reddit / Amazon actions | 2 |
| Wire `yt_comments` | 3 |
| Wire `yt_search`, `yt_video`, `yt_channel` | 1 |
| Agentic Search | 10 + 1 per URL |

### The recording budget

B2's one live session, demo brand + 2 competitors:

| Item | Calls | Credits |
|---|---|---|
| Onboard: map ×3 + crawl 20pp ×1 | 4 | ~23 |
| Reddit `rt_search`, 3 queries × 3 brands | 9 | 18 |
| YouTube `yt_search` ×3 + `yt_comments` ×6 | 9 | 21 |
| News/web Search API, 10 queries | 10 | 30 |
| URL Scraper on top 20 articles | 20 | 20 |
| Amazon search ×3 + reviews ×6 | 9 | 18 |
| App Store RSS ×3 | 3 | 3 |
| Play Store probe ×3 | 3 | 3 |
| **Subtotal** | | **~136** |
| Retry/headroom | | ~60 |
| Reserved for the stage live call | | ~10 |
| **Ceiling** | | **~206 of 300** |

Roughly 95 credits spare. `bp_core.budget.CreditBudget` enforces the ceiling;
B2 prints a dry-run estimate before spending anything.

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
