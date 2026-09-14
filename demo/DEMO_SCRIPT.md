# BrandPulse demo script

Everything you need to shoot the 90 second video, plus the longer walkthrough if
a judge asks for one. Every number in this file was read off the live system on
2026-09-20, not estimated.

**Read [`PREFLIGHT`](#preflight) first and run it. Then shoot.**

---

## The map: what is running where

| Thing | Address | What it is |
|---|---|---|
| Product dashboard | `http://localhost:3000` | Next.js, the screen a brand owner sees |
| BFF | `http://127.0.0.1:8080` | Fastify, the dashboard's only backend |
| Public tunnel | `https://unknowing-tubby-thrive.ngrok-free.dev` | The same BFF, reachable by DronaHQ |
| bp-orchestrator | `http://127.0.0.1:8000` | The pipeline agent, A2A |
| bp-onboarder | `http://127.0.0.1:8001` | The brand crawler, A2A |
| The other seven agents | container network only | Reachable by `NASIKO_API_URL`, not by you |
| Postgres | `127.0.0.1:5433` | One database, shared |

Nine agent containers, all up:

```
brandpulse-bp-orchestrator-1   127.0.0.1:8000->8000/tcp
brandpulse-bp-onboarder-1      127.0.0.1:8001->8000/tcp
brandpulse-bp-collector-1      8000/tcp
brandpulse-bp-enricher-1       8000/tcp
brandpulse-bp-clusterer-1      8000/tcp
brandpulse-bp-detector-1       8000/tcp
brandpulse-bp-responder-1      8000/tcp
brandpulse-bp-briefer-1        8000/tcp
brandpulse-bp-sov-1            8000/tcp
```

---

## The three brands, and why there are three

| Brand id | Name | Mentions | What it proves |
|---|---|---|---|
| `brd_mamaearth_ae6f` | Mamaearth | 162 | **Acquisition.** A real Indian D2C company, crawled live by Anakin and filled by a real Anakin collection run costing 93 credits. |
| `brd_demo` | Suncoast | 97 | **Alerting.** The recorded corpus with a crisis injected. A healthy brand cannot demonstrate crisis detection. |
| `brd_lumeo_dbe9` | Lumeo | 0 | **Empty states.** Proof the product does not fake data when it has none. |

Use Mamaearth for act one and Suncoast for act two. Say out loud that they are
two different brands doing two different jobs. A judge who thinks you switched
brands to hide something is worse than one who knows why you did.

---

## Mamaearth, the numbers to have in your head

Onboarding, from a live Anakin crawl of `mamaearth.in` in 28 seconds:

- **18 keywords**, including the misspellings real people type: `mamaearh`,
  `mamaarth`, `mamaertah`. Nobody typed those in. Anakin's crawl plus one LLM
  call produced them.
- **6 products**: rice dewy bright face wash, ubtan natural glow face wash,
  vitamin c daily glow sunscreen, rosemary anti-hair fall shampoo, onion
  shampoo, moisture matte longstay mini lipstick.
- **3 competitors**: Minimalist, Plum, WOW Skin Science.
- **4 negative keywords**: `earth`, `mama`, `baby`, `minimalist`. These are the
  words that mean a match is *not* this brand.

Collection, one run:

- **162 mentions**: web 80, news 61, reddit 21.
- **93 Anakin credits**, metered and stored on the run record.
- **55 of 162 judged genuinely about the brand.** Say this number. A classifier
  throwing out two thirds of its own input is the feature. Keyword search
  returns collisions, and an incumbent that counted all 162 reports a volume
  number that means nothing.
- `brief.numbers`: mentions 162, negative share 4.32, average sentiment 0.0898,
  share of voice 94.83.
- Brief headline, written by an LLM: *"Negative sentiment spikes amid hair loss
  concerns on Reddit."*

Four topics, every label written by an LLM off the clustered text:

| Topic | Size | What is in it |
|---|---|---|
| `ubtan skincare` | 17 | The ubtan face wash range, turmeric and saffron, tan removal |
| `sunscreen ingredients` | 6 | Vitamin C, rice water, aloe vera |
| `revenue growth` | 5 | Honasa Consumer's results |
| `hair loss concerns` | 4 | Thinning, shedding, hairline recession |

**Point at `revenue growth`.** Nothing told the clusterer that Honasa Consumer
is Mamaearth's parent company. It read that off the collected text. That is the
single most convincing thing on the screen and it takes four seconds to say.

20 alerts, 2 of them high severity. The larger one's `why` field reads verbatim:

> 2 mentions on web in the hour from 2026-09-09T00, a z-score of 18.3 against a
> 14-day mean of 0.0 per hour (fires at 3.0).

4 reply drafts, every one with `requires_human_approval: true`.

---

## Suncoast, the crisis numbers

97 mentions and 29 alerts in the database: 15 medium, 9 high, 3 critical, 2 low.

**The dashboard shows 20 of them, not 29.** `ALERT_LIMIT` in
`bff/src/routes/pulse.ts` caps the pulse response at 20, so one critical alert
is on screen even though three exist. If a judge counts, that is why. It is a
display limit, not a discrepancy.

The one to open is **"Negative surge on reddit"**, and its evidence array is the
whole reason it fired:

| metric | value | threshold | window |
|---|---|---|---|
| `volume_zscore` | 66.9 | 3 | 1h |
| `negative_share` | 1 | 0.6 | 1h |
| `mention_count` | 13 | 0.0327 | 1h |

**There is no LLM in `bp-detector`.** Those three rows are the entire reason the
alert exists. Read one off the screen. "Our AI detected unusual activity" is a
false statement about this system, not a friendlier phrasing of a true one.

9 reply drafts on screen. The one to open is the pump complaint, verbatim:

> We understand your concern about receiving a broken pump for the second time,
> and we're here to help. Please reach out to our support team and provide your
> order ID so we can assist you further. Thank you for bringing this to our
> attention!

Note what it does not do. It does not apologise in a way that admits fault, it
does not promise a refund, and it does not commit to a date. Those are three of
the 30 literal phrases `bp-responder` substring-matches every draft against, in
`internal/prompts/guardrails.md`.

Every draft carries `requires_human_approval: true` and there is no send route,
no approve route and no posting code path anywhere in this product.

---

## The 90 second script

Nine shots. Times are cumulative.

### 0:00 to 0:10, the problem

Dashboard open on Mamaearth at `http://localhost:3000/?brand=brd_mamaearth_ae6f`.

> "A mid-size Indian D2C brand pays about 77,000 rupees a month for social
> listening. This is the same job for 2,999."

### 0:10 to 0:25, add a brand live

**Flip `bp-onboarder` to `record` before this shot**, or the crawl is a replay
and a brand nobody recorded gets a thin profile. See
[Choosing the mode](#choosing-the-mode-for-the-add-brand-shot) below.

Click **Add brand**. Type a real Indian D2C brand and its URL. Submit. It takes
about 30 seconds, so either hold the shot or cut.

> "I give it a name and a URL. Anakin crawls the site, and one LLM call turns
> that crawl into a keyword set, a product list and a competitor list."

### 0:25 to 0:40, what the crawl produced

Switch the brand picker to **Mamaearth**, which was added exactly this way
earlier. Say that out loud. Show its profile and point at `mamaearh` and
`mamaertah`.

> "Here is one I added an hour ago. Eighteen keywords, including the
> misspellings people actually type. Nobody wrote those down, Anakin's crawl
> did. And 162 mentions collected for 93 credits, with the cost on the run
> record rather than in a footnote."

### 0:40 to 0:55, the classifier being honest

Scroll to the mention stream and the KPI strip.

> "162 collected, 55 judged actually about the brand. The rest are keyword
> collisions and it throws them out. A tool that counted all 162 would be
> selling you a number that means nothing."

### 0:55 to 1:05, topics

Scroll to the topic cards.

> "Four topics, every label written by the model off the text. That one is
> Honasa Consumer, Mamaearth's parent company. Nothing told it that."

### 1:05 to 1:20, the crisis, on Suncoast

Switch the brand picker to Suncoast. Open the critical alert.

> "Different brand, because a healthy one cannot show you this. Negative surge
> on reddit. Z-score 66.9 against a threshold of 3. There is no LLM in the
> detector. Those numbers are the entire reason it fired."

### 1:20 to 1:28, the draft and the guardrail

Open the reply draft.

> "It drafts the reply and it will not send it. Every draft is flagged for human
> approval, and there is no posting code path in this product at all."

### 1:28 to 1:30, close

Cut to the DronaHQ app or the chat agent.

> "Nine Go agents on Nasiko, Anakin for every byte of data, DronaHQ for the
> owner's chat and the analyst dashboard."

---

## Where each platform is visibly load-bearing

This is the table to have open if a judge asks. Every row is something you can
put on screen.

### Anakin

| Claim | How to show it |
|---|---|
| Anakin is the entire data acquisition layer | `internal/anakin/` is the only outbound HTTP client in the repo. There is no second scraper. |
| Anakin crawled the brand's site | The 18 Mamaearth keywords and 6 products, none typed by hand. Show `fixtures/web/` timestamps from the recording session. |
| Anakin collected the mentions | 162 mentions with real reddit, news and web URLs. Open one. It resolves. |
| Cost is metered per run | `credits_used: 93` on the run record, and `source_yield` holds credits per brand per source per day. |
| Cost control is real | That run came back `status: partial` with the error `spending 3 would take brand brd_mamaearth_ae6f to 39 of 37 credits today: credit budget exceeded`. The per-brand daily budget stopped it mid-run. That is a feature and it is worth showing. |

Credits: **163 of 300 spent, 137 remaining.**

### DronaHQ

| Claim | How to show it |
|---|---|
| Chat agent for brand owners | The agent in the Playground answering the questions in [`../dronahq/chat-agent/SETUP.md`](../dronahq/chat-agent/SETUP.md), with the tool-call trace visible. |
| Analyst dashboard | The app from [`../dronahq/dashboard/SETUP.md`](../dronahq/dashboard/SETUP.md): brand dropdown, four KPI tiles, mention table, run button. |
| It reads live data | Switch the dropdown from Mamaearth to Suncoast. Every tile changes. Nothing is hardcoded. |
| It reaches the real backend | The connector points at the ngrok tunnel, which is the same BFF the Next.js dashboard uses. |

**You have to build these two in the DronaHQ console yourself.** There is no
agent export, no agent import and no API that creates an app, so this cannot be
scripted. The three SETUP guides are click-by-click and the connector one takes
about ten minutes. Do `connectors/SETUP.md` first, everything else binds to it.

### Nasiko

| Claim | How to show it |
|---|---|
| Nine agents, A2A spec | `curl http://127.0.0.1:8000/.well-known/agent-card.json`. Nine containers, one card each, `protocolVersion: "1.0"`. |
| Cards validate | `python3 scripts/check_agent_cards.py` prints `ok` for all nine. |
| Agents address each other through Nasiko | `NASIKO_API_URL` is `http://{agent}:8000/` and `internal/a2a/call.go` substitutes `{agent}`. **No agent hardcodes a peer address.** Grep for it. |
| LLM traffic only through the injected router | `internal/llm` is the only LLM caller and it reads `OPENAI_BASE_URL`. No provider SDK config exists in any agent. |
| Four agents make no LLM call and say so | `llm_provider: null` in the cards for bp-collector, bp-sov, bp-detector and bp-orchestrator. |

**Be straight about one thing.** The agents are not running on a live Nasiko
cluster right now, because there is no Nasiko credential on this machine:
`nasiko status` says `Not logged in` and `nasiko list-clusters` says `No
clusters found`. They are running as the same containers Nasiko would deploy,
built to its AgentCard contract, addressed through `NASIKO_API_URL`. Say
"built for Nasiko and running the Nasiko contract", not "deployed on Nasiko",
unless you log in first. The CLI is installed at
`/private/tmp/claude-501/-Users-shubhamvs-Desktop-anakin-hack/b12a88c2-a1a6-4700-ae61-e46ecfb88901/scratchpad/nasiko-venv/bin/nasiko`
if you want to `nasiko login` before shooting.

---

## PREFLIGHT

Run this before you record. It should print all green.

```bash
cd ~/Desktop/anakin-hack/brandpulse

# 1. Nine agents and Postgres
docker ps --format '{{.Names}}\t{{.Status}}' | grep -c bp-      # expect 9
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:8000/healthz   # 200
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:8001/healthz   # 200

# 2. BFF and dashboard
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:8080/api/brands  # 200
curl -s -o /dev/null -w '%{http_code}\n' http://localhost:3000/            # 200

# 3. The tunnel DronaHQ uses
curl -s -H 'ngrok-skip-browser-warning: 1' \
  https://unknowing-tubby-thrive.ngrok-free.dev/health                     # {"ok":true}

# 4. The data is there
docker compose exec -T postgres psql -U brandpulse -d brandpulse -c \
  "select b.name, (select count(*) from mentions m where m.brand_id=b.id) from brands b;"
# expect Mamaearth 162, Suncoast 97, Lumeo 0

# 5. Nothing is in record mode, so nothing can spend credits by accident
docker inspect brandpulse-bp-collector-1 --format '{{range .Config.Env}}{{println .}}{{end}}' \
  | grep BP_FIXTURE_MODE     # expect replay
```

---

## Hazards, in the order they will bite you

**1. The tunnel host changes on every ngrok restart.** If you restart ngrok, the
DronaHQ connector breaks in eight places. Update `servers[0].url` in
`dronahq/connectors/brandpulse-openapi.json` and re-import. Do not restart ngrok
after you have built the DronaHQ app.

**2. A second "Run now" inside the same hour does nothing, and does not say so.**
`runs` carries `UNIQUE (brand_id, kind, time_bucket)` with an hourly bucket, so
the insert is `ON CONFLICT DO NOTHING` and you get the previous run back, byte
for byte, with HTTP 200. Rehearse at 14:50 and shoot at 15:05 and the take shows
the rehearsal. Either cross an hour boundary, or use a different `trigger`, or
delete the `runs` row between takes.

**3. Omitting `window_hours` means 24 hours.** The corpus was recorded at 504,
and the fixture cache key hashes the window, so a 24 hour run in replay matches
no fixture and collects nothing. Always send `"window_hours": 504`.

**4. Mentions dedupe on a content hash.** Re-running collection without deleting
the mentions collects zero new ones and the enrichment never re-runs. A real
reset is the restore below, not a partial delete.

**5. Both Anakin agents are in `replay`.** See the next section.

---

## Choosing the mode for the add-brand shot

Both `bp-onboarder` and `bp-collector` are in `BP_FIXTURE_MODE=replay` right
now, which means no network call and no credit spend. That is the safe default
and it is deliberately how I left the system.

| | `replay` | `record` |
|---|---|---|
| What happens | Reads `fixtures/`, no network call | Calls Anakin for real, then writes the response to `fixtures/` |
| Adding **Mamaearth** | Instant, free, full 18-keyword profile | A fresh live crawl, about 30 seconds, spends credits |
| Adding **any other brand** | **Thin profile.** No fixture exists, so the keywords are only what you typed | Real crawl, real keywords, about 30 seconds |
| Risk on camera | None | The crawl can be slow or fail, and it spends from the remaining 137 credits |

**My recommendation: flip `bp-onboarder` to `record`, leave `bp-collector` on
`replay`.** That gives you a genuinely live Anakin crawl for the add-brand shot,
which is the most convincing 15 seconds in the video, while the collection run
stays free. An onboard is capped at 30 credits by `maxAnakinCredits` in
`agents/bp-onboarder/onboarder.go`, so the worst case is 30 of your 137.

```bash
cd ~/Desktop/anakin-hack/brandpulse
COMPOSE_PARALLEL_LIMIT=1 BP_FIXTURE_MODE=record \
  docker compose -f docker-compose.yml -f docker-compose.agents.yml \
  up -d --no-build --force-recreate bp-onboarder

# confirm
docker inspect brandpulse-bp-onboarder-1 \
  --format '{{range .Config.Env}}{{println .}}{{end}}' | grep BP_FIXTURE_MODE
```

Put it back to `replay` the moment you have the shot, same command with
`BP_FIXTURE_MODE=replay`.

Brands worth adding live, all real Indian D2C with crawlable sites: `boAt`
(`boat-lifestyle.com`), `Sugar Cosmetics` (`sugarcosmetics.com`), `The Whole
Truth` (`thewholetruthfoods.com`). **Rehearse the one you pick once before the
take.** The rehearsal records the fixture, so if the live crawl fails during the
real take, the retry replays instantly off the rehearsal and still looks right.

---

## Reset between takes

A full snapshot of the database as it stands right now is at
`~/Desktop/anakin-hack/demo-snapshot.sql`. It holds all three brands with their
mentions, topics, alerts, briefs and drafts.

```bash
cd ~/Desktop/anakin-hack/brandpulse
docker compose exec -T postgres psql -U brandpulse -d brandpulse -q \
  < ~/Desktop/anakin-hack/demo-snapshot.sql
```

That is a full `--clean --if-exists` restore, so it drops and rebuilds every
table. **It wipes anything any other worktree wrote since the snapshot.** It is
the right tool between takes and the wrong tool if someone else is working.

To reset only the run bucket so "Run now" does real work again:

```bash
docker compose exec -T postgres psql -U brandpulse -d brandpulse -c \
  "delete from runs where brand_id='brd_mamaearth_ae6f' and kind='on_demand';"
```

---

## What not to claim

- Do not say the agents are deployed on Nasiko until `nasiko status` says you
  are logged in. Say they are built to the Nasiko contract and running it.
- Do not say anything was posted or sent. Nothing in this product posts.
- Do not describe an alert as "AI detected". The detector has no LLM in it.
- Do not read `share_of_voice` off the top level of the pulse response. It is
  hardcoded `null`. The real figure is `brief.numbers.share_of_voice`.
- Do not present the Suncoast corpus as organically collected. The crisis in it
  was injected deliberately to demonstrate detection, and the surrounding
  corpus is recorded. Mamaearth is the brand where the collection was real.
