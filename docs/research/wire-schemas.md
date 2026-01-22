# Wire and Search response schemas — read live

Every schema below was read from the live API on **2026-09-20** with the real
key. Nothing here is inferred from a catalogue prose description. This file is
the adapter spec: every `json` tag in `internal/anakin/sources/` comes from
here, and a field not listed here does not exist.

**Task 1 spend: 9 credits of the ≤10 budget.** Catalogue reads were free.

| Call | Credits | Why |
|---|---|---|
| `GET /v1/wire/catalog/{reddit,youtube,amazon}` | 0 | parameter schemas + live per-action costs |
| `rt_search` | 2 | Reddit response schema |
| `POST /v1/search` | 3 | field name, bounds, locale/freshness params |
| `yt_search` | 1 | YouTube response schema |
| `am_search_products` | 1 | ASIN discovery + is an Indian brand reachable |
| `am_product_reviews` ×2 | 2 | review schema, second call to confirm the finding |

Three `POST /v1/search` probes returned 400 and were **not billed**, which is
how the field name and the `limit` bounds were settled for free.

---

## 1. The job envelope, and the double nesting that will bite you

Submit is `POST /v1/wire/task`, and it returns **`job_id`**, not `id`:

```json
{"status":"processing","job_id":"ab8e2696-…","poll_url":"/v1/wire/jobs/ab8e2696-…"}
```

`GET /v1/wire/jobs/{job_id}` on completion:

```json
{"status":"completed","credits_used":2,"execution_ms":3415,"data":{…}}
```

That `data` is **not** the payload. It is a second envelope:

```json
{"status":"ok","data":{…},"error":null,"files":[],
 "meta":{"action_id":"rt_search","catalog_slug":"reddit",
         "execution_ms":3293,"scraped_at":"2026-09-20T09:13:35.083000+00:00",
         "envelope_version":"v1"}}
```

So the real payload is at **`job.data.data`**, and the array inside it is keyed
differently per action. Unmarshal one level short and you get a struct of zero
values with no error, which is the empty dashboard this task exists to prevent.

| Action | Path to the array |
|---|---|
| `rt_search` | `job.data.data.posts` |
| `yt_search` | `job.data.data.data` ← the payload key really is `data`, three deep |
| `am_search_products` | `job.data.data.products` |
| `am_product_reviews` | `job.data.data.reviews` |

`data.status` is `"ok"` on success and `data.error` is `null`. A partial failure
surfaces there, not in the job's own `status`, so adapters check both.

Polling: every job in this session completed on the **first** poll, 3.3–4.3 s of
`execution_ms`. Budget one poll at ~2 s, not a long backoff.

## 2. Live per-action credit costs

`credits_per_call` is a field on every catalogue action, so the cost table does
not have to be maintained by hand. **`internal/anakin` should read it** rather
than hardcode, per CONTRACTS §3 ("per-action Wire costs vary").

| Action | Credits | Note |
|---|---|---|
| all 7 `rt_*` Reddit actions | 2 | |
| `yt_search`, `yt_video`, `yt_channel`, `yt_related`, `yt_suggestions` | 1 | |
| `yt_comments` | 3 | |
| `am_*` Amazon actions | **1** | research/anakin.md §6 said 2. It is 1. |
| `act_amazon_*` (navigation, listing, cart) | 2 | none of these are ours |

Catalogue action counts: Reddit **7**, YouTube **6**, Amazon **15**.

## 3. Reddit — `rt_search`

Params (live, with defaults): `query*`, `sort=relevance`, `time=all`,
`limit=25` (1–100), `after=""`, `match=true`.

`match` is documented in the catalogue as a client-side keyword filter and is
the reason it exists: *"required for reliable keyword/brand monitoring because
Reddit's sort=new ignores the query server-side."* Leave it `true`.

```json
{"query":"boAt Airdopes","sort":"new","time":"month","limit":25,"match":true,
 "after":null,
 "posts":[{
   "id":"1wjvvd1",
   "name":"t3_1wjvvd1",
   "title":"I know they're just things but losing them genuinely broke my heart",
   "subreddit":"delhi",
   "author":"BookishByte",
   "score":null,
   "upvote_ratio":null,
   "num_comments":null,
   "created_utc":"2026-09-18T17:01:44+00:00",
   "url":"https://www.reddit.com/r/delhi/comments/1wjvvd1/i_know_theyre_just_things_but_losing_them/",
   "permalink":"/r/delhi/comments/1wjvvd1/i_know_theyre_just_things_but_losing_them/",
   "selftext":"I lost my black boAt Airdopes Prime 513 ANC while travelling …  submitted by   /u/BookishByte   to   r/delhi [link]   [comments]",
   "thumbnail":null,"is_video":null,"over_18":null,"link_flair_text":null}]}
```

**`created_utc` is an RFC 3339 string, not a Unix epoch**, despite the name.
Parse with `time.RFC3339`, then `.UTC()`.

**`score`, `upvote_ratio` and `num_comments` were `null` on all 15 posts.** The
`selftext` tail (`submitted by /u/… to r/… [link] [comments]`) shows this path
is backed by Reddit's RSS, which carries no counters. Consequences:

- `Mention.Engagement` is all zeros for Reddit. Do not invent values.
- `influencer_mention` cannot fire on Reddit. It has no follower count either.
- Recovering counters means `rt_post_details` at **2 credits per post**, which
  at 15 posts is 30 credits for engagement numbers nothing in the demo reads.
  Not worth it. Leave `Engagement` zeroed and say so.

`selftext` needs the RSS tail stripped before hashing, or the same post found by
two keywords hashes differently. Strip from `"  submitted by   /u/"` onward.

Use `title + "\n" + selftext` as `Mention.Text`: 3 of 15 posts had a title
carrying the complaint and a thin body.

## 4. YouTube — `yt_search`

Params: `query*`, `limit`.

```json
{"query":"boAt Airdopes review","count":10,
 "data":[{
   "video_id":"zLXOfWalsMk",
   "title":"Are open earbuds here to stay? boAt Airdopes Loop Review",
   "channel":"Unboxed by Croma",
   "channel_id":"UC2ED_m4SuzuBJMiaJq2Vvfg",
   "views":"32,743 views",
   "published":"1 year ago",
   "duration":"2:31",
   "thumbnail":"https://i.ytimg.com/vi/zLXOfWalsMk/hq720.jpg?…",
   "url":"https://www.youtube.com/watch?v=zLXOfWalsMk"}]}
```

Two fields are unusable as-is:

- **`views` is a string**, `"32,743 views"`. Strip `" views"` and the commas.
- **`published` is relative**, `"1 year ago"`. There is no absolute timestamp on
  a search result. `Mention.PostedAt` must be exact and `Validate()` rejects
  zero, so **a search result cannot become a Mention.** That is fine: the
  mention unit for YouTube is the *comment*, and `yt_search` is only how the
  adapter discovers `video_id`s to pass to `yt_comments`.

`video_id` is returned directly, so the adapter does **not** parse it out of a
URL. research/anakin.md §3 says "takes a video id, not a URL — the adapter
extracts the id"; no extraction is needed on this path.

**`yt_comments` is not schema-read.** It costs 3 credits and the budget was
exhausted at 9. It is the one adapter that must still be written against a
hand-written fixture and verified on the first live call of Task 7. Its
catalogue params are `video_id*`, `limit=50`, `include_replies=true`,
`sort=top`. Carry this as the one open schema.

## 5. Amazon — and why it cannot be a mention source

`am_search_products` (`query*`, `page=1`, `limit=24`, `sort=featured`) works
well:

```json
{"query":"boAt Airdopes","page":1,"total_results":…,"returned":10,"sort":"featured",
 "products":[{
   "asin":"B0CZ426LLT",
   "title":"boAt Airdopes 311 Pro Truly Wireless in Ear Earbuds, … (Lavender Rush)",
   "brand":"",
   "url":"https://www.amazon.com/dp/B0CZ426LLT",
   "image_url":"https://m.media-amazon.com/images/I/61+bsawcIJL._AC_UY218_.jpg",
   "price":29.99,"list_price":null,"currency":"USD",
   "rating":3.7,"review_count":16309,
   "prime":false,"sponsored":false,"badges":[]}]}
```

Note `"brand":""` — empty on every result. Do not filter on it.

`am_product_reviews` takes **`asin` and nothing else**: no page, no limit, no
sort. And it returns this:

```json
{"asin":"B0CZ426LLT","total_reviews":16309,"returned":4,
 "average_rating":3.7,"rating_breakdown":{},
 "reviews":[{
   "review_id":"R175KCT5NL1YYA",
   "rating":5.0,
   "title":"",
   "text":"",
   "author":"Madhypriya Reddy Vanukuri",
   "date":"Reviewed in the United Kingdom on July 24, 2026",
   "verified_purchase":true,
   "helpful_votes":0}]}
```

**Every review's `title` and `text` is an empty string.** Confirmed on two
ASINs, 10 reviews total:

| ASIN | total_reviews | returned | reviews with text |
|---|---|---|---|
| `B0CZ426LLT` | 16 309 | 4 | 0 |
| `B0C7QHHT63` | 14 401 | 6 | 0 |

So the action yields 4–6 of ~15 000 reviews, and none of them carry the review.
A `Mention` built from this has `Text: ""`, which means every Amazon mention
shares one `ContentHash` and the collector's dedupe collapses them to a single
row. There is nothing for the enricher to classify.

**`amazon` cannot ship as a mention source.** See the decision and the
consequences in `HACKATHON_NOTES.md`.

Two further facts about this catalogue, both independent of the above:

- The catalogue's `domain` is **`amazon.com`** and its tags say `"us"`. There is
  no country parameter on `am_search_products`. Prices came back in **USD** and
  the review `date` strings name the UK, UAE, Italy, Australia, the US and
  India. This is the global storefront, not `amazon.in`.
- The catalogue object says `auth_required: true, auth_type: cookie`, but every
  action we called says `auth_mode: "none", auth_required: false` and all three
  succeeded anonymously. **Action-level auth wins**; ignore the catalogue-level
  flag.

`date` is prose, `"Reviewed in <country> on <Month D, YYYY>"`. If a
ratings-only use survives, parse with
`^Reviewed in (?P<country>.+?) on (?P<date>\w+ \d{1,2}, \d{4})$` and
`time.Parse("January 2, 2006", …)`.

## 6. Search API — `POST /v1/search`

**The required field is `prompt`.** Sending `query` returns
`400 {"error":"invalid_request","message":"Prompt is required"}`, identical to
sending `{}`. `docs/research/anakin.md` §2 already documented `prompt`
correctly; the `HACKATHON_NOTES` entry that called that doc wrong is the thing
that was wrong. Both are corrected.

`limit` is bounded: `{"prompt":"…","limit":999}` returns
`400 {"error":"invalid_request","message":"limit must be between 0 and 20"}`.
Default 5, max 20, as documented.

Response, verbatim shape:

```json
{"id":"search_30e33488c9d4549a201f1321b37e5bfc",
 "results":[{"url":"…","title":"…","snippet":"…","date":"2025-01-29",
             "last_updated":"2025-01-29"}]}
```

### The locale and freshness question, resolved

research/anakin.md §2 carried these as **UNVERIFIED**. They are resolved, and
the answer is the awkward one:

`{"prompt":"boAt Airdopes reviews","limit":3,"country":"in","freshness":"week","date_range":"past_month"}`
returned **HTTP 200**. Unknown parameters are **silently accepted and ignored** —
there is no 400 to tell you the parameter does not exist. The proof they were
ignored is in the results: with `freshness: "week"` and
`date_range: "past_month"` set, the three results were dated **2025-01-29**,
**empty**, and **2022-09-14**.

Consequences for the `news` and `web` adapters:

1. There is no server-side locale or freshness filter. **Filter client-side on
   `date`**, and pay for results that get discarded, exactly as §2 feared.
2. Never trust a 200 to mean a parameter was honoured. Any future parameter
   guess must be checked against the *results*, not the status code.
3. **`date` can be an empty string.** One of three results had `"date":""` and
   `"last_updated":""`. A `Mention` with a zero `PostedAt` fails `Validate()`,
   so a result with no parseable date is **dropped and counted**, not defaulted
   to `time.Now()`. Dating a 2022 article as today would poison the 14-day
   baseline the detector runs on.
4. Recency scoping has to live in the prompt text, which is unreliable, so the
   adapter over-fetches at `limit: 20` and filters down.

### `snippet` is much larger than the name suggests

The brief said Search returns full page content; research/anakin.md corrected
that to "a snippet". Both are a little off. The Times of India result measured
**2 966 characters** of cleaned page text, carrying the review's design,
performance, latency and battery sections and its verdict heading. The other
two results in the same response were of comparable length; only the first was
measured, because the response body was not saved to disk and re-fetching it
costs 3 credits.

That is ample for sentiment, intent and aspect classification, which is all the
`web` adapter needs. **The planned Search → URL Scraper chain for article bodies
is not needed for classification**, which removes the "URL Scraper on top 20
articles: 20 credits" line from the §6 recording budget. Keep the chain
available for the handful of results whose snippet comes back short.

It is cleaned text, not prose: it carries nav crumbs (`* News\n* Technology
News`), `[link]` artefacts and image captions. Feed it through `redact.PII` as
required, and expect the enricher to see some furniture.

---

## Open schemas after this task

| What | Why it is still open | Plan |
|---|---|---|
| `yt_comments` response | 3 credits, budget hit 9 of 10 | hand-written fixture; verified on the first live call of Task 7 |
| App Store review RSS | not a Wire action, no credits spent yet | Task 7, it is a URL Scraper call |
| Play Store listing | Task 2's probe, 3 credits, separate budget | Task 2 |
