#!/usr/bin/env bash
#
# Proof that the locally running BrandPulse is wired together, not merely that
# some processes are alive.
#
# What it must not do: spend a credit, reach any network but this laptop, write
# a row of its own, drop or migrate anything, or hang. BP_FIXTURE_MODE is pinned
# to replay, every database statement here is a read, and the only write is
# bp-orchestrator's own pipeline, which is idempotent on its time bucket. This
# is run thirty times in a row before a pitch, so the thirtieth run has to look
# like the first.
#
# It does not start the agents. How a peer is resolved is Nasiko's, and a local
# port map written here would become a second, divergent way to run an agent:
# the same reason demo/run_demo.sh refuses to start them.
#
# --preflight stops after reachability and cards. That is the half
# demo/run_demo.sh calls, because the pipeline assertions below need a seeded
# brand and run_demo.sh seeds two steps later.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

BRAND_ID="${BRAND_ID:-brd_demo}"
# The two published ports: docker-compose.agents.yml maps 127.0.0.1:8000:8000
# for bp-orchestrator, bff/src/env.ts defaults the BFF to 8080. 8000 is also
# a2a.Serve's fallback port, the Dockerfile's EXPOSE and the card's url, so the
# agent has one port to remember rather than two.
BP_ORCHESTRATOR_URL="${BP_ORCHESTRATOR_URL:-http://localhost:8000}"
BFF_URL="${BFF_URL:-http://localhost:8080}"

# Pinned rather than defaulted. Note this binds this shell only: an agent
# already running in live mode is unaffected, which is why the run summary below
# prints credits_used instead of claiming zero spend.
export BP_FIXTURE_MODE=replay

# Every agent listens on 8000 inside its container. internal/a2a/serve.go's
# defaultPort says so, and every AgentCard.json url agrees.
AGENT_PORT=8000
HEALTH_PATH=/healthz
CARD_PATH=/.well-known/agent-card.json

HTTP_TIMEOUT=5
# Matches peerCallTimeout in internal/a2a/call.go, which matches
# NASIKO_FLOW_TIMEOUT_SECS. Waiting longer than the flow guard only turns a
# rejected run into a hung script.
RUN_TIMEOUT=180
PG_WAIT_SECONDS=60
# A container compose has just started answers nothing for a second or two, and
# the usual way to arrive here is straight out of `up -d --build`.
AGENT_WAIT_SECONDS=30

PREFLIGHT=0
for arg in "$@"; do
  case "$arg" in
    --preflight) PREFLIGHT=1 ;;
    *) echo "usage: $0 [--preflight]" >&2; exit 2 ;;
  esac
done

# An extra compose file is how the eight unpublished agents become reachable at
# all. It does not exist yet; when it does, including it here is what lets
# `docker compose exec` find them.
COMPOSE=(docker compose)
AGENTS_COMPOSE_FILE=docker-compose.agents.yml
if [ -f "$AGENTS_COMPOSE_FILE" ]; then
  COMPOSE=(docker compose -f docker-compose.yml -f "$AGENTS_COMPOSE_FILE")
fi

section() { printf '\n\033[1m%s\033[0m\n' "$1"; }
ok()      { printf '  \033[32m[ OK ]\033[0m %s\n' "$1"; }
warn()    { printf '  \033[33m[WARN]\033[0m %s\n         next: %s\n' "$1" "$2"; }

fail() {
  printf '\n  \033[1;31m[FAIL]\033[0m %s\n         next: %s\n\n' "$1" "$2" >&2
  exit 1
}

# Prints the response body, then a final line holding the HTTP status. A 000
# status is a connection that never happened, which is a different sentence to a
# server that answered badly.
http_get() {
  curl -sS --max-time "$HTTP_TIMEOUT" -w '\n%{http_code}' "$1" 2>/dev/null || printf '\n000'
}

status_of() { printf '%s' "${1##*$'\n'}"; }
body_of()   { printf '%s' "${1%$'\n'*}"; }

psql_q() {
  "${COMPOSE[@]}" exec -T postgres \
    psql -U brandpulse -d brandpulse -qtAX -v ON_ERROR_STOP=1 -c "$1"
}

# The distroless run stage of an agent image ships no shell, so
# `docker compose exec bp-collector curl` cannot work whatever the compose file
# says. The postgres container sits on the same compose network and has bash, so
# its /dev/tcp is the only in-network probe available without pulling an image.
agent_http_in_network() {
  local host="$1" path="$2"
  "${COMPOSE[@]}" exec -T postgres bash -c \
    "exec 3<>/dev/tcp/${host}/${AGENT_PORT} && printf 'GET %s HTTP/1.0\r\nHost: %s\r\n\r\n' '${path}' '${host}' >&3 && cat <&3"
}

# Strips the status line and headers off what agent_http_in_network returned.
http_body_after_headers() { sed '1,/^[[:space:]]*$/d'; }

AGENTS=()
for dir in agents/bp-*/; do
  AGENTS+=("$(basename "$dir")")
done

printf '\033[1mBrandPulse local smoke\033[0m  brand=%s  fixture_mode=%s\n' "$BRAND_ID" "$BP_FIXTURE_MODE"
if [ "$BP_ORCHESTRATOR_URL" = "$BFF_URL" ]; then
  warn "BP_ORCHESTRATOR_URL and BFF_URL are both $BFF_URL, so one of the two checks below is talking to the wrong process. bff/CLAUDE.md and demo/run_demo.sh both claim 8080." \
       "set BFF_URL or BP_ORCHESTRATOR_URL to the port that process actually listens on"
fi

# ---------------------------------------------------------------------------
# 1. Postgres
# ---------------------------------------------------------------------------
section "1. Postgres"

pg_deadline=$((SECONDS + PG_WAIT_SECONDS))
until "${COMPOSE[@]}" ps postgres 2>/dev/null | grep -q healthy; do
  if [ "$SECONDS" -ge "$pg_deadline" ]; then
    fail "Postgres did not report (healthy) within ${PG_WAIT_SECONDS}s, so nothing below it can be trusted." \
         "docker compose up -d --no-recreate postgres && docker compose logs --tail=50 postgres"
  fi
  sleep 2
done
ok "postgres reports (healthy) on 127.0.0.1:5433"

# pg_isready answers true while initdb is still replaying the migration mount,
# which is why docker-compose.yml's own healthcheck selects from a real table
# and why this asks for every table by name instead of asking whether the server
# is up.
missing_tables="$(psql_q "
  SELECT coalesce(string_agg(t, ', '), '')
  FROM unnest(ARRAY['brands','brand_profiles','mentions','mention_enrichment',
                    'topics','topic_mentions','alerts','reply_drafts','briefs',
                    'runs','source_yield','fetch_cache']) AS t
  WHERE to_regclass('public.' || t) IS NULL")" || fail \
  "Could not query the database even though the container is healthy." \
  "docker compose exec -T postgres psql -U brandpulse -d brandpulse -c 'SELECT 1'"

if [ -n "$missing_tables" ]; then
  fail "The schema is incomplete: $missing_tables is missing. The migration mount runs once, on an empty volume only." \
       "docker compose down -v && docker compose up -d postgres   # announce it first, one Postgres is shared by every track"
fi
ok "all 12 tables from db/migrations/001_init.sql are present"

brand_present="$(psql_q "SELECT count(*) FROM brands WHERE id = '$BRAND_ID'")"
if [ "$brand_present" = "0" ]; then
  warn "brand $BRAND_ID does not exist yet, so the pipeline has nothing to run over." \
       "go run ./demo/cmd/seed"
else
  ok "brand $BRAND_ID exists"
fi

# ---------------------------------------------------------------------------
# 2. Agent health
# ---------------------------------------------------------------------------
section "2. Agent health (${HEALTH_PATH}, from internal/a2a/serve.go)"

# Retried rather than asked once. A container that compose has just started is
# up before its listener is, and a cold `docker compose up -d --build` is the
# normal way to arrive here, so a single curl would report a fleet that is
# merely slow as a fleet that is missing.
health_deadline=$((SECONDS + AGENT_WAIT_SECONDS))
while :; do
  health="$(http_get "${BP_ORCHESTRATOR_URL}${HEALTH_PATH}")"
  health_status="$(status_of "$health")"
  health_body="$(body_of "$health")"
  if [ "$health_status" = "200" ] || [ "$SECONDS" -ge "$health_deadline" ]; then
    break
  fi
  sleep 2
done

if [ "$health_status" = "000" ]; then
  start_hint="start bp-orchestrator, or point BP_ORCHESTRATOR_URL at where it is listening (PORT defaults to ${AGENT_PORT})"
  if [ -f "$AGENTS_COMPOSE_FILE" ]; then
    # ps before up: a container that keeps crashing on boot is published on this
    # port and still answers nothing, and `up -d` on it reports success.
    start_hint="docker compose -f docker-compose.yml -f $AGENTS_COMPOSE_FILE ps bp-orchestrator   # then logs --tail=50 bp-orchestrator if it is restarting"
  fi
  fail "bp-orchestrator produced no HTTP response at ${BP_ORCHESTRATOR_URL}${HEALTH_PATH} after ${AGENT_WAIT_SECONDS}s of retrying, so either nothing is listening on that port or the container behind it is not up." \
       "$start_hint"
fi
if [ "$health_status" != "200" ]; then
  # internal/a2a's mux cannot produce a 404: the JSON-RPC handler is mounted at
  # "/" and answers every unmatched path with 200. So a 404 here is not a
  # broken agent, it is some other process holding this port.
  whatever="not an agent built by internal/a2a"
  if [ "$health_status" = "404" ]; then
    whatever="$whatever, whose mux cannot return 404 on any path"
  fi
  fail "Whatever is listening at $BP_ORCHESTRATOR_URL answered ${HEALTH_PATH} with HTTP $health_status, want 200. That is $whatever." \
       "lsof -nP -iTCP:${BP_ORCHESTRATOR_URL##*:} -sTCP:LISTEN   # then point BP_ORCHESTRATOR_URL at bp-orchestrator"
fi
# serve.go's health handler writes 200 and no body. The JSON-RPC handler mounted
# at "/" answers any unmatched path with 200 and a JSON-RPC error body, so a 200
# on its own proves only that something is listening. The empty body is what
# proves it is an agent built by internal/a2a rather than anything else.
if [ -n "$health_body" ]; then
  fail "bp-orchestrator answered ${HEALTH_PATH} with a body (${health_body}), which is the JSON-RPC catch-all replying, not the health handler. That process is not serving internal/a2a's mux." \
       "curl -i ${BP_ORCHESTRATOR_URL}${HEALTH_PATH}   # then check the image is current"
fi
ok "bp-orchestrator healthy at $BP_ORCHESTRATOR_URL"

PEERS_CHECKED=0
if [ ! -f "$AGENTS_COMPOSE_FILE" ]; then
  warn "$AGENTS_COMPOSE_FILE does not exist, so the other eight agents have no address on this machine and were not checked. Only bp-orchestrator is published to the host." \
       "write $AGENTS_COMPOSE_FILE, or run the fleet however your track runs it, then re-run this script"
else
  for agent in "${AGENTS[@]}"; do
    if [ "$agent" = "bp-orchestrator" ]; then continue; fi
    if peer="$(agent_http_in_network "$agent" "$HEALTH_PATH" 2>/dev/null)" \
       && printf '%s' "$peer" | head -1 | grep -q ' 200 '; then
      ok "$agent healthy on the compose network"
      PEERS_CHECKED=$((PEERS_CHECKED + 1))
    else
      warn "$agent did not answer ${HEALTH_PATH} on the compose network." \
           "docker compose -f docker-compose.yml -f $AGENTS_COMPOSE_FILE logs --tail=50 $agent"
    fi
  done
fi

# ---------------------------------------------------------------------------
# 3. Served AgentCard against the card on disk
# ---------------------------------------------------------------------------
section "3. AgentCard on the wire matches the card on disk"

# This is the check that catches a stale image. A container built before the
# last card edit serves the old name or the old version and everything else
# still looks fine.
compare_card() {
  local agent="$1" served="$2" source="$3"
  local disk_name disk_version served_name served_version

  disk_name="$(jq -r '.name' "agents/$agent/AgentCard.json")"
  disk_version="$(jq -r '.version' "agents/$agent/AgentCard.json")"

  if ! served_name="$(printf '%s' "$served" | jq -er '.name')"; then
    fail "$agent served something that is not an AgentCard at $CARD_PATH ($source)." \
         "curl -s ${BP_ORCHESTRATOR_URL}${CARD_PATH} | head -c 400"
  fi
  served_version="$(printf '%s' "$served" | jq -r '.version')"

  if [ "$served_name" != "$disk_name" ]; then
    fail "$agent serves name '$served_name' but agents/$agent/AgentCard.json says '$disk_name'. The running image is not built from this tree." \
         "docker compose -f docker-compose.yml -f $AGENTS_COMPOSE_FILE build --no-cache $agent && docker compose up -d $agent"
  fi
  if [ "$served_version" != "$disk_version" ]; then
    fail "$agent serves version '$served_version' but agents/$agent/AgentCard.json says '$disk_version'. That is a stale image, and every downstream failure will point somewhere else." \
         "docker compose -f docker-compose.yml -f $AGENTS_COMPOSE_FILE build --no-cache $agent && docker compose up -d $agent"
  fi
  ok "$agent card matches disk: $disk_name $disk_version"
}

card="$(http_get "${BP_ORCHESTRATOR_URL}${CARD_PATH}")"
if [ "$(status_of "$card")" != "200" ]; then
  fail "bp-orchestrator answered $CARD_PATH with HTTP $(status_of "$card"), want 200. A2A consumers and nasiko routing both read that path." \
       "curl -i ${BP_ORCHESTRATOR_URL}${CARD_PATH}"
fi
compare_card bp-orchestrator "$(body_of "$card")" "over HTTP"

if [ "$PEERS_CHECKED" -gt 0 ]; then
  for agent in "${AGENTS[@]}"; do
    if [ "$agent" = "bp-orchestrator" ]; then continue; fi
    if peer="$(agent_http_in_network "$agent" "$CARD_PATH" 2>/dev/null)"; then
      compare_card "$agent" "$(printf '%s' "$peer" | http_body_after_headers)" "on the compose network"
    fi
  done
else
  warn "cards were compared for bp-orchestrator only, because the other eight agents were not reachable above." \
       "python3 scripts/check_agent_cards.py   # validates the cards on disk, which is the half that needs no cluster"
fi

if [ "$PREFLIGHT" -eq 1 ]; then
  printf '\n\033[1mPreflight OK.\033[0m Postgres, bp-orchestrator and its card are good.\n'
  exit 0
fi

# ---------------------------------------------------------------------------
# 4. One real A2A round trip
# ---------------------------------------------------------------------------
section "4. A2A SendMessage against bp-orchestrator"

if [ "$brand_present" = "0" ]; then
  fail "There is no brand $BRAND_ID, so a pipeline run would fail on 'no confirmed profile' and tell you nothing about the wiring." \
       "go run ./demo/cmd/seed && go run ./demo/cmd/replay -brand $BRAND_ID"
fi

# The envelope is bff/src/orchestrator.ts, verbatim in shape, because that is the
# client this system already proves against a stub agent in its own tests.
# internal/a2a/serve.go mounts a2asrv.NewJSONRPCHandler, the A2A 1.0 handler, so
# the method is SendMessage and not 0.x's message/send, the role is ROLE_USER,
# and a Part is a flattened oneof with no kind discriminator. Every one of those
# differences fails silently if guessed.
#
# force is true so the thirtieth run is not silently served by the time-bucket
# idempotency the orchestrator applies to a repeat trigger.
task="$(curl -sS --max-time "$RUN_TIMEOUT" -X POST "$BP_ORCHESTRATOR_URL" \
  -H 'Content-Type: application/json' \
  -H 'A2A-Version: 1.0' \
  -d "$(cat <<JSON
{"jsonrpc":"2.0","id":"smoke-scheduled","method":"SendMessage","params":{"message":{
  "messageId":"smoke-scheduled-$(date +%s)","role":"ROLE_USER","parts":[{
    "data":{"brand_id":"$BRAND_ID","trigger":"scheduled","window_hours":504,"force":true},
    "mediaType":"application/json"}]}}}
JSON
)")" || fail \
  "The SendMessage call to $BP_ORCHESTRATOR_URL did not complete within ${RUN_TIMEOUT}s." \
  "docker compose logs --tail=100 bp-orchestrator"

if rpc_error="$(printf '%s' "$task" | jq -er '.error.message')"; then
  fail "bp-orchestrator rejected SendMessage: $rpc_error" \
       "check the RunInput shape against docs/CONTRACTS.md §2"
fi

# .result.task, not .result. The handler wraps its reply in an a2a.StreamResponse
# whose MarshalJSON keys the payload by kind, so the task sits one level down.
state="$(printf '%s' "$task" | jq -r '.result.task.status.state // "no state in the reply"')"
if [ "$state" != "TASK_STATE_COMPLETED" ]; then
  reason="$(printf '%s' "$task" | jq -r '[.result.task.status.message.parts[]?.text] | join("; ") // ""')"
  fail "The task came back in state $state, want TASK_STATE_COMPLETED. Reason: ${reason:-none given}." \
       "docker compose logs --tail=100 bp-orchestrator"
fi
ok "task completed: $(printf '%s' "$task" | jq -r '.result.task.id')"

run_summary="$(psql_q "
  SELECT id || '  status=' || status
       || '  mentions=' || mentions_collected
       || '  credits=' || credits_used
       || '  degraded=' || coalesce(degraded_reason, 'none')
  FROM runs WHERE brand_id = '$BRAND_ID' ORDER BY started_at DESC LIMIT 1")"
if [ -z "$run_summary" ]; then
  fail "The task completed but no row landed in runs for $BRAND_ID, so bp-orchestrator answered A2A without reaching the database this script is reading." \
       "compare DATABASE_URL in the orchestrator's environment with postgresql://brandpulse:brandpulse@postgres:5432/brandpulse"
fi
ok "latest run: $run_summary"

# ---------------------------------------------------------------------------
# 5. What the pipeline wrote
# ---------------------------------------------------------------------------
section "5. Rows the pipeline wrote for $BRAND_ID"

# One row per table bp-orchestrator/store.go inserts into, scoped to this brand.
# Four are required because a completed run cannot have happened without them;
# the rest are reported, because a quiet window legitimately produces no alert
# and therefore no draft, and failing on that would train everyone to ignore
# this script.
count_for() {
  case "$1" in
    runs)              psql_q "SELECT count(*) FROM runs WHERE brand_id = '$BRAND_ID'" ;;
    source_yield)      psql_q "SELECT count(*) FROM source_yield WHERE brand_id = '$BRAND_ID'" ;;
    mentions)          psql_q "SELECT count(*) FROM mentions WHERE brand_id = '$BRAND_ID'" ;;
    mention_enrichment) psql_q "SELECT count(*) FROM mention_enrichment e JOIN mentions m ON m.id = e.mention_id WHERE m.brand_id = '$BRAND_ID'" ;;
    topics)            psql_q "SELECT count(*) FROM topics WHERE brand_id = '$BRAND_ID'" ;;
    topic_mentions)    psql_q "SELECT count(*) FROM topic_mentions tm JOIN topics t ON t.id = tm.topic_id WHERE t.brand_id = '$BRAND_ID'" ;;
    alerts)            psql_q "SELECT count(*) FROM alerts WHERE brand_id = '$BRAND_ID'" ;;
    reply_drafts)      psql_q "SELECT count(*) FROM reply_drafts d WHERE d.alert_id IN (SELECT id FROM alerts WHERE brand_id = '$BRAND_ID') OR d.mention_id IN (SELECT id FROM mentions WHERE brand_id = '$BRAND_ID')" ;;
    briefs)            psql_q "SELECT count(*) FROM briefs WHERE brand_id = '$BRAND_ID'" ;;
  esac
}

REQUIRED_TABLES="runs mentions mention_enrichment briefs"
printf '  %-20s %8s  %s\n' "TABLE" "ROWS" "REQUIRED"
printf '  %-20s %8s  %s\n' "--------------------" "--------" "--------"
empty_required=""
for table in runs source_yield mentions mention_enrichment topics topic_mentions alerts reply_drafts briefs; do
  rows="$(count_for "$table")"
  required=no
  case " $REQUIRED_TABLES " in *" $table "*) required=yes ;; esac
  printf '  %-20s %8s  %s\n' "$table" "$rows" "$required"
  if [ "$required" = yes ] && [ "$rows" = "0" ]; then
    empty_required="$empty_required $table"
  fi
done

if [ -n "$empty_required" ]; then
  fail "The run completed but wrote nothing into:$empty_required. The table above says how far down the pipeline it got before it stopped." \
       "docker compose logs --tail=200 bp-orchestrator   # and check runs.degraded_reason printed above"
fi
ok "every required table holds rows"

# ---------------------------------------------------------------------------
# 6. The BFF serves what the database holds
# ---------------------------------------------------------------------------
section "6. BFF"

bff_health="$(http_get "$BFF_URL/health")"
if [ "$(status_of "$bff_health")" != "200" ]; then
  fail "The BFF is not answering at $BFF_URL/health (HTTP $(status_of "$bff_health")). The dashboard has no other backend." \
       "cd bff && npm run dev   # needs DASHBOARD_ORIGIN, ORCHESTRATOR_URL and DATABASE_URL set"
fi
ok "BFF healthy at $BFF_URL"

pulse="$(http_get "$BFF_URL/api/brands/$BRAND_ID/pulse")"
if [ "$(status_of "$pulse")" != "200" ]; then
  fail "The BFF answered /api/brands/$BRAND_ID/pulse with HTTP $(status_of "$pulse")." \
       "curl -i $BFF_URL/api/brands/$BRAND_ID/pulse"
fi

bff_brand="$(body_of "$pulse" | jq -r '.profile.name // ""')"
db_brand="$(psql_q "SELECT name FROM brands WHERE id = '$BRAND_ID'")"
if [ "$bff_brand" != "$db_brand" ]; then
  fail "The BFF calls brand $BRAND_ID '${bff_brand:-null}' and Postgres calls it '$db_brand'. The BFF is reading a different database than this script is." \
       "compare DATABASE_URL in the BFF's environment with postgresql://brandpulse:brandpulse@localhost:5433/brandpulse"
fi
ok "BFF and Postgres agree the brand is '$db_brand'"

pulse_errors="$(body_of "$pulse" | jq -r '.errors | join("; ")')"
if [ -n "$pulse_errors" ]; then
  warn "the pulse rendered with failed panels: $pulse_errors" \
       "check the BFF log for the failing query"
fi

printf '\n\033[1;32mLocal smoke passed.\033[0m\n'
