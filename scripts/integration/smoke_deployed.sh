#!/usr/bin/env bash
#
# The same wiring proof as smoke_local.sh, against a deployed fleet on Nasiko.
#
# HONESTY NOTE, READ THIS FIRST. This script has never been executed against a
# live cluster. Nobody on this team has a Nasiko credential yet, so everything
# below was written from docs/DEPLOY-NOTES.md and docs/research/nasiko.md §7,
# both of which are source reads of Nasiko at commit 58cfe60, not observations.
#
# The single assumption most likely to be wrong on first contact is the one
# DEPLOY-NOTES Finding 5 flags explicitly: that bp-orchestrator replaying the
# inbound x-nasiko-agent-token header on its outbound peer call is what
# authenticates that call. Finding 5 calls that an inference from how the server
# mints and consumes the token, not a documented contract, and says to confirm
# it on the first live two-agent call. Check 4 below is that call, and check 5
# is what tells you the inference held: a completed task whose RunRecord shows
# zero sources attempted and an empty mentions count is a run whose peer calls
# were all rejected, which looks like success at the protocol layer and is not.
#
# What it must not do: deploy anything, restart anything, write a secret, or
# spend a credit. Every request here is a read except the one SendMessage, and
# that run is idempotent on its time bucket. It must not fall back to a default
# base URL either: a smoke test that silently probes the wrong cluster is worse
# than one that refuses to start.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

BRAND_ID="${BRAND_ID:-brd_demo}"

# The control-plane base URL, no trailing slash. This is deliberately not
# NASIKO_API_URL: that variable, read by internal/a2a/call.go, holds a template
# carrying {agent}, and the deployed value of it is "$NASIKO_BASE_URL/api/agents/{agent}".
# Two names because they are two different strings.
NASIKO_BASE_URL="${NASIKO_BASE_URL:-}"

# Optional. Finding 5 documents only the agent-facing credential, the inbound
# delegation JWT; it says nothing about how an external caller authenticates to
# the control plane, so this is sent only when it is set rather than guessed at.
NASIKO_AUTH_TOKEN="${NASIKO_AUTH_TOKEN:-}"

HTTP_TIMEOUT=10
# Matches peerCallTimeout in internal/a2a/call.go and NASIKO_FLOW_TIMEOUT_SECS.
RUN_TIMEOUT=180

CARD_PATH=/.well-known/agent-card.json

section() { printf '\n\033[1m%s\033[0m\n' "$1"; }
ok()      { printf '  \033[32m[ OK ]\033[0m %s\n' "$1"; }
warn()    { printf '  \033[33m[WARN]\033[0m %s\n         next: %s\n' "$1" "$2"; }

fail() {
  printf '\n  \033[1;31m[FAIL]\033[0m %s\n         next: %s\n\n' "$1" "$2" >&2
  exit 1
}

if [ -z "$NASIKO_BASE_URL" ]; then
  cat >&2 <<'MESSAGE'

  [FAIL] NASIKO_BASE_URL is not set, and this script will not guess a cluster.

         Set it to the control-plane base URL you ran `nasiko connect` against,
         with no trailing slash and no /api suffix. Agents are reached under it
         at POST /api/agents/{agent_id}, per docs/research/nasiko.md section 7.

         next: NASIKO_BASE_URL=https://<control-plane> ./scripts/integration/smoke_deployed.sh
               NASIKO_AUTH_TOKEN=<token> is optional and sent as a bearer token
               when set; nothing in DEPLOY-NOTES states what an external caller
               must present, so it is not assumed.

         This script has never run against a live cluster. See the header.

MESSAGE
  exit 2
fi

NASIKO_BASE_URL="${NASIKO_BASE_URL%/}"

AUTH_HEADER=()
if [ -n "$NASIKO_AUTH_TOKEN" ]; then
  AUTH_HEADER=(-H "Authorization: Bearer $NASIKO_AUTH_TOKEN")
fi

# Prints the response body, then a final line holding the HTTP status. 000 means
# the connection never happened at all, which is a different sentence to a
# server that answered badly.
plane_get() {
  curl -sS --max-time "$HTTP_TIMEOUT" "${AUTH_HEADER[@]}" -w '\n%{http_code}' "$1" 2>/dev/null \
    || printf '\n000'
}

status_of() { printf '%s' "${1##*$'\n'}"; }
body_of()   { printf '%s' "${1%$'\n'*}"; }

AGENTS=()
for dir in agents/bp-*/; do
  AGENTS+=("$(basename "$dir")")
done

printf '\033[1mBrandPulse deployed smoke\033[0m  cluster=%s  brand=%s\n' "$NASIKO_BASE_URL" "$BRAND_ID"
if [ -z "$NASIKO_AUTH_TOKEN" ]; then
  warn "NASIKO_AUTH_TOKEN is unset, so every request below is unauthenticated. If the control plane requires a credential, expect a 401 on the next line and not a routing bug." \
       "nasiko auth login, then export the credential it writes as NASIKO_AUTH_TOKEN"
fi

# ---------------------------------------------------------------------------
# 1. The control plane answers
# ---------------------------------------------------------------------------
section "1. Control plane"

plane="$(plane_get "$NASIKO_BASE_URL/api/agents")"
plane_status="$(status_of "$plane")"
case "$plane_status" in
  000) fail "Nothing answered at $NASIKO_BASE_URL/api/agents. The host is unreachable, not merely unhappy." \
            "curl -v $NASIKO_BASE_URL/api/agents   # check the URL, DNS and any VPN" ;;
  200) ok "control plane answering at $NASIKO_BASE_URL" ;;
  401|403) fail "The control plane refused the request with HTTP $plane_status. Every check below needs the same credential, so fix this one first." \
                "nasiko auth login, then NASIKO_AUTH_TOKEN=<token> $0" ;;
  *) fail "The control plane answered /api/agents with HTTP $plane_status, want 200." \
          "curl -i $NASIKO_BASE_URL/api/agents" ;;
esac

# ---------------------------------------------------------------------------
# 2. Every agent is deployed and serving the card in this tree
# ---------------------------------------------------------------------------
section "2. Deployed AgentCards match the cards on disk"

# Same reasoning as the local smoke: a stale image is the likeliest integration
# failure, and it fails silently. Here it is likelier still, because
# `nasiko deploy` rewrites version back into the card via sync_card_version, so
# a mismatch can mean the tree moved on since the deploy rather than the deploy
# failing.
#
# Finding 5 says the server proxies /api/agents/{id}/{*rest} to the agent's root
# path, so the card should be reachable this way. That mapping is source-read
# and unconfirmed; a 404 here is as likely to be this assumption as a missing
# agent, which is why it is reported rather than fatal.
CARDS_CHECKED=0
for agent in "${AGENTS[@]}"; do
  card="$(plane_get "$NASIKO_BASE_URL/api/agents/$agent$CARD_PATH")"
  card_status="$(status_of "$card")"

  if [ "$card_status" != "200" ]; then
    warn "$agent did not serve $CARD_PATH through the control plane (HTTP $card_status). Either it is not deployed or /api/agents/{id}/{*rest} does not proxy a GET." \
         "nasiko ps --json | jq '.[] | select(.name==\"$agent\")'"
    continue
  fi

  served_name="$(body_of "$card" | jq -r '.name // ""')"
  served_version="$(body_of "$card" | jq -r '.version // ""')"
  disk_name="$(jq -r '.name' "agents/$agent/AgentCard.json")"
  disk_version="$(jq -r '.version' "agents/$agent/AgentCard.json")"

  if [ "$served_name" != "$disk_name" ]; then
    fail "$agent serves name '$served_name' and agents/$agent/AgentCard.json says '$disk_name'. Something else is deployed under that agent id." \
         "nasiko ps --json && nasiko deploy ./agents/$agent --name $agent"
  fi
  if [ "$served_version" != "$disk_version" ]; then
    fail "$agent serves version '$served_version' and agents/$agent/AgentCard.json says '$disk_version'. The cluster is running an older build than this tree." \
         "nasiko deploy ./agents/$agent --name $agent"
  fi
  ok "$agent card matches disk: $disk_name $disk_version"
  CARDS_CHECKED=$((CARDS_CHECKED + 1))
done

if [ "$CARDS_CHECKED" -eq 0 ]; then
  fail "Not one of the ${#AGENTS[@]} agents served its card through the control plane, so either nothing is deployed or the /api/agents/{id}/{*rest} proxy does not do what Finding 5 infers." \
       "nasiko ps --json   # if the agents are listed, the routing assumption is what is wrong, not the deploy"
fi

# ---------------------------------------------------------------------------
# 3. bp-orchestrator accepts A2A over the control plane
# ---------------------------------------------------------------------------
section "3. A2A SendMessage through POST /api/agents/bp-orchestrator"

# The envelope is bff/src/orchestrator.ts, verbatim in shape: A2A 1.0 names the
# method SendMessage, spells the role ROLE_USER and flattens Part's oneof so
# there is no kind discriminator. 0.x spellings are accepted by nothing that
# internal/a2a/serve.go mounts, and they fail as a bare RPC error rather than as
# anything that points at the wire version.
#
# force is true so the run is not served by the orchestrator's time-bucket
# idempotency, which would make check 4 assert nothing about this cluster now.
task="$(curl -sS --max-time "$RUN_TIMEOUT" -X POST "$NASIKO_BASE_URL/api/agents/bp-orchestrator" \
  "${AUTH_HEADER[@]}" \
  -H 'Content-Type: application/json' \
  -H 'A2A-Version: 1.0' \
  -d "$(cat <<JSON
{"jsonrpc":"2.0","id":"smoke-deployed","method":"SendMessage","params":{"message":{
  "messageId":"smoke-deployed-$(date +%s)","role":"ROLE_USER","parts":[{
    "data":{"brand_id":"$BRAND_ID","trigger":"scheduled","window_hours":504,"force":true},
    "mediaType":"application/json"}]}}}
JSON
)")" || fail \
  "The SendMessage call did not complete within ${RUN_TIMEOUT}s. That is also what a fail-closed flow guard looks like from here." \
  "nasiko logs bp-orchestrator -f   # and confirm Redis is up, per DEPLOY-NOTES Finding 6"

if rpc_error="$(printf '%s' "$task" | jq -er '.error.message')"; then
  fail "bp-orchestrator rejected SendMessage: $rpc_error" \
       "a -32009 VersionNotSupported here means an AgentCard shipped protocolVersion other than 1.0"
fi

# .result.task, not .result. a2asrv wraps its reply in an a2a.StreamResponse
# whose MarshalJSON keys the payload by kind, so the task sits one level down.
state="$(printf '%s' "$task" | jq -r '.result.task.status.state // "no state in the reply"')"
if [ "$state" != "TASK_STATE_COMPLETED" ]; then
  reason="$(printf '%s' "$task" | jq -r '[.result.task.status.message.parts[]?.text] | join("; ") // ""')"
  fail "The task came back in state $state, want TASK_STATE_COMPLETED. Reason: ${reason:-none given}." \
       "nasiko logs bp-orchestrator --tail 200"
fi
ok "task completed: $(printf '%s' "$task" | jq -r '.result.task.id')"

# ---------------------------------------------------------------------------
# 4. The peer calls inside that run actually happened
# ---------------------------------------------------------------------------
section "4. Two-agent confirmation (DEPLOY-NOTES Finding 5)"

# This is the check the whole file exists for. bp-orchestrator returns a
# completed task whether or not its peers answered: a2a.Call's failures become
# RunRecord.DegradedReason and the run carries on, by design. So the protocol
# layer saying "completed" is not evidence that the token replay authenticated
# anything. The RunRecord is.
# Selected by artifact name and by the part that carries data, the way
# bff/src/orchestrator.ts selects it, rather than by index. An agent is free to
# attach a second artifact and indexing would then read the wrong one.
run="$(printf '%s' "$task" | jq -c '
  [.result.task.artifacts[]? | select(.name == "RunRecord")
   | .parts[]? | select(has("data")) | .data] | first // empty')"
if [ -z "$run" ]; then
  fail "The completed task carried no artifact named RunRecord with a data part, so there is nothing to judge the peer calls by." \
       "printf '%s' \"\$task\" | jq '.result.task.artifacts'   # a2a-go v2 marshals a data part with no kind discriminator"
fi

attempted="$(printf '%s' "$run" | jq -r '.sources_attempted | length')"
mentions="$(printf '%s' "$run" | jq -r '.mentions_collected')"
degraded="$(printf '%s' "$run" | jq -r '.degraded_reason // ""')"
errors="$(printf '%s' "$run" | jq -r '.errors | join("; ")')"

printf '  run %s  status=%s  sources_attempted=%s  mentions=%s  credits=%s\n' \
  "$(printf '%s' "$run" | jq -r '.id')" \
  "$(printf '%s' "$run" | jq -r '.status')" \
  "$attempted" "$mentions" \
  "$(printf '%s' "$run" | jq -r '.credits_used')"

if [ "$attempted" = "0" ]; then
  fail "The run attempted no source at all, so bp-orchestrator never reached a single peer. This is what a rejected x-nasiko-agent-token replay looks like: the task completes and nothing happened. Errors: ${errors:-none}." \
       "nasiko logs bp-collector --tail 200   # a 401 or 403 there confirms Finding 5's inference is wrong"
fi
if [ "$mentions" = "0" ]; then
  fail "bp-orchestrator attempted $attempted source(s) and collected zero mentions, so the peer calls went out and came back empty. Errors: ${errors:-none}." \
       "nasiko logs bp-collector --tail 200"
fi
ok "peer calls reached $attempted source(s) and returned $mentions mentions: the token replay in internal/a2a/call.go works on this cluster"

if [ -n "$degraded" ]; then
  warn "the run degraded: $degraded" \
       "nasiko observe   # a fan-out cut is the flow guard, per DEPLOY-NOTES Finding 6"
fi
if [ -n "$errors" ]; then
  warn "the run reported errors: $errors" \
       "nasiko logs bp-orchestrator --tail 200"
fi

# Row counts and the BFF are deliberately absent. The deployed Postgres is not
# reachable from a laptop and the deployed BFF is not part of the Nasiko fleet,
# so asserting on either from here would mean inventing an address. Run
# smoke_local.sh for those two.
printf '\n\033[1;32mDeployed smoke passed.\033[0m Row counts and the BFF are local-only checks.\n'
