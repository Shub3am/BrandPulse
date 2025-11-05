# dronahq

## What this module owns

The two front ends, exported from DronaHQ as JSON and committed here: the
WhatsApp agent brand owners talk to, and the analyst dashboard.

## What it must not know about

Agent internals. Both front ends call deployed agent URLs over REST and bind
directly to the typed JSON artifacts.

## Entry points

| File | What it is |
|---|---|
| `whatsapp-agent.json` | Exported DronaHQ Agent: onboarding, daily brief, alerts. |
| `dashboard-app.json` | Exported DronaHQ app: mentions, sentiment, SOV, topics, alerts, cost. |
| `README.md` | Connector setup, trigger config, what to click to rebuild it. |
| `screenshots/` | Proof for the PR and the pitch. |

## Invariants and gotchas

- **Build against deployed agent URLs**, never a local port. A dashboard wired
  to `localhost` is a dashboard that dies on stage.
- **Export is manual**: publish the app first, then Config → App Export →
  Export app json. Git Sync is self-hosted-only, so nothing syncs itself. If you
  changed the app and did not re-export, the repo is stale.
- **Variables interpolate as `{{name}}`.** REST connector auth is API Key,
  target Header, value `Bearer <token>`.
- **Approve/reject is the human-in-the-loop guardrail**, built from Table Grid +
  Button + Action Flow + Toast. DronaHQ's native HITL approval UI is unverified;
  those four controls are not.
- **WhatsApp is two surfaces, not one.** Inbound is DronaHQ's native trigger on
  Meta's WhatsApp Business API. Outbound is the Twilio connector, numbers
  prefixed `whatsapp:` on both ends. Different provider, different credential.
- **The WhatsApp actionflow block is not a sender.** It opens WhatsApp on the
  viewer's own device with a prefilled message they must press Send on, and it
  has no credential field. It cannot deliver an alert. See
  [docs/research/dronahq.md](../docs/research/dronahq.md) §1.
- **Meta's 24-hour window applies inbound, Twilio's template rules outbound.** A
  9am brief to someone who has not messaged in 24 hours needs an approved
  template.
- **Agent export is unverified.** App export is documented; nothing documents
  exporting an Agent. If it does not exist, `whatsapp-agent.json` becomes a
  runbook and screenshots, and this table changes with it. B5 checks the console
  on day one.
- **Guardrails go in the agent's "Rules & Guardrails" instructions**: never
  promise a refund, never admit fault, never commit to a date, always escalate a
  crisis to a human. The responder enforces the same list server-side. Both,
  not either.
- **Nothing here posts to a social platform.** The dashboard's send button
  copies a draft for a human. There is no posting integration.

## Who calls this

Brand owners, over WhatsApp. Analysts, in a browser. Judges, at the demo.
