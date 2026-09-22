#!/usr/bin/env bash
# Black-box regression test for inbound SMTP storage (issue #288).
#
# The test starts a fresh PostgreSQL container and a real Daptin process,
# provisions mail resources through HTTP, delivers an RFC822 message with
# swaks, and verifies the stored mail through Daptin's JSON:API.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/daptin-smtp-inbound-e2e.XXXXXX")"
PG_CONTAINER="daptin-smtp-inbound-e2e-$$"
SERVER_PID=""
KEEP_ARTIFACTS="${DAPTIN_SMTP_E2E_KEEP_ARTIFACTS:-0}"

log() { printf '[smtp-inbound-e2e] %s\n' "$*"; }

cleanup() {
	local status=$?
	if [[ -n "$SERVER_PID" ]] && kill -0 "$SERVER_PID" >/dev/null 2>&1; then
		kill "$SERVER_PID" >/dev/null 2>&1 || true
		wait "$SERVER_PID" >/dev/null 2>&1 || true
	fi
	docker rm -f "$PG_CONTAINER" >/dev/null 2>&1 || true
	if [[ "$KEEP_ARTIFACTS" == "1" || "$status" -ne 0 ]]; then
		log "artifacts retained at $TMP_DIR"
	else
		rm -rf "$TMP_DIR"
	fi
	exit "$status"
}
trap cleanup EXIT

require_command() {
	if ! command -v "$1" >/dev/null 2>&1; then
		log "missing required command: $1"
		exit 1
	fi
}

free_port() {
	python3 - <<'PY'
import socket
s = socket.socket()
s.bind(("127.0.0.1", 0))
print(s.getsockname()[1])
s.close()
PY
}

for command in go curl jq docker swaks python3; do
	require_command "$command"
done

HTTP_PORT="$(free_port)"
HTTPS_PORT="$(free_port)"
SMTP_PORT="$(free_port)"
POSTGRES_PORT="$(free_port)"
OLRIC_PORT="$(free_port)"
BASE_URL="http://127.0.0.1:$HTTP_PORT"
BIN_PATH="$TMP_DIR/daptin"
FIRST_LOG="$TMP_DIR/daptin-bootstrap.log"
SMTP_LOG="$TMP_DIR/daptin-smtp.log"
SWAKS_LOG="$TMP_DIR/swaks.log"
TOKEN_FILE="$TMP_DIR/token"
SUBJECT="issue-288-inbound-$(date +%s)-$$"

PG_USER="daptin_e2e"
PG_PASSWORD="daptin_e2e_password"
PG_DATABASE="daptin_e2e"
PG_DSN="host=127.0.0.1 port=$POSTGRES_PORT user=$PG_USER password=$PG_PASSWORD dbname=$PG_DATABASE sslmode=disable"

stop_server() {
	if [[ -n "$SERVER_PID" ]] && kill -0 "$SERVER_PID" >/dev/null 2>&1; then
		kill "$SERVER_PID"
		wait "$SERVER_PID" || true
	fi
	SERVER_PID=""
}

wait_for_http() {
	local log_path="$1"
	for _ in $(seq 1 120); do
		if curl -fsS --max-time 2 "$BASE_URL/ping" >/dev/null 2>&1; then
			return 0
		fi
		if ! kill -0 "$SERVER_PID" >/dev/null 2>&1; then
			log "Daptin exited before HTTP readiness"
			tail -100 "$log_path" >&2 || true
			return 1
		fi
		sleep 1
	done
	log "Daptin did not become HTTP-ready"
	tail -100 "$log_path" >&2 || true
	return 1
}

wait_for_smtp() {
	for _ in $(seq 1 60); do
		if python3 - "$SMTP_PORT" <<'PY'
import socket
import sys
try:
    with socket.create_connection(("127.0.0.1", int(sys.argv[1])), timeout=1):
        pass
except OSError:
    raise SystemExit(1)
PY
		then
			return 0
		fi
		sleep 1
	done
	log "SMTP did not listen on 127.0.0.1:$SMTP_PORT"
	tail -100 "$SMTP_LOG" >&2 || true
	return 1
}

start_server() {
	local log_path="$1"
	mkdir -p "$TMP_DIR/cache" "$TMP_DIR/storage"
	(
		cd "$TMP_DIR"
		env \
			DAPTIN_CACHE_FOLDER="$TMP_DIR/cache" \
			DAPTIN_DISABLE_SMTP=false \
			DAPTIN_PORT=":$HTTP_PORT" \
			"$BIN_PATH" \
			-db_type postgres \
			-db_connection_string "$PG_DSN" \
			-port ":$HTTP_PORT" \
			-https_port ":$HTTPS_PORT" \
			-olric_port "$OLRIC_PORT" \
			-local_storage_path "$TMP_DIR/storage" \
			-runtime release \
			-log_level info
	) >"$log_path" 2>&1 &
	SERVER_PID=$!
	wait_for_http "$log_path"
}

signin() {
	curl --fail-with-body -sS \
		-X POST "$BASE_URL/action/user_account/signin" \
		-H 'Content-Type: application/json' \
		--data '{"attributes":{"email":"admin@example.test","password":"Issue288Pass!"}}' |
		jq -r '.[] | select(.ResponseType == "client.store.set" and .Attributes.key == "token") | .Attributes.value'
}

api_get() {
	curl --fail-with-body -sS "$BASE_URL$1" \
		-H "Authorization: Bearer $(<"$TOKEN_FILE")"
}

api_post() {
	local path="$1"
	local body="$2"
	curl --fail-with-body -sS -X POST "$BASE_URL$path" \
		-H "Authorization: Bearer $(<"$TOKEN_FILE")" \
		-H 'Content-Type: application/vnd.api+json' \
		--data "$body"
}

api_patch() {
	local path="$1"
	local body="$2"
	curl --fail-with-body -sS -X PATCH "$BASE_URL$path" \
		-H "Authorization: Bearer $(<"$TOKEN_FILE")" \
		-H 'Content-Type: application/vnd.api+json' \
		--data "$body"
}

require_id() {
	local name="$1"
	local value="$2"
	if [[ -z "$value" || "$value" == "null" ]]; then
		log "$name did not return an id"
		exit 1
	fi
}

log "starting disposable PostgreSQL 15 on port $POSTGRES_PORT"
docker run --rm -d \
	--name "$PG_CONTAINER" \
	-e POSTGRES_DB="$PG_DATABASE" \
	-e POSTGRES_USER="$PG_USER" \
	-e POSTGRES_PASSWORD="$PG_PASSWORD" \
	-p "127.0.0.1:$POSTGRES_PORT:5432" \
	postgres:15 >/dev/null

for _ in $(seq 1 60); do
	if docker exec "$PG_CONTAINER" pg_isready -U "$PG_USER" -d "$PG_DATABASE" >/dev/null 2>&1; then
		break
	fi
	sleep 1
done
if ! docker exec "$PG_CONTAINER" pg_isready -U "$PG_USER" -d "$PG_DATABASE" >/dev/null 2>&1; then
	log "PostgreSQL did not become ready"
	exit 1
fi

log "building Daptin"
(cd "$PROJECT_ROOT" && go build -o "$BIN_PATH" .)

log "starting fresh Daptin bootstrap server on $BASE_URL"
start_server "$FIRST_LOG"

curl --fail-with-body -sS \
	-X POST "$BASE_URL/action/user_account/signup" \
	-H 'Content-Type: application/json' \
	--data '{"attributes":{"name":"SMTP E2E Admin","email":"admin@example.test","password":"Issue288Pass!","passwordConfirm":"Issue288Pass!"}}' \
	>/dev/null
signin >"$TOKEN_FILE"
if [[ ! -s "$TOKEN_FILE" || "$(<"$TOKEN_FILE")" == "null" ]]; then
	log "signin did not return a token"
	exit 1
fi

curl --fail-with-body -sS \
	-X POST "$BASE_URL/action/world/become_an_administrator" \
	-H "Authorization: Bearer $(<"$TOKEN_FILE")" \
	-H 'Content-Type: application/json' \
	--data '{}' >/dev/null

ADMIN_ID="$(api_get '/api/user_account?page%5Bsize%5D=10' | jq -r '.data[] | select(.attributes.email == "admin@example.test") | .id')"
require_id "administrator resource" "$ADMIN_ID"

MAIL_SERVER_BODY="$(jq -n \
	--arg interface "127.0.0.1:$SMTP_PORT" \
	'{data:{type:"mail_server",attributes:{hostname:"example.test",is_enabled:true,listen_interface:$interface,always_on_tls:false,authentication_required:false,max_size:10485760,max_clients:10}}}')"
MAIL_SERVER_ID="$(api_post /api/mail_server "$MAIL_SERVER_BODY" | jq -r '.data.id')"
require_id "mail_server creation" "$MAIL_SERVER_ID"

MAIL_ACCOUNT_BODY="$(jq -n \
	--arg server "$MAIL_SERVER_ID" \
	'{data:{type:"mail_account",attributes:{username:"recipient@example.test",password:"Mailbox288Pass!",password_md5:"Mailbox288Pass!"},relationships:{mail_server_id:{data:{type:"mail_server",id:$server}}}}}')"
MAIL_ACCOUNT_ID="$(api_post /api/mail_account "$MAIL_ACCOUNT_BODY" | jq -r '.data.id')"
require_id "mail_account creation" "$MAIL_ACCOUNT_ID"

MAILBOX_BODY="$(jq -n \
	--arg account "$MAIL_ACCOUNT_ID" \
	'{data:{type:"mail_box",attributes:{name:"INBOX",subscribed:true,uidvalidity:1,nextuid:1,attributes:"\\HasNoChildren",flags:"\\Seen \\Answered \\Flagged \\Deleted \\Draft",permanent_flags:"\\Seen \\Answered \\Flagged \\Deleted \\Draft \\*"},relationships:{mail_account_id:{data:{type:"mail_account",id:$account}}}}}')"
MAILBOX_ID="$(api_post /api/mail_box "$MAILBOX_BODY" | jq -r '.data.id')"
require_id "mail_box creation" "$MAILBOX_ID"

DATA_EXCHANGE_BODY="$(jq -n \
	--arg owner "$ADMIN_ID" \
	'{data:{type:"data_exchange",attributes:{name:"issue-288-mail-lifecycle",source_type:"self",source_attributes:"{\"name\":\"mail\"}",target_type:"rest",target_attributes:"{\"url\":\"http://127.0.0.1:9/issue-288\",\"method\":\"POST\"}",attributes:"{\"name\":\"mail\",\"hook\":\"after\",\"methods\":[\"post\"]}",options:"{}"},relationships:{as_user_id:{data:{type:"user_account",id:$owner}}}}}')"
DATA_EXCHANGE_ID="$(api_post /api/data_exchange "$DATA_EXCHANGE_BODY" | jq -r '.data.id')"
require_id "data_exchange creation" "$DATA_EXCHANGE_ID"

log "restarting Daptin to activate SMTP on 127.0.0.1:$SMTP_PORT"
stop_server
start_server "$SMTP_LOG"
wait_for_smtp
signin >"$TOKEN_FILE"

log "delivering $SUBJECT with swaks"
swaks \
	--server "127.0.0.1:$SMTP_PORT" \
	--from sender@external.test \
	--to recipient@example.test \
	--header "Subject: $SUBJECT" \
	--body 'fresh PostgreSQL SMTP delivery' \
	--timeout 15s >"$SWAKS_LOG" 2>&1 || true

if ! grep -Eq '<-  +250 2\.0\.0 OK: queued as' "$SWAKS_LOG"; then
	log "SMTP delivery was not accepted"
	cat "$SWAKS_LOG" >&2
	log "matching Daptin log lines:"
	grep -E 'Mail from:|Failed to store mail|storage error|could not save email' "$SMTP_LOG" >&2 || true
	exit 1
fi

MAIL_RESPONSE="$(api_get '/api/mail?page%5Bsize%5D=100')"
DELIVERED_COUNT="$(jq --arg subject "$SUBJECT" '[.data[] | select(.attributes.subject == $subject)] | length' <<<"$MAIL_RESPONSE")"
if [[ "$DELIVERED_COUNT" != "1" ]]; then
	log "expected one stored mail resource for $SUBJECT, got $DELIVERED_COUNT"
	jq --arg subject "$SUBJECT" '[.data[] | select(.attributes.subject == $subject)]' <<<"$MAIL_RESPONSE" >&2
	exit 1
fi

STORED_OWNER="$(jq -r --arg subject "$SUBJECT" '.data[] | select(.attributes.subject == $subject) | .attributes.user_account_id' <<<"$MAIL_RESPONSE")"
STORED_MAILBOX="$(jq -r --arg subject "$SUBJECT" '.data[] | select(.attributes.subject == $subject) | .relationships.mail_box_id.data.id' <<<"$MAIL_RESPONSE")"
STORED_MAIL_ID="$(jq -r --arg subject "$SUBJECT" '.data[] | select(.attributes.subject == $subject) | .id' <<<"$MAIL_RESPONSE")"
if [[ "$STORED_OWNER" != "$ADMIN_ID" ]]; then
	log "stored mail owner $STORED_OWNER does not match recipient owner $ADMIN_ID"
	exit 1
fi
if [[ "$STORED_MAILBOX" != "$MAILBOX_ID" ]]; then
	log "stored mail mailbox $STORED_MAILBOX does not match INBOX $MAILBOX_ID"
	exit 1
fi

MAIL_DETAIL="$(api_get "/api/mail/$STORED_MAIL_ID")"
if [[ "$(jq -r '.data.attributes.subject' <<<"$MAIL_DETAIL")" != "$SUBJECT" || \
	"$(jq -r '.data.attributes.user_account_id' <<<"$MAIL_DETAIL")" != "$ADMIN_ID" || \
	"$(jq -r '.data.relationships.mail_box_id.data.id' <<<"$MAIL_DETAIL")" != "$MAILBOX_ID" || \
	"$(jq -r '.data.attributes.uid' <<<"$MAIL_DETAIL")" != "1" ]]; then
	log "single-resource API did not preserve the inbound mail identity, owner, mailbox, and UID"
	jq '.data' <<<"$MAIL_DETAIL" >&2
	exit 1
fi

MAILBOX_DETAIL="$(api_get "/api/mail_box/$MAILBOX_ID")"
if [[ "$(jq -r '.data.attributes.nextuid' <<<"$MAILBOX_DETAIL")" != "2" ]]; then
	log "mailbox nextuid was not advanced atomically after SMTP storage"
	jq '.data' <<<"$MAILBOX_DETAIL" >&2
	exit 1
fi

EXCHANGE_RUN_RESPONSE="$(api_get '/api/exchange_run?page%5Bsize%5D=100')"
EXCHANGE_RUN_COUNT="$(jq \
	--arg source "$STORED_MAIL_ID" \
	'[.data[] | select(.attributes.source_type == "mail" and .attributes.source_method == "post" and .attributes.source_reference_id == $source)] | length' \
	<<<"$EXCHANGE_RUN_RESPONSE")"
if [[ "$EXCHANGE_RUN_COUNT" != "1" ]]; then
	log "expected one lifecycle exchange run for mail $STORED_MAIL_ID, got $EXCHANGE_RUN_COUNT"
	jq '.data[] | {id, attributes}' <<<"$EXCHANGE_RUN_RESPONSE" >&2
	exit 1
fi

MAIL_PATCH_BODY="$(jq -n \
	--arg id "$STORED_MAIL_ID" \
	'{data:{type:"mail",id:$id,attributes:{seen:true}}}')"
api_patch "/api/mail/$STORED_MAIL_ID" "$MAIL_PATCH_BODY" >/dev/null
UPDATED_MAIL="$(api_get "/api/mail/$STORED_MAIL_ID")"
if [[ "$(jq -r '.data.attributes.seen' <<<"$UPDATED_MAIL")" != "true" || \
	"$(jq -r '.data.attributes.subject' <<<"$UPDATED_MAIL")" != "$SUBJECT" || \
	"$(jq -r '.data.relationships.mail_box_id.data.id' <<<"$UPDATED_MAIL")" != "$MAILBOX_ID" ]]; then
	log "normal mail JSON:API update did not preserve the SMTP-created resource"
	jq '.data' <<<"$UPDATED_MAIL" >&2
	exit 1
fi

log "PASS: SMTP storage, ownership, mailbox UID, lifecycle exchange, and mail JSON:API read/update agree"
