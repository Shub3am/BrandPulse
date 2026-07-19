#!/usr/bin/env bash
#
# The two-minute pitch, end to end, on one command.
#
# What it must not do: spend a credit by default, reach the network by default,
# post anything anywhere, or touch another track's data. BP_FIXTURE_MODE is
# pinned to replay unless --live is passed, --reset deletes the demo brand and
# nothing else, and there is no posting step here because there is no posting
# code path in this repo.
#
# It expects bp-orchestrator to be reachable at BP_ORCHESTRATOR_URL. It does not
# start the agents: how a peer is resolved is Nasiko's, and a local port map
# written here would become a second, divergent way to run an agent.

set -euo pipefail

# Exported, because step 1 hands the preflight to scripts/integration/smoke_local.sh
# and an override typed in front of this script has to reach it too.
export BRAND_ID="${BRAND_ID:-brd_demo}"
# 8000, not 8080: docker-compose.agents.yml publishes the orchestrator on
# 127.0.0.1:8000, which is also a2a.Serve's default port and the port its card
# advertises. The BFF keeps 8080 and the browser talks to the BFF directly.
export BP_ORCHESTRATOR_URL="${BP_ORCHESTRATOR_URL:-http://localhost:8000}"
export BP_FIXTURE_MODE="${BP_FIXTURE_MODE:-replay}"

RESET=0
LIVE=0
for arg in "$@"; do
  case "$arg" in
    --reset) RESET=1 ;;
    --live)  LIVE=1 ;;
    *) echo "usage: $0 [--reset] [--live]" >&2; exit 2 ;;
  esac
done

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

STEP=0
step() {
  STEP=$((STEP + 1))
  printf '\n\033[1m[%d/%d] %s\033[0m\n' "$STEP" "$TOTAL_STEPS" "$1"
}

TOTAL_STEPS=$((8 + LIVE))

# psql runs inside the container so the demo needs no client on the host.
psql_demo() {
  docker compose exec -T postgres psql -U brandpulse -d brandpulse -v ON_ERROR_STOP=1 "$@"
}

# One A2A SendMessage against the orchestrator. The trigger decides the time
# bucket, which is what makes a re-run inside the same bucket a no-op.
#
# The shape is bff/src/orchestrator.ts's, because that client is tested against
# a real stub agent. A2A is 1.0 here and every difference from 0.x fails
# silently: the method is SendMessage, the role is ROLE_USER, the version
# travels as a header, and a Part is a flattened oneof with no kind field.
#
# 504 hours, not the orchestrator's 24-hour default. In replay the reddit
# fixture is found by sha256 over {method, arg, opt}, and reddit's opt carries a
# time bucket derived from the width of the window. The recording ran over 21
# days, which is the "month" bucket, so a 24-hour window asks for "day" and
# hashes to six files that were never recorded: reddit silently collects 0 of
# its 112 mentions. 504 hours is those same 21 days.
orchestrate() {
  local trigger="$1" force="$2"
  curl -sS --fail-with-body -X POST "$BP_ORCHESTRATOR_URL" \
    -H 'Content-Type: application/json' \
    -H 'A2A-Version: 1.0' \
    -d "$(cat <<JSON
{"jsonrpc":"2.0","id":"demo-$trigger","method":"SendMessage","params":{"message":{
  "messageId":"demo-$trigger-$(date +%s)","role":"ROLE_USER","parts":[{
    "data":{"brand_id":"$BRAND_ID","trigger":"$trigger","window_hours":504,"force":$force},
    "mediaType":"application/json"}]}}}
JSON
)"
  echo
}

step "Checking Postgres and the orchestrator"
docker compose up -d --no-recreate postgres
# The checks themselves belong to the smoke test, so the demo, CI and a human
# debugging at 3am all get the same sentences. --preflight stops it before the
# pipeline assertions, which need the seed two steps below. It also reaches the
# real health path, /healthz: the line this replaced curled /health, which the
# JSON-RPC handler mounted at "/" answers with a 200 whatever is wrong.
"$REPO_ROOT/scripts/integration/smoke_local.sh" --preflight
echo "  fixture mode: $BP_FIXTURE_MODE"

step "Seeding the demo brand and the fixture corpus"
if [ "$RESET" -eq 1 ]; then
  # -reset deletes this brand and cascades. It never drops the database: one
  # Postgres is shared by every track in this repo.
  go run ./demo/cmd/seed -reset
else
  go run ./demo/cmd/seed
fi

step "Replaying the corpus so it ends now"
go run ./demo/cmd/replay -brand "$BRAND_ID"

step "Running the pipeline over the last 21 days"
orchestrate scheduled true

step "Injecting the crisis"
# It clears its own last injection first, so the thirtieth rehearsal gets a
# surge in the current hour rather than last hour's rows left where they lie.
go run ./demo/cmd/injectcrisis -brand "$BRAND_ID"

step "Re-running the pipeline so the detector sees the surge"
# crisis_replay buckets by the hour, so this is a separate run from the
# scheduled one above rather than an idempotent no-op on the same bucket.
orchestrate crisis_replay false

step "The alert, with the numbers that fired it"
psql_demo -x -c "
  SELECT kind, severity, title, why, jsonb_pretty(evidence) AS evidence
  FROM alerts
  WHERE brand_id = '$BRAND_ID' AND kind = 'crisis'
  ORDER BY created_at DESC
  LIMIT 1"

step "The reply draft, unsent"
# status is 'draft'. Nothing in this repo posts, and this query is the proof a
# judge can run themselves.
psql_demo -x -c "
  SELECT d.channel, d.status, d.tone, d.text
  FROM reply_drafts d
  JOIN alerts a ON a.id = d.alert_id
  WHERE a.brand_id = '$BRAND_ID'
  ORDER BY d.created_at DESC
  LIMIT 1"

step "The brief"
psql_demo -x -c "
  SELECT period, period_start, period_end, jsonb_pretty(payload) AS payload
  FROM briefs
  WHERE brand_id = '$BRAND_ID'
  ORDER BY created_at DESC
  LIMIT 1"

if [ "$LIVE" -eq 1 ]; then
  step "One live Anakin call"
  # Separate and optional on purpose: stage wifi must not be able to break the
  # eight steps above, all of which ran with no network.
  if [ -z "${ANAKIN_API_KEY:-}" ]; then
    echo "ANAKIN_API_KEY is not set, so there is no live call to make" >&2
    exit 1
  fi
  # The mode belongs to the agent process, not to this script: the orchestrator
  # and its collectors have to be running with BP_FIXTURE_MODE=live for this
  # call to reach Anakin.
  orchestrate on_demand true
fi

printf '\n\033[1mDone.\033[0m Re-run with --reset to start from a clean brand.\n'
