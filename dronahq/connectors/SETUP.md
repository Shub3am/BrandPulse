# Build the BrandPulse connector in DronaHQ

Ten minutes, console open, no code. At the end you have one connector named
`brandpulse-bff` with eight endpoints, and a tested call returning real JSON
from the live BrandPulse API.

Every menu name and button label below is quoted from DronaHQ's own
documentation, checked on 2026-09-20. Where the docs paraphrase rather than
quote a label, this file says so instead of inventing one.

## Before you start

### The live base URL

```
https://unknowing-tubby-thrive.ngrok-free.dev
```

**This host changes every time the ngrok tunnel restarts.** When it changes,
two things need the new value and nothing else does:

1. `servers[0].url` in [`brandpulse-openapi.json`](brandpulse-openapi.json).
   Change it there, then re-import the file.
2. The **URL** field on each endpoint already created in the DronaHQ connector.
   DronaHQ's REST connector puts the full URL on the endpoint, not on a
   connector-wide base URL field, so a stale host lives in eight places, not
   one. Re-importing the corrected spec is faster than editing them by hand.

Check the tunnel is alive before you open DronaHQ:

```bash
curl -s -H 'ngrok-skip-browser-warning: 1' \
  https://unknowing-tubby-thrive.ngrok-free.dev/health
```

Expected: `{"ok":true}`. Anything else and there is no point configuring a
connector yet.

### The one header that makes or breaks this

Every request must carry:

```
ngrok-skip-browser-warning: 1
```

Free ngrok answers browser-like clients with an HTML interstitial page instead
of your API response. DronaHQ then receives HTML where it expects JSON, and the
endpoint test fails with a parse error that does not mention ngrok at all. This
is the single most likely way this setup goes wrong.

Proof. Without the header, with a browser user agent:

```
$ curl -s -A 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120 Safari/537.36' \
    https://unknowing-tubby-thrive.ngrok-free.dev/api/brands | head -c 120
<!DOCTYPE html>
<html class="h-full" lang="en-US" dir="ltr">
  <head>
    <meta charset="utf-8">
```

With the header, same user agent:

```
$ curl -s -A 'Mozilla/5.0 ... Chrome/120 Safari/537.36' \
    -H 'ngrok-skip-browser-warning: 1' \
    https://unknowing-tubby-thrive.ngrok-free.dev/api/brands
[{"id":"brd_lumeo_dbe9","name":"Lumeo","website":"https://lumeo.in"},{"id":"brd_mamaearth_ae6f","name":"Mamaearth","website":"https://mamaearth.in"},{"id":"brd_demo","name":"Suncoast","website":"https://suncoast.example.in"}]
```

The header is declared on all eight operations in `brandpulse-openapi.json`, as
a required header parameter with a default of `1`. Part 2 below is how you check
DronaHQ is actually sending it.

### The API has no authentication

The BrandPulse BFF has no auth hook, no API key and no bearer token. Choose
**None** in the Authentication selector. Configuring API Key Authentication here
would send a header the server never reads, which looks like security and is not.

---

## Part 1: create the connector and import the spec

**1.** Log in to DronaHQ. Go to **Connectors** in the left navigation.

**2.** Click **+ Connector**.

**3.** Choose the **REST API** connector tile. This is the generic custom API
connector, not one of the named SaaS connectors.

**4.** On the connector screen, fill:

| Field | Value |
|---|---|
| **Connector name** | `brandpulse-bff` |
| **Authentication** | **None** |

The name is load-bearing. A DronaHQ app export matches its dependencies by exact
name, and both the chat agent and the dashboard reference this connector as
`brandpulse-bff`.

Click **Save**.

**5.** Under **Custom API connectors**, find your new connector and click the
**… (More Options)** menu against it. Choose **Import API**.

**6.** Click **File Upload** and select
[`brandpulse-openapi.json`](brandpulse-openapi.json) from disk.

DronaHQ's importer accepts an **Open API 3.0** document as a valid **JSON** file.
Ours is pinned to `3.0.3` for exactly that reason. Do not convert it to 3.1 and
do not convert it to YAML. There is no URL-based import, only file upload.

**7.** Click **Select All**, then click **Import**.

You should now see eight endpoints, named by their `operationId`:

| Endpoint | Method | Path |
|---|---|---|
| `getHealth` | GET | `/health` |
| `listBrands` | GET | `/api/brands` |
| `createBrand` | POST | `/api/brands` |
| `getBrandPulse` | GET | `/api/brands/{id}/pulse` |
| `getMentions` | GET | `/api/brands/{id}/mentions` |
| `getAlerts` | GET | `/api/brands/{id}/alerts` |
| `startRun` | POST | `/api/runs` |
| `getRun` | GET | `/api/runs/{id}` |

Those names and their summaries are what appear on screen, so they are written
to read well on camera.

---

## Part 2: make sure the ngrok header is really being sent

**8.** Open the `getHealth` endpoint. On the API configuration screen, expand
the **Advance** section and find **Headers**, which sets the key and value pairs
sent in the request header.

Confirm a row reading:

| Key | Value |
|---|---|
| `ngrok-skip-browser-warning` | `1` |

If the import brought the header through with an empty value, type `1` into the
value box. If the row is missing, add it.

**Repeat for every endpoint you intend to use.** A header that arrives with an
empty value is the same as no header at all, and the failure shows up as a JSON
parse error rather than as a missing header.

There is no documented connector-wide Headers tab on a plain REST API connector.
The only connector-level request parameter mechanism DronaHQ documents,
**Configure Request Parameter** with **+ Add parameter**, belongs to the
Multistep Authentication flow, which does not apply here. So this is per
endpoint. It is eight rows and two minutes, and it is the difference between a
working demo and an unexplained parse error on camera.

---

## Part 3: test and save

**9.** Still on `getHealth`, click **Test and Save**. Expected response body:

```json
{ "ok": true }
```

If you get HTML starting with `<!DOCTYPE html>`, go back to step 8. The header is
not being sent.

**10.** Open `listBrands` and run **Test and Save**. Expected, verified against
the live API immediately before this file was written:

```json
[
  { "id": "brd_lumeo_dbe9", "name": "Lumeo", "website": "https://lumeo.in" },
  { "id": "brd_mamaearth_ae6f", "name": "Mamaearth", "website": "https://mamaearth.in" },
  { "id": "brd_demo", "name": "Suncoast", "website": "https://suncoast.example.in" }
]
```

The roster grows whenever someone calls `createBrand`, so the count you see may
be higher. `brd_demo` is the one with data in it.

Note the field name. This endpoint calls the identifier **`id`**, where every
other response in this API calls it `brand_id`. A Dropdown bound to `brand_id`
against this endpoint renders a list of blanks. It is the most common binding
mistake in this connector.

**11.** Open `getBrandPulse`. Set the path parameter `id` to `brd_demo` and
**Test and Save**. You get a large object whose top-level keys are exactly:

```
brand_id, profile, run, brief, share_of_voice, alerts, topics, mentions, drafts, errors
```

Two of those are empty by design, always:

- `share_of_voice` is always `null`. The share of voice figure to render lives
  at `brief.numbers.share_of_voice`.
- `topics[].top_examples` is always `[]`.

A control bound to either renders empty forever, and that is the API behaving
correctly, not a broken binding.

**12.** If an endpoint imported without a response schema, use **Refresh
Response** on it. It calls the endpoint once and generates the keys you can bind
to from what actually came back. Worth doing on `getBrandPulse` regardless:
bindings DronaHQ has seen beat bindings it has only been told about.

The connector is done. Leave it named `brandpulse-bff`.

---

## Fallback: Import cURL, one endpoint at a time

If **Import API** misbehaves, is missing on your plan, or produces endpoints
with a stale host, you do not need the spec at all.

On the REST API configuration screen there is an **Import cURL** box, sitting
between **Authentication** and **Method**. Pasting a cURL request auto-fills the
request method, URL, headers and parameters.

Click **Add API**, give it a **Connector API Name** from the table in step 7,
click **Import cURL**, paste one of the commands below, then **Test and Save**.
Every command here already carries the ngrok header.

`listBrands`, the one the demo dropdown needs:

```bash
curl --location 'https://unknowing-tubby-thrive.ngrok-free.dev/api/brands' \
  --header 'ngrok-skip-browser-warning: 1'
```

`getBrandPulse`, the one the dashboard is built on:

```bash
curl --location 'https://unknowing-tubby-thrive.ngrok-free.dev/api/brands/brd_demo/pulse?limit=50' \
  --header 'ngrok-skip-browser-warning: 1'
```

`getMentions`:

```bash
curl --location 'https://unknowing-tubby-thrive.ngrok-free.dev/api/brands/brd_demo/mentions?limit=50' \
  --header 'ngrok-skip-browser-warning: 1'
```

`getAlerts`:

```bash
curl --location 'https://unknowing-tubby-thrive.ngrok-free.dev/api/brands/brd_demo/alerts?limit=20' \
  --header 'ngrok-skip-browser-warning: 1'
```

`getRun`:

```bash
curl --location 'https://unknowing-tubby-thrive.ngrok-free.dev/api/runs/run_01M2ZC57CB9S6JJT5J2PFS71SP' \
  --header 'ngrok-skip-browser-warning: 1'
```

`startRun`. **Spends Anakin credits and LLM tokens.** Keep it manual:

```bash
curl --location 'https://unknowing-tubby-thrive.ngrok-free.dev/api/runs' \
  --header 'ngrok-skip-browser-warning: 1' \
  --header 'Content-Type: application/json' \
  --data '{"brand_id":"brd_demo","trigger":"on_demand"}'
```

`createBrand`. **Also spends credits, and no route can delete the row it
writes.** Here for completeness, not for the demo:

```bash
curl --location 'https://unknowing-tubby-thrive.ngrok-free.dev/api/brands' \
  --header 'ngrok-skip-browser-warning: 1' \
  --header 'Content-Type: application/json' \
  --data '{"name":"Acme","website":"https://acme.example","keywords":["acme"]}'
```

After a cURL import, replace the literal `brd_demo` in the URL with `{{brandid}}`
so the endpoint follows the brand dropdown.

---

## Things that will cost you time if you do not know them

**DronaHQ variable names must not contain an underscore.** Use `brandid` and
`runid`, never `brand_id` or `run_id`. This binds DronaHQ variable names only.
Our JSON keys stay snake_case and are untouched.

**`createBrand` returns 201, not 200.** A success condition written as
`statusCode == 200` treats every successfully created brand as a failure.

**`startRun` and `createBrand` both spend money.** Leave both on a manual
trigger. An auto-running query on either is a recurring bill.

**`limit` caps silently.** Asking `getMentions` for 500 returns 200 and no
error. Ceilings: mentions 200, alerts 100.

**`since` on `getAlerts` must be a `created_at` this API returned**, never a
browser clock reading. A clock running fast against the database skips an alert,
and this feed may never silently miss a crisis.

**Mentions are two levels deep**, `{ mention, enrichment }`. A **Table Grid**
wants a flat array of objects, so flatten in the query's **Transform response**
section. The exact transform is in
[`../dashboard/SETUP.md`](../dashboard/SETUP.md).

**There is no drafts endpoint, no approve, no acknowledge and no send.** Reply
drafts arrive on `getBrandPulse` under `drafts`, `requires_human_approval` is
`true` on every one of them, and nothing in this product posts anything
anywhere. Do not build a control that implies otherwise.

**If you need staging and production hosts**, DronaHQ's mechanism is **Data
Environments**: the **… (three dots)** against the data source, then **Manage
Environment**. The three environments are Production, which is the default,
Staging and Development, and they cannot be added to or renamed.

---

## What is live right now

Verified by curl against the base URL above, immediately before this file was
written:

| Call | Result |
|---|---|
| `GET /health` | `{"ok":true}` |
| `GET /api/brands` | 3 brands: `brd_lumeo_dbe9` (Lumeo), `brd_mamaearth_ae6f` (Mamaearth), `brd_demo` (Suncoast) |
| `GET /api/brands/brd_demo/mentions?limit=200` | 97 mentions |
| `GET /api/brands/brd_demo/alerts?limit=100` | 29 alerts |
| `GET /api/brands/brd_demo/pulse` | 4 topics, 9 drafts, brief present, 20 alerts, `share_of_voice` null |

Only `brd_demo` has mentions. `brd_lumeo_dbe9` has a profile, a failed run and
zero mentions. **Demo on `brd_demo`.**
