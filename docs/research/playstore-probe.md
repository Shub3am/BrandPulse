# Play Store probe

B2 Task 2. Does `POST /v1/url-scraper/scrape` with `useBrowser: true` return
usable Play Store reviews, or does `playstore` get dropped?

App probed: **boAt Hearables**, `com.boAt.hearables`, Imagine Marketing Limited,
1Cr+ downloads, 2.7L reviews, rated 4.6. It is the demo brand's app (see Task 3).

Probed 2026-09-20. Spend: **4 credits**, one over the 3 the brief allowed. The
overspend is explained at the bottom and I am not hiding it.

---

## Verdict

**Conditional pass. `playstore` ships, and it ships small.**

Two findings, and they pull in opposite directions.

**1. Every field we need is there, but in `html`, not `markdown`.** The brief's
pass line was "review text, star rating and date are all extractable from the
returned markdown". On `markdown` alone that is a **fail**: the star rating
disappears entirely and the date is left dangling next to the developer's reply
where it reads as the reply date. On `html`, from the same response and the
same 1 credit, all six fields come out cleanly.

**2. A listing renders exactly three reviews, and no method moved that number.**
Three calls, three approaches, the same three reviews every time. Against 2.7
lakh reviews.

So the source is real and it is honest, and it is worth about **3 mentions per
brand per run**. It is not a review-bomb detector. Anyone writing the pitch:
see "What this means for the demo" below before you promise anything.

---

## What comes out, per review

Extracted live from the call 1 response, all three reviews, nothing hand-edited:

```
f82e5cc1-7726-4443-a3dd-89dd5f5deb5f | 3/5 | 16 July 2026      | thumbs 11 | Bilal Musani       | Please add dB values to the Custom EQ sliders. Currently, there is no …
c5056eac-6c4c-40af-b3b2-255547be19b5 | 2/5 | 12 September 2026 | thumbs  0 | argha bhattacharya | The app represents a lazy work of software engineering. Old school UI.…
34b7b64c-d102-46e9-bfb7-0d0b39d432aa | 1/5 | 6 January 2025    | thumbs 46 | Piyush Kumar Singh | I have boat niravana ion, it's very difficult to connect with my samsu…
```

That is a complete `Mention` and then some:

| `models.Mention` field | Source in the DOM |
|---|---|
| `ExternalID` | `data-review-id`, a UUID, stable across all three calls |
| `Rating` (`*float64`) | `div.iXRFPc[aria-label]`, "Rated 3 stars out of five stars" |
| `PostedAt` | `span.bp9Aid`, "16 July 2026", `2 January 2006` layout |
| `Text` | `div.h3YV2d` |
| `Engagement.Likes` | `data-original-thumbs-up-count` |
| author | `div.X5PpBb`. **Real names. `redact.PII` runs before any prompt.** |

Note the ratings: 3, 2 and 1 out of 5, on an app rated 4.6 overall. Play's
default listing surface is the "most helpful" set, which skews negative because
complaints get upvoted. For a **listening** product that bias is useful, but it
is a bias and `bp-detector` must not read three angry reviews as a sentiment
collapse. Three mentions is below any sane alerting threshold anyway.

---

## The markdown trap, in full

This is the part worth reading twice, because a markdown-only adapter would
look like it worked.

Play renders the star rating as an **empty div carrying an `aria-label`**, with
five inline SVGs inside for the visuals. There is no text node. Every
HTML-to-markdown converter drops it, and Anakin's is no exception: `out of 5`
appears **0 times** in 11 925 characters of returned markdown.

The date is worse, because it does not vanish, it lies. Here is the markdown
around the first review, verbatim:

```
Please add dB values to the Custom EQ sliders. … Overall, the earbuds are
great, but this feature is really needed.

11 people found this review helpful

Imagine Marketing Limited

16 July 2026

Hi Bilal, thanks for the feedback, we will forward this to our concerned team.
```

Read that as a human and "16 July 2026" is obviously the developer's reply
date: it sits under the developer's name, above the developer's reply. It is
not. In the DOM the review date is `span.bp9Aid` inside the review's own
`<header>`, *before* the body, and the developer reply date is a different
element, `div.I9Jtec`, inside `div.ocpBU`. Markdown flattens two different
fields into one visually plausible and wrong order.

The fix is free: ask for `formats: ["markdown","html"]`. Same 1 credit. The
adapter parses `html` and ignores `markdown` entirely.

---

## Three attempts at volume, three failures

| # | Method | Reviews returned |
|---|---|---|
| 1 | Plain listing, `useBrowser: true` | 3 |
| 2 | `&showAllReviews=true` appended | 3, byte-identical markdown |
| 3 | `actions`: click "See all reviews", wait 8 s, scroll, wait, scroll, wait | 3 |

Call 2 is the documented old trick and it is dead. Modern Play ignores
`showAllReviews`; the returned markdown was the same 11 925 characters.

Call 3 did run. Duration went from 20 s to 40.5 s and the HTML grew from
1 314 952 to 1 329 482 bytes, so the click and the scrolls executed and the
dialog opened. It just did not bring more reviews into the captured DOM. Play
fills that dialog from an internal `batchexecute` RPC on scroll inside the
dialog's own scroll container, and a page-level scroll action does not drive it.

Per the brief, that is the cap: two retries, no more, and no faking the
difference. **3 reviews per listing is the number.**

### Free finding: the `actions` DSL, which nothing documented

Three deliberate 400s, which Anakin does not bill, gave up the whole schema:

```
{"type":"bogus"}     → actions[0]: unknown type "bogus"
                       (supported: wait, wait_for, click, scroll, write, press)
{"type":"click"}     → actions[0]: click requires a selector
{"type":"wait_for"}  → actions[0]: wait_for requires a selector
{"type":"press"}     → actions[0]: press requires a key
{"type":"wait"}      → actions[0]: wait milliseconds must be 1-15000
```

So: six action types, `click`/`wait_for` take `selector`, `press` takes `key`,
`wait` takes `milliseconds` bounded 1 to 15 000, `scroll` takes nothing.
[anakin.md](anakin.md) §4 lists `actions` as an empty array with no schema; this
fills it in. B1, `internal/anakin` can type this properly now.

---

## What this means for the demo

**The Play Store review-bomb story does not work.** It needed volume and there
are three reviews. Do not build a narrative beat on it, and do not let the
pitch imply Play Store review velocity is being monitored, because it is not.

What `playstore` is actually good for: a real sixth source, three genuine
in-market Indian reviews per brand per run, with a star rating that most other
sources do not give us. `Mention.Rating` is `*float64` precisely because most
sources have no rating, and Play is one of the few that does. It earns its
place as coverage, not as a headline.

**Source count is now six, not seven**: reddit, youtube, news, web, appstore,
playstore. Amazon died in Task 1 for unrelated reasons (empty review text). B5,
the README and the pitch say **six sources**.

## For whoever writes the adapter

- Request `formats: ["markdown","html"]`. Parse `html`. Ignore `markdown`.
- `useBrowser: true` is mandatory and costs nothing extra. Without it the
  reviews are not in the DOM at all.
- `cleanedHtml` is **not** a substitute. Anakin's cleaner strips the
  `aria-label` star div and the reviewer name; at 78 506 characters it keeps the
  review bodies and throws away the rating. Use the full `html`.
- **The class names are Google's obfuscated build output and they will rotate.**
  `h3YV2d`, `bp9Aid`, `X5PpBb`, `iXRFPc`, `I9Jtec`, `ocpBU` are not an API. When
  they change, the selectors match nothing.
  **So: if the parse yields zero reviews, return an error, never an empty
  batch.** A source that silently returns nothing is the exact failure this
  whole probe exists to prevent, and it would look identical to "no reviews this
  week". The fixtures recorded in Task 7 keep `replay` mode green regardless.
- `hl=en_IN&gl=IN` on the URL. Without them you may get another locale's review
  set, which is what killed Amazon.
- Take the rating from the `aria-label` integer, not from counting filled SVG
  paths. The filled and empty stars use the same `<path>` and differ only by the
  wrapper class (`Z1Dz7b` filled, `Wi7KIe` empty), which is one more obfuscated
  name to depend on.

---

## Credit accounting, including the overspend

| Call | What | Credits |
|---|---|---|
| 1 | Listing, `useBrowser: true` | 1 |
| 2 | Listing + `showAllReviews=true` | 1 |
| 3 | Listing + click/scroll `actions` | 1 |
| — | `actions` schema probes, four 400s | 0, not billed |
| ✗ | **`{"type":"scroll"}` against `example.com`** | **1** |
| | **Task 2 total** | **4**, budget was 3 |

The fourth credit was my error. I was probing the `actions` schema with
deliberate 400s, which are free, and `{"type":"scroll"}` turned out to be
**valid with no required parameter**. It returned HTTP 200 and billed a full
scrape of `example.com`. The lesson generalises: probing a schema with bad
input is free only while the input stays bad, and a parameterless valid value
is indistinguishable from a typo until the bill lands.

Running total: Task 1 9 + Task 2 4 = **13 of 300**.
