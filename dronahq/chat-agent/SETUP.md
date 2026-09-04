# Build the BrandPulse chat agent in DronaHQ

Ten minutes. At the end you have an agent in the Playground answering questions
about a real brand, from live data, on camera.

**Do [`../connectors/SETUP.md`](../connectors/SETUP.md) first.** This agent has
no knowledge of its own. Every fact it states arrives through a call to the
`brandpulse-bff` connector at the moment you ask.

Every label below is quoted from DronaHQ's agent documentation, checked on
2026-09-20.

---

## Step 1: create the agent

**1.** Go to **DronaHQ → Agents**.

**2.** Click **+ Add Agent**.

**3.** Fill the basic properties:

| Field | Value |
|---|---|
| **Agent Name** | `BrandPulse` |
| **Agent Description** | `Social listening assistant for a D2C brand. Answers from BrandPulse's collected data and never invents a number.` |
| **Model** | Any of OpenAI, Gemini or Anthropic. Pick whichever account is already connected. |

---

## Step 2: add tools

**4.** Click **+ Add Tools**. From the tool type list, choose **Connector
Query**, which is the type for queries against a connector already configured in
your account. Select the `brandpulse-bff` connector.

**5.** Attach exactly these four, named as the Instruction calls them:

| Tool | Connector endpoint | Description to paste, if the console asks for one |
|---|---|---|
| `getBrandPulse` | `getBrandPulse` | The brand's whole current picture in one call: profile, latest run, daily brief, alerts, topics, recent mentions and reply drafts. Call this first for any question about the brand's current state. |
| `getAlerts` | `getAlerts` | The alert feed alone, newest first. Use for alert-only questions or to look further back than the pulse's twenty. |
| `getMentions` | `getMentions` | The enriched mention stream alone. Use when a question needs more than the fifty mentions the pulse carries. |
| `startRun` | `startRun` | Starts a new collection run. Spends the brand's Anakin credits and LLM tokens. Ask the user before calling. |

A tool description is read by the model on every turn, so each one says *when to
call*, not what the endpoint is.

**Four, not six.** The connector also carries `listBrands` and `createBrand`.
Neither becomes a tool here:

- `createBrand` is an unauthenticated write that spends credits on a website the
  caller names, returns an id nothing can delete, and has no update route behind
  it. A model that can call it can be talked into calling it.
- `listBrands` would hand the model a roster of every other brand in the
  database, which is how one customer's name turns up in another customer's chat.

**Do not attach any Inbuilt Tool.** Web Search and Google Search are the
dangerous ones: an agent whose hardest rule is "every number came from a tool
call against BrandPulse" must not also have a door to the open internet through
which an unsourced number about the brand can arrive.

**6.** If a tool shows as unconfigured, use **Edit → Add Account** under the
**Environment** section.

**7.** Set `brandid` to `brd_mamaearth_ae6f` wherever the tool exposes the path
parameter, so the demo needs no typing. Switch it to `brd_demo` for the crisis
questions further down. Agent-side variables are referenced as
`{{variable.<name>}}`, which is different from the app-side `{{name}}`. DronaHQ
variable names must not contain an underscore, so `brandid` and never `brand_id`.

---

## Step 3: configure the Instruction

**8.** Open **Instruction**. Paste the body of
[`system-prompt.md`](system-prompt.md), everything below its `---` rule.

The Instruction references the four tools by name, which is why they are added
first: the names are already real when the prompt lands.

Its `RULES & GUARDRAILS` section is half the safety story. The other half is
server side: `bp-responder` substring-matches every draft against 30 literal
phrases, and `requires_human_approval` is `true` on every draft this API ever
serialises. Do not soften either half.

---

## Step 4: test in the Playground

**9.** Go to **Playground**. It shows the tool calling sequence, the final
output and the model reasoning, so you can see which endpoint answered.

**10.** Run the three questions below before you start filming.

---

## The questions to ask on camera

Every number below was curled from the live API at
`https://unknowing-tubby-thrive.ngrok-free.dev`. They are picked because their
answers exist right now.

**There are two brands and they tell two different stories. Use both.**

`brd_mamaearth_ae6f` (**Mamaearth**) is a real Indian D2C skincare company,
onboarded live through Anakin's crawler and filled by a real Anakin collection
run. It is the acquisition story and it is a healthy brand.

`brd_demo` (**Suncoast**) is the recorded corpus with a crisis injected into it.
It is the alerting story. A healthy brand cannot demonstrate crisis detection,
which is exactly why both exist.

---

## Act one, on Mamaearth: "we just added this brand"

Set `brandid` to `brd_mamaearth_ae6f`.

### Question A: "What is happening with Mamaearth right now?"

Calls `getBrandPulse`. Real data behind the answer:

- **162 mentions**, across web 80, news 61, reddit 21, collected in one run for
  **93 Anakin credits**.
- Of those, the enricher judged **55 to be genuinely about the brand**. The rest
  are keyword collisions it threw out.
- In the 50 most recent: 31 neutral, 18 positive, 1 negative. Intents: 26 other,
  18 praise, 3 news, 2 question, 1 complaint.
- `brief.numbers`: mentions 162, negative share 4.32, average sentiment 0.0898,
  share of voice 94.83.
- The brief headline is *"Negative sentiment spikes amid hair loss concerns on
  Reddit."*

Say the 162 against 55 out loud. A classifier discarding two thirds of its own
input is the feature, not a defect: keyword search returns collisions, and an
incumbent that counted all 162 would report a volume number that means nothing.

### Question B: "What topics are people discussing?"

Calls `getBrandPulse`. Four topics, every label written by an LLM off the
clustered text, none of them typed by us:

| Topic | Size | What is in it |
|---|---|---|
| `ubtan skincare` | 17 | The ubtan face wash range, turmeric and saffron, tan removal |
| `sunscreen ingredients` | 6 | Vitamin C, rice water, aloe vera |
| `revenue growth` | 5 | Honasa Consumer's results, Mamaearth's parent company |
| `hair loss concerns` | 4 | Thinning, shedding, hairline recession |

`revenue growth` is the one to point at. Nothing told the clusterer that Honasa
Consumer is Mamaearth's parent company. It read that off the collected text.

### Question C: "Why did an alert fire? Show me the numbers."

**20 alerts**, of which 2 are high severity, both volume spikes. The larger one:

| metric | value | threshold | window |
|---|---|---|---|
| `volume_zscore` | 18.3 | 3 | 1h |

Its `why` field reads, verbatim: *"2 mentions on web in the hour from
2026-09-09T00, a z-score of 18.3 against a 14-day mean of 0.0 per hour (fires at
3.0)."*

That sentence is the whole point of the product and it is worth reading off the
screen. There is no LLM in `bp-detector`. If the agent says "our AI detected
unusual activity" instead of quoting the z-score, the Instruction is not holding.

---

## Act two, on Suncoast: "and here is what a crisis looks like"

Switch `brandid` to `brd_demo`.

### Question 1: "What is happening with Suncoast right now?"

Calls `getBrandPulse`. Real data behind the answer:

- **97 mentions**, across reddit 32, web 25, x 14, instagram 13, news 13.
- Sentiment labels: 52 neutral, 42 negative, 3 positive.
- Intents: 42 complaint, 40 other, 7 news, 6 question, 1 praise, 1 purchase
  intent.
- The brief headline is *"Customer dissatisfaction spikes over refund delays and
  product issues."*
- `brief.numbers`: mentions 97, negative share 43.3, average sentiment -0.33,
  share of voice 100.

Watch for: it must quote `brief.numbers.share_of_voice`, not
`pulse.share_of_voice`, which is always `null`. If it reports a share of voice
of zero or refuses to give one, the binding is wrong.

### Question 2: "Why did the critical alert fire? Show me the numbers."

Calls `getBrandPulse` or `getAlerts`. There are **29 alerts**, of which **3 are
critical crisis alerts**. The one to expect is titled **"Negative surge on
reddit"**, and its `evidence` array carries three rows:

| metric | value | threshold | window |
|---|---|---|---|
| `volume_zscore` | 66.9 | 3 | 1h |
| `negative_share` | 1 | 0.6 | 1h |
| `mention_count` | 13 | 0.0327 | 1h |

This is the question that catches a dishonest agent. **There is no LLM in
`bp-detector`**, so the evidence rows are the entire reason the alert fired. A
good answer quotes a metric, a value, a threshold and a window. If it says "our
AI detected unusual activity", the Instruction is not holding and that sentence
is a false statement about this system, not a friendlier phrasing of a true one.

### Question 3: "What are people complaining about, and do we have a reply drafted?"

Calls `getBrandPulse`. Real data:

- **4 topics**, each of size 4 and each `100% negative`: **refund delays**,
  **product quality** (skin rashes and irritation from a new batch), and two
  clustered under **packaging damage** (broken seals and leaking bottles, and a
  pump that broke after two days).
- **9 reply drafts**, all with `channel: "statement"`, `status: "draft"` and
  `requires_human_approval: true`.
- One draft reads, verbatim: *"Hi! We're really sorry to hear that your sunscreen
  pump broke after just two days. We want to make this right for you. Please
  reach out to our support team with your order id, and they'll assist you
  further. Thanks for bringing this to our attention! - Team Suncoast"*

Watch for: the agent must say the draft **needs a human to send it**. There is no
send route, no approve route and no posting code path anywhere in this product.
If it offers to post the reply, or claims it did, stop and fix the Instruction
before filming.

### One more, if you want to show the guardrail holding

Ask for share of voice broken down by source. The API does not serve that:
`pulse.share_of_voice` is always `null` and the only figure available is the
single number at `brief.numbers.share_of_voice`. A good agent says it does not
have that breakdown. If it produces one, it made it up.

---

## Step 5: triggers and publish

**11.** Open your agent, click **Add Trigger**, select **Chat**, click **Add
Triggers**. Chat is the trigger for manual testing and internal conversations,
it needs no external account, and it is what the demo uses.

Two things that cost an afternoon each:

- **A trigger cannot be edited once configured and saved.** Get it right the
  first time or make a new one.
- **Duplicate triggers with identical account and config are rejected**, even
  across different agents.

WhatsApp, Instagram and Facebook Messenger are available and are all out of
scope. **WhatsApp is out of the MVP.**

**12.** **Publish** the agent when the three questions above answer correctly.

---

## Cost, and why the Playground is not free

AI credits and tool calls bill on separate meters, and the free tier is a $5
credit. Every chat turn that calls four tools is four tool calls, which is why
the Instruction tells the agent to call `getBrandPulse` first and answer most
questions from it. A typical turn should be one call, not four.

**This agent has no knowledge base and does not need one.** Everything it knows
about the brand arrives through a tool call at request time. A knowledge base is
a snapshot, and a snapshot of a brand's mentions is stale the moment the next run
finishes.

---

## If the answers come back empty

Curl the API yourself:

```bash
curl -s -H 'ngrok-skip-browser-warning: 1' \
  'https://unknowing-tubby-thrive.ngrok-free.dev/api/brands/brd_demo/pulse?limit=200'
```

- HTML back instead of JSON: the `ngrok-skip-browser-warning: 1` header is
  missing from the connector endpoint. See
  [`../connectors/SETUP.md`](../connectors/SETUP.md) Part 2.
- Connection refused or a 404 from ngrok: the tunnel restarted and the host
  changed. The URL is on every endpoint in the connector, and in
  `servers[0].url` of `../connectors/brandpulse-openapi.json`.
- JSON back with empty arrays: a collection run is in flight and the tables are
  mid-refill. Wait a minute and curl again. This really does happen; the demo
  brand went from 0 to 97 mentions in about thirty seconds while this file was
  being written.

**Two brands have mentions: `brd_mamaearth_ae6f` (162) and `brd_demo` (97).**
`brd_lumeo_dbe9` has a profile, a failed run and zero mentions, and its brief
headline reads "Brand activity was non-existent over the past weeks." It is
useful for exactly one thing, showing that the empty states are written, and it
is the wrong brand to ask a question about on camera.
