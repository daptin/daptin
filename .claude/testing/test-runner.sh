#!/bin/bash
# Daptin Documentation Testing Framework
# Usage: ./test-runner.sh <command> [args]

DAPTIN_HOST="${DAPTIN_HOST:-http://localhost:6336}"
DAPTIN_LOG="/tmp/daptin.log"
TOKEN_FILE="/tmp/daptin-token.txt"
TIMEOUT=5

# Check if server is running (quick check)
check_server() {
    curl -s --max-time 2 --connect-timeout 2 "$DAPTIN_HOST/api/world" > /dev/null 2>&1
}

# Get auth token
get_token() {
    curl -s --max-time $TIMEOUT --connect-timeout $TIMEOUT \
        -X POST "$DAPTIN_HOST/action/user_account/signin" \
        -H "Content-Type: application/json" \
        -d '{"attributes":{"email":"admin@admin.com","password":"adminadmin"}}' \
        | jq -r '.[] | select(.ResponseType == "client.store.set") | .Attributes.value // empty' \
        | tee "$TOKEN_FILE"
}

# Read token from cache or get new
read_token() {
    if [ -f "$TOKEN_FILE" ]; then
        cat "$TOKEN_FILE"
    else
        get_token
    fi
}

# API GET
api_get() {
    curl -s --max-time $TIMEOUT --connect-timeout $TIMEOUT \
        "$DAPTIN_HOST$1" \
        -H "Authorization: Bearer $(read_token)"
}

# API POST
api_post() {
    curl -s --max-time $TIMEOUT --connect-timeout $TIMEOUT \
        -X POST "$DAPTIN_HOST$1" \
        -H "Authorization: Bearer $(read_token)" \
        -H "Content-Type: application/json" \
        -d "$2"
}

# Call action
call_action() {
    api_post "/action/$1/$2" "{\"attributes\":$3}"
}

# Start server in background
start_server() {
    echo "Stopping existing..."
    pkill -9 -f "go run main.go" 2>/dev/null || true
    pkill -9 -f daptin 2>/dev/null || true
    sleep 2

    echo "Starting server..."
    cd /Users/artpar/workspace/code/github.com/daptin/daptin
    nohup go run main.go > "$DAPTIN_LOG" 2>&1 &
    echo "PID: $!"

    echo "Waiting for server..."
    for i in {1..30}; do
        if check_server; then
            echo "Server ready!"
            return 0
        fi
        sleep 1
        echo -n "."
    done
    echo ""
    echo "Server failed to start. Check: tail -20 $DAPTIN_LOG"
    return 1
}

# Command router
case "$1" in
    check)
        if check_server; then echo "running"; else echo "stopped"; fi
        ;;
    start)
        start_server
        ;;
    stop)
        pkill -9 -f "go run main.go" 2>/dev/null
        pkill -9 -f daptin 2>/dev/null
        echo "stopped"
        ;;
    token)
        get_token
        ;;
    get)
        api_get "$2" | jq .
        ;;
    post)
        api_post "$2" "$3" | jq .
        ;;
    action)
        call_action "$2" "$3" "$4" | jq .
        ;;
    logs)
        tail -"${2:-20}" "$DAPTIN_LOG"
        ;;
    errors)
        grep -E "ERRO|FATAL|panic" "$DAPTIN_LOG" | tail -20
        ;;
    *)
        echo "Usage: $0 {check|start|stop|token|get|post|action|logs|errors}"
        echo ""
        echo "  check              - Check if server running"
        echo "  start              - Start server"
        echo "  stop               - Stop server"
        echo "  token              - Get auth token"
        echo "  get /api/entity    - GET request"
        echo "  post /api/x '{}'   - POST request"
        echo "  action e a '{}'    - Call action"
        echo "  logs [n]           - Show logs"
        echo "  errors             - Show errors"
        ;;
esac
