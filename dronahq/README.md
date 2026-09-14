# DronaHQ setup

Two front ends over the BrandPulse BFF: a chat agent brand owners talk to, and
an ops dashboard analysts read. Both are built by a human in the DronaHQ
console, following the three guides below, against a live public API.

## The public base URL

```
https://unknowing-tubby-thrive.ngrok-free.dev
```

**Every request must carry the header `ngrok-skip-browser-warning: 1`.** Free
ngrok returns an HTML interstitial page to browser-like clients without it, and
DronaHQ then receives HTML where it expects JSON. The failure looks like a JSON
parse error and says nothing about ngrok. Every operation in the OpenAPI spec
declares that header, and step 8 of the connector guide is how you confirm
DronaHQ is really sending it.

**This host changes every time the ngrok tunnel restarts.** When it does, fix it
in two places and nowhere else:

1. `servers[0].url` in `connectors/brandpulse-openapi.json`, then re-import.
2. The **URL** field on each endpoint already created in the DronaHQ connector.

Check it is alive before opening the console:

```bash
curl -s -H 'ngrok-skip-browser-warning: 1' \
  https://unknowing-tubby-thrive.ngrok-free.dev/health
```

Expected: `{"ok":true}`.

## The demo brand

Build both front ends against `brd_mamaearth_ae6f`, **Mamaearth**. It is a real
Indian D2C skincare company, onboarded through a live Anakin crawl and filled by
a real Anakin collection run: 162 mentions off web, news and reddit, 4 topics an
LLM named, 33 alerts, 4 reply drafts, 93 credits.

`brd_demo` (**Suncoast**) is the recorded corpus with a crisis injected into it,
and it is the brand to switch to when you want to show alerting, because a
healthy brand cannot demonstrate crisis detection. `brd_lumeo_dbe9` is empty on
purpose and exists to prove the empty states are written.

The recording kit, including which numbers to quote and what not to claim, is
[`../demo/DEMO_SCRIPT.md`](../demo/DEMO_SCRIPT.md).

## Start here, in this order

| Guide | What it gets you | Time |
|---|---|---|
| **[`connectors/SETUP.md`](connectors/SETUP.md)** | One REST connector named `brandpulse-bff`, eight endpoints, imported from the OpenAPI spec and tested. Both other guides depend on it. | 10 min |
| **[`chat-agent/SETUP.md`](chat-agent/SETUP.md)** | The `BrandPulse` agent, four tools, the Instruction pasted, and three demo questions whose answers exist in the live data right now. | 10 min |
| **[`dashboard/SETUP.md`](dashboard/SETUP.md)** | A small app: brand Dropdown, four Statistics tiles, a Table Grid of mentions, and a Button that starts a collection run. | 10 min |

The connector guide has a fallback path using DronaHQ's **Import cURL** box, with
the literal command for each endpoint, for when the whole-spec import misbehaves.

## Everything else in this directory

| Path | What it is |
|---|---|
| `connectors/brandpulse-openapi.json` | The BFF's eight endpoints as an OpenAPI 3.0.3 spec. The one machine-importable file here. Validated: parses, every `$ref` resolves, passes `openapi-spec-validator`. |
| `connectors/rest-connector.md` | Reference material on the connector: every endpoint, every default, every ceiling, read out of `bff/src/routes/`. The SETUP guide is the walkthrough; this is the detail behind it. |
| `chat-agent/system-prompt.md` | The agent's **Instruction**, pasted verbatim. |
| `chat-agent/agent-config.md` | Reference material on the agent: model, the four tools and why not six, triggers, cost, and the test script that catches a dishonest agent. |

## Why these are guides and not importable files

**Agents do not export and do not import**, and there is no API to create one.
DronaHQ's developer surface for agents is API keys and request logs. So the agent
is a runbook plus an Instruction, which is the honest form.

**The app export's internal schema is not published.** Round-tripping an export
is supported; synthesising one is not. A `dashboard-app.json` can exist only
after someone builds the app in the console and exports it.

**The connector layer is the exception.** DronaHQ documents an Open API 3.0 JSON
import for custom API connectors and an Import cURL paste box, and both are used
here. That is why `brandpulse-openapi.json` is pinned to 3.0.3 rather than 3.1,
and why nullable fields use OpenAPI 3.0's `nullable: true`.

## What is live right now

Curled against the base URL above on 2026-09-20, immediately before this file
was written.

| Call | Result |
|---|---|
| `GET /health` | `{"ok":true}` |
| `GET /api/brands` | 3 brands: `brd_lumeo_dbe9` (Lumeo), `brd_mamaearth_ae6f` (Mamaearth), `brd_demo` (Suncoast) |
| `GET /api/brands/brd_mamaearth_ae6f/mentions?limit=200` | 162 mentions: web 80, news 61, reddit 21 |
| `GET /api/brands/brd_mamaearth_ae6f/pulse?limit=50` | mentions 162, negative share 4.32, avg sentiment 0.0898, share of voice 94.83. 4 topics, 20 alerts, 4 drafts, `errors` empty |
| `GET /api/brands/brd_demo/mentions?limit=200` | 97 mentions: reddit 32, web 25, x 14, instagram 13, news 13 |
| `GET /api/brands/brd_demo/alerts?limit=100` | 29 alerts: 15 medium, 9 high, 3 critical, 2 low |
| `GET /api/brands/brd_demo/pulse?limit=50` | mentions 97, negative share 43.3, avg sentiment -0.33, share of voice 100. 4 topics, 20 alerts, 9 drafts |

**Two brands carry data.** `brd_mamaearth_ae6f` (Mamaearth) is the build target
and `brd_demo` (Suncoast) is the crisis fallback, per "The demo brand" above.
`brd_lumeo_dbe9` has a profile, a failed run and zero mentions, and the roster
grows whenever anyone calls `createBrand`.

**The pulse caps its alert array at 20.** `ALERT_LIMIT` in
`bff/src/routes/pulse.ts` is the ceiling, so Suncoast's 29 alerts arrive as 20
through `getBrandPulse` and as all 29 through `getAlerts`. A screen reading the
pulse and captioned "29 alerts" is wrong on camera. Read the count off the array
you actually bound, or call `getAlerts`.

`share_of_voice` at the top level of the pulse is `null` on both brands, always,
by design. The real figure is `brief.numbers.share_of_voice`.

Data can be momentarily empty while a collection run is in flight. Suncoast went
from 0 to 97 mentions in about thirty seconds while these guides were being
written. If the pulse comes back with empty arrays, wait a minute and curl again.

## The rules these front ends must not break

- **Nothing auto-posts, and there is no posting code path to add one to.** The
  chat agent has no send tool and must never claim anything was sent. The
  dashboard copies a draft to the clipboard. `requires_human_approval` is `true`
  on every draft this API ever serialises.
- **There is no approve, reject, acknowledge or snooze endpoint.** Do not build
  a control for one. A button that changes a badge in the browser and nothing in
  Postgres tells an analyst their approval was recorded when it was not.
- **Alerts ship the numbers that fired them.** There is no LLM in `bp-detector`,
  so an alert's `evidence` rows are the entire reason it fired. Render `metric`,
  `value`, `threshold` and `window`. "Our AI detected unusual activity" is a
  false statement about this system.
- **`pulse.share_of_voice` is always `null`** and **`topics[].top_examples` is
  always `[]`**. Both are hardcoded in the BFF for stated reasons. Share of voice
  comes from `brief.numbers.share_of_voice`.
- **`GET /api/brands` says `id`, everything else says `brand_id`.** A brand
  picker bound to `brand_id` renders a list of blanks.
- **`POST /api/runs` and `POST /api/brands` both spend money.** Leave both on a
  manual trigger.
- **The BFF has no authentication.** Connector auth is **None**. Anything that
  can reach this host can read a brand's mentions and can spend the account's
  credits. That is a `bff/` concern, and it is the reason this tunnel should not
  outlive the demo.

## What has and has not been verified

**Verified by running it, against the live API:** every path, every response
shape, every field name and every count quoted in these guides, plus the ngrok
interstitial behaviour with and without the header. The OpenAPI file parses,
every `$ref` resolves, and it passes `openapi-spec-validator` as OpenAPI 3.0.

**Verified against DronaHQ's current documentation, on 2026-09-20:** every menu
name, button label, control name and binding syntax in the three guides, each
quoted from the page that documents it. Where the docs paraphrase a label rather
than print it, the guide says so rather than inventing one.

**Not verified, because it needs a console login:** the clicks themselves. Nobody
who wrote these files has run them inside DronaHQ.
