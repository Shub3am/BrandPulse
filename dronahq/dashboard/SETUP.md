# Build the BrandPulse dashboard app in DronaHQ

Ten minutes, four controls, three queries. A brand dropdown, a KPI row, a table
of mentions, and a button that starts a collection run.

This is the demo-sized app deliberately. The full six-screen design, with the
alert feed, the draft screen and the evidence tables, is out of scope here.

**Do [`../connectors/SETUP.md`](../connectors/SETUP.md) first.** Every control
below binds to the `brandpulse-bff` connector.

Every control name, panel name and binding expression below is quoted from
DronaHQ's own documentation, checked on 2026-09-20. Where the docs paraphrase a
label rather than print it, this file says so.

---

## Step 0: one variable

The app needs one variable, `brandid`, holding the brand on screen. Start it at
`brd_mamaearth_ae6f`.

That is the demo brand: a real Indian D2C skincare company, onboarded live
through Anakin's crawler and filled by a real Anakin collection run. It holds
162 mentions off reddit, news and the open web, 4 LLM-named topics, 33 alerts,
4 reply drafts and a brief, at a metered cost of 93 Anakin credits. The other
brand with data, `brd_demo` (Suncoast), is the recorded fixture corpus and is
the safe fallback if the tunnel is down.

**DronaHQ variable names must not contain an underscore.** It is `brandid`, never
`brand_id`. This binds the DronaHQ variable name only. Our JSON keys stay
snake_case and are untouched.

---

## Step 1: three data queries

Go to **Data queries → New → Connector query** and pick the `brandpulse-bff`
connector. Create these three. Give each one the name in the first column,
because the bindings below use it.

| Query name | Connector endpoint | Variables | Run query |
|---|---|---|---|
| `brandlist` | `listBrands` | none | auto, on app open |
| `pulse` | `getBrandPulse` | `id` = `{{brandid}}`, `limit` = `50` | auto, re-runs when `brandid` changes |
| `startrun` | `startRun` | body `{"brand_id":"{{brandid}}","trigger":"on_demand","window_hours":504}` | **manual** |

**`window_hours` is not optional here, whatever the schema says.** Omitting it
means 24, which is `defaultWindowHours` in `agents/bp-orchestrator/pipeline.go`.
The recorded corpus was captured at a 504 hour window, and the fixture cache key
is a hash that includes the window, so a 24 hour run in replay mode matches no
fixture, collects nothing and returns a run with `mentions_collected: 0`. On
camera that looks exactly like a broken product. Send 504.

The auto versus manual choice is the **Run query** dropdown on the query. It
offers auto execution when a variable changes, or manual triggering. The
DronaHQ docs name the dropdown but do not print its two option strings
verbatim, so pick by meaning, not by matching a label in this file.

**`startrun` must be manual.** It spends Anakin credits and LLM tokens on every
call. An auto-running query here is a recurring bill.

Use **Test Query** on `brandlist` and `pulse` before you save either. If a test
returns HTML instead of JSON, the ngrok header is missing from that connector
endpoint.

---

## Step 2: the brand dropdown

**1.** From **Controls** in the left navigation, drag a **Dropdown** onto the
canvas. Label it `Brand`.

**2.** Select it, open the **Data** pane on the right, and bind:

| Property | Value |
|---|---|
| Options source | `{{brandlist.data}}` |
| Option label key | `name` |
| Option value key | `id` |

**The value key is `id`, not `brand_id`.** `GET /api/brands` returns the BFF's
own picker shape and calls the identifier `id`, where every other response in
this API says `brand_id`. A Dropdown bound to `brand_id` here renders a list of
blanks, with no error anywhere. This is the single most common mistake in this
app.

**3.** On the Dropdown's **events pane**, write the selected value into the
`brandid` variable. That is what makes `pulse` re-run when the brand changes.

Live right now, this dropdown shows three options: **Lumeo**, **Mamaearth** and
**Suncoast**. Pick **Mamaearth**, which is `brd_mamaearth_ae6f` and the brand
carrying real crawled data. **Suncoast** is the recorded corpus. **Lumeo** is
empty on purpose and is the control that proves the empty states work. The
roster grows whenever anyone calls `createBrand`, which is what the dashboard
looks like the moment you add a brand on camera.

---

## Step 3: the KPI row

Drag four **Statistics** controls into a row. Statistics is the display control
for a quantity with its trend. If you want a plain number with no trend, the
**Metric** control is the other option.

Bind each one in its **Data** pane:

| Tile label | Binding | Mamaearth | Suncoast |
|---|---|---|---|
| Mentions | `{{pulse.data.brief.numbers.mentions}}` | 162 | 97 |
| Negative share | `{{pulse.data.brief.numbers.negative_share}}` | 4.32 | 43.3 |
| Avg sentiment | `{{pulse.data.brief.numbers.sentiment_avg}}` | 0.0898 | -0.33 |
| Share of voice | `{{pulse.data.brief.numbers.share_of_voice}}` | 94.83 | 100 |

Both columns are read off the live API, not typed in. Switching the dropdown
between the two brands swaps every tile, which is the cheapest way to prove on
camera that the tiles are bound rather than hardcoded.

**Share of voice comes from the brief, never from `{{pulse.data.share_of_voice}}`.**
That top-level slot is hardcoded `null` in the BFF and a tile bound to it renders
empty forever. The same trap applies to `topics[].top_examples`, which is always
an empty array.

If you have room for a fifth control, add a **Text** control bound to
`{{pulse.data.brief.headline}}`. On Mamaearth it reads *"Negative sentiment
spikes amid hair loss concerns on Reddit."*, and on Suncoast *"Customer
dissatisfaction spikes over refund delays and product issues."* A sentence an
LLM wrote about this brand's own week is a better line to have on screen than
any number.

### The error banner, worth the extra minute

Add a **Text** control, styled red, visible when
`{{pulse.data.errors.length}} > 0`, bound to `{{pulse.data.errors}}`.

The pulse route runs seven independent queries and a failed one names itself in
`errors` rather than failing the request. So a pulse whose `alerts` query died
still returns HTTP 200 with an empty alerts array, and without this banner that
is indistinguishable from a quiet day.

---

## Step 4: the mentions table

**4.** Drag a **Table Grid** onto the canvas below the KPI row.

**5.** Mentions arrive two levels deep, `{ mention, enrichment }`, and a Table
Grid wants a flat array of objects. Flatten in the `pulse` query's **Transform
response** section with this JavaScript:

```js
return data.mentions.map(function (row) {
  return {
    source: row.mention.source,
    posted_at: row.mention.posted_at,
    author: row.mention.author,
    followers: row.mention.author_followers,
    text: row.mention.text,
    label: row.enrichment.sentiment_label,
    sentiment: row.enrichment.sentiment,
    intent: row.enrichment.intent,
    url: row.mention.url
  };
});
```

A transform on `pulse` changes what every binding against `pulse` sees, so if you
want the KPI row to keep reading `brief.numbers`, make this a **second** query,
`mentionfeed`, against `getMentions` with the same transform, and bind the Table
Grid to that instead. Two queries is the safer ten-minute build.

**6.** With the Table Grid selected, open the **Data** pane and set its data to
`{{mentionfeed.data}}`, or `{{pulse.data}}` if you transformed `pulse` itself.
Show these columns:

| Column | Comes from |
|---|---|
| `source` | `mention.source` |
| `posted_at` | `mention.posted_at` |
| `author` | `mention.author`, absent on sources with no author |
| `text` | `mention.text` |
| `label` | `enrichment.sentiment_label` |
| `intent` | `enrichment.intent` |

Live on Mamaearth, this table fills with **162 rows**: web 80, news 61, reddit
21. The enricher judged 55 of them to be genuinely about the brand, labelled 7
negative and gave 6 the intent `complaint`. On Suncoast it fills with **97
rows**: reddit 32, web 25, x 14, instagram 13, news 13, of which 42 are
negative and 42 are complaints.

The gap between 162 collected and 55 about the brand is worth saying out loud
on camera rather than hiding. Keyword search returns collisions, and the
classifier throwing out two thirds of its own input is the feature: an
incumbent that counted all 162 would report a volume number that means nothing.

Two fields to render carefully if you add them. `mention.rating` is **absent** on
non-review sources, and absent is not the same fact as `0`: a `0` is a real
one-star review and the review-bomb rule fires on it. Render absent as a dash,
never as zero. `mention.engagement` defaults to an empty object in the database,
so a key inside it can be missing, same rule.

---

## Step 5: the run button

**7.** Drag a **Button** onto the canvas. Label it `Run collection now`.

**8.** On the **events pane** to the right, select the **`button_click`** event.
The event is literally named `button_click`, not `onClick`.

**9.** Click **+ (Add) → Run Data Queries**, choose `startrun` in **Select Data
Query**, then **Continue** and **Finish**.

If you would rather call the connector directly without a saved query, the other
documented route is **+ (Add) → Server Side Actions → brandpulse-bff →
startRun**.

**10.** On the success leg, add a second **Run Data Queries** block that re-runs
`pulse`, so the KPI row and the table refresh with the new run in place.

### Four things about this button

**A second click inside the same hour is a no-op, and it does not say so.**
`runs` carries `UNIQUE (brand_id, kind, time_bucket)` and the bucket is hourly,
so the insert is `ON CONFLICT DO NOTHING` and the route hands back the run that
already exists, byte for byte, with HTTP 200. Nothing in the response
distinguishes that from a fresh run. This is the single most dangerous thing on
this page for a recording: rehearse the click at 14:50, shoot the real take at
15:05, and the take shows the rehearsal's run. Either rehearse in a different
hour, or change `trigger` to `crisis_replay` for the take, or delete the `runs`
row between takes.

**It spends money.** Every click starts a real collection run against Anakin and
an LLM. Put a confirmation step in front of it before anyone else touches the
app.

**The response is a RunRecord, not a success flag.** It comes back with
`status` of `running`, `ok`, `partial` or `failed`. A `partial` with a populated
`errors` array is a real run that got through some of its sources, and it is an
HTTP 200. Do not paint it red.

**The new mentions do not appear instantly.** The run is asynchronous. The
`RunRecord` returned has `status: "running"` and `mentions_collected: 0`, and
the tables fill over the following seconds. If you want to show the run
completing, add a `getRun` query bound to the returned `id` and poll it. For a
ten-minute build, just re-run `pulse` a few seconds later.

---

## Binding syntax, because there are three of them

| Scope | Syntax | Example |
|---|---|---|
| App, query result | `{{queryname.data}}` | `{{pulse.data.brief.numbers.mentions}}` |
| App, query metadata | `{{queryname.META.property}}` | `{{startrun.META.statusCode}}` |
| App, another control | `{{controlname.column}}` | `{{tablegrid.text}}` |
| Agent variable | `{{variable.name}}` | `{{variable.brandid}}` |

Three near-identical syntaxes across the platform is the likeliest source of a
binding that silently resolves to nothing. A DronaHQ binding to a missing key
renders empty rather than failing, so nothing on screen will tell you.

---

## Controls not to build

**There is no approve, reject, acknowledge or snooze endpoint.** Do not add a
button for one. A control that changes a badge in the browser and nothing in
Postgres tells an analyst their approval was recorded when it was not.

**There is no send endpoint and no posting code path anywhere in this product.**
If you add the reply drafts from `{{pulse.data.drafts}}`, the only action on them
is copy to clipboard. Every draft carries `requires_human_approval: true`, and
all 4 of Mamaearth's live drafts do, as do all 9 of Suncoast's. Show that field.

**Alerts ship the numbers that fired them.** If you add the alert feed from
`{{pulse.data.alerts}}`, render each alert's `evidence` rows as `metric`,
`value`, `threshold` and `window`. There is no LLM in the detector, so those four
numbers are the entire reason an alert exists. "Our AI detected unusual activity"
is a false statement about this system.

---

## If the app is blank

The URL `https://unknowing-tubby-thrive.ngrok-free.dev` **changes every time the
ngrok tunnel restarts**. When it changes, update `servers[0].url` in
`../connectors/brandpulse-openapi.json` and re-import, or edit the **URL** field
on each endpoint in the connector.

A query returning HTML instead of JSON means the `ngrok-skip-browser-warning: 1`
header is missing from that endpoint. See
[`../connectors/SETUP.md`](../connectors/SETUP.md) Part 2.
