#!/bin/bash
# E2E Test: publish_to_topic HTTP Action API (Issue #184)
#
# Tests that HTTP POST /action/world/publish_to_topic delivers messages
# to WebSocket subscribers via Olric PubSub.
#
# Prerequisites:
#   - Server running on port 6336 with fresh DB
#   - daptin-cli configured (context "e2etest")
#   - websocat installed (brew install websocat)

set -euo pipefail

PASS=0; FAIL=0
log()  { echo "[$(date +%H:%M:%S)] $*"; }
assert_pass() { PASS=$((PASS+1)); log "  PASS: $1"; }
assert_fail() { FAIL=$((FAIL+1)); log "  FAIL: $1"; }

TOKEN=$(cat /tmp/daptin-token.txt)

log ""
log "=========================================="
log "  E2E Test: publish_to_topic Action"
log "=========================================="

# ── Test 1: Error on non-existent topic ──────────────────────────────────────

log ""
log "--- Test 1: Publish to non-existent topic returns error ---"

RESULT=$(daptin-cli -o json execute world publish_to_topic \
    topicName=nonexistent-topic message='{"hello":"world"}' 2>&1)

if echo "$RESULT" | grep -q "topic not found"; then
    assert_pass "Non-existent topic returns 'topic not found'"
else
    assert_fail "Expected 'topic not found', got: $RESULT"
fi

# ── Test 2: Create user topic, publish via HTTP, receive on WebSocket ────────

log ""
log "--- Test 2: HTTP publish to user topic reaches WebSocket subscriber ---"

TOPIC_NAME="e2e-http-publish-$(date +%s)"
WS_OUT="/tmp/ws-publish-test.txt"
rm -f "$WS_OUT"

# 2a. Create user topic via WebSocket
log "Creating topic: $TOPIC_NAME"
CREATE_OUT="/tmp/ws-create-topic.txt"
rm -f "$CREATE_OUT"
(
    echo "{\"method\":\"create-topicName\",\"id\":\"ct1\",\"attributes\":{\"name\":\"$TOPIC_NAME\"}}"
    sleep 3
) | websocat --header="Authorization: Bearer $TOKEN" \
    "ws://localhost:6336/live" > "$CREATE_OUT" 2>/dev/null

if grep -q '"ok":true' "$CREATE_OUT" 2>/dev/null; then
    assert_pass "Topic '$TOPIC_NAME' created"
else
    assert_fail "Failed to create topic (output: $(cat "$CREATE_OUT" 2>/dev/null))"
fi

# 2b. Subscribe to topic via WebSocket (background)
log "Subscribing to topic via WebSocket..."
(
    echo "{\"method\":\"subscribe\",\"id\":\"sub1\",\"attributes\":{\"topicName\":\"$TOPIC_NAME\"}}"
    sleep 10
) | websocat --header="Authorization: Bearer $TOKEN" \
    "ws://localhost:6336/live" > "$WS_OUT" 2>/dev/null &
WS_PID=$!

sleep 3

# Check subscription succeeded
if grep -q '"ok":true' "$WS_OUT" 2>/dev/null; then
    assert_pass "WebSocket subscribed to '$TOPIC_NAME'"
else
    assert_fail "WebSocket subscription failed (output: $(cat "$WS_OUT" 2>/dev/null))"
fi

# 2c. Publish via HTTP action
log "Publishing via HTTP action..."
HTTP_RESULT=$(daptin-cli -o json execute world publish_to_topic \
    topicName="$TOPIC_NAME" message='{"command":"open_tab","url":"https://example.com"}' 2>&1)

if echo "$HTTP_RESULT" | grep -qi "published\|success"; then
    assert_pass "HTTP publish_to_topic returned success"
else
    assert_fail "HTTP publish failed: $HTTP_RESULT"
fi

# 2d. Wait and check WebSocket received the event
sleep 3

if grep -q '"event":"new-message"' "$WS_OUT" 2>/dev/null; then
    assert_pass "WebSocket received new-message event"
else
    assert_fail "WebSocket did NOT receive event (output: $(cat "$WS_OUT" 2>/dev/null))"
fi

# Data field is base64-encoded in WebSocket wire format
EVENT_DATA=$(grep '"event":"new-message"' "$WS_OUT" 2>/dev/null | grep -o '"data":"[^"]*"' | head -1 | sed 's/"data":"//;s/"$//')
DECODED_DATA=$(echo "$EVENT_DATA" | base64 -d 2>/dev/null || echo "")
if echo "$DECODED_DATA" | grep -q "open_tab"; then
    assert_pass "WebSocket event contains published payload (base64 decoded)"
else
    assert_fail "WebSocket event missing payload (decoded: $DECODED_DATA)"
fi

if grep -q "\"topic\":\"$TOPIC_NAME\"" "$WS_OUT" 2>/dev/null; then
    assert_pass "WebSocket event has correct topic name"
else
    assert_fail "WebSocket event wrong topic (output: $(cat "$WS_OUT" 2>/dev/null))"
fi

kill $WS_PID 2>/dev/null || true
wait $WS_PID 2>/dev/null || true

# ── Test 3: System topic publish ─────────────────────────────────────────────

log ""
log "--- Test 3: HTTP publish to system topic ---"

# First become admin (needed for CanCreate on system topics)
daptin-cli execute world become_an_administrator 2>&1 > /dev/null || true
sleep 2
# Re-login to get updated token with admin groups
./scripts/testing/test-runner.sh token 2>&1 > /dev/null
TOKEN=$(cat /tmp/daptin-token.txt)

# Update CLI config with fresh token
cat > ~/.daptin/config.yaml <<EOF
currentContext: e2etest
hosts:
- endpoint: http://localhost:6336
  name: e2etest
  token: $TOKEN
EOF

SYS_OUT="/tmp/ws-sys-publish-test.txt"
rm -f "$SYS_OUT"

# Subscribe to system topic "json_schema" via WebSocket
log "Subscribing to system topic 'json_schema'..."
(
    echo '{"method":"subscribe","id":"sys1","attributes":{"topicName":"json_schema"}}'
    sleep 10
) | websocat --header="Authorization: Bearer $TOKEN" \
    "ws://localhost:6336/live" > "$SYS_OUT" 2>/dev/null &
SYS_WS_PID=$!

sleep 3

if grep -q '"ok":true' "$SYS_OUT" 2>/dev/null; then
    assert_pass "Subscribed to system topic 'json_schema'"
else
    assert_fail "System topic subscription failed (output: $(cat "$SYS_OUT" 2>/dev/null))"
fi

# Publish to system topic via HTTP
log "Publishing to system topic via HTTP..."
SYS_RESULT=$(daptin-cli -o json execute world publish_to_topic \
    topicName=json_schema message='{"test":"system-topic-event"}' 2>&1)

if echo "$SYS_RESULT" | grep -qi "published\|success"; then
    assert_pass "HTTP publish to system topic returned success"
else
    assert_fail "System topic publish failed: $SYS_RESULT"
fi

sleep 3

if grep -q '"event":"new-message"' "$SYS_OUT" 2>/dev/null; then
    assert_pass "System topic WebSocket received new-message event"
else
    assert_fail "System topic WebSocket did NOT receive event (output: $(cat "$SYS_OUT" 2>/dev/null))"
fi

kill $SYS_WS_PID 2>/dev/null || true
wait $SYS_WS_PID 2>/dev/null || true

# ── Test 4: Permission denied without auth ───────────────────────────────────

log ""
log "--- Test 4: Unauthenticated publish is rejected ---"

NOAUTH_RESULT=$(curl -s -X POST http://localhost:6336/action/world/publish_to_topic \
    -H "Content-Type: application/json" \
    -d '{"attributes":{"topicName":"json_schema","message":{"test":"noauth"}}}' 2>&1)

if echo "$NOAUTH_RESULT" | grep -qi "unauthorized\|error\|denied\|forbidden"; then
    assert_pass "Unauthenticated publish rejected"
else
    assert_fail "Unauthenticated publish NOT rejected: $NOAUTH_RESULT"
fi

# ── Summary ──────────────────────────────────────────────────────────────────

log ""
log "=========================================="
log "  Results: $PASS passed, $FAIL failed"
log "=========================================="

if [ "$FAIL" -gt 0 ]; then
    exit 1
fi
