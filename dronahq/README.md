# DronaHQ setup

Two front ends over the BrandPulse BFF: a chat agent brand owners talk to, and
an ops dashboard analysts read.

## What is in here

| Path | What it is | Real file or specification |
|---|---|---|
| `connectors/brandpulse-openapi.json` | The BFF's eight endpoints as an OpenAPI 3.0 spec | **A real, machine-importable file.** DronaHQ documents an OpenAPI 3.0 JSON import for custom API connectors. Not a DronaHQ export; a standard description of our API. |
| `connectors/rest-connector.md` | Connector name, auth, base URL, and the eight endpoints by hand | Specification. Connectors have no export format. |
| `chat-agent/system-prompt.md` | The agent's Instruction, to paste verbatim | **A real deliverable.** Text is the native format here. |
| `chat-agent/agent-config.md` | Model, tools, triggers, cost, test script | Specification. Agents have no export and no creation API. |
| `dashboard/build-spec.md` | Six screens, every query, every bound field with its JSON path | Specification. The app export's schema is not published. |

## Read this before you believe anything below

**I have no DronaHQ console login.** I could not click through the product, so
nothing in this file that happens inside DronaHQ has been run by me. What I did
verify, I verified two ways:

**Verified against this repository, by reading the code:**

- Every endpoint, path, query parameter, default and ceiling, from
  `bff/src/routes/`.
- Every response field bound anywhere in these files, from
  `bff/src/contracts.ts` and `bff/src/rows.ts`.
- That the BFF has **no authentication** on any route.
- That the BFF has **two write routes**, `POST /api/runs` and `POST /api/brands`,
  and no approve, no acknowledge and no send route.
- That `pulse.share_of_voice` is hardcoded `null` and `topics[].top_examples` is
  hardcoded `[]`.

**Verified by running it:**

- `connectors/brandpulse-openapi.json` parses, every `$ref` resolves, and it
  passes `openapi-spec-validator` as OpenAPI 3.0. That is the only thing in this
  directory I executed.

**Verified against DronaHQ's current documentation, on 2026-09-20:** the import
paths, the field names, the binding syntax and the restrictions quoted
throughout, each cited at the point it is used.

**Not verified, because it needs the console:** every click below. See
[Needs the console](#needs-the-console) at the end for the explicit list.

## Prerequisites

A running BrandPulse with data in it. From the repo root:

```bash
cp .env.example .env
docker compose up -d --no-recreate postgres
docker compose ps                    # must show (healthy)
./demo/run_demo.sh                   # seeds brand brd_demo
```

Then confirm the BFF answers, because a connector pointed at a dead BFF fails in
a way DronaHQ will describe unhelpfully:

```bash
curl -fsS http://localhost:8080/health
curl -fsS http://localhost:8080/api/brands/brd_demo/pulse | jq 'keys'
```

That second command should print the ten pulse keys: `alerts`, `brand_id`,
`brief`, `drafts`, `errors`, `mentions`, `profile`, `run`, `share_of_voice`,
`topics`.

## Variables to set

DronaHQ variable names **must not contain an underscore**; its REST API
configuration docs say to "refrain from using `_` in the variable name". Hence
one word each.

| Variable | Local | Deployed | Used by |
|---|---|---|---|
| `baseurl` | `http://localhost:8080` | the deployed BFF's origin | Both. No trailing slash. |
| `brandid` | `brd_demo` | the real brand id | Both. On the dashboard the header dropdown writes it, from `GET /api/brands`. The chat agent needs it set. |
| `windowhours` | unset | unset | Dashboard only, and only if an analyst overrides the run window. Leave it unset so `bp-orchestrator` applies its own default of 24. |

**Do not hardcode a host anywhere.** The same configuration has to work locally
and on the cluster. DronaHQ's documented per-connector mechanism for this is
**Data Environments**, which ship as Production, Staging and Development with
Production as the default, reachable from the connector's three-dot `Manage
Environment` menu. There is no documented cloud-wide global variable with an
interpolation syntax for a connector base URL, and the `%DRONAHQ_SHARED_<NAME>%`
secret syntax is self-hosted only. `connectors/rest-connector.md` spells this out.

## Step 1: create the connector

1. `DronaHQ → Connectors → + Connector`, pick **REST API**.
2. Name it **`brandpulse-bff`**, exactly, case included. This name is
   load-bearing: app imports match their dependencies by exact name, so a
   different name here breaks the dashboard import in step 3.
3. Auth type: **No Authentication**. The BFF has none, and configuring an API
   key would mean sending a header the server does not read.
4. Set the base URL from `baseurl` per the Data Environments note above.

## Step 2: load the eight endpoints

**Either** import the spec: under **Custom API connectors**, open the
**"… (More Options)"** menu → **Import API**, and upload
`connectors/brandpulse-openapi.json`. The import is documented as taking an
OpenAPI 3.0 spec as a JSON file from local disk, which is what that file is.

After importing, do two things:

- **Check the base URL.** Whether DronaHQ's importer maps an OpenAPI server
  variable onto its own variable, or flattens it to the default literal, is not
  documented. If `http://localhost:8080` has been baked in, fix it.
- **Run Refresh Response** on each endpoint against the seeded BFF. The docs
  describe this as the fix for a spec without response schemas. Ours has them,
  but a binding DronaHQ has actually seen beats a binding it has only been told
  about.

**Or** add the six by hand from the table in `connectors/rest-connector.md`.
There is also a documented **Import cURL** paste box that auto-fills method,
URL, headers and parameters, and that file ends with a curl line per endpoint.

Either way, verify against the table in `connectors/rest-connector.md` before
moving on. That table is the checked source; the spec file is a convenience.

## Step 3: build the dashboard

Follow `dashboard/build-spec.md`. It gives six screens, six queries, and every
bound field with its exact JSON path.

There is **no app JSON to import**, and this is worth being precise about rather
than apologetic. DronaHQ's app export is a console action and the internal
schema of the exported file is not published anywhere: not on docs.dronahq.com,
not on sdk.dronahq.com, not in context7's DronaHQ index. There is no
app-creation API and no CLI. Global Git Sync version-controls the same file but
documents no schema for it either. Round-tripping an export is supported;
synthesising one is not. So a hand-written `dashboard-app.json` in this
directory would be a file I invented wearing DronaHQ's name, and it is better to
ship a precise specification than a confident fake.

Read the four warnings at the top of `build-spec.md` first. They are properties
of our API that look like platform bugs when you meet them in the binding
editor, and between them they will save an hour.

**Once it is built: publish it, then `Config → App Export → Export app json`,
and commit the file here as `dashboard/dashboard-app.json`.** The app must be
published first or the export prompts you to publish. At that point the export
becomes the artifact and `build-spec.md` becomes the thing it is checked
against.

**Read the exported file before committing it.** The docs do not state whether
credentials travel with an export. Ours has none to leak, since the connector is
No Authentication, but check rather than assume.

## Step 4: build the chat agent

Follow `chat-agent/agent-config.md` for the five build steps, then paste
`chat-agent/system-prompt.md` into the **Instruction** field at step 3.

There is no agent JSON to import and there never will be. DronaHQ documents no
agent export, no agent import and no agent-creation API; its developer surface
for agents is API keys and request logs. This was an open question in
`docs/research/dronahq.md` §9 and it is now closed: the answer is no.

Test on the **Chat** trigger against the seeded brand before wiring anything
else. `agent-config.md` has the three questions that catch a dishonest agent.

## What these front ends cannot do

Say this at the demo rather than being asked.

- **Nothing posts.** BrandPulse has no posting code path and none is to be
  added. The chat agent shows a draft and says a human must approve it. The
  dashboard's draft screen copies text to the clipboard. Every reply draft
  carries `requires_human_approval: true` on every serialisation, because the Go
  marshaller emits it unconditionally with no struct field and no column behind
  it.
- **No approve, reject, acknowledge or snooze.** The BFF's pool is opened
  read-only on purpose and the agents own every write. A button that changed a
  badge in the browser and nothing in Postgres would be worse than no button.
- **No editing or deleting a brand.** `GET /api/brands` lists and
  `POST /api/brands` creates, and that is the whole of it. There is no PUT, no
  PATCH and no DELETE, and a stored profile is always version 1.
- **Share of voice is one number**, `brief.numbers.share_of_voice`. The richer
  `ShareOfVoice` type is declared in the contracts but nothing persists it, so
  `pulse.share_of_voice` is always null.
- **WhatsApp is out of the MVP** and no Meta account is on the critical path.

## Needs the console

Explicitly, the things that cannot be done from this repository. Every one of
these is a click by someone logged in to DronaHQ.

**Blocking, must happen before anything works:**

1. Create the `brandpulse-bff` REST connector. There is no connector import
   format and no API to create one.
2. Set the base URL per environment, and find out what the account actually
   offers: whether `{{baseurl}}` resolves in a connector's base URL field, or
   whether Data Environments are the only route. Not documented; I could not
   resolve it.
3. Import `brandpulse-openapi.json`, or enter the eight endpoints by hand.
4. Build the dashboard screen by screen from `build-spec.md`.
5. Create the agent, attach the four Connector Query tools, paste the
   Instruction, test in the playground, publish.

**Unresolved questions only the console can answer.** Each of these is a real
open question, not a formality:

6. **Does DronaHQ call our API server-side or from the browser?** This matters.
   `bff/src/app.ts` allows exactly one CORS origin, `DASHBOARD_ORIGIN`, never a
   list and never `*`. If DronaHQ calls from its own backend, CORS never
   applies. If it calls from the browser, the DronaHQ app's origin must *be*
   `DASHBOARD_ORIGIN`, and it cannot simultaneously be `web/`'s origin. Open the
   network tab on the published app and look. If it is client-side, this is a
   genuine conflict and it needs `bff/`'s owner.
7. **Does the OpenAPI importer honour the server variable**, or bake in the
   default literal?
8. **What does the app export file actually contain?** Read it before
   committing.
9. **How does a Table Grid render a nested object in a cell?** Undocumented.
   It decides how much JavaScript Transformation the mention grid needs.
10. **Does `{{queryname.isLoading}}` exist?** Undocumented. Loading states
    depend on it.
11. **What is the API base host for this account?** Only needed if something
    calls the agent from outside. Every doc example says
    `<your-dronahq-host>`; read the real one off Developer → API Keys. Do not
    assume `api.dronahq.com`.

**After it is built, to close the loop in this repo:**

12. Export the published app and commit it as
    `dashboard/dashboard-app.json`, after reading it.
13. Screenshots into `screenshots/`, for the PR and the pitch.

Export is manual and Git Sync is self-hosted only, so nothing here syncs itself.
If someone edits the app in the console and does not re-export, this directory
is stale and it will not announce it.
