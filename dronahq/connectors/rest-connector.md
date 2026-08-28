# The `brandpulse-bff` REST connector

One connector, eight endpoints, entered once and reused by both the dashboard
app and the chat agent.

## How this gets into DronaHQ

There are two documented routes, and unusually for this module, the fast one is
real rather than aspirational.

**Route A, the import.** DronaHQ documents an OpenAPI import for custom API
connectors: `Custom API connectors → "… (More Options)" → Import API`, taking an
**OpenAPI 3.0 spec as a JSON file** uploaded from disk. That is what
[`brandpulse-openapi.json`](brandpulse-openapi.json) is, and it is why that file
is pinned to 3.0.3 rather than 3.1. Upload it and the eight endpoints below
should arrive already defined.

The docs also note that when a spec carries no response schema, you use the
**Refresh Response** feature to call the endpoint once and generate bindable
keys. Ours does carry response schemas, but Refresh Response against a seeded
BFF is still worth doing, because it is the difference between bindings that are
declared and bindings DronaHQ has actually seen.

**Route B, by hand.** The table below. DronaHQ also documents an **Import cURL**
paste box on the API configuration screen that auto-fills method, URL, headers
and parameters from a pasted cURL command, so the curl lines at the end of this
file are a per-endpoint shortcut if the whole-spec import misbehaves.

Whichever route, the table below is the checked source of truth, because it was
read out of `bff/src/routes/` rather than remembered.

**What is not importable:** there is no export or import format for a connector
*definition*, and no API to create a connector programmatically. The OpenAPI
import populates a connector you have already created; it does not create one
from a file. So the connector itself is made in the console, by hand, once.

## The name is load-bearing

App exports match their dependencies **by exact name**, and DronaHQ's migration
docs are explicit that a connector missing from the target account must be
created before the app is imported. So if the connector in the target account is
called anything other than `brandpulse-bff`, the dashboard import is broken.

Worth knowing for a second import: the docs say connector *queries* travel with
the app export and are either added to the matching connector or **overwrite**
the existing queries of the same name. The connector and its account do not
travel. Nothing in the file list here contains a credential, because this
connector has none to carry.

The shapes these endpoints return are in
[`brandpulse-openapi.json`](brandpulse-openapi.json), which is a description of
our API in a standard format. It is not a DronaHQ export file and DronaHQ did not
produce it.

## Connector

Created at `DronaHQ → Connectors → + Connector`. Connected accounts are managed
afterwards under `Connector → Manage Account`.

| Setting | Value |
|---|---|
| Connector type | REST API |
| **Connector name** | `brandpulse-bff`. **Exact, case included.** App imports match dependencies by name. |
| Base URL | `{{baseurl}}`, per the section below |
| Auth type | **No Authentication** |
| Content type for bodies | RAW, which sends `application/json` |

### On `No Auth`, plainly

The BFF has no authentication. I grepped `bff/src/` for an auth hook, an API
key, a bearer token and a `preHandler`, and there is none: `app.ts` registers
CORS, a health route and the five route modules, and nothing else. Configuring
API Key auth here would mean sending a header the server does not read, which
looks like security and is not.

This is worth saying out loud to whoever runs the deployment: **the BFF's only
access control today is CORS**, and CORS constrains browsers, not servers.
Anything that can reach the BFF's host can read a brand's mentions. That is a
`bff/` concern and not mine to change, but a DronaHQ connector pointed at a
public URL is the moment it starts to matter.

It got sharper while I was writing this. `POST /api/brands` landed in the BFF
after I wrote the first draft of this file, and it is an unauthenticated write
that spends the account's Anakin credits and LLM tokens on a website of the
caller's choosing. Read access to mentions leaking is bad; an open endpoint that
bills the account is a different category. Same conclusion, more urgency: this
belongs to `bff/`'s owner, and it wants settling before the BFF is reachable
from anywhere but localhost.

### The base URL must not be a hardcoded host, and this is the fiddly part

The requirement is simple: the same configuration has to work against
`http://localhost:8080` and against the deployed cluster. How DronaHQ wants that
expressed is less simple, and I want to be straight about what I could and could
not establish from the documentation.

**What is documented.** A variable interpolates with double curly braces, and
DronaHQ's REST API configuration page says verbatim to *"refrain from using `_`
in the variable name"*. So it is `baseurl`, never `base_url`. That restriction
binds DronaHQ variable names only; our JSON keys stay snake_case and are
untouched by it.

**What is also documented, and is probably the right mechanism.** DronaHQ ships
**Data Environments** on a connector, `Production, Staging, and Development,
where Production is the default`, managed from the connector's three-dot
`Manage Environment` menu. That is the documented way one connector points at
different hosts in different environments, and it is per-connector rather than
global.

**What I could not establish.** There is no documented cloud-wide global
variable with an interpolation syntax for a connector's base URL. The three
nearby things are each something else: app-level **Variables** are a data query
inside one app, **Global JS Objects** are account-wide JavaScript rather than a
config value, and the `%DRONAHQ_SHARED_<NAME>%` secret syntax is **self-hosted
only**, from version 3.7.0-edge. Whether a plain `{{baseurl}}` resolves in a
connector's base URL field on a cloud account is not something the docs state,
and I have no console to try it in.

So: **hold the two hosts in Data Environments**, which is documented, and treat
`{{baseurl}}` in the table below as the placeholder for whichever mechanism the
console actually offers on the account. What must not happen is the string
`http://localhost:8080` being typed into the connector, because a dashboard
wired to localhost is a dashboard that dies on stage.

| Environment | Value |
|---|---|
| Development | `http://localhost:8080` |
| Production | the deployed BFF service's origin |

No trailing slash, either way, because every path below begins with one.

The OpenAPI file expresses the same thing as a server variable:

```json
"servers": [{ "url": "{baseurl}", "variables": { "baseurl": { "default": "http://localhost:8080" } } }]
```

Whether DronaHQ's importer maps an OpenAPI server variable onto its own variable
or flattens it to the default literal is not documented. **Check the base URL
after importing** and fix it if the importer baked localhost in.

## The eight APIs

Add each one under the connector. `{{brandid}}` and `{{runid}}` are DronaHQ
variables, again without underscores. The BFF declares both path parameters as
`:id`; the rename is ours, so that two different ids can coexist in one app.

| API name | Method | URL | Query | Body |
|---|---|---|---|---|
| `getHealth` | GET | `{{baseurl}}/health` | none | none |
| `listBrands` | GET | `{{baseurl}}/api/brands` | none | none |
| `createBrand` | POST | `{{baseurl}}/api/brands` | none | RAW JSON, below |
| `getBrandPulse` | GET | `{{baseurl}}/api/brands/{{brandid}}/pulse` | `limit` optional | none |
| `getAlerts` | GET | `{{baseurl}}/api/brands/{{brandid}}/alerts` | `limit`, `since` optional | none |
| `getMentions` | GET | `{{baseurl}}/api/brands/{{brandid}}/mentions` | `limit` optional | none |
| `getRun` | GET | `{{baseurl}}/api/runs/{{runid}}` | none | none |
| `startRun` | POST | `{{baseurl}}/api/runs` | none | RAW JSON, below |

There is no ninth. In particular **there is no drafts endpoint**: reply drafts
arrive on `getBrandPulse` under `drafts`. And there is no approve, no
acknowledge and no send endpoint.

The BFF now has **two** write routes, not one: `POST /api/runs` starts a
collection run and `POST /api/brands` creates a brand. Both spend money. Neither
publishes anything anywhere, which is the invariant that matters.

### `listBrands` returns `id`, not `brand_id`

Every other response in this connector names the brand `brand_id`. This one
names it `id`, because `bff/src/rows.ts` declares `BrandSummary` as the BFF's
own picker shape rather than one of the agents' contract types. Three fields:
`id`, `name`, and `website` which is **absent** when the column is NULL rather
than null or empty. Unpaginated and ordered by name.

A binding to `{{listBrands.data[0].brand_id}}` will resolve to nothing, silently,
which is the whole reason this paragraph exists.

### `createBrand` body

```json
{
  "name": "{{brandname}}",
  "website": "{{brandwebsite}}",
  "keywords": [],
  "competitors": []
}
```

Four things about this one, all of which cost money to learn the hard way:

- **`website` must carry its scheme** and must be `http` or `https`. A bare
  `example.com` is a 400 here. That is deliberate: the route would otherwise
  hand it to bp-onboarder, which hands it to Anakin's crawler, and the failure
  would surface thirty seconds later as a 502 from an agent.
- **It returns 201, not 200.** A DronaHQ success condition written as
  `statusCode == 200` treats every created brand as a failure.
- **Read `brand_id` off the response.** The route mints it from a slug of the
  name plus a random four-character tail, so it is not predictable and must not
  be constructed client-side.
- **`onboarder_error` is present only when bp-onboarder failed**, and the brand
  was created anyway from the typed keywords alone. A screen that ignores that
  key tells the owner an agent enriched their profile when nothing did. Show it.

There is no update route and no delete route. A brand created here is created.

### `startRun` body

```json
{
  "brand_id": "{{brandid}}",
  "trigger": "on_demand"
}
```

`trigger` is one of `onboard`, `scheduled`, `on_demand`, `crisis_replay`. The BFF
returns 400 with the list when it is anything else.

`window_hours` and `force` are optional and **must be omitted rather than sent
empty**. `"window_hours": 0` is not a neutral default in Go, it is a window of
zero hours; the BFF rejects a non-positive integer with a 400, and
`bp-orchestrator` applies its own default of 24 when the key is absent. If you
add `window_hours` to the body template, make it conditional on the variable
being set.

### Defaults and ceilings on `limit`

The BFF caps silently, so a caller asking for 500 gets a capped page and no
error.

| API | Default | Ceiling |
|---|---|---|
| `getBrandPulse` | 50 mentions | 200 |
| `getMentions` | 50 | 200 |
| `getAlerts` | 20 | 100 |

The alert, topic and draft slices inside the pulse are fixed at 20, 12 and 20 and
`limit` does not move them.

### `since` on `getAlerts`

Exclusive, RFC 3339, and it must be a `created_at` that this same route returned
earlier. Not a browser clock reading. `bff/src/routes/alerts.ts` is explicit
about why: a clock running fast against the database skips an alert, and this
feed may never silently miss a crisis.

## Testing it without the console

Every one of these is a plain HTTP call, so the connector can be checked against
a running BFF before anyone opens DronaHQ:

```bash
BASEURL=http://localhost:8080
BRANDID=brd_demo

curl -fsS "$BASEURL/health"
curl -fsS "$BASEURL/api/brands" | jq '.[] | {id, name}'
curl -fsS "$BASEURL/api/brands/$BRANDID/pulse?limit=50" | jq 'keys'
curl -fsS "$BASEURL/api/brands/$BRANDID/alerts?limit=20" | jq 'length'
curl -fsS "$BASEURL/api/brands/$BRANDID/mentions?limit=50" | jq '.[0] | keys'
curl -fsS -X POST "$BASEURL/api/runs" \
  -H 'content-type: application/json' \
  -d "{\"brand_id\":\"$BRANDID\",\"trigger\":\"on_demand\"}"

curl -fsS -X POST "$BASEURL/api/brands" \
  -H 'content-type: application/json' \
  -d '{"name":"Acme","website":"https://acme.example","keywords":["acme"]}'
```

The last two spend credits, and the last one also writes two Postgres rows that
no route can delete. Read `dronahq/README.md` before running either.

I have not run these. They are transcribed from the route definitions, not from
a session against a live BFF.

## A note on CORS

`bff/src/app.ts` registers CORS for exactly one origin, `DASHBOARD_ORIGIN`, and
never `*`.

If DronaHQ executes connector calls **server-side**, from its own backend, CORS
never enters the picture and nothing needs changing. If any call is made from the
**browser**, the DronaHQ app's origin has to be the value of `DASHBOARD_ORIGIN`,
and since that variable takes one origin and not a list, it cannot be both the
Next.js dashboard and the DronaHQ app at once.

I could not confirm from DronaHQ's documentation which of the two it does. It
is the first thing to find out in the console: open the browser network tab on
the published app and see whether the request to the BFF leaves the browser. If
it does, this is a real conflict with `web/` and it needs `bff/`'s owner, not a
workaround here.
