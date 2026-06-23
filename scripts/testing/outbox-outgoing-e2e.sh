#!/usr/bin/env bash
# E2E test for process_outbox outbound SMTP delivery.
#
# This is intentionally black-box at the Daptin boundary:
#   - runs Daptin in Docker with test DNS
#   - creates a real outbox row through the HTTP API
#   - calls /action/outbox/process_outbox over HTTP
#   - captures SMTP DATA on fake remote MX servers
#
# The first MX consumes DATA and returns a transient failure. The second MX must
# receive the full RFC822 message, including From:. This catches exhausted-reader
# bugs in outbound SMTP retry paths.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DAPTIN_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

TEST_NAME="daptin-outbox-e2e-$$"
NETWORK="${TEST_NAME}-net"
if [ -n "${OUTBOX_E2E_HTTP_PORT:-}" ]; then
    HTTP_PORT="$OUTBOX_E2E_HTTP_PORT"
else
    HTTP_PORT="$(python3 - <<'PY'
import socket
s = socket.socket()
s.bind(("127.0.0.1", 0))
print(s.getsockname()[1])
s.close()
PY
)"
fi

DOMAIN="outbox-e2e.test"
RECIPIENT="receiver@${DOMAIN}"
SENDER="login@sender.test"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/daptin-outbox-e2e.XXXXXX")"
CAPTURE_DIR="$TMP_DIR/capture"
DB_FILE="$TMP_DIR/src/daptin.db"
TOKEN_FILE="$TMP_DIR/token"

PASS=0
FAIL=0

log() { echo "[$(date +%H:%M:%S)] $*"; }
pass() { PASS=$((PASS + 1)); log "  PASS: $*"; }
fail() { FAIL=$((FAIL + 1)); log "  FAIL: $*"; }

cleanup() {
    docker rm -f "${TEST_NAME}-daptin" "${TEST_NAME}-dns" "${TEST_NAME}-mx1" "${TEST_NAME}-mx2" >/dev/null 2>&1 || true
    docker network rm "$NETWORK" >/dev/null 2>&1 || true
    rm -rf "$TMP_DIR"
}
trap cleanup EXIT

require_cmd() {
    if ! command -v "$1" >/dev/null 2>&1; then
        echo "ERROR: missing required command: $1" >&2
        exit 1
    fi
}

wait_for_http() {
    for _ in $(seq 1 "${OUTBOX_E2E_READY_TIMEOUT:-240}"); do
        if curl -s --max-time 2 "http://127.0.0.1:${HTTP_PORT}/api/world" >/dev/null 2>&1; then
            return 0
        fi
        sleep 1
    done
    return 1
}

container_ip() {
    docker inspect -f "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}" "$1"
}

api_post() {
    local path="$1" body="$2"
    curl -s --max-time 15 \
        -X POST "http://127.0.0.1:${HTTP_PORT}${path}" \
        -H "Authorization: Bearer $(cat "$TOKEN_FILE" 2>/dev/null || true)" \
        -H "Content-Type: application/json" \
        -d "$body"
}

api_post_jsonapi() {
    local path="$1" body="$2"
    curl -s --max-time 15 \
        -X POST "http://127.0.0.1:${HTTP_PORT}${path}" \
        -H "Authorization: Bearer $(cat "$TOKEN_FILE" 2>/dev/null || true)" \
        -H "Content-Type: application/vnd.api+json" \
        -d "$body"
}

write_smtp_server() {
    cat > "$TMP_DIR/fake_smtp.py" <<'PY'
#!/usr/bin/env python3
import os
import socketserver
import time

CAPTURE_DIR = os.environ["CAPTURE_DIR"]
NAME = os.environ["SMTP_NAME"]
FAIL_AFTER_DATA = os.environ.get("FAIL_AFTER_DATA") == "1"

class Handler(socketserver.StreamRequestHandler):
    def send_line(self, line):
        self.wfile.write((line + "\r\n").encode("ascii"))
        self.wfile.flush()

    def handle(self):
        self.send_line("220 fake-smtp ESMTP")
        data_mode = False
        data = []
        while True:
            raw = self.rfile.readline()
            if not raw:
                break
            line = raw.rstrip(b"\r\n")
            upper = line.upper()
            if data_mode:
                if line == b".":
                    os.makedirs(CAPTURE_DIR, exist_ok=True)
                    stamp = str(int(time.time() * 1000))
                    with open(os.path.join(CAPTURE_DIR, f"{NAME}-{stamp}.eml"), "wb") as f:
                        f.write(b"\r\n".join(data))
                        f.write(b"\r\n")
                    data_mode = False
                    if FAIL_AFTER_DATA:
                        self.send_line("451 requested test failure after DATA")
                    else:
                        self.send_line("250 queued")
                    continue
                if line.startswith(b".."):
                    line = line[1:]
                data.append(line)
                continue
            if upper.startswith(b"EHLO") or upper.startswith(b"HELO"):
                self.send_line("250-fake-smtp")
                self.send_line("250 8BITMIME")
            elif upper.startswith(b"MAIL FROM:"):
                self.send_line("250 sender ok")
            elif upper.startswith(b"RCPT TO:"):
                self.send_line("250 recipient ok")
            elif upper == b"DATA":
                data = []
                data_mode = True
                self.send_line("354 end with <CR><LF>.<CR><LF>")
            elif upper == b"QUIT":
                self.send_line("221 bye")
                break
            else:
                self.send_line("250 ok")

class ReusableTCPServer(socketserver.ThreadingTCPServer):
    allow_reuse_address = True

with ReusableTCPServer(("0.0.0.0", 25), Handler) as server:
    server.serve_forever()
PY
}

write_dns_config() {
    mkdir -p "$TMP_DIR/coredns"
    cat > "$TMP_DIR/coredns/Corefile" <<EOF
${DOMAIN}:53 {
    file /zones/db.${DOMAIN} ${DOMAIN}
    errors
}
.:53 {
    forward . 1.1.1.1 8.8.8.8
    errors
}
EOF
    cat > "$TMP_DIR/coredns/db.${DOMAIN}" <<EOF
\$ORIGIN ${DOMAIN}.
@ 3600 IN SOA ns.${DOMAIN}. hostmaster.${DOMAIN}. 1 7200 3600 1209600 3600
@ 3600 IN NS ns.${DOMAIN}.
ns 3600 IN A ${DNS_IP}
@ 3600 IN MX 10 mx1.${DOMAIN}.
@ 3600 IN MX 20 mx2.${DOMAIN}.
mx1 3600 IN A ${MX1_IP}
mx2 3600 IN A ${MX2_IP}
EOF
}

bootstrap_admin() {
    curl -s --max-time 15 \
        -X POST "http://127.0.0.1:${HTTP_PORT}/action/user_account/signup" \
        -H "Content-Type: application/json" \
        -d '{"attributes":{"name":"Admin","email":"admin@admin.com","password":"adminadmin","passwordConfirm":"adminadmin"}}' >/dev/null || true

    curl -s --max-time 15 \
        -X POST "http://127.0.0.1:${HTTP_PORT}/action/user_account/signin" \
        -H "Content-Type: application/json" \
        -d '{"attributes":{"email":"admin@admin.com","password":"adminadmin"}}' \
        | jq -r '.[] | select(.ResponseType == "client.store.set") | .Attributes.value // empty' > "$TOKEN_FILE"

    if [ ! -s "$TOKEN_FILE" ]; then
        echo "ERROR: failed to get admin token" >&2
        docker logs "${TEST_NAME}-daptin" >&2 || true
        exit 1
    fi

    api_post "/action/world/become_an_administrator" '{"attributes":{}}' >/dev/null || true
}

insert_outbox_row() {
    local raw_mail mail_b64
    raw_mail=$'From: login@sender.test\r\nTo: receiver@outbox-e2e.test\r\nSubject: Outbox E2E\r\nDate: Tue, 23 Jun 2026 10:48:00 +0000\r\n\r\nBody from outbox e2e\r\n'
    mail_b64="$(printf '%s' "$raw_mail" | base64 | tr -d '\n')"

    local body response outbox_id
    body="$(jq -n \
        --arg from "$SENDER" \
        --arg to "$RECIPIENT" \
        --arg host "$DOMAIN" \
        --arg mail "$mail_b64" \
        '{data:{type:"outbox",attributes:{from_address:$from,to_address:$to,to_host:$host,mail:$mail,sent:false,retry_count:0}}}')"
    response="$(api_post_jsonapi "/api/outbox" "$body")"
    outbox_id="$(echo "$response" | jq -r '.data.id // empty')"
    if [ -z "$outbox_id" ]; then
        echo "ERROR: failed to create outbox row" >&2
        echo "$response" >&2
        exit 1
    fi
    log "Created outbox row ${outbox_id}"
}

main() {
    require_cmd docker
    require_cmd curl
    require_cmd jq
    require_cmd sqlite3
    require_cmd base64
    require_cmd python3

    mkdir -p "$CAPTURE_DIR"
    write_smtp_server

    log "Building Daptin test binary in Docker..."
    docker run --rm \
        -v "$DAPTIN_DIR:/src:ro" \
        -v "$TMP_DIR:/work" \
        -v daptin-outbox-e2e-gomod:/go/pkg/mod \
        -v daptin-outbox-e2e-gocache:/root/.cache/go-build \
        -e GOTMPDIR=/work/gotmp \
        -e TMPDIR=/work/gotmp \
        golang:1.25-bookworm \
        sh -lc 'mkdir -p /work/src /work/gotmp && tar -C /src --exclude .git --exclude daptin.db -cf - . | tar -C /work/src -xf - && cd /work/src && /usr/local/go/bin/go build -o /work/daptin .' >/dev/null

    log "Creating Docker network ${NETWORK}..."
    docker network create "$NETWORK" >/dev/null

    log "Starting fake SMTP MX servers..."
    docker run -d --name "${TEST_NAME}-mx1" --network "$NETWORK" \
        -v "$TMP_DIR/fake_smtp.py:/fake_smtp.py:ro" \
        -v "$CAPTURE_DIR:/capture" \
        -e CAPTURE_DIR=/capture -e SMTP_NAME=mx1 -e FAIL_AFTER_DATA=1 \
        python:3.12-alpine python /fake_smtp.py >/dev/null
    docker run -d --name "${TEST_NAME}-mx2" --network "$NETWORK" \
        -v "$TMP_DIR/fake_smtp.py:/fake_smtp.py:ro" \
        -v "$CAPTURE_DIR:/capture" \
        -e CAPTURE_DIR=/capture -e SMTP_NAME=mx2 \
        python:3.12-alpine python /fake_smtp.py >/dev/null

    MX1_IP="$(container_ip "${TEST_NAME}-mx1")"
    MX2_IP="$(container_ip "${TEST_NAME}-mx2")"
    DNS_IP="127.0.0.1"
    write_dns_config
    log "MX1=${MX1_IP}, MX2=${MX2_IP}"

    log "Starting test DNS server..."
    docker run -d --name "${TEST_NAME}-dns" --network "$NETWORK" \
        -v "$TMP_DIR/coredns:/zones:ro" \
        coredns/coredns:1.11.3 -conf /zones/Corefile >/dev/null
    DNS_IP="$(container_ip "${TEST_NAME}-dns")"
    log "DNS=${DNS_IP}"

    log "Starting Daptin with test DNS..."
    docker run -d --name "${TEST_NAME}-daptin" --network "$NETWORK" \
        --dns "$DNS_IP" -p "127.0.0.1:${HTTP_PORT}:6336" \
        -v "$TMP_DIR:/work" \
        -w /work/src \
        golang:1.25-bookworm \
        /work/daptin >/dev/null

    if ! wait_for_http; then
        echo "ERROR: Daptin did not become ready" >&2
        docker logs "${TEST_NAME}-daptin" >&2 || true
        exit 1
    fi

    bootstrap_admin
    insert_outbox_row

    log "Calling process_outbox..."
    local process_response
    process_response="$(api_post "/action/outbox/process_outbox" '{"attributes":{}}')"
    sleep 2

    local mx1_count mx2_count sent retry_count captured row_state
    mx1_count="$(find "$CAPTURE_DIR" -name 'mx1-*.eml' | wc -l | tr -d ' ')"
    mx2_count="$(find "$CAPTURE_DIR" -name 'mx2-*.eml' | wc -l | tr -d ' ')"
    sent="$(sqlite3 "$DB_FILE" "SELECT count(*) FROM outbox WHERE from_address='${SENDER}' AND sent=1;")"
    retry_count="$(sqlite3 "$DB_FILE" "SELECT retry_count FROM outbox WHERE from_address='${SENDER}' ORDER BY id DESC LIMIT 1;")"
    row_state="$(sqlite3 "$DB_FILE" "SELECT id, from_address, to_address, to_host, sent, retry_count, last_error FROM outbox WHERE from_address='${SENDER}' ORDER BY id DESC LIMIT 1;" 2>/dev/null || true)"

    [ "$mx1_count" -ge 1 ] && pass "first MX received DATA before returning transient failure" || fail "first MX did not receive DATA"
    [ "$mx2_count" -ge 1 ] && pass "second MX received retried DATA" || fail "second MX did not receive retried DATA"
    [ "$sent" = "1" ] && pass "outbox row marked sent after second MX accepted message" || fail "outbox row was not marked sent"
    [ "$retry_count" = "0" ] && pass "retry_count remains 0 after successful delivery" || fail "retry_count changed to ${retry_count}"

    captured="$(find "$CAPTURE_DIR" -name 'mx2-*.eml' | head -1)"
    if [ -n "$captured" ] && grep -q '^From: login@sender\.test' "$captured"; then
        pass "captured SMTP DATA preserved From header"
    else
        fail "captured SMTP DATA is missing From header"
        [ -n "$captured" ] && sed -n '1,40p' "$captured" || true
    fi
    if [ -n "$captured" ] && grep -q '^Subject: Outbox E2E' "$captured" && grep -q 'Body from outbox e2e' "$captured"; then
        pass "captured SMTP DATA preserved subject and body"
    else
        fail "captured SMTP DATA missing subject/body"
    fi

    if [ "$FAIL" -ne 0 ]; then
        log "process_outbox response: ${process_response}"
        log "outbox row: ${row_state}"
        log "Daptin log tail:"
        docker logs --tail 80 "${TEST_NAME}-daptin" 2>&1 || true
        log "MX1 log tail:"
        docker logs --tail 40 "${TEST_NAME}-mx1" 2>&1 || true
        log "MX2 log tail:"
        docker logs --tail 40 "${TEST_NAME}-mx2" 2>&1 || true
    fi

    log "Results: ${PASS} passed, ${FAIL} failed"
    [ "$FAIL" -eq 0 ]
}

main "$@"
