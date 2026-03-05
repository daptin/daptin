#!/bin/bash
# Black-Box SMTP/IMAP Test Suite for Daptin
#
# Runs established external test tools against Daptin's mail infrastructure:
#   SMTP: smtp-source (Postfix) for protocol transaction testing
#   IMAP: dovecot ImapTest for RFC 3501 compliance + stress testing
#
# Usage:
#   ./mail-blackbox-test.sh              # Full run (bootstrap + SMTP + IMAP)
#   ./mail-blackbox-test.sh smtp         # SMTP tests only
#   ./mail-blackbox-test.sh imap         # IMAP tests only
#   ./mail-blackbox-test.sh --no-bootstrap  # Skip bootstrap, assume Daptin running
#
# Prerequisites:
#   - docker (for imaptest)
#   - swaks (brew install swaks) — used only for bootstrap mail delivery
#   - /usr/libexec/postfix/smtp-source (ships with macOS)

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DAPTIN_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
DAPTIN_HOST="${DAPTIN_HOST:-http://localhost:6336}"
TOKEN_FILE="/tmp/daptin-token.txt"
SMTP_LOG="/tmp/mail-blackbox-smtp.log"
IMAP_LOG="/tmp/mail-blackbox-imap.log"
DB_FILE="$DAPTIN_DIR/daptin.db"

SMTP_PORT=2525
IMAP_PORT=993  # implicit TLS — required by imaptest (no STARTTLS support)
MAIL_USER="testuser@localhost"
MAIL_PASS='TestPass1234'

SMTP_SOURCE="/usr/libexec/postfix/smtp-source"
IMAPTEST_IMAGE="daptin-imaptest"

NO_BOOTSTRAP=false
RUN_SMTP=true
RUN_IMAP=true

# ── Parse arguments ──────────────────────────────────────────────────────────

for arg in "$@"; do
    case "$arg" in
        smtp)
            RUN_IMAP=false
            ;;
        imap)
            RUN_SMTP=false
            ;;
        --no-bootstrap)
            NO_BOOTSTRAP=true
            ;;
    esac
done

# ── Helpers ──────────────────────────────────────────────────────────────────

log()  { echo "[$(date +%H:%M:%S)] $*"; }

read_token() {
    if [ -f "$TOKEN_FILE" ]; then
        cat "$TOKEN_FILE"
    else
        echo ""
    fi
}

api_post_jsonapi() {
    local path="$1" body="$2"
    curl -s --max-time 10 \
        -X POST "$DAPTIN_HOST$path" \
        -H "Authorization: Bearer $(read_token)" \
        -H "Content-Type: application/vnd.api+json" \
        -d "$body"
}

api_get() {
    curl -s --max-time 10 \
        "$DAPTIN_HOST$1" \
        -H "Authorization: Bearer $(read_token)"
}

config_set() {
    local key="$1" value="$2"
    curl -s --max-time 10 \
        -X PUT "$DAPTIN_HOST/_config/backend/$key" \
        -H "Authorization: Bearer $(read_token)" \
        -H "Content-Type: text/plain" \
        -d "$value"
}

wait_for_port() {
    local port="$1" label="$2" max="${3:-30}"
    log "Waiting for $label on port $port..."
    for i in $(seq 1 "$max"); do
        if nc -z 127.0.0.1 "$port" 2>/dev/null; then
            log "$label is listening on port $port"
            return 0
        fi
        sleep 1
    done
    log "TIMEOUT: $label not listening on port $port after ${max}s"
    return 1
}

# ── Step 1: Prerequisites ────────────────────────────────────────────────────

check_prerequisites() {
    log "Checking prerequisites..."

    if [ ! -x "$SMTP_SOURCE" ]; then
        echo "ERROR: smtp-source not found at $SMTP_SOURCE"
        echo "  macOS ships with it at /usr/libexec/postfix/smtp-source"
        exit 1
    fi
    log "smtp-source: $SMTP_SOURCE"

    if ! command -v docker &>/dev/null; then
        echo "ERROR: docker not found. Required for imaptest."
        exit 1
    fi
    log "docker: $(docker --version 2>&1)"

    # Build imaptest image if not present
    if ! docker image inspect "$IMAPTEST_IMAGE" &>/dev/null; then
        log "Building $IMAPTEST_IMAGE Docker image..."
        docker build --platform linux/amd64 -t "$IMAPTEST_IMAGE" \
            -f "$SCRIPT_DIR/imaptest.Dockerfile" "$SCRIPT_DIR" 2>&1 || {
            echo "ERROR: Failed to build imaptest Docker image"
            exit 1
        }
    fi
    log "imaptest Docker image: $IMAPTEST_IMAGE"

    if ! command -v swaks &>/dev/null; then
        echo "ERROR: swaks not found. Install with: brew install swaks"
        exit 1
    fi

    if ! command -v jq &>/dev/null; then
        echo "ERROR: jq not found. Install with: brew install jq"
        exit 1
    fi

    if ! command -v sqlite3 &>/dev/null; then
        echo "ERROR: sqlite3 not found"
        exit 1
    fi
}

# ── Step 2: Bootstrap Daptin ─────────────────────────────────────────────────

bootstrap_daptin() {
    log "=== Bootstrapping Daptin ==="

    "$SCRIPT_DIR/test-runner.sh" stop
    rm -f "$DB_FILE"
    sleep 1

    "$SCRIPT_DIR/test-runner.sh" start
    sleep 2

    log "Creating admin user..."
    curl -s --max-time 10 \
        -X POST "$DAPTIN_HOST/action/user_account/signup" \
        -H "Content-Type: application/json" \
        -d '{"attributes":{"name":"Admin","email":"admin@admin.com","password":"adminadmin","passwordConfirm":"adminadmin"}}' \
        > /dev/null

    log "Getting auth token..."
    "$SCRIPT_DIR/test-runner.sh" token > /dev/null
    local token
    token=$(read_token)
    if [ -z "$token" ]; then
        echo "ERROR: Failed to get auth token"
        exit 1
    fi
    log "Token acquired"

    # become_an_administrator required for config API PUT access
    log "Becoming administrator..."
    curl -s --max-time 10 \
        -X POST "$DAPTIN_HOST/action/world/become_an_administrator" \
        -H "Authorization: Bearer $token" \
        -H "Content-Type: application/json" \
        -d '{"attributes":{}}' > /dev/null

    sleep 10
    for i in $(seq 1 30); do
        if curl -s --max-time 2 "$DAPTIN_HOST/api/world" > /dev/null 2>&1; then
            log "Server ready after become_an_administrator"
            break
        fi
        sleep 1
    done
}

# ── Step 3: Create mail infrastructure ───────────────────────────────────────

create_mail_infra() {
    log "=== Creating mail infrastructure ==="

    log "Creating mail_server..."
    local ms_body
    ms_body=$(jq -n --arg iface "0.0.0.0:$SMTP_PORT" \
        '{data:{type:"mail_server",attributes:{hostname:"localhost",is_enabled:true,listen_interface:$iface,max_size:1048576,max_clients:100,xclient_on:false,always_on_tls:false,authentication_required:false}}}')
    local ms_resp
    ms_resp=$(api_post_jsonapi "/api/mail_server" "$ms_body")
    local ms_id
    ms_id=$(echo "$ms_resp" | jq -r '.data.id // empty')
    if [ -z "$ms_id" ]; then
        echo "ERROR: Failed to create mail_server"
        echo "$ms_resp" | jq . 2>/dev/null || echo "$ms_resp"
        exit 1
    fi
    log "mail_server created: $ms_id"

    log "Creating mail_account..."
    local user_id
    user_id=$(api_get "/api/user_account" | jq -r '.data[0].id // empty')
    if [ -z "$user_id" ]; then
        echo "ERROR: Could not find user_account"
        exit 1
    fi

    local ma_body
    ma_body=$(jq -n \
        --arg user "$MAIL_USER" \
        --arg pass "$MAIL_PASS" \
        --arg msid "$ms_id" \
        --arg uid "$user_id" \
        '{data:{type:"mail_account",attributes:{username:$user,password:$pass,password_md5:$pass},relationships:{mail_server_id:{data:{type:"mail_server",id:$msid}},user_account_id:{data:{type:"user_account",id:$uid}}}}}')
    local ma_resp
    ma_resp=$(api_post_jsonapi "/api/mail_account" "$ma_body")
    local ma_id
    ma_id=$(echo "$ma_resp" | jq -r '.data.id // empty')
    if [ -z "$ma_id" ]; then
        echo "ERROR: Failed to create mail_account"
        echo "$ma_resp" | jq . 2>/dev/null || echo "$ma_resp"
        exit 1
    fi
    log "mail_account created: $ma_id"

    # Enable IMAP on port 993 (implicit TLS, required by imaptest)
    log "Enabling IMAP on port $IMAP_PORT (implicit TLS)..."
    config_set "imap.enabled" "true"
    config_set "imap.listen_interface" ":$IMAP_PORT"

    log "Restarting Daptin to activate mail listeners..."
    "$SCRIPT_DIR/test-runner.sh" stop
    sleep 2
    "$SCRIPT_DIR/test-runner.sh" start
    sleep 3

    wait_for_port "$SMTP_PORT" "SMTP" 30 || { echo "ERROR: SMTP not listening"; exit 1; }

    if $RUN_IMAP; then
        wait_for_port "$IMAP_PORT" "IMAP" 30 || log "WARNING: IMAP not listening"
    fi
}

# ── Step 4: SMTP Tests (smtp-source) ────────────────────────────────────────

run_smtp_tests() {
    log ""
    log "=========================================="
    log "  SMTP Tests (Postfix smtp-source)"
    log "=========================================="
    > "$SMTP_LOG"

    # ── Test 1: Basic protocol compliance (50 messages, 5 sessions) ──────
    log ""
    log "--- Test 1: Protocol compliance (50 msgs, 5 concurrent sessions) ---"

    local mail_before
    mail_before=$(sqlite3 "$DB_FILE" "SELECT count(*) FROM mail;" 2>/dev/null || echo "0")

    local smtp_exit=0
    $SMTP_SOURCE -s 5 -m 50 -l 512 \
        -f sender@test.com -t "$MAIL_USER" \
        127.0.0.1:$SMTP_PORT >> "$SMTP_LOG" 2>&1 || smtp_exit=$?

    local mail_after
    mail_after=$(sqlite3 "$DB_FILE" "SELECT count(*) FROM mail;" 2>/dev/null || echo "0")
    local delivered=$((mail_after - mail_before))

    if [ "$smtp_exit" -eq 0 ]; then
        log "  EXIT: 0 (success)"
    else
        log "  EXIT: $smtp_exit (PROTOCOL ERROR)"
    fi
    log "  Delivered: $delivered / 50 messages"
    echo "" >> "$SMTP_LOG"

    # ── Test 2: High concurrency (10 sessions, 100 messages) ─────────────
    log ""
    log "--- Test 2: High concurrency (100 msgs, 10 concurrent sessions) ---"

    mail_before=$(sqlite3 "$DB_FILE" "SELECT count(*) FROM mail;" 2>/dev/null || echo "0")

    smtp_exit=0
    $SMTP_SOURCE -s 10 -m 100 -l 1024 \
        -f loadtest@test.com -t "$MAIL_USER" \
        127.0.0.1:$SMTP_PORT >> "$SMTP_LOG" 2>&1 || smtp_exit=$?

    mail_after=$(sqlite3 "$DB_FILE" "SELECT count(*) FROM mail;" 2>/dev/null || echo "0")
    delivered=$((mail_after - mail_before))

    if [ "$smtp_exit" -eq 0 ]; then
        log "  EXIT: 0 (success)"
    else
        log "  EXIT: $smtp_exit (PROTOCOL ERROR)"
    fi
    log "  Delivered: $delivered / 100 messages"
    echo "" >> "$SMTP_LOG"

    # ── Test 3: Large messages ───────────────────────────────────────────
    log ""
    log "--- Test 3: Large messages (10 msgs at 500KB, limit 1MB) ---"

    mail_before=$(sqlite3 "$DB_FILE" "SELECT count(*) FROM mail;" 2>/dev/null || echo "0")

    smtp_exit=0
    $SMTP_SOURCE -s 2 -m 10 -l 512000 \
        -f bigmail@test.com -t "$MAIL_USER" \
        127.0.0.1:$SMTP_PORT >> "$SMTP_LOG" 2>&1 || smtp_exit=$?

    mail_after=$(sqlite3 "$DB_FILE" "SELECT count(*) FROM mail;" 2>/dev/null || echo "0")
    delivered=$((mail_after - mail_before))

    if [ "$smtp_exit" -eq 0 ]; then
        log "  EXIT: 0 (success)"
    else
        log "  EXIT: $smtp_exit (PROTOCOL ERROR)"
    fi
    log "  Delivered: $delivered / 10 messages"
    echo "" >> "$SMTP_LOG"

    # ── Test 4: Oversized messages ───────────────────────────────────────
    log ""
    log "--- Test 4: Oversized messages (5 msgs at 1.5MB, limit 1MB) ---"

    smtp_exit=0
    $SMTP_SOURCE -s 1 -m 5 -l 1572864 \
        -f oversize@test.com -t "$MAIL_USER" \
        127.0.0.1:$SMTP_PORT >> "$SMTP_LOG" 2>&1 || smtp_exit=$?

    if [ "$smtp_exit" -ne 0 ]; then
        log "  EXIT: $smtp_exit (rejected as expected)"
    else
        log "  EXIT: 0 (WARNING: oversized messages accepted)"
    fi

    # ── Test 5: STARTTLS + AUTH via openssl ──────────────────────────────
    log ""
    log "--- Test 5: STARTTLS negotiation (openssl s_client) ---"

    local tls_out
    tls_out=$(echo "QUIT" | openssl s_client -connect 127.0.0.1:$SMTP_PORT -starttls smtp -verify_quiet 2>&1)
    echo "$tls_out" >> "$SMTP_LOG"
    if echo "$tls_out" | grep -qi "SSL-Session\|Protocol.*TLS"; then
        log "  STARTTLS: negotiated successfully"
        log "  $(echo "$tls_out" | grep "Protocol  :" | head -1 | xargs)"
        log "  $(echo "$tls_out" | grep "Cipher    :" | head -1 | xargs)"
    else
        log "  STARTTLS: FAILED"
    fi

    # ── Test 6: AUTH LOGIN via swaks ─────────────────────────────────────
    log ""
    log "--- Test 6: AUTH LOGIN (swaks) ---"

    local auth_out
    auth_out=$(swaks --server 127.0.0.1:$SMTP_PORT \
        --auth LOGIN \
        --auth-user "$MAIL_USER" \
        --auth-password "$MAIL_PASS" \
        -tlso \
        --quit-after AUTH 2>&1)
    echo "$auth_out" >> "$SMTP_LOG"
    if echo "$auth_out" | grep -q "Authentication succeeded"; then
        log "  AUTH LOGIN: succeeded"
    else
        log "  AUTH LOGIN: FAILED"
    fi

    local bad_out
    bad_out=$(swaks --server 127.0.0.1:$SMTP_PORT \
        --auth LOGIN \
        --auth-user "$MAIL_USER" \
        --auth-password "wrongpassword" \
        -tlso \
        --quit-after AUTH 2>&1)
    echo "$bad_out" >> "$SMTP_LOG"
    if echo "$bad_out" | grep -qi "failed\|535\|error"; then
        log "  AUTH LOGIN bad creds: correctly rejected"
    else
        log "  AUTH LOGIN bad creds: WARNING — not rejected"
    fi

    # ── Summary ──────────────────────────────────────────────────────────
    local total_mail
    total_mail=$(sqlite3 "$DB_FILE" "SELECT count(*) FROM mail;" 2>/dev/null || echo "0")
    log ""
    log "--- SMTP Summary ---"
    log "  Total messages in DB: $total_mail"
    log "  Full log: $SMTP_LOG"
}

# ── Step 5: IMAP Tests (ImapTest) ───────────────────────────────────────────

run_imap_tests() {
    log ""
    log "=========================================="
    log "  IMAP Tests (dovecot ImapTest)"
    log "=========================================="
    > "$IMAP_LOG"

    if ! nc -z 127.0.0.1 "$IMAP_PORT" 2>/dev/null; then
        log "IMAP port $IMAP_PORT not listening, skipping IMAP tests"
        return 1
    fi

    # Helper: run imaptest in Docker with proper timeout and cleanup
    run_imaptest() {
        local name="$1" logfile="$2" max_secs="$3"
        shift 3
        # Remove any leftover container with this name
        docker rm -f "$name" 2>/dev/null || true

        docker run --name "$name" --platform linux/amd64 \
            --add-host=host.docker.internal:host-gateway \
            "$IMAPTEST_IMAGE" \
            host=host.docker.internal port=$IMAP_PORT \
            user="$MAIL_USER" pass="$MAIL_PASS" \
            ssl=any-cert \
            mbox=/default.mbox \
            "$@" \
            > "$logfile" 2>&1 &
        local docker_pid=$!

        # Wait up to max_secs
        local elapsed=0
        while kill -0 "$docker_pid" 2>/dev/null && [ "$elapsed" -lt "$max_secs" ]; do
            sleep 1
            elapsed=$((elapsed + 1))
        done

        # Kill container if still running
        if kill -0 "$docker_pid" 2>/dev/null; then
            log "  Timeout after ${max_secs}s, stopping container..."
            docker stop -t 2 "$name" 2>/dev/null || true
            wait "$docker_pid" 2>/dev/null || true
        else
            wait "$docker_pid" 2>/dev/null || true
        fi

        docker rm -f "$name" 2>/dev/null || true

        # Show output
        cat "$logfile"
    }

    # ── Test 1: Scripted compliance suite (35 test groups, ~400 commands) ─
    log ""
    log "--- Test 1: RFC 3501 Compliance Suite (35 test groups) ---"
    log "  Running dovecot ImapTest scripted tests..."

    run_imaptest "imaptest-compliance" "$IMAP_LOG" 120 test=/tests

    log ""

    # Parse compliance results
    if grep -q "test groups:" "$IMAP_LOG"; then
        local compliance_line
        compliance_line=$(grep "test groups:" "$IMAP_LOG")
        log "  $compliance_line"

        local base_line
        base_line=$(grep "base protocol:" "$IMAP_LOG" || true)
        [ -n "$base_line" ] && log "  $base_line"

        local ext_line
        ext_line=$(grep "extensions:" "$IMAP_LOG" || true)
        [ -n "$ext_line" ] && log "  $ext_line"
    else
        log "  Compliance suite did not produce a summary (may have crashed)"
        local fail_count
        fail_count=$(grep -c "^\*\*\* Test .* failed" "$IMAP_LOG" 2>/dev/null || echo "0")
        log "  Individual test failures found in log: $fail_count"
        if [ "$fail_count" -gt 0 ]; then
            grep "^\*\*\* Test .* failed\|^ - Command:" "$IMAP_LOG" | head -20 | while read -r line; do
                log "    $line"
            done
        fi
        local panic_in_compliance
        panic_in_compliance=$(grep -c "^Panic:" "$IMAP_LOG" 2>/dev/null || echo "0")
        if [ "$panic_in_compliance" -gt 0 ]; then
            log "  ImapTest panicked (internal assertion failure from unexpected server responses)"
        fi
    fi

    # ── Test 2: Stress test (5 clients, 30 seconds) ─────────────────────
    log ""
    log "--- Test 2: Stress Test (5 clients, 30 seconds) ---"
    log "  Running dovecot ImapTest stress mode..."

    local stress_log="/tmp/mail-blackbox-imap-stress.log"
    run_imaptest "imaptest-stress" "$stress_log" 45 clients=5 secs=30

    log ""

    # Parse stress results
    local error_count
    error_count=$(grep -c "^Error:" "$stress_log" 2>/dev/null || echo "0")

    local panic_count
    panic_count=$(grep -c "^Panic:" "$stress_log" 2>/dev/null || echo "0")

    log "  Errors: $error_count"
    log "  Panics: $panic_count"

    # Extract totals
    if grep -q "^Totals:" "$stress_log"; then
        local totals
        totals=$(grep "^Totals:" -A2 "$stress_log" | tail -1)
        log "  Totals: $totals"
    fi

    # Categorize errors
    if [ "$error_count" -gt 0 ]; then
        log ""
        log "  Error categories:"
        grep "^Error:" "$stress_log" | sed 's/Error: [^:]*: //' | sort | uniq -c | sort -rn | head -10 | while read -r line; do
            log "    $line"
        done
    fi

    log "  Stress log: $stress_log"

    # ── Test 3: Higher concurrency (10 clients, 30 seconds) ─────────────
    log ""
    log "--- Test 3: High Concurrency (10 clients, 30 seconds) ---"
    log "  Running dovecot ImapTest stress mode..."

    local hc_log="/tmp/mail-blackbox-imap-hc.log"
    run_imaptest "imaptest-hc" "$hc_log" 45 clients=10 secs=30

    log ""

    error_count=$(grep -c "^Error:" "$hc_log" 2>/dev/null || echo "0")
    panic_count=$(grep -c "^Panic:" "$hc_log" 2>/dev/null || echo "0")

    log "  Errors: $error_count"
    log "  Panics: $panic_count"

    if [ "$error_count" -gt 0 ]; then
        log ""
        log "  Error categories:"
        grep "^Error:" "$hc_log" | sed 's/Error: [^:]*: //' | sort | uniq -c | sort -rn | head -10 | while read -r line; do
            log "    $line"
        done
    fi

    log "  High-concurrency log: $hc_log"

    # ── IMAP Summary ────────────────────────────────────────────────────
    log ""
    log "--- IMAP Summary ---"
    log "  Compliance log: $IMAP_LOG"
    log "  Stress log: $stress_log"
    log "  High-concurrency log: $hc_log"
}

# ── Results ──────────────────────────────────────────────────────────────────

report_results() {
    log ""
    log "=========================================="
    log "  Test Run Complete"
    log "=========================================="
    log "  SMTP log: $SMTP_LOG"
    log "  IMAP log: $IMAP_LOG"
    log ""
    log "Review the logs above for pass/fail details."
    log "ImapTest compliance failures indicate RFC 3501 divergences."
    log "ImapTest stress errors indicate runtime issues under load."
}

# ── Main ─────────────────────────────────────────────────────────────────────

cleanup() {
    docker rm -f imaptest-compliance imaptest-stress imaptest-hc 2>/dev/null || true
}
trap cleanup EXIT

check_prerequisites

if ! $NO_BOOTSTRAP; then
    bootstrap_daptin
    create_mail_infra
fi

if $RUN_SMTP; then
    run_smtp_tests
fi

if $RUN_IMAP; then
    run_imap_tests
fi

report_results
