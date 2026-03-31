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

**Task 4 added 4 more credits** to close the schemas Task 1 could not afford:
`yt_comments` for 3 (§4) and one URL Scraper call for 1, which settled both the
App Store feed and the scraper's own response envelope. Running total across
Tasks 1 to 4 is 21 of 300; the per-task breakdown lives in research/anakin.md §6.

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

So the real payload is two `data` hops down from the poll response root, and
the array inside it is keyed differently per action. Unmarshal one level short
and you get a struct of zero values with no error, which is the empty dashboard
this task exists to prevent.

**There is no `job` key.** Where this file and HACKATHON_NOTES write
`job.data.data`, "job" names the poll *response*, not a field. Verified
2026-09-20: the poll response's top-level keys are exactly
`["credits_used", "data", "execution_ms", "status"]`. In Go that is

```go
var resp struct {
    Status      string `json:"status"`
    CreditsUsed int    `json:"credits_used"`
    ExecutionMS int    `json:"execution_ms"`
    Data        struct {
        Status string          `json:"status"`
        Error  *string         `json:"error"`
        Data   json.RawMessage `json:"data"` // the per-action payload
    } `json:"data"`
}
```

and the per-action struct is unmarshalled from `resp.Data.Data`. Read
`credits_used` off the poll response rather than assuming the catalogue price;
it is the only number that is actually true.

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
Reddit's sort=new ignores the query server-side."*

**`match: true` is a phrase filter, and it returns a silent zero.** Measured
2026-09-20 during Task 3. The query `"boAt vs Noise vs boult earbuds"` came
back with `post_count: 0`, `posts: []`, `error: null`, **and
`posts_dropped_by_filter: 22`**. Reddit returned 22 posts and the filter threw
away every one, because it wants the whole query string present in the post,
not any of its terms.

Two rules follow, and neither is optional:

1. **One brand term per call.** A multi-word comparison query is a guaranteed
   empty result. Share of voice is N searches, not one clever search.
2. **The adapter must read `posts_dropped_by_filter` and log it.** `post_count:
   0` with a non-zero drop count means "the query was too specific", and
   `post_count: 0` with a zero drop count means "Reddit had nothing". Those are
   different problems and the response distinguishes them. An adapter that
   reads only `posts` reports a quiet week during an outage.

The response also carries `subreddits[]`, a parallel array of full subreddit
objects (id, title, public_description, url, created_utc). Nothing in
`models.Mention` has a home for it, so ignore it, but know it is most of the
response size.

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

## 4. YouTube — `yt_search` and `yt_comments`

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

### `yt_comments`, read live in Task 4

Task 1 left this open: "it costs 3 credits and the budget was exhausted at 9,
so it must be written against a hand-written fixture and verified on the first
live call of Task 7". That is no longer true. It was read live on **2026-09-20**
for 3 credits against `video_id=zLXOfWalsMk`, the first video `yt_search`
returned in §4 above, and the adapter is written against the real shape. **No
YouTube schema is open.**

Params sent: `video_id=zLXOfWalsMk`, `limit=50`, `include_replies=true`,
`sort=top`.

```json
{"video_id":"zLXOfWalsMk",
 "title":"Are open earbuds here to stay? boAt Airdopes Loop Review",
 "channel_id":"UC2ED_m4SuzuBJMiaJq2Vvfg",
 "comments_count":20,"count":10,
 "data":[{
   "comment_id":"Ugy1taqzks8jSFGM4gN4AaABAg",
   "author":"@ANBARASANNanbumechanical",
   "author_channel_id":"UCaaAvRM1lxA-R1LU9hH-drg",
   "text":"Im using it , Need to set 100 % volume , u can't experience great music, …",
   "likes":"6",
   "published":"10 months ago (edited)",
   "is_pinned":false,
   "is_owner_reply":false,
   "reply_count":1,
   "parent_id":null,
   "depth":0}]}
```

Six things the adapter has to survive, all of them observed on this one call:

- **`likes` is a string**, `"6"`. Same shape as `views` on a search result, and
  every one of the ten came back as a plain integer in a string. The adapter
  parses it and falls back to 0, which `Engagement` already means as unknown,
  rather than guessing at an abbreviation it has never seen returned.
- **`published` is relative here too**, and this is the only timestamp on a
  comment. There is no absolute date anywhere in the payload. So the mention
  unit that *can* become a `Mention` still needs its `PostedAt` resolved from
  prose; see the `relativeTime` doc comment in `youtube.go` for why that is safe
  enough and what is recorded in `Mention.Raw` to keep it honest.
- **`published` carries a ` (edited)` suffix.** Two of the ten did. A regex
  anchored without it drops those comments.
- **`reply_count` is `null`, not `0`,** on every comment that has no replies,
  and on every reply. `json.Unmarshal` puts a `null` into an `int` as 0 without
  erroring, so `ReplyCount int` is correct and no pointer is needed.
- **`count` is not `comments_count`.** The video reports 20 comments and the
  call returned 10 with `limit=50`. `limit` is a ceiling, not a request, and
  `comments_count` is YouTube's own total, which includes comments the API did
  not hand back. Neither number is a count of what arrived; `len(data)` is.
- **`include_replies=true` flattens the thread into the same array.** Replies
  arrive as ordinary elements with `depth:1` and a `parent_id` pointing at the
  root comment. Four of the ten were replies. The adapter keeps replies: a
  reply is where the complaint usually lands ("is it still working??" under a
  year-old recommendation) and dropping `depth>0` would throw away 40% of this
  video's corpus.
- **`is_owner_reply` means the video's uploader, not the brand.** The one
  comment carrying it is `@UnboxedbyCroma`, whose `author_channel_id` equals
  the payload's own `channel_id`. On a reviewer's video that is a reviewer
  replying to a viewer, which is ordinary user content. It would only mean
  brand voice on a video uploaded by the brand's own channel, and nothing in
  `BrandProfile` records a brand's YouTube channel id to compare against. The
  adapter therefore does not read the field, and self-mention filtering is not
  a thing this source can do today. `mentions.Filter` drops on the collection
  window and `p.NegativeKeywords` only; neither would catch it.

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

## Open schemas

Task 1 closed with three open. All three are now closed, and no adapter in
`internal/anakin/sources/` is written against a guessed field.

| What | How it was closed | Where it is written down |
|---|---|---|
| `yt_comments` response | read live in Task 4, 3 credits | §4 above |
| App Store review RSS | read live in Task 4 via URL Scraper, 1 credit | research/anakin.md, "Read `html`, not `markdown`" |
| Play Store listing | Task 2's probe, 3 credits | research/playstore-probe.md |

The App Store call also settled what a URL Scraper response looks like, which
had been carried as UNVERIFIED in `sources/scrape`: the fields arrive at the
top level with no envelope, and only `html` carries a JSON document verbatim.
