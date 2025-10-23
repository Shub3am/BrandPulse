# Source strategy

The brief names eight sources. Research (2026-09-20) found that Anakin's Wire
catalogue does not carry four of them. This document records what we ship
instead, and what that costs the pitch.

Evidence for every claim here is in [research/anakin.md](research/anakin.md) §1.

---

## What changed

| Brief assumed | Reality | What we do |
|---|---|---|
| Wire covers X/Twitter | Not in catalogue (404, confirmed by Anakin's own blog) | Search API scoped `site:x.com`, snippets only — **degraded** |
| Wire covers Instagram | Not in catalogue (404) | **Dropped** |
| Wire covers Google Play reviews | Not in catalogue (404) | URL Scraper with `useBrowser: true` — **probe, 3 credits, early** |
| Wire covers App Store reviews | Not in catalogue (404) | Apple's public review RSS feed — **solid replacement** |
| Wire covers Flipkart reviews | Products only, no reviews action | **Dropped as a mention source** |
| Search API returns full page content | Returns `snippet` only | Chain Search → URL Scraper for bodies; budget 3 + N credits |

## What we ship

Seven sources, all real, none fabricated.

| Source | Mechanism | Gives us | Status |
|---|---|---|---|
| `reddit` | Wire `rt_search`, `rt_subreddit_posts`, `rt_post_details` | posts, comments, score, author | **Solid** |
| `youtube` | Wire `yt_search`, `yt_comments` | video metadata, comment threads | **Solid** |
| `amazon` | Wire `am_search_products`, `am_product_reviews` | star ratings, review text, dates | **Solid** |
| `news` | Search API | headlines, outlets, dates | **Solid** |
| `web` | Search API → URL Scraper | blog and forum bodies | **Solid** |
| `appstore` | `itunes.apple.com/in/rss/customerreviews/id=<id>/sortBy=mostRecent/json` via URL Scraper | star ratings, review text | **Good** — public documented feed |
| `playstore` | URL Scraper, `useBrowser: true` | star ratings, review text | **Probe** — validate first |

`x` ships only if the Search API's `site:x.com` results prove usable. It has no
engagement metrics and no follower counts, so it cannot feed the
`influencer_mention` rule. It is enabled as a low-yield source and is the first
thing the flow guard drops.

`instagram` and `flipkart` stay in the `Source` enum — the schema does not
change — but are not in any brand's enabled sources.

---

## Consequences for the pitch

Three parts of the brief's narrative touch a source we lost. Each has an
honest replacement.

**"A negative thread on Reddit or X at 11pm."** Reddit is solid and is the
better half of that sentence anyway — Reddit is where Indian D2C complaints
actually cluster, and it has full thread bodies rather than 280 characters.
The demo leads with Reddit. X is mentioned as a source we read, not as the
crisis trigger.

**"A burst of 1-star Play Store reviews after an app update."** This survives
only if the probe works. If it does not, the review-bomb rule fires on
**Amazon and the App Store**, which are both solid. The rule is
source-agnostic — it triggers on any review source — so no code changes. Only
the sentence in the pitch changes.

**"Seven sources" in the demo.** Holds either way: six solid plus App Store is
seven without Play Store.

We say this out loud in the README and on stage. A judge who knows Anakin's
catalogue will know X is not in Wire, and claiming it would cost more
credibility than the source is worth.

---

## The probe, and the rule about it

B2 runs the Play Store probe in Phase 2 **before** the main recording session:
three URL Scraper calls, three credits, against the demo brand's Play listing
with `useBrowser: true`. Pass means review text, rating and date are
extractable from the returned markdown. Fail means the source is dropped from
`sources_enabled`, the README says six sources, and the pitch line changes.

**No source is faked.** If a probe fails, the source does not appear in a
fixture, a dashboard, a count, or a sentence. Synthetic data exists in exactly
one place in this repo, `demo/inject_crisis.go`, and it is labelled as an
injection in the UI when it runs. `web/` has its own synthetic set in
`web/lib/demoData.ts` for a deliberately fictional brand, labelled in the top
bar and in the page footnote.

---

## Reversing this

If Anakin adds X, Instagram or Play Store to Wire before the deadline, the
change is small by construction: a new file at
`internal/anakin/sources/<name>.go` holding one unexported adapter, one line
added to `sources.Registry`, plus that source added to the brand's `sources`
list. The adapter signature and the registry are in CONTRACTS §4. No contract change, no schema
change, no orchestrator change. The enum already has the values.
