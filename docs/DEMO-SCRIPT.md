# Demo script

Two minutes. Six beats. One thing to say at each, and one thing to say if it
breaks.

**The timings below are budgets, not measurements.** They are what each beat is
allowed to take for the whole thing to fit in 120 seconds. B7 replaces them
with real numbers in Phase 5, after three consecutive clean runs against the
real deployment. Until that happens, treat this as the rehearsal plan.

---

## Before you walk up

| Check | Command | Must show |
|---|---|---|
| Database up | `docker compose ps` | `(healthy)` |
| Demo brand seeded | `./demo/run_demo.sh --reset` | exits 0 |
| Nine agents live | `nasiko ps --json \| jq -r '.[] \| "\(.name) \(.status)"'` | nine `running` |
| Traces flowing | `nasiko observe --agent bp-sov --last 5m` | a span, not an empty result |
| Dashboard loaded | browser tab on the DronaHQ app | rendered, not a spinner |

Have the fallback video open in a second tab, muted, paused at 0:00. Do not
mention it unless you need it.

Run `./demo/run_demo.sh` once more, fully, in the five minutes before you go
on. It is re-runnable by design and it will have been run thirty times before
this. The failure you want to find is the one you find in the green room.

---

## The two minutes

### Beat 1, 0:00 to 0:20: the problem, in one person's words

> "This is a skincare brand doing about ₹40 lakh a month, mostly Amazon and
> their own site. Two people. No social team. Last month a batch shipped with
> leaking caps, and they found out four days later, from a customer who called
> to complain that nobody had replied to her Reddit thread."

Do not open with the architecture. Open with the four days.

> "The cheapest thing that would have caught it is about ₹77,000 a month, on an
> annual contract. Sprinklr killed the only self-serve tier in the market in
> April. So they use nothing, and they find out late, every time."

**If you are running behind:** cut this to the first and last sentence.

---

### Beat 2, 0:20 to 0:35: what it is

> "Nine agents. They read seven public sources through Anakin, classify what
> they find, group it into topics, and fire alerts on five statistical rules.
> Four of the nine never call a model at all."

Show the agent list on screen, not in words.

> "The one that decides whether something is a crisis is one of the four. It
> ships the numbers that fired it. You can check its arithmetic."

**If somebody asks why that matters:** because a brand owner woken at 11pm
needs to know whether to act, and "the model thought so" is not something you
can act on.

---

### Beat 3, 0:35 to 1:05: the run

Trigger it live. Talk while it runs.

> "This is one run for one brand. It picks the sources by yesterday's yield in
> mentions per credit, fans out to a collector per source, enriches, then
> clusters, counts share of voice and runs the detector in parallel."

Point at the Nasiko trace view as spans land.

> "Every one of those is a separate container answering an A2A call, and every
> call is going through Nasiko's proxy. The fan-out is capped at eight and the
> guard fails closed, so we cannot accidentally stampede."

**Budget: 30 seconds.** If the run has not finished by 1:05, stop narrating it
and move to beat 4 anyway; the crisis is already in the database from the
injection and the dashboard will show it.

**If the run errors:** say so, plainly, and keep going.

> "That is a source failing, which is the normal case, not the exceptional one.
> The run degrades instead of dying: the brief comes out with the hole labelled
> rather than not coming out."

Then show the degraded run record. A visible degradation is a better slide than
a clean run, and it is true.

---

### Beat 4, 1:05 to 1:30: the crisis

> "Forty negative mentions landed in an hour. Volume z-score 4.2, negative
> share 71%. Both thresholds crossed in the same hour, so that is the crisis
> rule, not the spike rule."

Read the evidence off the alert. The numbers and the thresholds are both on
screen; that is the point of the beat.

> "This is injected. It is labelled as injected, here", point at the badge,
> "because we are not going to blur the line between what we scraped and what
> we made up."

Say the injected line. A judge who spots it themselves has caught you; a judge
who hears it from you has learned that the rest of the numbers are real.

**If the alert is not there:** check the hour bucket. Alerts dedupe on
`<kind>:<source>:<hour_bucket>`, so a run that already fired this hour will not
fire again. `--reset` clears it.

---

### Beat 5, 1:30 to 1:50: the reply, and the thing we did not build

> "It drafts a reply. Never promise a refund, never admit fault, never commit
> to a date, always escalate a crisis to a human. Those guardrails are in the
> agent's instructions and enforced again server-side, because a guardrail that
> only lives in a prompt is a suggestion."

Then the sentence that lands:

> "And it cannot post it. There is no posting code in this repository. Approve
> just hands it to the human who then posts it themselves if they want to. That
> is deliberate."

**If asked whether that is a limitation:** no, it is the product decision. An
autonomous system that can speak in a brand's voice in public is a liability,
and the people buying this are two-person teams who cannot absorb one.

---

### Beat 6, 1:50 to 2:00: the number

Put the real figure from `go run ./eval/cost` on screen against reported
Brandwatch, Sprinklr and Meltwater contract pricing. One slide, three bars, no
animation.

> "Cents a day per brand, against a floor of ₹48,000 a month from the cheapest
> incumbent that will quote at all. That is the whole pitch."

Say "reported contract pricing", not "list price". None of the three publish
one, and a judge who knows that will notice which phrase you used.

**If the number misses the ₹15 target:** show it anyway and say what closes it.
See [PRICING.md](PRICING.md). A real number that missed beats a made-up number
that hit, and the room can tell the difference.

---

## When something breaks

Rank the failures by what you do, not by what went wrong.

| What broke | What you say | What you do |
|---|---|---|
| A source returns nothing | "That is degradation working, not the demo failing." | Keep going, show the labelled hole in the brief |
| A run is slow | Narrate the trace view while it lands | Move to beat 4 on the budget, do not wait |
| An agent is down | "Eight of nine. The orchestrator records what it could not reach." | Show the degraded run record |
| Stage wifi dies | Nothing about wifi | The demo runs on replayed fixtures; only the optional live Anakin call needs the network |
| The dashboard will not load | "Let me show you the same thing from the chat agent." | Switch to DronaHQ chat |
| Two or more of the above | "I am going to show you the recorded run rather than spend your time on this." | Fallback video, second tab, already open |

The rule underneath all of these: **never explain a failure for more than one
sentence.** One sentence is credibility, three is an apology, and the clock is
running either way.

And the rule that outranks it: **never fix the demo by special-casing the
code.** If the injection does not fire the real detector rule, the injection is
wrong and we fix the injection. Nothing in this repository gets a branch that
exists to make a stage work.

---

## The live call, and why it is optional

One live Anakin call is reserved for the stage, out of the 300 free credits.
It is a separate, optional step behind `./demo/run_demo.sh --live`.

Use it if the network is solid and you have the time. Skip it without comment
if either is in doubt. The demo is complete without it, because everything else
runs on the recorded corpus and replays deterministically: same fixtures in,
same timestamps out, every run. That is what makes this rehearsable at all.
