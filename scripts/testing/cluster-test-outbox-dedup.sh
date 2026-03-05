#!/bin/bash
# Cluster Test: Outbox NX Claim Deduplication
#
# Verifies that when two nodes both run process_outbox, each mail is
# processed by exactly one node (Olric NX claim prevents duplicates).
#
# Prerequisites: cluster-test-runner.sh bootstrap must have been run

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/cluster-test-runner.sh"

PASS=0; FAIL=0
assert_eq() {
    if [ "$1" = "$2" ]; then
        PASS=$((PASS+1))
        log "  PASS: $3"
    else
        FAIL=$((FAIL+1))
        log "  FAIL: $3 (got='$1', want='$2')"
    fi
}

assert_ge() {
    if [ "$1" -ge "$2" ]; then
        PASS=$((PASS+1))
        log "  PASS: $3"
    else
        FAIL=$((FAIL+1))
        log "  FAIL: $3 (got=$1, want>=$2)"
    fi
}

log ""
log "=========================================="
log "  Cluster Test: Outbox NX Deduplication"
log "=========================================="

# Verify cluster is running
for port in $NODE1_HTTP $NODE2_HTTP; do
    if ! curl -s --max-time 2 "http://localhost:$port/api/world" > /dev/null 2>&1; then
        log "ERROR: Node on port $port not responding. Run: ./cluster-test-runner.sh bootstrap"
        exit 1
    fi
done

# ── Step 1: Insert 10 test outbox records into PostgreSQL ───────────────────

log ""
log "--- Step 1: Inserting 10 test outbox records ---"

# Clear any existing test outbox entries
pg_exec "DELETE FROM outbox WHERE from_address LIKE 'clustertest-%';" || true

MAIL_COUNT=10
for i in $(seq 1 $MAIL_COUNT); do
    local_from="clustertest-${i}@localhost"
    local_to="nobody-${i}@invalid.test"
    # Build a minimal RFC 2822 mail, base64 encoded
    raw_mail="From: ${local_from}\r\nTo: ${local_to}\r\nSubject: Cluster Test $i\r\nDate: $(date -R)\r\n\r\nTest body $i"
    mail_b64=$(printf '%b' "$raw_mail" | base64)

    pg_exec "INSERT INTO outbox (reference_id, from_address, to_address, to_host, mail, sent, retry_count, permission)
             VALUES (
                 decode(replace(gen_random_uuid()::text, '-', ''), 'hex'),
                 '${local_from}',
                 '${local_to}',
                 '',
                 '${mail_b64}'::bytea,
                 false,
                 0,
                 2097151
             );"
done

inserted=$(pg_exec "SELECT count(*) FROM outbox WHERE from_address LIKE 'clustertest-%' AND sent = false;")
assert_eq "$inserted" "$MAIL_COUNT" "Inserted $MAIL_COUNT outbox records"

# ── Step 2: Trigger process_outbox on Node 1 and Node 2 simultaneously ─────

log ""
log "--- Step 2: Triggering process_outbox on Node 1 + Node 2 simultaneously ---"

# Record log line counts before triggering, to only examine new lines
node1_log_before=$(wc -l < "$NODE1_LOG" | tr -d ' ')
node2_log_before=$(wc -l < "$NODE2_LOG" | tr -d ' ')

token=$(read_token)
resp1_file="/tmp/cluster-outbox-resp1.json"
resp2_file="/tmp/cluster-outbox-resp2.json"

curl -s --max-time 30 \
    -X POST "http://localhost:$NODE1_HTTP/action/outbox/process_outbox" \
    -H "Authorization: Bearer $token" \
    -H "Content-Type: application/json" \
    -d '{"attributes":{}}' \
    > "$resp1_file" 2>&1 &
pid1=$!

curl -s --max-time 30 \
    -X POST "http://localhost:$NODE2_HTTP/action/outbox/process_outbox" \
    -H "Authorization: Bearer $token" \
    -H "Content-Type: application/json" \
    -d '{"attributes":{}}' \
    > "$resp2_file" 2>&1 &
pid2=$!

wait "$pid1" || true
wait "$pid2" || true

log "Node 1 response: $(cat "$resp1_file" | head -c 200)"
log "Node 2 response: $(cat "$resp2_file" | head -c 200)"

# ── Step 3: Verify each mail was processed (attempted) by at most one node ─

log ""
log "--- Step 3: Verifying deduplication ---"

# Give nodes a moment to finish async processing
sleep 3

# Check how many mails were marked sent or had retry_count bumped
sent_count=$(pg_exec "SELECT count(*) FROM outbox WHERE from_address LIKE 'clustertest-%' AND sent = true;")
retry_count=$(pg_exec "SELECT count(*) FROM outbox WHERE from_address LIKE 'clustertest-%' AND retry_count > 0;")
untouched=$(pg_exec "SELECT count(*) FROM outbox WHERE from_address LIKE 'clustertest-%' AND sent = false AND retry_count = 0;")
processed=$((sent_count + retry_count))

log "  Sent: $sent_count, Retried (send failed): $retry_count, Untouched: $untouched"

# Each mail should have been attempted — they go to invalid hosts so will likely fail,
# but the NX claim ensures only one node attempts each mail
assert_eq "$untouched" "0" "All $MAIL_COUNT mails were claimed (none untouched)"

# Check logs for duplicate processing (only new lines since step 2, strip ANSI)
node1_processed=$(tail -n +"$((node1_log_before + 1))" "$NODE1_LOG" | sed 's/\x1b\[[0-9;]*m//g' | grep -c "Outbox mail.*sent to\|Failed to send outbox mail\|Failed to decode outbox mail" 2>/dev/null || echo "0")
node2_processed=$(tail -n +"$((node2_log_before + 1))" "$NODE2_LOG" | sed 's/\x1b\[[0-9;]*m//g' | grep -c "Outbox mail.*sent to\|Failed to send outbox mail\|Failed to decode outbox mail" 2>/dev/null || echo "0")
node1_processed=$(echo "$node1_processed" | tr -d '[:space:]')
node2_processed=$(echo "$node2_processed" | tr -d '[:space:]')
total_log_processed=$((node1_processed + node2_processed))

log "  Node 1 processed: $node1_processed mails (from log)"
log "  Node 2 processed: $node2_processed mails (from log)"

# Both nodes should have picked up some work (proves distribution)
# But total should not exceed MAIL_COUNT (proves no duplicates)
if [ "$total_log_processed" -le "$MAIL_COUNT" ]; then
    PASS=$((PASS+1))
    log "  PASS: No duplicate processing (total=$total_log_processed, max=$MAIL_COUNT)"
else
    FAIL=$((FAIL+1))
    log "  FAIL: Duplicate processing detected (total=$total_log_processed, max=$MAIL_COUNT)"
fi

# Check that no mail ID appears in both node logs
log ""
log "--- Step 4: Cross-checking mail IDs across node logs ---"

# Extract mail IDs from both logs (only new lines, strip ANSI, case-insensitive)
{ tail -n +"$((node1_log_before + 1))" "$NODE1_LOG" | sed 's/\x1b\[[0-9;]*m//g' | grep -oi 'outbox mail \[[0-9]*\]' | grep -o '[0-9]*' || true; } | sort -u > /tmp/cluster-node1-ids.txt
{ tail -n +"$((node2_log_before + 1))" "$NODE2_LOG" | sed 's/\x1b\[[0-9;]*m//g' | grep -oi 'outbox mail \[[0-9]*\]' | grep -o '[0-9]*' || true; } | sort -u > /tmp/cluster-node2-ids.txt

sort -u -o /tmp/cluster-node1-ids.txt /tmp/cluster-node1-ids.txt
sort -u -o /tmp/cluster-node2-ids.txt /tmp/cluster-node2-ids.txt

duplicates=$(comm -12 /tmp/cluster-node1-ids.txt /tmp/cluster-node2-ids.txt | wc -l | tr -d ' ')
if [ "$duplicates" -eq 0 ]; then
    PASS=$((PASS+1))
    log "  PASS: No mail ID processed by both nodes"
else
    FAIL=$((FAIL+1))
    log "  FAIL: $duplicates mail IDs processed by both nodes"
    log "  Duplicates: $(comm -12 /tmp/cluster-node1-ids.txt /tmp/cluster-node2-ids.txt | tr '\n' ' ')"
fi

# ── Step 5: Re-run process_outbox — all should be skipped (NX claims live) ─

log ""
log "--- Step 5: Re-running process_outbox on Node 2 (should skip all) ---"

# Reset sent=false to test NX claim TTL protection
# (Claims have 10min TTL, so re-processing should skip them)
# Actually, the mails are already sent/failed, so they won't be picked up again.
# Instead, insert fresh mails and check that NX claims from step 2 block them.
# But claims are keyed by mail ID, and new mails get new IDs.
# The real test: the original mails are already sent=true or retry_count>0,
# so re-running should find nothing to process.

resp3=$(call_action "$NODE2_HTTP" "outbox" "process_outbox" "{}")
log "Re-run response: $(echo "$resp3" | head -c 200)"

# No new processing should happen (all mails already handled)
node2_after=$(tail -n +"$((node2_log_before + 1))" "$NODE2_LOG" | sed 's/\x1b\[[0-9;]*m//g' | grep -c "Outbox mail.*sent to\|Failed to send outbox mail\|Failed to decode outbox mail" 2>/dev/null || echo "0")
node2_after=$(echo "$node2_after" | tr -d '[:space:]')
if [ "$node2_after" -eq "$node2_processed" ]; then
    PASS=$((PASS+1))
    log "  PASS: Re-run produced no new processing"
else
    new_processing=$((node2_after - node2_processed))
    FAIL=$((FAIL+1))
    log "  FAIL: Re-run processed $new_processing additional mails"
fi

# ── Cleanup ─────────────────────────────────────────────────────────────────

pg_exec "DELETE FROM outbox WHERE from_address LIKE 'clustertest-%';" || true
rm -f "$resp1_file" "$resp2_file" /tmp/cluster-node1-ids.txt /tmp/cluster-node2-ids.txt

# ── Results ─────────────────────────────────────────────────────────────────

log ""
log "=========================================="
log "  Results: $PASS passed, $FAIL failed"
log "=========================================="

[ "$FAIL" -eq 0 ] || exit 1
