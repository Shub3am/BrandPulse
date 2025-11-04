# DronaHQ: verified findings

Researched 2026-09-20 against **docs.dronahq.com** and **dronahq.com**. Auth
header is `api-key`, never `Authorization: Bearer`. Our `sk_` key belongs to the
**Agentic platform**, not the app platform.

**Read §1 before building either front end. It changes the WhatsApp plan.**

---

## 1. The blocker: inbound WhatsApp is native, outbound push is not

The product wedge is "a crisis alert reaches the founder's WhatsApp within
minutes". That is an **outbound, agent-initiated** message. DronaHQ's WhatsApp
support is **inbound-first**, and the two things are built completely
differently.

| Direction | Mechanism | Status |
|---|---|---|
| Brand owner messages the agent, agent replies | WhatsApp trigger on the Agentic platform, Meta WhatsApp Business API, webhook | **Native and documented** |
| Agent messages the brand owner unprompted | Not documented as an agent capability | **Not native** |
| App pushes a WhatsApp message server-side | Twilio connector, `SendWhatsappTextMessage` | **Documented, third party** |
| Actionflow "WhatsApp Messages" block | **Client-side deep link only** | **Does not solve this** |

### The trap: the WhatsApp actionflow block is not a sender

`docs.dronahq.com/reference/actionflow-blocks/whatsapp/` reads like an outbound
send. It is not. The page describes opening WhatsApp on the viewer's own device
with a prefilled message box that the viewer must then confirm and send:

> "When you preview the app, it will open the Configure WhatsApp web on your
> system. After publishing the app and running it on your mobile device,
> clicking the 'Send Message' button will open WhatsApp on your device."

Its only fields are **Message Text** and **Phone Number** (format
`+(country code)(phone number)`). There is no account credential field because
nothing is sent server-side. **This block cannot deliver a 3am crisis alert.**
Anyone who wires the alert path to it gets a demo where nothing arrives.
Source: https://docs.dronahq.com/reference/actionflow-blocks/whatsapp/

### What actually delivers the alert

The Twilio connector. Configured with **Account SID** and **Auth Token**, it
exposes these actions:

| Action | Use |
|---|---|
| `SendWhatsappTextMessage` | **The crisis alert path** |
| `SendWhatsappMediaMessage` | Chart or screenshot with the alert |
| `SendMessage` | SMS fallback |
| `SendVerify`, `SendVerifyWithConfiguration`, `VerifyCode` | OTP, unused here |

The docs state plainly: *"Add `whatsapp:` before adding the sender's and
recipient's numbers."* Both the from and the to number carry the prefix. This
confirms the guess already written into `dronahq/CLAUDE.md`.
Source: https://docs.dronahq.com/reference/connectors/twillio/

### Consequence for the architecture

Two WhatsApp surfaces, not one:

1. **Conversational**: DronaHQ Agent + WhatsApp trigger. Brand owner asks
   "how are we doing today", agent answers. Meta's own API, no Twilio.
2. **Alerting**: `bp-detector` fires, a DronaHQ app (or the agent via a tool)
   calls Twilio `SendWhatsappTextMessage`. Different credential, different
   provider, different phone number unless the founder onboards one number
   through both.

Meta's 24-hour customer-service window applies to path 1 and Twilio's
template rules apply to path 2. Neither is documented by DronaHQ, both are
Meta policy, and a 9am daily brief to someone who has not messaged in 24 hours
needs an approved template. **Budget a template approval, or make the demo
founder message the agent first.**

---

## 2. What DronaHQ is

A low-code platform for building internal tools: drag pre-built controls onto a
grid canvas, bind them to a database or REST connector, and wire behaviour with
Actionflows or JavaScript. It ships cloud-hosted and self-hosted.

Separately it sells an **Agentic platform** (`agents.dronahq.com`): LLM agents
with instructions, a knowledge base, memory, tool calling and triggers, which is
where the WhatsApp agent lives. **These are two products with two price meters
and two API surfaces**, which is the single most common source of confusion when
reading the docs. Sources:
https://docs.dronahq.com/getting-started/introduction/ and
https://www.dronahq.com/agents

## 3. WhatsApp trigger, in full

Meta's WhatsApp Business API, over webhooks. No Twilio, no Gupshup on this path.

**What the brand owner must have:** a Meta for Developers account with a
WhatsApp Business API app, and a WhatsApp Business number.

**Setup:**

1. Add a WhatsApp trigger to the agent in DronaHQ.
2. DronaHQ generates a **Webhook URL** and a **Verify Token**. Optionally paste
   the **Meta App Secret** (Meta → My Apps → Settings → Basic → App Secret).
3. In Meta → My Apps → [Your App] → Use cases → WhatsApp → Configuration →
   Webhook, paste the URL and token.
4. Subscribe to the `messages` event.
5. Confirm the webhook shows **Verified**.

Payload fields the agent reads:

| Path | What it is |
|---|---|
| `entry[0].changes[0].value.contacts[0].wa_id` | Sender's WhatsApp id |
| `entry[0].changes[0].value.contacts[0].profile.name` | Sender's name |
| `entry[0].changes[0].value.messages[0].text.body` | Message text |
| `entry[0].changes[0].value.messages[0].id` | Message id, **required to reply** |
| `entry[0].changes[0].value.metadata.phone_number_id` | Our business number |

Reply is automatic: the agent responds in-thread using the message id.
Source: https://docs.dronahq.com/agents/triggers/inbuilt-triggers/whatsapp/

Other inbuilt triggers: **Chat** (testing), **Webhook**, **Scheduler**,
**Instagram**, **Facebook Messenger**. Email is "coming soon". Plus 145+
platform triggers (Gmail, Slack, Outlook, GitHub, Salesforce).
Source: https://docs.dronahq.com/agents/triggers/introduction/

**Scheduler caveat:** cron is a five-field expression
(`minute hour day-of-month month day-of-week`), and you **cannot set both
day-of-month and day-of-week**, one must be `?`. Examples go down to `*/2 * * * ?`.
The docs do **not** say where a scheduled agent's output goes. A 9am brief
therefore needs an explicit delivery tool in the agent, not a schedule alone.
Source: https://docs.dronahq.com/agents/triggers/inbuilt-triggers/scheduler/

## 4. Auth

Two surfaces. Both use the header `api-key`. Neither uses `Authorization`.

| Surface | Key shape | Where it is made | Scope |
|---|---|---|---|
| **Agentic platform** | **`sk_...`** | Developer → API Keys → New Key | Agent, Data Agent, Voice Agent, plus per-agent access |
| App platform REST API | account or micro-app `tokenkey` | Admin Console → Settings | Account-wide, or one micro-app |

**Our `sk_` key is an Agentic platform key.** Its form takes Name, Scopes
(additive), Agent Access (All Agents or specific), and Expires (Never, 30 days,
90 days, 1 year, custom). The full secret shows **once**, after that only the
last 4 characters.

```
api-key: sk_<your-secret>
```

The documented example is a host-relative path, not a fixed public base URL:

```bash
curl -X POST https://<your-dronahq-host>/voice/outbound/dispatch \
  -H "api-key: sk_your-generated-secret-here" \
  -H "Content-Type: application/json" \
  -d '{"agent_id": "<voice-agent-uuid>", "destination_phonenumber": ["+14155551234"]}'
```

Sources: https://docs.dronahq.com/agents/developer/api-keys/ and
https://docs.dronahq.com/agents/developer/introduction/ and
https://docs.dronahq.com/rest-apis/api-authentication/

**Do not send this key anywhere yet.** The base host is unresolved (see §9).

### Calling into an agent from Go

The **Webhook trigger** is the inbound door for `bp-detector`. It generates a
URL, takes an optional generated API key on the same `api-key` header, accepts
GET and POST, and exposes `body`, `query` and `headers` (headers normalised to
lowercase with underscores) to the agent.

```bash
curl -X POST https://your-webhook-url \
  -H "api-key: your-generated-api-key-here" \
  -H "Content-Type: application/json" \
  -d '{"message": "Hello"}'
```

Response type is configurable: **Standard** returns structured JSON
synchronously against a JSON Schema, **None** is fire-and-forget. Stream is
"coming soon". Standard plus a JSON Schema is what lets a Go agent post an
`Alert` struct and read the agent's decision back in one call.
Source: https://docs.dronahq.com/agents/triggers/inbuilt-triggers/webhook/

## 5. Calling our agent URLs from DronaHQ

Two mechanisms, both fine for typed JSON.

**REST API connector** (reusable, the right choice for the dashboard). Name it,
pick an auth type, then add individual APIs with name, URL and method
(GET/POST/PUT/DELETE), test, save. Managed under Connector → Manage Account.
Auth types available across the platform: No Auth, API Key, Basic, AWS, Digest,
Hawk, JWT Bearer, NTLM, OAuth 2.0, OAuth PKCE, OAuth Client Credentials, OAuth
JWT, OAuth 1.0, Multistep. For API Key you pick a **Target**: `Header`,
`Querystring`, `Body` or `None`.

Custom headers and JSON bodies are both supported. Content types:

| Setting | Content-Type |
|---|---|
| RAW | `application/json` |
| Regular Form | `application/x-www-form-urlencoded` |
| Body/Form Parameters | string in a named format, or key-value pairs |

Dynamic values interpolate with double braces inside the raw body:

```json
{ "username": "{{variablename}}" }
```

**Gotcha, verbatim:** *"Wrap the variable name inside double curly braces to
make it dynamic, refrain from using `_` in the variable name."* This constrains
**DronaHQ variable names**, not our JSON keys. Name DronaHQ variables
`brandid`, not `brand_id`. Our `internal/models` snake_case field names are
untouched by this.

**Call REST API actionflow block** (one-off, inside a flow). Fields:
**Endpoint Url**, **HTTP Method** (GET/POST/PUT/PATCH/DELETE), **Headers**,
**Data**, **Proxy**, **Timeout** (milliseconds). Result is read as
`{{CALLRESTAPI.output}}`.

Sources: https://docs.dronahq.com/reference/connectors/rest-api/ and
https://docs.dronahq.com/rest-apis/configuring-apis/ and
https://docs.dronahq.com/reference/actionflow-blocks/callrestapi/

## 6. Binding typed JSON to controls

`{{ }}` everywhere. A control binds to a query's result as `{{data_query1.data}}`.
Nested access is dot notation. Response metadata is available as
`{{_queryName_.META.statusCode}}`.

Read queries (SELECT, GET) **run automatically** on screen load and re-run when
a referenced variable changes. Write queries (INSERT, POST) must be set to
**Manual trigger** and fired from an actionflow. Relevant to us: a dashboard
bound to a GET on an agent URL refreshes itself, which is what we want, and the
approve/reject POST must be manual, which is what we want too.

**Table Grid takes an array of objects directly**, which is exactly the shape of
every list field in our artifacts:

```
[
  { id: 1, name: 'Carlyn Bartle', email: 'Carlyn.Bartle@example.com' },
  { id: 2, name: 'Murry Rowsel', email: 'Murry.Rowsel@example.com' }
]
```

Bind statically, or dynamically from Data queries, Sheets or Custom functions.

**The caveat that matters.** The docs say direct binding *"will work only on
JSON data which isn't too nested."* For deeper structures DronaHQ offers **SQL
over JSON** and **DQL** (DronaHQ Query Language) to flatten first, plus custom
JavaScript. The claim in the brief that DronaHQ "binds to the JSON directly
rather than through a translation layer" holds for a flat array of objects and
**stops holding** for a struct-of-structs. Either the artifact exposes a flat
top-level array per table, or DQL becomes the translation layer we said we did
not want.

Sources: https://docs.dronahq.com/binding-data/data-binding-framework/ and
https://docs.dronahq.com/reference/controls/table-grid/

## 7. Export and import

**Apps: yes, JSON, committable.**

| Step | Path |
|---|---|
| Export | Config → App Export → **Export app json** |
| Import | Apps → My Apps → **+** → **Import App**, upload the json |

The app **must be published first** or you get a prompt to publish. The export
sidebar lists every dependency used: **Connectors, Sheets, PDFs, Custom Controls
and Widgets**. On import you get a dependency summary, then Confirm and Install.

**The migration gotcha:** dependencies are matched **by exact name**. The target
account must already contain connectors, custom controls and widgets with
identical names or the import is broken. Our `README.md` in `dronahq/` must list
connector names verbatim.

Git Sync exists but is **self-hosted only, not on cloud**, and the per-app
version is marked deprecated in favour of Global Git Sync. On cloud, export is
manual and the repo goes stale the moment someone edits the app and forgets.

Sources: https://docs.dronahq.com/building-apps-concepts/app-export/ and
https://docs.dronahq.com/building-apps-concepts/migrating-apps-between-accounts/
and https://docs.dronahq.com/git-sync/

**Agents: no documented export.** Repeated searches of the agents docs for an
export, download or duplicate path returned nothing. App Export is documented
under `building-apps-concepts`, which is the app platform, and it lists app
dependencies only. **This directly contradicts `dronahq/CLAUDE.md`, which
declares `whatsapp-agent.json` as an exported DronaHQ Agent.** See §9.

## 8. Money and demo hazards

Two separate meters. Do not quote one while using the other.

**Agentic platform** (this is the one the `sk_` key bills against):

| Plan | Price | AI credits/mo | Tool calls/mo | Tools per agent | Branding | Tracing |
|---|---|---|---|---|---|---|
| Free | **$5 credit** | from that credit | from that credit | n/a | n/a | n/a |
| Starter | $100/mo yearly, $140 monthly | 5,000 | 10,000 | **10** | **"Powered by DronaHQ" forced** | 30 days |
| Business | $500/mo yearly, $650 monthly | 25,000 | 100,000 | Unlimited | Removable, custom domain | 90 days |
| Enterprise | Custom | | | | plus voice, Gen UI, SSO | |

Add-ons: 25,000 AI credits for $50 (rollover), 5,000 tool calls for $10.

**App platform:**

| Plan | Price | Tasks/mo |
|---|---|---|
| Free trial | **One month of Business, no credit card** | Business limits |
| Starter | $100/mo yearly, $140 monthly | 25,000 |
| Business | $500/mo yearly, $700 monthly | 125,000 |
| Self-hosted Business | $1,500/mo | 300,000 |

Add-ons: 10,000 tasks for $20, AI credits $10 per 5,000.

Sources: https://www.dronahq.com/agents/pricing/ and https://www.dronahq.com/pricing/

**What bites in a hackathon demo:**

- **A $5 agent credit is small.** Every knowledge-base embedding, vector search
  and LLM turn draws on it. Build the agent's instructions and knowledge base
  once, do not iterate on embeddings live, and test the conversation on the
  **Chat** trigger before wiring WhatsApp.
- **"Powered by DronaHQ" branding** is present below Business on the agents
  platform. Judges will see it. Fine, but do not promise a white-labelled
  founder experience in the pitch.
- **Tool calls are metered separately from AI credits.** Nine agent URLs behind
  one chat turn is nine tool calls.
- **Ten tools per agent on Starter.** We have nine Go agents. Add the Twilio
  send and a Postgres read and we are at the cap.
- **Export requires publish.** Nothing lands in `dronahq/` until the app is
  published. Build that into the end-of-day routine, not the last hour.
- **The API key shows once.** Losing it costs a regeneration and a re-paste
  everywhere.

## 9. Unverified

Confirm these before anyone writes code against them. Everything here failed to
resolve from official docs, it is not a summary of what we expect to be true.

- **The API base host.** The only documented example is
  `https://<your-dronahq-host>/voice/outbound/dispatch`. Whether cloud accounts
  get a fixed host (`api.dronahq.com`? a workspace subdomain?) is not stated on
  any page found. **Do not send the `sk_` key anywhere until this is read off
  the account's own Developer → API Keys screen.**
- **The endpoint that invokes a chat Agent from outside.** Only the Voice Agent
  dispatch path appears in an example. Whether a text agent has an equivalent
  synchronous invoke endpoint, and its path, is unconfirmed. The **Webhook
  trigger** is the verified way in and should be treated as the only one.
- **Agent export to a file.** Not found anywhere in the agents docs.
  `dronahq/CLAUDE.md` lists `whatsapp-agent.json` as an exported Agent. If no
  export exists, that file cannot be produced and the agent is reproducible only
  from a written runbook. **Whoever owns the DronaHQ track resolves this in the
  console on day one** and, if it is absent, rewrites `dronahq/CLAUDE.md` to
  replace `whatsapp-agent.json` with a `whatsapp-agent.md` runbook covering
  instructions text, tool list, trigger config and knowledge base sources.
- **Proactive outbound WhatsApp from an agent.** No agents-docs page describes
  an agent initiating a WhatsApp message. Twilio is the verified path and it is
  an app-side connector. Whether an agent can call the Twilio connector as a
  tool, rather than an app actionflow doing it, is unconfirmed.
- **Meta's 24-hour window and template approval**, as applied by DronaHQ's
  trigger. DronaHQ's page does not mention message windows, templates or rate
  limits at all. These are Meta rules and they will apply regardless.
- **Whether Table Grid auto-detects columns** from an array of objects, and how
  it renders a nested object inside a row. The control page does not say.
- **What an exported app JSON omits.** The export page does not state whether
  connector credentials travel with the file. Assume they do not, and assume
  they might, which means **the exported JSON gets read before it is committed**
  in case an Account SID or Auth Token is sitting in it.
- **Native human-in-the-loop approval UI.** Unconfirmed, as
  `dronahq/CLAUDE.md` already notes. Table Grid + Button + Actionflow + Toast
  are each individually verified and are the safe build.
- **Row limits on a Table Grid** and API response size caps. Not found.
- **Rate limits** on the agents API or the webhook trigger. Not found on any
  page.

---

## Confidence

**Solid:** the `api-key` header and the `sk_` prefix belonging to the Agentic
platform; the API key creation form and its scopes; WhatsApp trigger setup
steps, the Meta path and the payload field names; the full inbuilt trigger list;
webhook trigger auth, methods and synchronous Standard response; REST connector
and Call REST API field names, content types and `{{ }}` interpolation; the
`{{query.data}}` binding model, auto-run versus manual trigger, Table Grid
taking an array of objects; app export and import paths and the publish
requirement; Git Sync being self-hosted only; Twilio action names and the
`whatsapp:` prefix; **the WhatsApp actionflow block being a client-side deep
link and not a sender**; both pricing tables.

**Shaky, resolve in the console on day one:** the API base host; any external
invoke endpoint for a text agent; whether agents export at all; whether an agent
can call Twilio as a tool; nesting depth Table Grid tolerates before DQL is
needed; what a published export file contains.

Open these: https://docs.dronahq.com/agents/triggers/inbuilt-triggers/whatsapp/,
https://docs.dronahq.com/agents/developer/api-keys/,
https://docs.dronahq.com/agents/triggers/inbuilt-triggers/webhook/,
https://docs.dronahq.com/reference/connectors/twillio/,
https://docs.dronahq.com/reference/actionflow-blocks/whatsapp/,
https://docs.dronahq.com/binding-data/data-binding-framework/,
https://docs.dronahq.com/building-apps-concepts/app-export/,
https://www.dronahq.com/agents/pricing/
