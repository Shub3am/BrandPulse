# Ops/analyst dashboard: build specification

## Why this is a specification and not an export

DronaHQ's app export is a console action, `Config → App Export → Export app
json`, and the *internal schema of that file is not published anywhere*. I
searched docs.dronahq.com, sdk.dronahq.com and context7's DronaHQ index for a
field reference, an annotated example or a declarative app-as-code format, and
found none: the export page's entire content is the click path plus a list of
the dependencies the file carries. There is also no app-creation API and no CLI.
DronaHQ's own platform API exposes Users, Groups and Notification, and nothing
else. Global Git Sync version-controls the same JSON at
`App → <App Name> → <App name>.json`, but documents no schema for it either,
which is version control for a platform artifact rather than app-as-code.

Round-tripping an export is supported. Synthesising one is not. An `app.json`
written from this repo would therefore be a JSON file I invented and labelled
with DronaHQ's name, which is exactly the thing not to do.

The connector layer is the exception and it does have a real importable file:
see [`../connectors/brandpulse-openapi.json`](../connectors/brandpulse-openapi.json).
The app layer does not.

What follows instead is everything the console needs, at the precision where
guessing stops: every screen, every query, every control, every bound field with
its exact JSON path into a real response, and every control that writes. Build
it once in the console, publish it, export it, and commit the export beside this
file. At that point this file becomes the thing the export is checked against.

Every path below was read out of `bff/src/routes/`, `bff/src/contracts.ts` and
`bff/src/rows.ts`. The schemas are in
[`../connectors/brandpulse-openapi.json`](../connectors/brandpulse-openapi.json).

## The four facts that will otherwise waste an hour

These are properties of our API, verified in the code, that look like bugs when
you meet them in the binding editor.

1. **`pulse.share_of_voice` is always `null`.** Not sometimes. Always. There is
   no `share_of_voice` table and no SOV field on `RunRecord`, so nothing
   persists an agent output of that type. The share of voice figure that exists
   is `brief.numbers.share_of_voice`. Bind to that. A control bound to the
   top-level field renders an empty panel forever.
   (`bff/src/routes/pulse.ts`, the `share_of_voice: null` literal.)

2. **`topics[].top_examples` is always `[]`.** `rows.ts` hard-codes it, because
   "at most 3 mentions chosen by engagement" is `bp-clusterer`'s rule and it has
   no column. To show examples for a topic, join `topics[].mention_ids` against
   `mentions[]` in a transform. Do not bind a grid to `top_examples`.
   (`bff/src/rows.ts`, `toTopic`.)

3. **Mentions are two levels deep.** Each item is
   `{ "mention": {...}, "enrichment": {...} }`. DronaHQ's Table Grid reference
   says the control "accepts array of objects to display data", and how it
   renders a nested object inside a cell is not documented anywhere. So the
   mention grid gets a flattening transform first. The documented tool for this
   is **JavaScript Transformations**, in the connector query's **Transform**
   section; DronaHQ has a page for exactly this case, "Display Nested JSON Data
   in a Tablegrid", and its examples are plain JavaScript assigning to `data`.
   Alerts, topics and drafts are flat enough to bind directly; only mentions and
   `alerts[].sample_mentions` are not.

   Two sibling features exist and neither is the flattener. **DQL** is not SQL,
   it is XPath-3.1-inspired, and its function reference has no `flatten` or
   `unnest`. **Query JSON using SQL** is a separate AlaSQL-backed feature that
   takes an array of JSON objects in its `FROM` clause and is useful for joining
   two queries, which is what the topic examples panel on screen 3 needs. Use JS
   for flattening and AlaSQL for the join, and do not reach for DQL for either.

4. **There is no draft approve/reject endpoint, and no send endpoint.** The BFF
   has two write routes and neither one touches a draft: `POST /api/runs` starts
   a collection run and `POST /api/brands` creates a brand. `bff/CLAUDE.md`
   states that the agents own every write and that `db.ts` opens the read pool
   with `default_transaction_read_only=on`, so an INSERT added to a read route
   later fails at the Postgres server rather than in review. **The drafts screen
   therefore reads and copies. It does not approve and it does not post.**
   Anything else on that screen would be a button wired to nothing. See "What
   this dashboard cannot do" at the end.

## Variables

Set these in the app. DronaHQ variable names must not contain an underscore
(DronaHQ's REST connector docs say so verbatim), which is why these are one word.

| Variable | Example | What it is |
|---|---|---|
| `baseurl` | `http://localhost:8080` | The BFF origin, no trailing slash. Never a hardcoded host, because the same app must run against localhost and against the cluster. |
| `brandid` | `brd_demo` | The brand on screen. Bound to the brand selector on the header. |
| `windowhours` | `24` | Passed to a manual run. Omit the field entirely to let `bp-orchestrator` apply its own default; see the run screen. |

## Queries

DronaHQ gives a data query two run options, named **"Every time variables
change"** and **"Manual trigger"**. Its binding docs say that queries which do
not alter data, such as a GET, run automatically when the screen opens, and that
queries which modify data, such as a POST, should run only when explicitly
triggered.

So `brandlist`, `pulse`, `alertfeed` and `mentionfeed` are "Every time variables
change", and `startrun` is "Manual trigger". That is exactly the behaviour we
want: the dashboard refreshes itself when the analyst switches brand, and
nothing spends the brand's credits without a click.

`createBrand` is deliberately **not** a query on this dashboard. It is an
analyst's read-and-respond tool, and brand creation belongs on the onboarding
form in `web/`. Adding it here would put an unauthenticated, credit-spending,
undeletable write behind a screen whose whole job is looking at numbers.

| Query | Connector endpoint | Trigger | Notes |
|---|---|---|---|
| `brandlist` | `listBrands` | Auto on load, once | Populates the header's brand dropdown. Landed in the BFF after this spec's first draft. |
| `pulse` | `getBrandPulse` | Auto on load, re-runs on `brandid` change | The whole first paint. |
| `alertfeed` | `getAlerts` | Auto, plus a refresh on an interval | Pass `since` for cheap polling; see below. |
| `mentionfeed` | `getMentions` | Auto, on the Mentions screen only | Needed only when the analyst wants more than the 50 the pulse carries. |
| `runlookup` | `getRun` | Manual | Poll a run id after starting one. |
| `startrun` | `startRun` | **Manual** | Spends credits. Confirmation dialog first. |

`{{pulse.data}}` is the response body; the documented binding form is
`{{queryname.data}}`. Response metadata is available as
`{{pulse.META.statusCode}}`, documented generally as
`{{_queryName_.META._propertyName_}}`.

`{{pulse.isLoading}}` and `{{pulse.error}}` are **not documented**. Do not build
a loading state or an error banner on them without checking they exist; the
error banner in the header below reads `pulse.data.errors`, which is a field of
our own response and definitely there.

### The `since` rule on `alertfeed`

`since` is exclusive and it must be a `created_at` value this same route
returned earlier, never a clock reading taken in the browser. `bff/src/routes/alerts.ts`
says why: a browser clock running fast against the database skips an alert, and
the one thing this feed may never do is silently miss a crisis. So the polling
loop stores `{{alertfeed.data[0].created_at}}` after each successful poll and
sends that value back on the next call. On the first poll, omit `since`.

---

## Screen 1: Overview

The analyst's landing screen. Answers "what happened, and is anything on fire".

### Header, on every screen

| Control | Binding | Notes |
|---|---|---|
| Dropdown "Brand" | Options from `{{brandlist.data}}`, writes `brandid` | Label `name`, value **`id`**. Not `brand_id`: `GET /api/brands` returns the BFF's own `BrandSummary` shape and its key is `id`, where every other response in this app says `brand_id`. A dropdown bound to `brand_id` here renders a list of blanks. |
| Text "Brand name" | `{{pulse.data.profile.name}}` | Empty when `profile` is null, which means no confirmed brand profile has been written yet. |
| Banner, red, visible when `{{pulse.data.errors.length}} > 0` | `{{pulse.data.errors}}` | One line per failed slot, formatted `slot: message`. This banner is not decoration. A pulse response with a failed `alerts` slot returns HTTP 200 with an empty alerts array, and without this banner that is indistinguishable from a quiet day. |

### Numbers row, five stat tiles

All five come from `brief.numbers`. **If `{{pulse.data.brief}}` is null, hide the
whole row and show "No daily brief generated for this brand yet."** Do not render
zeros: a zero here reads as a real measurement.

| Tile | Binding | Unit |
|---|---|---|
| Mentions | `{{pulse.data.brief.numbers.mentions}}` | count |
| Change | `{{pulse.data.brief.numbers.mentions_delta_pct}}` | percent, versus the prior period |
| Avg sentiment | `{{pulse.data.brief.numbers.sentiment_avg}}` | score |
| Negative share | `{{pulse.data.brief.numbers.negative_share}}` | fraction, 0 to 1 |
| Share of voice | `{{pulse.data.brief.numbers.share_of_voice}}` | fraction. **From the brief, not from `pulse.data.share_of_voice`.** |

Label the row with the window it actually covers:
`{{pulse.data.brief.period_start}}` to `{{pulse.data.brief.period_end}}`. An
unlabelled "mentions: 412" invites the analyst to assume it is today's.

### Headline and actions

| Control | Binding |
|---|---|
| Text, large | `{{pulse.data.brief.headline}}` |
| Rich text / Markdown viewer | `{{pulse.data.brief.markdown}}` |
| List "Suggested actions" | `{{pulse.data.brief.suggested_actions}}` |
| List "Competitor watch" | `{{pulse.data.brief.competitor_watch}}` |

### Mentions by source

A bar chart, one bar per source, counting `mentions[]` grouped on
`mention.source`. This is a **client-side grouping of the rows the API returned**,
so label it "of the last N mentions", where N is `{{pulse.data.mentions.length}}`.
It is not the period total. The period total is
`brief.numbers.mentions` and the two will differ, because the pulse embeds at
most 50 mentions by default and 200 at the ceiling.

Sources are the ten in the `Source` enum: `x`, `reddit`, `youtube`, `news`,
`playstore`, `appstore`, `amazon`, `flipkart`, `instagram`, `web`.

### Sentiment over the window

A line or stacked area chart over `mentions[]`, bucketed by `mention.posted_at`,
split on `enrichment.sentiment_label` (`negative`, `neutral`, `positive`,
`mixed`). Same caveat and same label: it is over the embedded slice, not the
period.

Filter to `enrichment.is_about_brand == true` before charting, and say so in the
subtitle. A mention with `is_about_brand` false matched a keyword but is not
about this brand, and leaving those in inflates every sentiment figure on the
screen.

---

## Screen 2: Mentions

### Query

`mentionfeed`, with `limit` bound to a control. Default 50, and the BFF silently
caps it at 200, so a user typing 500 gets 200 and no error. Show the cap in the
control's help text rather than letting the number quietly change.

### The flattening transform

Table Grid takes a flat array of objects. The response is an array of
`{mention, enrichment}`. Flatten with a JS transform on the query, producing one
row per mention:

| Column | Source path on the item |
|---|---|
| `source` | `mention.source` |
| `posted_at` | `mention.posted_at` |
| `author` | `mention.author` (absent on sources with no author) |
| `followers` | `mention.author_followers` |
| `text` | `mention.text` |
| `url` | `mention.url` (absent when the source gave none) |
| `rating` | `mention.rating` (**absent on non-review sources. Absent and 0 are different facts: 0 is a real one-star review and the `review_bomb` rule fires on it. Render absent as a dash, never as 0.**) |
| `likes` | `mention.engagement.likes` (the column defaults to `{}`, so a key can be missing; render missing as a dash, not 0) |
| `sentiment` | `enrichment.sentiment` |
| `label` | `enrichment.sentiment_label` |
| `emotion` | `enrichment.emotion` |
| `intent` | `enrichment.intent` |
| `aspects` | `enrichment.aspects`, joined with commas |
| `about_brand` | `enrichment.is_about_brand` |
| `competitor` | `enrichment.about_competitor` (absent unless the mention is about a competitor) |
| `mention_id` | `mention.id`, hidden, needed by the topic and alert joins |

### Controls

| Control | Effect |
|---|---|
| Dropdown "Source" | Client-side filter on `source`. Options are the ten enum values. |
| Dropdown "Sentiment" | Client-side filter on `label`. |
| Dropdown "Intent" | Client-side filter on `intent`. Options: `complaint`, `praise`, `question`, `purchase_intent`, `comparison`, `spam`, `news`, `other`. |
| Toggle "About this brand only" | Filter on `about_brand`. **Default on.** |
| Row click | Opens a detail panel with the full `text` and a link to `url`. |

Every one of these filters the rows already fetched. None of them is a server
query: `bff/src/routes/params.ts` says the read routes take a limit and a since
and nothing else, and that "give me the alerts that matter" is a business rule
belonging to `bp-detector`, not to a query string. Do not add a filter parameter
to the connector that the BFF will ignore.

---

## Screen 3: Topics

Bind a Table Grid directly to `{{pulse.data.topics}}`. Flat enough, except
`sentiment_mix`, which is an object.

| Column | Path | Notes |
|---|---|---|
| Label | `label` | The cluster's name, from `bp-clusterer`. |
| Summary | `summary` | |
| Size | `size` | Mentions in the cluster. |
| Trend | `trend` | This window's size over the prior window's. **1.0 is flat, and 1.0 is also what you get when there is no prior window.** Above 1.0 is growing. It is never 0; a zero would mean nobody set the field, and `Topic.Validate()` rejects it. |
| Negative | `sentiment_mix.negative` | An object keyed by sentiment label. **A label with no mentions has no key at all, not a zero.** Render a missing key as a dash. |
| Positive | `sentiment_mix.positive` | Same. |
| Window | `window_start` to `window_end` | |

**Examples panel**, below the grid, for the selected topic: take the selected
row's `mention_ids`, match them against the flattened mentions from screen 2,
and show what you find. State how many of the ids you resolved, because the
pulse carries at most 50 mentions and a topic of 80 will not resolve all of
them. Do not bind this panel to `top_examples`; see fact 2 above.

---

## Screen 4: Alerts

The screen the whole product exists for, so it is the one that must not lie.

### Grid

Bind to `{{alertfeed.data}}` (or `{{pulse.data.alerts}}` for the first paint).

| Column | Path |
|---|---|
| Created | `created_at` |
| Severity | `severity`. One of `low`, `medium`, `high`, `critical`. Colour it, but do not reorder by it: the feed is `created_at` order and severity is `bp-detector`'s output, not the dashboard's opinion. |
| Kind | `kind`. One of `spike`, `crisis`, `influencer_mention`, `competitor_move`, `review_bomb` |
| Title | `title` |
| Why | `why` |
| Status | `status`. One of `open`, `acked`, `snoozed`, `resolved` |

### Evidence panel: the reason this dashboard is trustworthy

On row select, render `{{alertfeed.data[i].evidence}}` as a small table. **Never
collapse it into a sentence.**

| Column | Path | Meaning |
|---|---|---|
| Metric | `metric` | What was measured |
| Value | `value` | What it reached |
| Threshold | `threshold` | What it had to cross |
| Window | `window` | Over what period |
| Detail | `detail` | Optional |

There is no language model in `bp-detector`. Every alert in this system is a
threshold comparison and these four fields are the entire reason it fired. The
repo rule is that alerts ship the numbers that fired them, and this panel is
where that rule becomes visible to a judge. A tooltip saying "our AI detected
unusual activity" would be a worse product and a false one.

For context when reading a row, the five rules and their thresholds are in
`docs/CONTRACTS.md` §2 under `bp-detector`. Do not restate them in the UI as if
they were live values; render `threshold` from the row.

### Sample mentions panel

`sample_mentions` on the selected alert, as a grid: `source`, `author`,
`posted_at`, `text`, `url`. This array is already whole `Mention` objects, one
level deep, so it flattens with a single map.

If it is shorter than you expect, that is correct behaviour and not a bug:
`rows.ts` drops a sample id whose mention is no longer stored rather than
standing in for it, because a placeholder mention on a crisis alert is a
fabricated quote. Show what is there. Do not add a "and N more" that you counted
from somewhere else.

### Polling

Refresh `alertfeed` on an interval, passing `since` as described under Queries.
Prepend new rows. Do not clear the grid on each poll.

---

## Screen 5: Reply drafts

Bind to `{{pulse.data.drafts}}`. This is the only place drafts are served; there
is no `/drafts` route.

| Column | Path | Notes |
|---|---|---|
| Channel | `channel` | A source value, or `whatsapp`, `email`, `statement`. |
| Text | `text` | The draft. Show it in full in the detail panel, not truncated in the grid only. |
| Tone | `tone` | |
| Status | `status` | `draft`, `approved`, `rejected`, `sent`. |
| Approval | `requires_human_approval` | **Always `true`, on every row, forever.** `ReplyDraft.MarshalJSON` in Go emits it unconditionally; there is no struct field and no column behind it. Render it as a fixed badge reading "Needs human approval". |
| Answers | `alert_id` or `mention_id` | Exactly one is present. Link it to the alert row or the mention row. |
| Do not say | `do_not_say` | The brand's own banned phrases, on top of the global guardrail floor in `internal/prompts/guardrails.md`. Show these next to the text, because they are what an editor must not reintroduce. |

### Controls

| Control | Effect |
|---|---|
| Button "Copy draft" | Copies `text` to the clipboard. |
| Static notice, always visible on this screen | "BrandPulse does not post. Copy this text and publish it from your own account." |

**There is no Approve button, no Reject button and no Send button on this
screen**, and adding one is not a small improvement. The repo rule is that
nothing auto-posts and that there is no posting code path to be added. The BFF
has no endpoint to write a draft status to. A button that changes a badge in the
browser and nothing in Postgres is worse than no button: it tells an analyst
their approval was recorded when it was not.

If approval is wanted later it is a new agent-side write path and a new BFF
route, and it belongs in `bff/` and `agents/`, not in a DronaHQ actionflow.

---

## Screen 6: Run and cost

### Latest run

From `{{pulse.data.run}}`. Hide the panel when it is null.

| Field | Path | Notes |
|---|---|---|
| Status | `status` | `running`, `ok`, `partial`, `failed`. **`partial` is a real run that got through some of its sources. Render it as amber, with the skipped sources beside it, never as a failure.** |
| Kind | `kind` | `onboard`, `scheduled`, `on_demand`, `crisis_replay` |
| Started | `started_at` | |
| Finished | `finished_at` | **Absent while the run is still going.** Render absent as "running", not as a blank cell. |
| Mentions collected | `mentions_collected` | |
| Credits | `credits_used` | Anakin credits. **0 is a run that has not spent yet, not a missing value.** |
| Tokens | `tokens_used` | |
| Cost | `cost_paise` | **Paise.** Divide by 100 for rupees and label the unit on screen. |
| Sources attempted | `sources_attempted` | |
| Sources skipped | `sources_skipped` | |
| Degraded reason | `degraded_reason` | Present when sources were skipped. |
| Errors | `errors` | A list. A populated list with status `partial` is normal. |

### Start a run

A button firing the manual `startrun` query.

Request body, RAW, `application/json`:

```json
{ "brand_id": "{{brandid}}", "trigger": "on_demand" }
```

`trigger` must be one of `onboard`, `scheduled`, `on_demand`, `crisis_replay`;
anything a person clicked is `on_demand`. The BFF rejects anything else with a
400 and a message naming the four.

**Send `window_hours` only if the analyst actually set it.** Omitting the key
lets `bp-orchestrator` apply its own default of 24. Sending `"window_hours": 0`
is not a neutral default in Go, it is a window of zero hours, and the BFF
rejects a non-positive integer with a 400 rather than silently accepting it.
`force` is likewise optional, a boolean, and re-runs a time bucket that already
has a run row.

The actionflow:

1. Confirmation dialog. It must say the run spends the brand's Anakin credits
   and LLM tokens. This is the only control in the app that costs money.
2. Call `startrun`.
3. Toast with `{{startrun.data.status}}` and
   `{{startrun.data.mentions_collected}}`.
4. Re-run `pulse`.

A 502 means `bp-orchestrator` was unreachable or answered badly; a 504 means it
did not answer within the BFF's timeout, which defaults to 120 seconds. Both
arrive as `{ "error": "..." }`. Show the message rather than a generic failure:
"bp-orchestrator did not answer within 120000ms" tells the operator where to
look and "something went wrong" does not.

---

## What this dashboard cannot do, and why

State these in the demo rather than being asked.

| Not here | Why |
|---|---|
| Approve or reject a draft | No BFF write route exists for it, and the BFF's pool is opened read-only on purpose. |
| Send or post anything | There is no posting code path anywhere in BrandPulse and none is to be added. |
| Acknowledge or snooze an alert | `status` is served but there is no route to write it back. |
| Edit or delete a brand | `POST /api/brands` creates. There is no PUT, no PATCH and no DELETE, and a profile is always version 1. |
| Create a brand from this dashboard | Possible, but deliberately not built. It belongs on `web/`'s onboarding form; see the note under Queries. |
| Show share of voice by source or by competitor | `ShareOfVoice` is a declared type with no table behind it. Only `brief.numbers.share_of_voice`, a single fraction, is served. |
| Filter mentions server-side | The read routes take `limit` and `since` only, deliberately. |
| Show a period total for mentions by source | The pulse embeds a capped recent slice. `brief.numbers.mentions` is the period total; the chart is over the slice, and it is labelled as such. |
