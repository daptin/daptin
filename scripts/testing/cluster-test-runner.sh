#!/bin/bash
# Cluster Test Runner — manages PostgreSQL Docker + 3 Daptin nodes
#
# Usage:
#   ./cluster-test-runner.sh start       # Start PG + all 3 nodes
#   ./cluster-test-runner.sh stop        # Tear down everything
#   ./cluster-test-runner.sh bootstrap   # start + signup admin + become_an_administrator
#   ./cluster-test-runner.sh smoke       # bootstrap + verify 3 nodes + tear down
#   ./cluster-test-runner.sh status      # Show status of PG + nodes
#
# When sourced by other scripts, provides helper functions:
#   api_get $port $path
#   api_post $port $path $body
#   call_action $port $entity $action $attrs
#   read_token
#   wait_for_port $port $label $max_seconds
#   pg_exec $sql

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DAPTIN_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

# ── Port layout ─────────────────────────────────────────────────────────────
NODE1_HTTP=16336; NODE1_OLRIC=15336; NODE1_MEMBER=15337
NODE2_HTTP=16338; NODE2_OLRIC=15338; NODE2_MEMBER=15339
NODE3_HTTP=16340; NODE3_OLRIC=15340; NODE3_MEMBER=15341

ALL_HTTP_PORTS="$NODE1_HTTP $NODE2_HTTP $NODE3_HTTP"
ALL_OLRIC_PORTS="$NODE1_OLRIC $NODE2_OLRIC $NODE3_OLRIC"
ALL_MEMBER_PORTS="$NODE1_MEMBER $NODE2_MEMBER $NODE3_MEMBER"

# ── PostgreSQL ──────────────────────────────────────────────────────────────
PG_CONTAINER="daptin-cluster-pg"
PG_PORT=15433
PG_USER="daptin"
PG_PASS="daptin"
PG_DB="daptin"
PG_CONN="host=localhost port=$PG_PORT user=$PG_USER password=$PG_PASS dbname=$PG_DB sslmode=disable"

# ── Logs ────────────────────────────────────────────────────────────────────
NODE1_LOG="/tmp/daptin-node1.log"
NODE2_LOG="/tmp/daptin-node2.log"
NODE3_LOG="/tmp/daptin-node3.log"
PID_FILE="/tmp/daptin-cluster-pids.txt"
TOKEN_FILE="/tmp/daptin-cluster-token.txt"
CLUSTER_BIN="/tmp/daptin-cluster-test-bin"

# Peers must use MEMBERSHIP ports (not bind ports) — memberlist.Join() connects to these.
# Olric's SetupNetworkConfig() resolves BindAddr to the primary interface IP (not 127.0.0.1),
# so peers must use the same IP that memberlist actually binds to.
OLRIC_HOST=$(python3 -c "import socket; s=socket.socket(socket.AF_INET,socket.SOCK_DGRAM); s.connect(('8.8.8.8',80)); print(s.getsockname()[0]); s.close()" 2>/dev/null || echo "127.0.0.1")
OLRIC_PEERS="$OLRIC_HOST:$NODE1_MEMBER,$OLRIC_HOST:$NODE2_MEMBER,$OLRIC_HOST:$NODE3_MEMBER"
TIMEOUT=10

# ── Helper functions (available when sourced) ───────────────────────────────

log() { echo "[$(date +%H:%M:%S)] $*"; }

wait_for_port() {
    local port="$1" label="$2" max="${3:-60}"
    for i in $(seq 1 "$max"); do
        if nc -z 127.0.0.1 "$port" 2>/dev/null; then
            return 0
        fi
        sleep 1
    done
    log "TIMEOUT: $label not listening on port $port after ${max}s"
    return 1
}

wait_for_http() {
    local port="$1" label="$2" max="${3:-60}"
    for i in $(seq 1 "$max"); do
        if curl -fsS --max-time 2 --connect-timeout 2 "http://127.0.0.1:$port/ready" > /dev/null 2>&1; then
            return 0
        fi
        sleep 1
    done
    log "TIMEOUT: $label HTTP not ready on port $port after ${max}s"
    return 1
}

read_token() {
    if [ -f "$TOKEN_FILE" ]; then
        cat "$TOKEN_FILE"
    else
        echo ""
    fi
}

api_get() {
    local port="$1" path="$2"
    curl -s --max-time $TIMEOUT --connect-timeout $TIMEOUT \
        "http://localhost:$port$path" \
        -H "Authorization: Bearer $(read_token)"
}

api_post() {
    local port="$1" path="$2" body="$3"
    curl -s --max-time $TIMEOUT --connect-timeout $TIMEOUT \
        -X POST "http://localhost:$port$path" \
        -H "Authorization: Bearer $(read_token)" \
        -H "Content-Type: application/json" \
        -d "$body"
}

api_post_jsonapi() {
    local port="$1" path="$2" body="$3"
    curl -s --max-time $TIMEOUT --connect-timeout $TIMEOUT \
        -X POST "http://localhost:$port$path" \
        -H "Authorization: Bearer $(read_token)" \
        -H "Content-Type: application/vnd.api+json" \
        -d "$body"
}

call_action() {
    local port="$1" entity="$2" action="$3" attrs="${4:-{}}"
    api_post "$port" "/action/$entity/$action" "{\"attributes\":$attrs}"
}

pg_exec() {
    PGPASSWORD="$PG_PASS" psql -h localhost -p "$PG_PORT" -U "$PG_USER" -d "$PG_DB" -tAc "$1"
}

# ── Kill cluster processes ──────────────────────────────────────────────────

kill_cluster() {
    log "Stopping cluster processes..."

    if [ -f "$PID_FILE" ]; then
        while read -r pid; do
            if ps -p "$pid" -o command= 2>/dev/null | grep -Fq "$CLUSTER_BIN"; then
                kill -TERM "$pid" 2>/dev/null || true
            fi
        done < "$PID_FILE"

        for _ in $(seq 1 35); do
            local running=0
            while read -r pid; do
                if ps -p "$pid" -o command= 2>/dev/null | grep -Fq "$CLUSTER_BIN"; then
                    running=1
                    break
                fi
            done < "$PID_FILE"
            if [ "$running" -eq 0 ]; then
                break
            fi
            sleep 1
        done

        while read -r pid; do
            if ps -p "$pid" -o command= 2>/dev/null | grep -Fq "$CLUSTER_BIN"; then
                kill -KILL "$pid" 2>/dev/null || true
            fi
        done < "$PID_FILE"
        rm -f "$PID_FILE"
    fi
    rm -f "$CLUSTER_BIN"
}

# ── PostgreSQL management ───────────────────────────────────────────────────

start_postgres() {
    log "Starting PostgreSQL container..."

    # Remove old container if exists
    docker rm -f "$PG_CONTAINER" 2>/dev/null || true

    docker run -d --name "$PG_CONTAINER" \
        -p "$PG_PORT:5432" \
        -e POSTGRES_USER="$PG_USER" \
        -e POSTGRES_PASSWORD="$PG_PASS" \
        -e POSTGRES_DB="$PG_DB" \
        postgres:16 > /dev/null

    log "Waiting for PostgreSQL..."
    for i in $(seq 1 30); do
        if docker exec "$PG_CONTAINER" pg_isready -U "$PG_USER" > /dev/null 2>&1; then
            log "PostgreSQL ready"
            return 0
        fi
        sleep 1
    done
    log "ERROR: PostgreSQL failed to start"
    return 1
}

stop_postgres() {
    docker rm -f "$PG_CONTAINER" 2>/dev/null || true
}

# ── Start a single node ────────────────────────────────────────────────────

start_node() {
    local node_num="$1" http_port="$2" olric_port="$3" logfile="$4"
    local member_port=$((olric_port + 1))

    log "Starting Node $node_num (HTTP=$http_port, Olric=$olric_port, Member=$member_port)..."

    local pid
    pid=$(python3 - "$DAPTIN_DIR" "$CLUSTER_BIN" "$http_port" "$olric_port" "$PG_CONN" "$OLRIC_PEERS" "$logfile" <<'PY'
import subprocess
import sys

root, binary, http_port, olric_port, database, peers, logfile = sys.argv[1:]
with open(logfile, "wb") as output:
    process = subprocess.Popen(
        [binary, "-port", ":" + http_port, "-db_type", "postgres",
         "-db_connection_string", database, "-olric_peers", peers,
         "-olric_port", olric_port, "-olric_env", "local"],
        cwd=root, stdin=subprocess.DEVNULL, stdout=output,
        stderr=subprocess.STDOUT, start_new_session=True,
    )
print(process.pid)
PY
)
    echo "$pid" >> "$PID_FILE"
    log "Node $node_num PID: $pid"
}

# ── Start cluster ──────────────────────────────────────────────────────────

start_cluster() {
    kill_cluster
    stop_postgres
    rm -f "$PID_FILE"

    for port in $ALL_HTTP_PORTS $ALL_OLRIC_PORTS $ALL_MEMBER_PORTS "$PG_PORT"; do
        if lsof -tiTCP:"$port" -sTCP:LISTEN > /dev/null 2>&1; then
            log "ERROR: cluster port $port is already in use"
            return 1
        fi
    done

    (cd "$DAPTIN_DIR" && go build -o "$CLUSTER_BIN" ./cmd/daptin)

    start_postgres

    # Node 1 first — it creates the schema
    start_node 1 "$NODE1_HTTP" "$NODE1_OLRIC" "$NODE1_LOG"
    log "Waiting for Node 1 to initialize schema..."
    if ! wait_for_http "$NODE1_HTTP" "Node 1" 90; then
        log "ERROR: Node 1 failed to start. Check $NODE1_LOG"
        tail -30 "$NODE1_LOG"
        return 1
    fi
    log "Node 1 ready"

    # Nodes 2 and 3 in parallel
    start_node 2 "$NODE2_HTTP" "$NODE2_OLRIC" "$NODE2_LOG"
    start_node 3 "$NODE3_HTTP" "$NODE3_OLRIC" "$NODE3_LOG"

    log "Waiting for Node 2..."
    if ! wait_for_http "$NODE2_HTTP" "Node 2" 60; then
        log "ERROR: Node 2 failed. Check $NODE2_LOG"
        return 1
    fi

    log "Waiting for Node 3..."
    if ! wait_for_http "$NODE3_HTTP" "Node 3" 60; then
        log "ERROR: Node 3 failed. Check $NODE3_LOG"
        return 1
    fi

    # Inspect Olric peer joins after all nodes are ready.
    local peer_msgs=0
    for logf in "$NODE1_LOG" "$NODE2_LOG" "$NODE3_LOG"; do
        local count
        count=$(grep -c "Joining from\|Member joined\|members are joining\|memberlist" "$logf" 2>/dev/null || true)
        count="${count:-0}"
        count=$(echo "$count" | tr -d '[:space:]')
        peer_msgs=$((peer_msgs + count))
    done
    if [ "$peer_msgs" -gt 0 ]; then
        log "Olric cluster: detected $peer_msgs peer join messages across nodes"
    else
        log "WARNING: No Olric peer join messages found — cluster may not have formed"
    fi

    log "All 3 nodes running"
}

# ── Bootstrap (signup + admin) ──────────────────────────────────────────────

bootstrap_cluster() {
    start_cluster

    log "=== Bootstrapping admin ==="

    log "Signing up admin user..."
    curl -s --max-time $TIMEOUT \
        -X POST "http://localhost:$NODE1_HTTP/action/user_account/signup" \
        -H "Content-Type: application/json" \
        -d '{"attributes":{"name":"Admin","email":"admin@admin.com","password":"adminadmin","passwordConfirm":"adminadmin"}}' \
        > /dev/null

    log "Getting auth token..."
    local token
    token=$(curl -s --max-time $TIMEOUT \
        -X POST "http://localhost:$NODE1_HTTP/action/user_account/signin" \
        -H "Content-Type: application/json" \
        -d '{"attributes":{"email":"admin@admin.com","password":"adminadmin"}}' \
        | jq -r '.[] | select(.ResponseType == "client.store.set") | .Attributes.value // empty')

    if [ -z "$token" ]; then
        log "ERROR: Failed to get auth token"
        return 1
    fi
    echo "$token" > "$TOKEN_FILE"
    chmod 600 "$TOKEN_FILE"
    log "Token acquired"

    log "Becoming administrator..."
    curl -s --max-time $TIMEOUT \
        -X POST "http://localhost:$NODE1_HTTP/action/world/become_an_administrator" \
        -H "Authorization: Bearer $token" \
        -H "Content-Type: application/json" \
        -d '{"attributes":{}}' > /dev/null

    for port in $ALL_HTTP_PORTS; do
        if ! wait_for_http "$port" "Node on $port (post-admin)" 30; then
            return 1
        fi
        local node_token
        node_token=$(curl -fsS --max-time "$TIMEOUT" \
            -X POST "http://127.0.0.1:$port/action/user_account/signin" \
            -H "Content-Type: application/json" \
            -d '{"attributes":{"email":"admin@admin.com","password":"adminadmin"}}' \
            | jq -r '.[] | select(.ResponseType == "client.store.set") | .Attributes.value // empty')
        if [ -z "$node_token" ]; then
            log "ERROR: admin account is not available on node $port"
            return 1
        fi
    done

    log "=== Bootstrap complete ==="
    log "Token file: $TOKEN_FILE"
    log "Nodes: $NODE1_HTTP, $NODE2_HTTP, $NODE3_HTTP"
    log "PostgreSQL: localhost:$PG_PORT ($PG_DB)"
}

# ── Status ──────────────────────────────────────────────────────────────────

show_status() {
    echo "=== Cluster Status ==="

    # PostgreSQL
    if docker inspect "$PG_CONTAINER" > /dev/null 2>&1; then
        local pg_status
        pg_status=$(docker inspect -f '{{.State.Status}}' "$PG_CONTAINER")
        echo "PostgreSQL: $pg_status (port $PG_PORT)"
    else
        echo "PostgreSQL: not running"
    fi

    # Nodes
    for port in $NODE1_HTTP $NODE2_HTTP $NODE3_HTTP; do
        if curl -fsS --max-time 2 --connect-timeout 2 "http://127.0.0.1:$port/ready" > /dev/null 2>&1; then
            echo "Node (port $port): running"
        else
            echo "Node (port $port): not responding"
        fi
    done

    # Token
    if [ -f "$TOKEN_FILE" ]; then
        echo "Token: present"
    else
        echo "Token: not set"
    fi

    # PIDs
    if [ -f "$PID_FILE" ]; then
        echo "PIDs: $(cat "$PID_FILE" | tr '\n' ' ')"
    fi
}

# ── Stop ────────────────────────────────────────────────────────────────────

stop_cluster() {
    kill_cluster
    stop_postgres
    rm -f "$TOKEN_FILE"
    log "Cluster stopped"
}

# ── Command router (only when executed, not sourced) ────────────────────────

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    case "${1:-}" in
        start)
            start_cluster
            ;;
        stop)
            stop_cluster
            ;;
        bootstrap)
            bootstrap_cluster
            ;;
        smoke)
            trap stop_cluster EXIT
            bootstrap_cluster
            ;;
        status)
            show_status
            ;;
        *)
            echo "Usage: $0 {start|stop|bootstrap|smoke|status}"
            echo ""
            echo "  start      - Start PG + 3 Daptin nodes"
            echo "  stop       - Tear down everything"
            echo "  bootstrap  - start + signup admin + become_an_administrator"
            echo "  smoke      - bootstrap and verify all 3 nodes, then stop"
            echo "  status     - Show status of PG + nodes"
            ;;
    esac
fi
