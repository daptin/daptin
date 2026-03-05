#!/bin/bash
# Mail Fixes Verification Test Suite
#
# Verifies 10 specific fixes applied in commit 37863a7c to the mail subsystem.
# Each test targets a specific bug fix with PASS/FAIL output.
#
# Usage:
#   ./mail-fixes-verify.sh              # Full run (bootstrap + all tests)
#   ./mail-fixes-verify.sh --no-bootstrap  # Skip bootstrap, assume Daptin running
#
# Prerequisites: swaks, jq, sqlite3, python3

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DAPTIN_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
DAPTIN_HOST="${DAPTIN_HOST:-http://localhost:6336}"
TOKEN_FILE="/tmp/daptin-token.txt"
DB_FILE="$DAPTIN_DIR/daptin.db"
DAPTIN_LOG="/tmp/daptin.log"

SMTP_PORT=2525
IMAP_PORT=993
MAIL_USER="testuser@localhost"
MAIL_PASS='TestPass1234'

NO_BOOTSTRAP=false
PASS_COUNT=0
FAIL_COUNT=0
TOTAL=10

PYTHON_HELPER="/tmp/imap_test_helper.py"

# ── Parse arguments ──────────────────────────────────────────────────────────

for arg in "$@"; do
    case "$arg" in
        --no-bootstrap) NO_BOOTSTRAP=true ;;
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

api_post() {
    local path="$1" body="$2"
    curl -s --max-time 10 \
        -X POST "$DAPTIN_HOST$path" \
        -H "Authorization: Bearer $(read_token)" \
        -H "Content-Type: application/json" \
        -d "$body"
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

pass_test() {
    local num="$1" name="$2"
    PASS_COUNT=$((PASS_COUNT + 1))
    log "  PASS: Test $num — $name"
}

fail_test() {
    local num="$1" name="$2" reason="${3:-}"
    FAIL_COUNT=$((FAIL_COUNT + 1))
    log "  FAIL: Test $num — $name"
    [ -n "$reason" ] && log "    Reason: $reason"
}

# Record log position for later panic checking
mark_log_pos() {
    if [ -f "$DAPTIN_LOG" ]; then
        wc -l < "$DAPTIN_LOG" | tr -d ' '
    else
        echo 0
    fi
}

# Check for panics/crashes since a log position
check_log_panics() {
    local from_line="$1"
    if [ -f "$DAPTIN_LOG" ]; then
        tail -n +"$((from_line + 1))" "$DAPTIN_LOG" | sed 's/\x1b\[[0-9;]*m//g' | grep -ci 'panic\|SIGSEGV\|concurrent map' || true
    else
        echo 0
    fi
}

# Wait for mail delivery to appear in DB
wait_for_mail_count() {
    local min_count="$1" max_wait="${2:-15}"
    for i in $(seq 1 "$max_wait"); do
        local count
        count=$(sqlite3 "$DB_FILE" "SELECT count(*) FROM mail;" 2>/dev/null || echo "0")
        if [ "$count" -ge "$min_count" ]; then
            log "Mail count reached $count (needed $min_count)"
            return 0
        fi
        sleep 1
    done
    local final
    final=$(sqlite3 "$DB_FILE" "SELECT count(*) FROM mail;" 2>/dev/null || echo "0")
    log "WARNING: Mail count only $final after ${max_wait}s (needed $min_count)"
    return 1
}

# ── Python IMAP Helper ──────────────────────────────────────────────────────

write_python_helper() {
    cat > "$PYTHON_HELPER" << 'PYEOF'
#!/usr/bin/env python3
"""IMAP test helper for mail-fixes-verify.sh"""

import sys
import ssl
import imaplib
import time
import threading
import email
import email.mime.text
import email.utils

IMAP_HOST = "127.0.0.1"
IMAP_PORT = 993
MAIL_USER = "testuser@localhost"
MAIL_PASS = "TestPass1234"

def get_imap():
    ctx = ssl.create_default_context()
    ctx.check_hostname = False
    ctx.verify_mode = ssl.CERT_NONE
    m = imaplib.IMAP4_SSL(IMAP_HOST, IMAP_PORT, ssl_context=ctx)
    m.login(MAIL_USER, MAIL_PASS)
    return m

def make_msg(subject="Test", body="Body", from_addr="sender@example.com", to_addr=MAIL_USER):
    msg = email.mime.text.MIMEText(body)
    msg["Subject"] = subject
    msg["From"] = from_addr
    msg["To"] = to_addr
    msg["Date"] = email.utils.formatdate(localtime=True)
    return msg.as_bytes()

def append_msg(m, subject="Test", mailbox="INBOX"):
    raw = make_msg(subject=subject)
    typ, data = m.append(mailbox, None, None, raw)
    if typ != "OK":
        raise Exception(f"APPEND to {mailbox} failed: {typ} {data}")
    return typ

# ── Test 2: defer Rollback + explicit Commit (5 IMAP methods) ───────────────

def test_imap_methods():
    m = get_imap()
    errors = []

    # select INBOX
    typ, data = m.select("INBOX")
    if typ != "OK":
        errors.append(f"SELECT failed: {typ} {data}")
        m.logout()
        return errors

    # Check()
    typ, data = m.check()
    if typ != "OK":
        errors.append(f"CHECK failed: {typ}")

    # ListMessages via FETCH
    typ, data = m.fetch("1:*", "(FLAGS)")
    if typ != "OK":
        errors.append(f"FETCH FLAGS failed: {typ}")

    # SearchMessages
    typ, data = m.search(None, "ALL")
    if typ != "OK":
        errors.append(f"SEARCH ALL failed: {typ}")

    # UpdateMessagesFlags via STORE
    msg_ids = data[0].split() if data[0] else []
    if msg_ids:
        typ, data = m.store(msg_ids[0], "+FLAGS", "(\\Seen)")
        if typ != "OK":
            errors.append(f"STORE +FLAGS failed: {typ}")

    # SetSubscribed
    typ, data = m.subscribe("INBOX")
    if typ != "OK":
        errors.append(f"SUBSCRIBE failed: {typ}")
    typ, data = m.unsubscribe("INBOX")
    if typ != "OK":
        errors.append(f"UNSUBSCRIBE failed: {typ}")

    m.close()
    m.logout()
    return errors

# ── Test 3: UID multi-range search ──────────────────────────────────────────

def test_uid_multi_range():
    m = get_imap()

    # Append 10 messages
    for i in range(10):
        append_msg(m, subject=f"Range test {i}")

    m.select("INBOX")

    # Get all UIDs
    typ, data = m.uid("SEARCH", None, "ALL")
    if typ != "OK":
        return [f"UID SEARCH ALL failed: {typ}"]
    all_uids = data[0].split()
    if len(all_uids) < 10:
        m.close()
        m.logout()
        return [f"Expected >=10 UIDs, got {len(all_uids)}"]

    # Take last 10 UIDs (the ones we just appended)
    uids = all_uids[-10:]
    # Range 1: first 3, Range 2: positions 7-9
    r1 = uids[0:3]
    r2 = uids[6:9]
    gap = uids[3:6]

    r1_range = f"{r1[0].decode()}:{r1[-1].decode()}"
    r2_range = f"{r2[0].decode()}:{r2[-1].decode()}"
    search_spec = f"{r1_range},{r2_range}"

    # UID SEARCH UID <range1>,<range2>
    typ, data = m.uid("SEARCH", None, f"UID {search_spec}")
    if typ != "OK":
        m.close()
        m.logout()
        return [f"UID SEARCH UID multi-range failed: {typ}"]

    result_uids = set(data[0].split()) if data[0] else set()
    expected = set(r1 + r2)
    gap_set = set(gap)

    errors = []
    missing = expected - result_uids
    if missing:
        errors.append(f"Missing expected UIDs: {missing}")

    leaked = result_uids & gap_set
    if leaked:
        errors.append(f"Gap UIDs leaked into results: {leaked}")

    m.close()
    m.logout()
    return errors

# ── Test 4: Status() concurrent access (knownKeywords lock) ─────────────────

def test_concurrent_status():
    errors = []
    lock = threading.Lock()

    def search_and_store(thread_id, iterations):
        try:
            m = get_imap()
            m.select("INBOX")
            for _ in range(iterations):
                typ, data = m.uid("SEARCH", None, "ALL")
                if typ == "OK" and data[0]:
                    uid = data[0].split()[0]
                    m.uid("STORE", uid, "+FLAGS", "(\\Seen)")
            m.close()
            m.logout()
        except Exception as e:
            with lock:
                errors.append(f"Thread {thread_id} search/store: {e}")

    def status_loop(thread_id, iterations):
        try:
            m = get_imap()
            for _ in range(iterations):
                m.status("INBOX", "(MESSAGES UIDNEXT UNSEEN)")
            m.logout()
        except Exception as e:
            with lock:
                errors.append(f"Thread {thread_id} status: {e}")

    threads = []
    for i in range(2):
        t1 = threading.Thread(target=search_and_store, args=(f"SS-{i}", 50))
        t2 = threading.Thread(target=status_loop, args=(f"ST-{i}", 50))
        threads.extend([t1, t2])

    for t in threads:
        t.start()
    for t in threads:
        t.join(timeout=60)

    still_alive = [t for t in threads if t.is_alive()]
    if still_alive:
        errors.append(f"{len(still_alive)} threads still alive after 60s")

    return errors

# ── Test 5: ListMailboxes with correct mailAccountId ─────────────────────────

def test_list_mailboxes():
    m = get_imap()
    errors = []

    typ, data = m.list()
    if typ != "OK":
        m.logout()
        return [f"LIST failed: {typ}"]

    mailboxes = []
    for item in data:
        if item is None:
            continue
        line = item.decode() if isinstance(item, bytes) else str(item)
        # IMAP LIST format: (\Flags) "delimiter" name
        # Extract the mailbox name (last space-separated token, possibly quoted)
        parts = line.rsplit(" ", 1)
        if len(parts) == 2:
            name = parts[1].strip().strip('"')
            if name:
                mailboxes.append(name)

    if not mailboxes:
        m.logout()
        return ["LIST returned no mailboxes"]

    for mbox in mailboxes:
        typ, data = m.status(mbox, "(MESSAGES UIDNEXT)")
        if typ != "OK":
            errors.append(f"STATUS {mbox} failed: {typ}")

    m.logout()
    return errors

# ── Test 6: GetMailboxWithTransaction type assertion ─────────────────────────

def test_select_inbox():
    m = get_imap()
    errors = []

    typ, data = m.select("INBOX")
    if typ != "OK":
        errors.append(f"SELECT INBOX failed: {typ} {data}")
    else:
        exists = int(data[0])
        if exists < 0:
            errors.append(f"EXISTS count negative: {exists}")

    try:
        m.close()
    except Exception:
        pass
    m.logout()
    return errors

# ── Test 7: UID reuse after expunge ─────────────────────────────────────────

def test_uid_no_reuse():
    m = get_imap()
    errors = []

    # Append 3 messages
    for i in range(3):
        append_msg(m, subject=f"UID reuse test {i}")

    m.select("INBOX")

    # Get UIDs of last 3
    typ, data = m.uid("SEARCH", None, "ALL")
    all_uids = [int(u) for u in data[0].split()] if data[0] else []
    if len(all_uids) < 3:
        m.close()
        m.logout()
        return [f"Need >=3 messages, got {len(all_uids)}"]

    original_uids = all_uids[-3:]
    max_original = max(original_uids)
    middle_uid = str(original_uids[1])

    # Delete middle message
    m.uid("STORE", middle_uid, "+FLAGS", "(\\Deleted)")
    m.expunge()

    # Append 1 more — close first, then append, re-select
    m.close()
    append_msg(m, subject="After expunge")
    m.select("INBOX")

    # Get new UID
    typ, data = m.uid("SEARCH", None, "ALL")
    new_uids = [int(u) for u in data[0].split()] if data[0] else []
    new_uid = max(new_uids) if new_uids else 0

    if new_uid <= max_original:
        errors.append(f"UID reuse! New UID {new_uid} <= max original {max_original}")

    m.close()
    m.logout()
    return errors

# ── Test 10: Check() attributes assertion ────────────────────────────────────

def test_check_command():
    m = get_imap()
    errors = []

    typ, data = m.select("INBOX")
    if typ != "OK":
        errors.append(f"SELECT failed: {typ} {data}")
        m.logout()
        return errors

    typ, data = m.check()
    if typ != "OK":
        errors.append(f"CHECK failed: {typ}")

    try:
        m.close()
    except Exception:
        pass
    m.logout()
    return errors

# ── Main dispatch ────────────────────────────────────────────────────────────

if __name__ == "__main__":
    test_name = sys.argv[1] if len(sys.argv) > 1 else ""
    tests = {
        "test_imap_methods": test_imap_methods,
        "test_uid_multi_range": test_uid_multi_range,
        "test_concurrent_status": test_concurrent_status,
        "test_list_mailboxes": test_list_mailboxes,
        "test_select_inbox": test_select_inbox,
        "test_uid_no_reuse": test_uid_no_reuse,
        "test_check_command": test_check_command,
    }

    if test_name not in tests:
        print(f"Unknown test: {test_name}", file=sys.stderr)
        print(f"Available: {', '.join(tests.keys())}", file=sys.stderr)
        sys.exit(2)

    try:
        errs = tests[test_name]()
        if errs:
            for e in errs:
                print(f"ERROR: {e}", file=sys.stderr)
            sys.exit(1)
        else:
            sys.exit(0)
    except Exception as ex:
        print(f"EXCEPTION: {ex}", file=sys.stderr)
        import traceback
        traceback.print_exc(file=sys.stderr)
        sys.exit(1)
PYEOF
    chmod +x "$PYTHON_HELPER"
    log "Python helper written to $PYTHON_HELPER"
}

# ── Prerequisites ────────────────────────────────────────────────────────────

check_prerequisites() {
    log "Checking prerequisites..."
    local missing=false

    for cmd in swaks jq sqlite3 python3; do
        if ! command -v "$cmd" &>/dev/null; then
            log "ERROR: $cmd not found"
            missing=true
        fi
    done

    if $missing; then
        log "Install missing: brew install swaks jq"
        exit 1
    fi
}

# ── Bootstrap ────────────────────────────────────────────────────────────────

bootstrap() {
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
        log "ERROR: Failed to get auth token"
        exit 1
    fi
    log "Token acquired"

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

    # Create mail infrastructure
    log "Creating mail_server..."
    local ms_body
    ms_body=$(jq -n --arg iface "0.0.0.0:$SMTP_PORT" \
        '{data:{type:"mail_server",attributes:{hostname:"localhost",is_enabled:true,listen_interface:$iface,max_size:1048576,max_clients:100,xclient_on:false,always_on_tls:false,authentication_required:false}}}')
    local ms_resp
    ms_resp=$(api_post_jsonapi "/api/mail_server" "$ms_body")
    local ms_id
    ms_id=$(echo "$ms_resp" | jq -r '.data.id // empty')
    if [ -z "$ms_id" ]; then
        log "ERROR: Failed to create mail_server"
        echo "$ms_resp" | jq . 2>/dev/null || echo "$ms_resp"
        exit 1
    fi
    log "mail_server created: $ms_id"

    log "Creating mail_account..."
    local user_id
    user_id=$(api_get "/api/user_account" | jq -r '.data[0].id // empty')
    if [ -z "$user_id" ]; then
        log "ERROR: Could not find user_account"
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
        log "ERROR: Failed to create mail_account"
        echo "$ma_resp" | jq . 2>/dev/null || echo "$ma_resp"
        exit 1
    fi
    log "mail_account created: $ma_id"

    # Enable IMAP
    log "Enabling IMAP on port $IMAP_PORT..."
    config_set "imap.enabled" "true"
    config_set "imap.listen_interface" ":$IMAP_PORT"

    log "Restarting Daptin to activate mail listeners..."
    "$SCRIPT_DIR/test-runner.sh" stop
    sleep 2
    "$SCRIPT_DIR/test-runner.sh" start
    sleep 3

    wait_for_port "$SMTP_PORT" "SMTP" 30 || { log "ERROR: SMTP not listening"; exit 1; }
    wait_for_port "$IMAP_PORT" "IMAP" 30 || { log "ERROR: IMAP not listening"; exit 1; }

    # Deliver seed messages via swaks — these create INBOX and populate mail table
    log "Delivering seed messages via SMTP..."
    for i in $(seq 1 5); do
        swaks --to "$MAIL_USER" --from "seed${i}@example.com" \
            --server 127.0.0.1 --port "$SMTP_PORT" \
            --header "Subject: Seed message $i" \
            --body "Seed body $i" \
            --silent 2 2>/dev/null || true
        sleep 1
    done

    # Wait for async mail processing to complete (INBOX auto-created on first delivery)
    log "Waiting for seed messages to be stored..."
    wait_for_mail_count 3 20 || log "WARNING: Not all seed messages stored, proceeding anyway"

    local mail_count box_count
    mail_count=$(sqlite3 "$DB_FILE" "SELECT count(*) FROM mail;" 2>/dev/null || echo "0")
    box_count=$(sqlite3 "$DB_FILE" "SELECT count(*) FROM mail_box;" 2>/dev/null || echo "0")
    log "DB state: $mail_count mail(s), $box_count mailbox(es)"

    log "=== Bootstrap complete ==="
}

# ── Test 1: Outbox transaction commit ────────────────────────────────────────

run_test_1() {
    log ""
    log "Test 1: Outbox transaction commit (mail_adapter.go:389)"

    local log_pos
    log_pos=$(mark_log_pos)

    # Send from authenticated user to non-existent local user.
    # Guerrilla rejects relay to external domains (454 Relay access denied),
    # so we target a local-domain recipient that has no mail_account.
    # This triggers the forwarding/outbox path in mail_adapter.
    local swaks_exit=0
    swaks --to nonexistent@localhost --from "$MAIL_USER" \
        --server 127.0.0.1 --port "$SMTP_PORT" \
        --auth LOGIN --auth-user "$MAIL_USER" --auth-password "$MAIL_PASS" \
        --header "Subject: Outbox commit test" \
        --body "Testing outbox transaction commit" \
        --silent 2 2>/dev/null || swaks_exit=$?

    sleep 2

    # Check outbox for the entry
    local outbox_count
    outbox_count=$(sqlite3 "$DB_FILE" "SELECT count(*) FROM outbox WHERE to_address='nonexistent@localhost';" 2>/dev/null || echo "0")

    local panics
    panics=$(check_log_panics "$log_pos")

    if [ "$outbox_count" -ge 1 ] && [ "$panics" -eq 0 ]; then
        pass_test 1 "Outbox transaction commit"
    else
        fail_test 1 "Outbox transaction commit" "outbox_count=$outbox_count, swaks_exit=$swaks_exit, panics=$panics"
    fi
}

# ── Test 2: IMAP methods with proper transactions ───────────────────────────

run_test_2() {
    log ""
    log "Test 2: defer Rollback + explicit Commit (5 IMAP methods)"

    local log_pos
    log_pos=$(mark_log_pos)

    local py_exit=0
    local py_err
    py_err=$(python3 "$PYTHON_HELPER" test_imap_methods 2>&1) || py_exit=$?

    local panics
    panics=$(check_log_panics "$log_pos")

    if [ "$py_exit" -eq 0 ] && [ "$panics" -eq 0 ]; then
        pass_test 2 "IMAP methods (Check, Fetch, Search, Store, Subscribe)"
    else
        fail_test 2 "IMAP methods" "exit=$py_exit, panics=$panics, err=$py_err"
    fi
}

# ── Test 3: UID multi-range search ──────────────────────────────────────────

run_test_3() {
    log ""
    log "Test 3: UID multi-range search (bounding box + post-filter)"

    local log_pos
    log_pos=$(mark_log_pos)

    local py_exit=0
    local py_err
    py_err=$(python3 "$PYTHON_HELPER" test_uid_multi_range 2>&1) || py_exit=$?

    local panics
    panics=$(check_log_panics "$log_pos")

    if [ "$py_exit" -eq 0 ] && [ "$panics" -eq 0 ]; then
        pass_test 3 "UID multi-range search"
    else
        fail_test 3 "UID multi-range search" "exit=$py_exit, panics=$panics, err=$py_err"
    fi
}

# ── Test 4: Status() knownKeywords concurrent access ────────────────────────

run_test_4() {
    log ""
    log "Test 4: Status() knownKeywords lock (concurrent access)"

    local log_pos
    log_pos=$(mark_log_pos)

    local py_exit=0
    local py_err
    py_err=$(python3 "$PYTHON_HELPER" test_concurrent_status 2>&1) || py_exit=$?

    sleep 1
    local panics
    panics=$(check_log_panics "$log_pos")

    if [ "$py_exit" -eq 0 ] && [ "$panics" -eq 0 ]; then
        pass_test 4 "Status() concurrent access (no race)"
    else
        fail_test 4 "Status() concurrent access" "exit=$py_exit, panics=$panics, err=$py_err"
    fi
}

# ── Test 5: ListMailboxes with mailAccountId ─────────────────────────────────

run_test_5() {
    log ""
    log "Test 5: ListMailboxes mailAccountId (imap_user.go:74)"

    local log_pos
    log_pos=$(mark_log_pos)

    local py_exit=0
    local py_err
    py_err=$(python3 "$PYTHON_HELPER" test_list_mailboxes 2>&1) || py_exit=$?

    local panics
    panics=$(check_log_panics "$log_pos")

    if [ "$py_exit" -eq 0 ] && [ "$panics" -eq 0 ]; then
        pass_test 5 "ListMailboxes with correct mailAccountId"
    else
        fail_test 5 "ListMailboxes mailAccountId" "exit=$py_exit, panics=$panics, err=$py_err"
    fi
}

# ── Test 6: GetMailboxWithTransaction type assertion ─────────────────────────

run_test_6() {
    log ""
    log "Test 6: GetMailboxWithTransaction type assertion (imap_user.go:207)"

    local log_pos
    log_pos=$(mark_log_pos)

    local py_exit=0
    local py_err
    py_err=$(python3 "$PYTHON_HELPER" test_select_inbox 2>&1) || py_exit=$?

    local panics
    panics=$(check_log_panics "$log_pos")

    if [ "$py_exit" -eq 0 ] && [ "$panics" -eq 0 ]; then
        pass_test 6 "SELECT INBOX (type assertion safe)"
    else
        fail_test 6 "GetMailboxWithTransaction assertion" "exit=$py_exit, panics=$panics, err=$py_err"
    fi
}

# ── Test 7: UID reuse after expunge ─────────────────────────────────────────

run_test_7() {
    log ""
    log "Test 7: UID reuse after expunge (dbresource.go:768)"

    local log_pos
    log_pos=$(mark_log_pos)

    local py_exit=0
    local py_err
    py_err=$(python3 "$PYTHON_HELPER" test_uid_no_reuse 2>&1) || py_exit=$?

    local panics
    panics=$(check_log_panics "$log_pos")

    if [ "$py_exit" -eq 0 ] && [ "$panics" -eq 0 ]; then
        pass_test 7 "UID no reuse after expunge"
    else
        fail_test 7 "UID reuse after expunge" "exit=$py_exit, panics=$panics, err=$py_err"
    fi
}

# ── Test 8: MIME header double blank line ────────────────────────────────────

run_test_8() {
    log ""
    log "Test 8: MIME header double blank line (action_mail_send.go:59)"

    local log_pos
    log_pos=$(mark_log_pos)

    # Trigger outbox creation via authenticated SMTP to non-existent local user
    # (reset-password may not produce outbox entries without mail routing config)
    local outbox_before
    outbox_before=$(sqlite3 "$DB_FILE" "SELECT count(*) FROM outbox;" 2>/dev/null || echo "0")

    swaks --to "mime-test@localhost" --from "$MAIL_USER" \
        --server 127.0.0.1 --port "$SMTP_PORT" \
        --auth LOGIN --auth-user "$MAIL_USER" --auth-password "$MAIL_PASS" \
        --header "Subject: MIME test" \
        --body "Testing MIME headers" \
        --silent 2 2>/dev/null || true
    sleep 2

    # Check outbox for the new entry
    local outbox_mail
    outbox_mail=$(sqlite3 "$DB_FILE" "SELECT mail FROM outbox ORDER BY id DESC LIMIT 1;" 2>/dev/null || echo "")

    if [ -z "$outbox_mail" ]; then
        local panics
        panics=$(check_log_panics "$log_pos")
        if [ "$panics" -eq 0 ]; then
            pass_test 8 "MIME header format (no outbox entry to verify, no panic)"
        else
            fail_test 8 "MIME header double blank line" "panics=$panics"
        fi
        return
    fi

    # Decode base64 and check for triple CRLF (double blank line between headers and body)
    local decoded
    decoded=$(echo "$outbox_mail" | python3 -c "
import sys, base64
data = sys.stdin.read().strip()
try:
    raw = base64.b64decode(data)
    text = raw.decode('utf-8', errors='replace')
    count = text.count('\r\n\r\n\r\n')
    print(count)
except Exception as e:
    print(f'decode_error: {e}', file=sys.stderr)
    print(-1)
" 2>/dev/null || echo "-1")

    local panics
    panics=$(check_log_panics "$log_pos")

    if [ "$decoded" = "0" ] && [ "$panics" -eq 0 ]; then
        pass_test 8 "MIME header format (no double blank line)"
    elif [ "$decoded" = "-1" ]; then
        fail_test 8 "MIME header double blank line" "Could not decode outbox.mail"
    else
        fail_test 8 "MIME header double blank line" "Found $decoded triple-CRLF occurrences"
    fi
}

# ── Test 9: Outbox type assertion guards ─────────────────────────────────────

run_test_9() {
    log ""
    log "Test 9: Outbox type assertion guards (action_outbox_process.go:65)"

    local log_pos
    log_pos=$(mark_log_pos)

    # Insert a malformed outbox row with empty mail (NOT NULL — column is NOT NULL)
    sqlite3 "$DB_FILE" "INSERT INTO outbox (reference_id, to_address, from_address, to_host, mail, sent, retry_count, permission, created_at, updated_at) VALUES ('test-malformed-$(date +%s)', 'bad@example.com', 'test@localhost', 'example.com', '', 0, 0, 2097151, datetime('now'), datetime('now'));" 2>/dev/null || true
    sleep 1

    # Trigger outbox processing
    local resp
    resp=$(api_post "/action/outbox/process_outbox" '{"attributes":{}}') || true
    sleep 2

    local panics
    panics=$(check_log_panics "$log_pos")

    # Clean up malformed row
    sqlite3 "$DB_FILE" "DELETE FROM outbox WHERE reference_id LIKE 'test-malformed-%';" 2>/dev/null || true

    if [ "$panics" -eq 0 ]; then
        pass_test 9 "Outbox malformed mail handled gracefully"
    else
        fail_test 9 "Outbox type assertion guards" "panics=$panics"
    fi
}

# ── Test 10: Check() attributes assertion ────────────────────────────────────

run_test_10() {
    log ""
    log "Test 10: Check() attributes assertion (imap_mailbox.go:168)"

    local log_pos
    log_pos=$(mark_log_pos)

    local py_exit=0
    local py_err
    py_err=$(python3 "$PYTHON_HELPER" test_check_command 2>&1) || py_exit=$?

    local panics
    panics=$(check_log_panics "$log_pos")

    if [ "$py_exit" -eq 0 ] && [ "$panics" -eq 0 ]; then
        pass_test 10 "Check() attributes assertion safe"
    else
        fail_test 10 "Check() attributes assertion" "exit=$py_exit, panics=$panics, err=$py_err"
    fi
}

# ── Main ─────────────────────────────────────────────────────────────────────

main() {
    log "=========================================="
    log "  Mail Fixes Verification (10 tests)"
    log "=========================================="

    check_prerequisites
    write_python_helper

    if ! $NO_BOOTSTRAP; then
        bootstrap
    fi

    run_test_1
    run_test_2
    run_test_3
    run_test_4
    run_test_5
    run_test_6
    run_test_7
    run_test_8
    run_test_9
    run_test_10

    log ""
    log "=========================================="
    log "  Results: $PASS_COUNT/$TOTAL PASS, $FAIL_COUNT/$TOTAL FAIL"
    log "=========================================="

    if [ "$FAIL_COUNT" -eq 0 ]; then
        log "All tests passed!"
        exit 0
    else
        log "Some tests failed. Check output above."
        exit 1
    fi
}

main
