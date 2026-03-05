#!/bin/bash
# Cluster Test: Cross-Node WebSocket PubSub
#
# Verifies that events published on one node reach subscribers on another
# node via Olric PubSub.
#
# Sub-test A: System topic — subscribe on Node 1, CRUD create via Node 2
# Sub-test B: User topic — create topic on Node 1, publish via Node 2
#
# Prerequisites:
#   - cluster-test-runner.sh bootstrap must have been run
#   - websocat (brew install websocat)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/cluster-test-runner.sh"

PASS=0; FAIL=0
assert_pass() { PASS=$((PASS+1)); log "  PASS: $1"; }
assert_fail() { FAIL=$((FAIL+1)); log "  FAIL: $1"; }

# Check prerequisites
if ! command -v websocat &>/dev/null; then
    log "ERROR: websocat not found. Install with: brew install websocat"
    exit 1
fi

log ""
log "=========================================="
log "  Cluster Test: Cross-Node WebSocket PubSub"
log "=========================================="

# Verify cluster is running
for port in $NODE1_HTTP $NODE2_HTTP; do
    if ! curl -s --max-time 2 "http://localhost:$port/api/world" > /dev/null 2>&1; then
        log "ERROR: Node on port $port not responding. Run: ./cluster-test-runner.sh bootstrap"
        exit 1
    fi
done

token=$(read_token)
if [ -z "$token" ]; then
    log "ERROR: No auth token. Run: ./cluster-test-runner.sh bootstrap"
    exit 1
fi

# Helper: check WS response by request ID
ws_check_ok() {
    local file="$1" req_id="$2"
    grep "\"id\":\"$req_id\"" "$file" 2>/dev/null | grep -q '"ok":true'
}

ws_check_error() {
    local file="$1" req_id="$2"
    grep "\"id\":\"$req_id\"" "$file" 2>/dev/null | grep -o '"error":"[^"]*"' | head -1 || echo ""
}

# ── Sub-test A: System topic (CRUD events) ───────────────────────────────────

log ""
log "--- Sub-test A: System topic cross-node events ---"
log "Subscribe on Node 1, CRUD create via Node 2"

out_a="/tmp/ws-subtest-a.txt"
rm -f "$out_a"

(
    echo '{"method":"subscribe","id":"sub1","attributes":{"topicName":"json_schema"}}'
    sleep 15
) | websocat --header="Authorization: Bearer $token" \
    "ws://localhost:$NODE1_HTTP/live" > "$out_a" 2>/dev/null &
ws_pid_a=$!

sleep 3

# Check subscription response
if ws_check_ok "$out_a" "sub1"; then
    assert_pass "Subscribed to json_schema on Node 1"
else
    assert_fail "Subscription to json_schema failed"
fi

# CRUD create on Node 2 (triggers event middleware → Olric PubSub)
schema_name="ws-test-$(date +%s)"
log "  Creating json_schema '$schema_name' via Node 2 CRUD API..."

curl -s --max-time $TIMEOUT \
    -X POST "http://localhost:$NODE2_HTTP/api/json_schema" \
    -H "Authorization: Bearer $token" \
    -H "Content-Type: application/vnd.api+json" \
    -d "{\"data\":{\"type\":\"json_schema\",\"attributes\":{\"schema_name\":\"$schema_name\",\"json_schema\":\"{}\"}}}" > /dev/null

sleep 5

# Check if Node 1 received an event
event_count=$(grep -c '"type":"event"' "$out_a" 2>/dev/null || echo "0")
event_count=$(echo "$event_count" | tr -d '[:space:]')
if [ "$event_count" -gt 0 ]; then
    assert_pass "Node 1 received $event_count event(s) from cross-node CRUD create"
else
    assert_fail "Node 1 received no events from cross-node create"
fi

kill "$ws_pid_a" 2>/dev/null || true
wait "$ws_pid_a" 2>/dev/null || true
sleep 1

# Cleanup test record
curl -s --max-time 5 "http://localhost:$NODE1_HTTP/api/json_schema?filter=schema_name=$schema_name" \
    -H "Authorization: Bearer $token" | \
    python3 -c "import sys,json; d=json.load(sys.stdin); [print(r['id']) for r in d.get('data',[])]" 2>/dev/null | \
    while read -r rid; do
        curl -s --max-time 5 -X DELETE "http://localhost:$NODE1_HTTP/api/json_schema/$rid" \
            -H "Authorization: Bearer $token" > /dev/null 2>&1
    done

# ── Sub-test B: User-created topic ──────────────────────────────────────────

log ""
log "--- Sub-test B: User-created topic cross-node messaging ---"
log "Create + subscribe on Node 1, publish via Node 2"

TOPIC_NAME="cluster-test-$(date +%s)"

out_b1="/tmp/ws-subtest-b1.txt"
out_b2="/tmp/ws-subtest-b2.txt"
rm -f "$out_b1" "$out_b2"

# Node 1: create topic, set permission, subscribe, then wait for events
(
    echo "{\"method\":\"create-topicName\",\"id\":\"ct1\",\"attributes\":{\"name\":\"$TOPIC_NAME\"}}"
    sleep 2
    echo "{\"method\":\"set-topic-permission\",\"id\":\"sp1\",\"attributes\":{\"topicName\":\"$TOPIC_NAME\",\"permission\":2097151}}"
    sleep 2
    echo "{\"method\":\"subscribe\",\"id\":\"sub2\",\"attributes\":{\"topicName\":\"$TOPIC_NAME\"}}"
    sleep 20
) | websocat --header="Authorization: Bearer $token" \
    "ws://localhost:$NODE1_HTTP/live" > "$out_b1" 2>/dev/null &
ws_pid_b1=$!

sleep 8

# Check Node 1 setup responses
if ws_check_ok "$out_b1" "ct1"; then
    assert_pass "Topic '$TOPIC_NAME' created on Node 1"
else
    assert_fail "Topic creation failed"
fi

if ws_check_ok "$out_b1" "sp1"; then
    assert_pass "Permission set to ALLOW_ALL"
else
    assert_fail "Permission set failed"
fi

if ws_check_ok "$out_b1" "sub2"; then
    assert_pass "Subscribed to user topic on Node 1"
else
    assert_fail "Subscription to user topic failed"
fi

# Node 2: publish message to the topic
log "  Publishing message via Node 2..."
(
    echo "{\"method\":\"new-message\",\"id\":\"nm1\",\"attributes\":{\"topicName\":\"$TOPIC_NAME\",\"message\":{\"text\":\"hello from node 2\",\"ts\":$(date +%s)}}}"
    sleep 5
) | websocat --header="Authorization: Bearer $token" \
    "ws://localhost:$NODE2_HTTP/live" > "$out_b2" 2>/dev/null &
ws_pid_b2=$!

sleep 7

# Check if Node 2 publish succeeded
if ws_check_ok "$out_b2" "nm1"; then
    log "  Node 2 publish accepted"
else
    err_msg=$(ws_check_error "$out_b2" "nm1")
    log "  Node 2 publish response: ${err_msg:-no response}"
fi

# Check if Node 1 received the event
event_received=$(grep -c '"event":"new-message"' "$out_b1" 2>/dev/null || echo "0")
event_received=$(echo "$event_received" | tr -d '[:space:]')
if [ "$event_received" -gt 0 ]; then
    assert_pass "Node 1 received cross-node user topic message"
else
    assert_fail "Node 1 did not receive cross-node user topic message"
fi

# Cleanup
kill "$ws_pid_b1" "$ws_pid_b2" 2>/dev/null || true
wait "$ws_pid_b1" "$ws_pid_b2" 2>/dev/null || true

# Destroy topic
(
    echo "{\"method\":\"destroy-topicName\",\"id\":\"dt1\",\"attributes\":{\"topicName\":\"$TOPIC_NAME\"}}"
    sleep 2
) | websocat --header="Authorization: Bearer $token" \
    "ws://localhost:$NODE1_HTTP/live" > /dev/null 2>/dev/null || true

rm -f "$out_a" "$out_b1" "$out_b2"

# ── Results ─────────────────────────────────────────────────────────────────

log ""
log "=========================================="
log "  Results: $PASS passed, $FAIL failed"
log "=========================================="

[ "$FAIL" -eq 0 ] || exit 1
