# Chat agent: configuration runbook

Everything the DronaHQ agent builder needs, other than the Instruction itself,
which is [`system-prompt.md`](system-prompt.md).

## Why a runbook and not `chat-agent.json`

**There is no agent export and no agent import in DronaHQ.** That is not an
assumption; a search across all 49 `/agents/*` pages in DronaHQ's own sitemap
for "export agent", "import agent" and "export json" returns nothing. The App
Export feature documented under `building-apps-concepts` is the *app* platform,
and the sidebar of dependencies it lists is Connectors, Sheets, PDFs, Custom
Controls and Widgets. Agents are not on that list.

There is also no API to create an agent. DronaHQ's developer surface for agents
is scoped in its own words to "a way to authenticate API traffic to your agents,
and a way to see what that traffic did", meaning API keys and request logs.
The platform API at `plugin.api.dronahq.com` exposes Users, Groups and
Notification, and nothing else.

Agents are created in the UI, by a human, from this runbook.

This settles the open question `docs/research/dronahq.md` §9 raised and that
`dronahq/CLAUDE.md` flagged: the answer is no, agents do not export, so there is
no `chat-agent.json` and there will not be one. `dronahq/CLAUDE.md` has been
updated to match.

## Build order

DronaHQ's documented agent flow is five steps: create, add tools, configure
instruction, playground, publish. Follow it in that order. The Instruction
references the tools by name, so adding tools first means the names are already
real when you paste it.

### Step 1: create the agent

| Field | Value |
|---|---|
| Agent Name | `BrandPulse` |
| Agent Description | Social listening assistant for a D2C brand. Answers from BrandPulse's collected data and never invents a number. |
| Model | Any of OpenAI, Gemini or Anthropic. See the note on cost below. |

### Step 2: add tools

Tools attach from **+ Add Tools** in the agent builder. The documented tool
types are DronaHQ Tools, Connector Library, Connector Query, Automations, Code,
MCP and Inbuilt Tools.

Ours are **Connector Query** tools over the `brandpulse-bff` connector, built
first from [`../connectors/rest-connector.md`](../connectors/rest-connector.md).
Attach four, named exactly as the Instruction calls them:

| Tool name | Connector API | When the agent uses it |
|---|---|---|
| `getBrandPulse` | `getBrandPulse` | Almost every question. Called first. |
| `getAlerts` | `getAlerts` | Alert-only questions, and paging past the pulse's 20. |
| `getMentions` | `getMentions` | When more than the pulse's 50 mentions are needed. |
| `startRun` | `startRun` | Only after the owner agrees to spend credits. |

**Four, not six.** The connector also carries `listBrands` and `createBrand`,
which landed in the BFF after this runbook's first draft. Neither becomes an
agent tool:

- `createBrand` is an unauthenticated write that spends Anakin credits and LLM
  tokens on a website the caller names, returns an id nothing can delete, and
  has no update route behind it. A model that can call it can be talked into
  calling it. Brand creation is a form in `web/`, where a human fills the fields
  and presses a button.
- `listBrands` is harmless but pointless here. This agent serves one brand
  owner, `brandid` is already set for the conversation, and handing the model a
  roster of every other brand in the database is a way for one customer's name
  to turn up in another customer's chat.

Do not attach any **Inbuilt Tool**. The documented set is Web Search, Google
Search, Calculator, Time, Delay, File Parser and URL Parser. Web Search and
Google Search are the dangerous ones here: an agent whose single hardest rule is
"every number came from a tool call against BrandPulse" must not also have a
door to the open internet through which a number about the brand can arrive
unsourced. Calculator and Time are harmless but unnecessary.

**Per-tool field names are not documented.** DronaHQ's tools-overview page gives
the attach flow and the type list but no field table for a regular agent's tool;
the only explicit field tables in the agent docs are for MCP servers (name, URL,
description) and for Voice Agent Skills. So whatever the console asks for
beyond a name, fill it from the endpoint's entry in
[`../connectors/brandpulse-openapi.json`](../connectors/brandpulse-openapi.json),
which carries an `operationId`, a `summary`, a `description` and a typed schema
for every parameter and every response.

If the console asks for a tool description, these are the ones to use. A tool
description is read by the model on every turn, so it is short and it says when
to call, not what the endpoint is:

- `getBrandPulse`: "The brand's whole current picture in one call: profile, latest run, daily brief, alerts, topics, recent mentions and reply drafts. Call this first for any question about the brand's current state."
- `getAlerts`: "The alert feed alone, newest first. Use for alert-only questions or to look further back than the pulse's twenty."
- `getMentions`: "The enriched mention stream alone. Use when a question needs more than the fifty mentions the pulse carries."
- `startRun`: "Starts a new collection run. Spends the brand's Anakin credits and LLM tokens. Ask the user before calling."

### Step 3: configure the Instruction

Paste the body of [`system-prompt.md`](system-prompt.md), everything below its
`---` rule.

### Step 4: playground

Test on the **Chat** trigger before anything else. The test questions are the
five under `# EXAMPLES` in the Instruction, plus these three, which are the ones
that catch a dishonest agent:

1. Ask for a metric the API does not serve, for example share of voice broken
   down by source. It must say it does not have that. If it produces a
   breakdown, the Instruction is not holding.
2. Ask it to post a reply. It must refuse and say you publish from your own
   account.
3. Ask why an alert fired. The answer must contain a metric, a value, a
   threshold and a window. If it says an AI detected a pattern, it failed.

Run these against a seeded BFF so there is real data. `demo/run_demo.sh` seeds
brand `brd_demo`.

### Step 5: publish

## Triggers

**Chat** for the MVP. It needs no external account and it is what the demo uses.

Two documented gotchas that will cost an afternoon each:

- **A trigger cannot be edited once configured and saved.** Get it right the
  first time or create a new one.
- **Duplicate triggers with identical account and config are rejected**, even
  across different agents.

The full inbuilt list is Chat, Webhook, Scheduler, WhatsApp, Instagram and
Facebook Messenger, plus 145-plus platform triggers. Email is marked coming soon
and is not selectable.

**WhatsApp is out of the MVP** and needs no decision here. If it is ever
switched on, `docs/research/dronahq.md` §1 is the verified route, and the short
version is that the WhatsApp actionflow block is a client-side deep link that
cannot deliver anything: outbound goes through a REST connector to Meta's Cloud
API.

**Webhook**, later, is how `bp-detector` would push an alert into the agent. It
generates its own URL and its own key, and that key is **not** the `sk_`
agentic-platform key; do not conflate the two. Nothing in the repo posts to it
today.

**Scheduler**, later, for a 9am brief. Its cron is five fields and you cannot
set both day-of-month and day-of-week, one must be `?`. The docs do not say
where a scheduled agent's output goes, so a scheduled brief needs an explicit
delivery tool in the agent, not a schedule alone.

## Cost, and why the playground is not free

The agentic platform bills AI credits and tool calls on **separate meters**, and
the free tier is a $5 credit. Two consequences for a hackathon:

- Every chat turn that calls four tools is four tool calls. The Instruction
  tells the agent to call `getBrandPulse` first and use it for most questions
  precisely so that a typical turn is one call, not four.
- Iterating on a knowledge base burns credits on embeddings. **This agent has no
  knowledge base and does not need one.** Everything it knows about the brand
  arrives through a tool call at request time, which is the whole design: a
  knowledge base is a snapshot, and a snapshot of a brand's mentions is stale
  the moment the next run finishes.

Starter forces a "Powered by DronaHQ" badge and caps an agent at ten tools. We
use four, so the cap is not near.

## Auth, for when something calls the agent from outside

Not needed for the Chat trigger. Recorded because it is easy to get wrong.

The header is **`api-key`**, never `Authorization: Bearer`. DronaHQ's own docs
say so in those words. The agentic key is `sk_` prefixed, is shown once at
creation, and there is no rotate operation.

**The API base host is not documented.** Every example in the docs uses the
placeholder `<your-dronahq-host>`. Do not assume `api.dronahq.com`. Read the
real host off the account's Developer → API Keys screen before sending the key
anywhere.

## Variables, if you use them

Agent-side variables are referenced as **`{{variable.<name>}}`**, which is
*different* from the app-side `{{name}}` and `{{query.data}}`. Three
near-identical syntaxes across the platform is the likeliest source of a binding
that silently resolves to nothing.

The agent needs `brandid` to call the connector. Whether that comes from a
variable, from the connector's own default, or from the conversation is a console
decision I cannot make from here; the demo brand is `brd_demo`.
