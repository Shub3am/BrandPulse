# dronahq

## What this module owns

The two front ends over the BFF: the chat agent brand owners talk to, and the
analyst dashboard. It owns their configuration in whatever form DronaHQ actually
supports, which is one importable file and three written specifications, not the
two JSON exports this file used to promise. See "Formats" below.

## What it must not know about

Agent internals, Postgres, and Anakin. Both front ends reach BrandPulse only
through the BFF's HTTP routes and bind to the typed JSON those routes return.

It also must not grow a write path of its own. The BFF has two, `POST /api/runs`
and `POST /api/brands`, and both spend money. Neither publishes anything
anywhere, and that is the invariant to hold.

## Entry points

| File | What it is |
|---|---|
| `README.md` | Import steps, variables to set, and the explicit "needs the console" list. Start here. |
| `connectors/brandpulse-openapi.json` | The BFF's eight endpoints as an OpenAPI **3.0** spec. The one machine-importable file in this directory. |
| `connectors/rest-connector.md` | The connector by hand: name, auth, base URL, eight endpoints. The checked source of truth; the spec file is the convenience. |
| `chat-agent/system-prompt.md` | The agent's **Instruction**, pasted verbatim. Structured on DronaHQ's documented template. |
| `chat-agent/agent-config.md` | Model, the four tools, triggers, cost, and the test script that catches a dishonest agent. |
| `dashboard/build-spec.md` | Six screens, six queries, every bound field with its exact JSON path. |
| `dashboard/dashboard-app.json` | **Does not exist yet.** Produced by publishing the app and exporting it, then committed here. |
| `screenshots/` | Does not exist yet. Proof for the PR and the pitch. |

## Invariants and gotchas

### Formats, resolved 2026-09-20

The open question this file used to carry is closed, and the answer changed the
deliverable.

- **Agents do not export and do not import.** There is no agent export, no
  agent import and no API to create an agent; DronaHQ's developer surface for
  agents is API keys and request logs. So `chat-agent.json` cannot be produced
  and is not coming. The agent is a runbook plus an Instruction, which is the
  honest form. This supersedes what `docs/research/dronahq.md` §9 left open.
- **The app export's internal schema is not published anywhere.** Round-tripping
  an export is supported; synthesising one is not. `dashboard-app.json` exists
  only after someone builds the app in the console and exports it. Until then
  `build-spec.md` is the artifact.
- **The connector layer is the exception.** DronaHQ documents an OpenAPI **3.0**
  JSON import for custom API connectors, and an Import cURL paste box. Hence
  `brandpulse-openapi.json` is pinned to 3.0.3: 3.1 is not mentioned on that
  page. Downgrading cost us the JSON Schema null type, so nullable fields use
  OpenAPI 3.0's `nullable: true`.

### Binding against the BFF

- **The BFF has no authentication.** Connector auth is **No Authentication**.
  This file previously said API Key with a `Bearer` header; that was wrong and
  the code is the source of truth. `bff/src/app.ts` registers CORS, `/health`
  and five route modules, and no auth hook anywhere. `POST /api/brands` makes
  this sharper than it was: an unauthenticated write that spends credits.
- **Base URL is never a hardcoded host.** A dashboard wired to `localhost` dies
  on stage. DronaHQ's documented per-connector mechanism is **Data
  Environments**; there is no documented cloud-wide global variable for a base
  URL, and `%DRONAHQ_SHARED_*%` is self-hosted only.
- **DronaHQ variable names must not contain an underscore**, per DronaHQ's own
  docs. So `baseurl` and `brandid`, never `base_url`. This binds DronaHQ
  variable names only; our snake_case JSON keys are untouched.
- **Three interpolation syntaxes, different scopes.** App bindings are
  `{{name}}` and `{{query.data}}`; agent variables are `{{variable.<name>}}`;
  connector request bodies take `{{variablename}}`. This is the likeliest source
  of a binding that silently resolves to nothing.
- **`GET /api/brands` says `id`, everything else says `brand_id`.** That route
  returns the BFF's own `BrandSummary` shape from `rows.ts`, not one of the
  agents' contract types. A picker bound to `brand_id` renders a list of blanks.
- **`POST /api/brands` returns 201 and an unpredictable id.** A success
  condition written as `statusCode == 200` fails every created brand, and the
  id is a slug plus a random tail so it cannot be constructed client-side. When
  `onboarder_error` is present the brand was still created, from the owner's
  typed keywords alone, and a screen that hides that key claims an agent
  enriched a profile that nothing enriched.
- **`pulse.share_of_voice` is always `null`** and **`topics[].top_examples` is
  always `[]`**. Both are hardcoded in the BFF for stated reasons. A control
  bound to either renders empty forever. Share of voice comes from
  `brief.numbers.share_of_voice`.
- **Mentions are two levels deep** (`{mention, enrichment}`) and a Table Grid
  takes a flat array of objects. The documented flattener is **JavaScript
  Transformations** in the connector query's Transform section, not DQL. DQL is
  XPath-inspired and has no `flatten`; "Query JSON using SQL" is a separate
  AlaSQL feature, useful for joining two queries.
- **Read queries auto-run, write queries are Manual trigger.** The two options
  are named "Every time variables change" and "Manual trigger". `startRun` is
  manual, which is also the credit guard.
- **`since` on the alert feed is a `created_at` the route returned**, never a
  browser clock reading, because a fast clock skips an alert.

### Product rules these front ends must not break

- **Nothing auto-posts, and there is no posting code path to add one to.** The
  chat agent has no send tool and must never claim anything was sent. The
  dashboard's draft screen copies to the clipboard. `requires_human_approval` is
  `true` on every serialisation of every draft: Go's `MarshalJSON` emits it
  unconditionally and there is no struct field and no column behind it.
- **There is no approve, reject, acknowledge or snooze endpoint.** Do not build
  a control for one. A button that changes a badge in the browser and nothing in
  Postgres tells an analyst their approval was recorded when it was not. If
  approval is wanted, it is a new agent-side write path and a new BFF route.
- **Alerts ship the numbers that fired them.** There is no LLM in `bp-detector`,
  so an alert's `evidence` entries are the entire reason it fired. The dashboard
  renders `metric`, `value`, `threshold` and `window` as a table and the chat
  agent quotes all four. "Our AI detected unusual activity" is a false statement
  about this system, not a friendlier phrasing of a true one.
- **Guardrails live in the Instruction's `RULES & GUARDRAILS` section**: no
  refund, no admission of fault, no medical or safety claim, no naming an
  employee, no date commitment, no legal characterisation, and escalate a
  critical alert to a human. That list is not the same as the server-side floor:
  `internal/prompts/guardrails.md` holds 30 literal lowercase phrases in 6
  groups that `bp-responder` substring-matches against a draft, which catches
  wording but cannot express "escalate to a human". That one is carried by
  `requires_human_approval`. Both halves, not either.
- **WhatsApp is out of the MVP.** The chat agent runs on the **Chat** trigger.
  If it is ever switched on, `docs/research/dronahq.md` §1 is the verified
  route, and the trap is that the WhatsApp actionflow block is a client-side
  deep link that cannot deliver anything.

### Keeping this directory honest

- **Export is manual and Git Sync is cloud-unavailable.** If someone edits the
  app in the console and does not re-export, this directory is stale and will
  not say so.
- **The app must be published before it can be exported**, so exporting belongs
  in the end-of-day routine, not the last hour.
- **Read an export before committing it.** The docs do not state whether
  credentials travel with the file.
- **Nothing in this directory has been run inside DronaHQ.** Nobody here has a
  console login. `README.md` separates what was verified from what was not, and
  that separation is maintained rather than quietly dropped once someone does
  log in.

## Who calls this

Brand owners, in the DronaHQ chat. Analysts, in a browser. Judges, at the demo.

Downstream of `bff/` only. If a field bound here disappears from
`bff/src/contracts.ts`, this directory breaks silently, because a DronaHQ
binding to a missing key renders empty rather than failing.
