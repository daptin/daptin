#!/usr/bin/env bash
# End-user cluster/load E2E for outbound mail delivery.
#
# This is intentionally a full user-flow test:
#   - runs PostgreSQL, CoreDNS, Mailpit, and 3 Daptin nodes in Docker
#   - creates users through /action/user_account/signup
#   - triggers /action/user_account/reset-password across all nodes
#   - reset-password invokes otp.generate -> mail.send -> outbox -> SMTP
#   - Mailpit is the receiver-side SMTP server and assertion API
#   - Mailpit Chaos verifies retry bookkeeping and later queue drain
#
# Tunables:
#   MAIL_FLOW_LOAD_COUNT=60
#   MAIL_FLOW_FAIL_COUNT=10
#   MAIL_FLOW_CONCURRENCY=12
#   MAIL_FLOW_PROFILE=1       # enable Daptin CPU/heap profile dumps and sampler logs
#   MAIL_FLOW_SAMPLE_INTERVAL=30
#   MAIL_FLOW_NODE_MAX_OPEN_CONNECTIONS=25
#   MAIL_FLOW_MAILPIT_MAX_MESSAGES=2110
#   MAIL_FLOW_SKIP_BUILD=1   # reuse an existing binary copied into the temp dir is not supported

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DAPTIN_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

TEST_NAME="daptin-mail-flow-$$"
NETWORK="${TEST_NAME}-net"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/daptin-mail-flow.XXXXXX")"
TOKEN_FILE="$TMP_DIR/token"
DNS_DIR="$TMP_DIR/coredns"
ARTIFACT_DIR="${MAIL_FLOW_ARTIFACT_DIR:-}"
KEEP_ARTIFACTS="${MAIL_FLOW_KEEP_ARTIFACTS:-0}"

LOAD_COUNT="${MAIL_FLOW_LOAD_COUNT:-60}"
FAIL_COUNT="${MAIL_FLOW_FAIL_COUNT:-10}"
CONCURRENCY="${MAIL_FLOW_CONCURRENCY:-12}"
READY_TIMEOUT="${MAIL_FLOW_READY_TIMEOUT:-240}"
CLAIM_TTL_SECONDS="${MAIL_FLOW_CLAIM_TTL_SECONDS:-5}"
MAILPIT_IMAGE="${MAILPIT_IMAGE:-axllent/mailpit:latest}"
PROFILE_ENABLED="${MAIL_FLOW_PROFILE:-0}"
PROFILE_DUMP_PERIOD="${MAIL_FLOW_PROFILE_DUMP_PERIOD:-1}"
SAMPLE_INTERVAL="${MAIL_FLOW_SAMPLE_INTERVAL:-30}"
NODE_MAX_OPEN_CONNECTIONS="${MAIL_FLOW_NODE_MAX_OPEN_CONNECTIONS:-25}"
NODE_MAX_IDLE_CONNECTIONS="${MAIL_FLOW_NODE_MAX_IDLE_CONNECTIONS:-5}"
MAILPIT_MAX_MESSAGES="${MAIL_FLOW_MAILPIT_MAX_MESSAGES:-$((LOAD_COUNT + FAIL_COUNT + 100))}"

DOMAIN="load-mail.test"
MAIL_SERVER_HOSTNAME="mail.sender.test"
MAIL_FROM="no-reply@localhost"
MAIL_SERVER_ID=""

PG_USER="daptin"
PG_PASS="daptin"
PG_DB="daptin"
PG_CONTAINER="${TEST_NAME}-postgres"
MAILPIT_CONTAINER="${TEST_NAME}-mailpit"
DNS_CONTAINER="${TEST_NAME}-dns"

NODE1="${TEST_NAME}-node1"
NODE2="${TEST_NAME}-node2"
NODE3="${TEST_NAME}-node3"

NODE1_HTTP_PORT=""
NODE2_HTTP_PORT=""
NODE3_HTTP_PORT=""
MAILPIT_HTTP_PORT=""

PASS=0
FAIL=0
SAMPLER_PID=""
CURRENT_PHASE="initializing"

if [ -z "$ARTIFACT_DIR" ]; then
	ARTIFACT_DIR="$TMP_DIR/artifacts"
fi
PROFILE_DIR="$ARTIFACT_DIR/profiles"
SAMPLE_DIR="$ARTIFACT_DIR/samples"
LOG_DIR="$ARTIFACT_DIR/logs"

log() { echo "[$(date +%H:%M:%S)] $*"; }
pass() { PASS=$((PASS + 1)); log "  PASS: $*"; }
fail() { FAIL=$((FAIL + 1)); log "  FAIL: $*"; }

cleanup() {
	if [ -n "$SAMPLER_PID" ]; then
		kill "$SAMPLER_PID" >/dev/null 2>&1 || true
		wait "$SAMPLER_PID" >/dev/null 2>&1 || true
	fi
	collect_artifacts >/dev/null 2>&1 || true
	analyze_profiles >/dev/null 2>&1 || true
	docker rm -f "$NODE1" "$NODE2" "$NODE3" "$DNS_CONTAINER" "$MAILPIT_CONTAINER" "$PG_CONTAINER" >/dev/null 2>&1 || true
	docker network rm "$NETWORK" >/dev/null 2>&1 || true
	if [ "$KEEP_ARTIFACTS" = "1" ] || [ "$PROFILE_ENABLED" = "1" ] || [ "$FAIL" -ne 0 ]; then
		log "Artifacts retained at ${ARTIFACT_DIR}"
		if [ "$ARTIFACT_DIR" != "$TMP_DIR/artifacts" ]; then
			rm -rf "$TMP_DIR"
		fi
	else
		rm -rf "$TMP_DIR"
	fi
}
trap cleanup EXIT

require_cmd() {
	if ! command -v "$1" >/dev/null 2>&1; then
		echo "ERROR: missing required command: $1" >&2
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

container_ip() {
	docker inspect -f "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}" "$1"
}

wait_for_http() {
	local port="$1" label="$2"
	for _ in $(seq 1 "$READY_TIMEOUT"); do
		if curl -s --max-time 2 "http://127.0.0.1:${port}/api/world" >/dev/null 2>&1; then
			return 0
		fi
		sleep 1
	done
	log "TIMEOUT: ${label} HTTP not ready on ${port}"
	return 1
}

wait_for_mailpit() {
	for _ in $(seq 1 120); do
		if curl -s --max-time 2 "http://127.0.0.1:${MAILPIT_HTTP_PORT}/api/v1/messages?limit=1" >/dev/null 2>&1; then
			return 0
		fi
		sleep 1
	done
	return 1
}

api_post() {
	local port="$1" path="$2" body="$3"
	curl -s --max-time 30 \
		-X POST "http://127.0.0.1:${port}${path}" \
		-H "Authorization: Bearer $(cat "$TOKEN_FILE" 2>/dev/null || true)" \
		-H "Content-Type: application/json" \
		-d "$body"
}

api_post_jsonapi() {
	local port="$1" path="$2" body="$3"
	curl -s --max-time 30 \
		-X POST "http://127.0.0.1:${port}${path}" \
		-H "Authorization: Bearer $(cat "$TOKEN_FILE" 2>/dev/null || true)" \
		-H "Content-Type: application/vnd.api+json" \
		-d "$body"
}

config_set() {
	local port="$1" key="$2" value="$3"
	curl -s --max-time 30 \
		-X PUT "http://127.0.0.1:${port}/_config/backend/${key}" \
		-H "Authorization: Bearer $(cat "$TOKEN_FILE" 2>/dev/null || true)" \
		-H "Content-Type: text/plain" \
		-d "$value" >/dev/null
}

mailpit_api() {
	curl -s --max-time 30 "http://127.0.0.1:${MAILPIT_HTTP_PORT}$1"
}

mailpit_count() {
	mailpit_api "/api/v1/messages?limit=1" | jq -r '.total // 0'
}

pg_exec() {
	docker exec -e PGPASSWORD="$PG_PASS" "$PG_CONTAINER" \
		psql -U "$PG_USER" -d "$PG_DB" -tAc "$1"
}

pg_exec_file() {
	local output_file="$1" sql="$2"
	docker exec -e PGPASSWORD="$PG_PASS" "$PG_CONTAINER" \
		psql -U "$PG_USER" -d "$PG_DB" -P pager=off -c "$sql" >"$output_file" 2>&1 || true
}

node_port_for_index() {
	local index="$1"
	case $(( (index - 1) % 3 )) in
		0) echo "$NODE1_HTTP_PORT" ;;
		1) echo "$NODE2_HTTP_PORT" ;;
		*) echo "$NODE3_HTTP_PORT" ;;
	esac
}

write_dns_config() {
	local mailpit_ip="$1"
	mkdir -p "$DNS_DIR"
	cat > "$DNS_DIR/Corefile" <<EOF
${DOMAIN}:53 {
    file /zones/db.${DOMAIN} ${DOMAIN}
    errors
}
.:53 {
    forward . 127.0.0.11 1.1.1.1 8.8.8.8
    errors
}
EOF
	cat > "$DNS_DIR/db.${DOMAIN}" <<EOF
\$ORIGIN ${DOMAIN}.
@ 3600 IN SOA ns.${DOMAIN}. hostmaster.${DOMAIN}. 1 7200 3600 1209600 3600
@ 3600 IN NS ns.${DOMAIN}.
ns 3600 IN A ${mailpit_ip}
@ 3600 IN MX 10 mailpit.${DOMAIN}.
mailpit 3600 IN A ${mailpit_ip}
EOF
}

start_node() {
	local name="$1" http_port="$2" olric_port="$3"
	local peers="${NODE1}:5337,${NODE2}:5339,${NODE3}:5341"
	local profile_args=()
	if [ "$PROFILE_ENABLED" = "1" ]; then
		mkdir -p "$PROFILE_DIR/${name}"
		profile_args=(-runtime profile -profile_dump_path /profiles/ -profile_dump_period "$PROFILE_DUMP_PERIOD")
	fi
	docker run -d --name "$name" --network "$NETWORK" --dns "$(container_ip "$DNS_CONTAINER")" \
		-p "127.0.0.1:${http_port}:6336" \
		-v "$TMP_DIR:/work" \
		-v "$PROFILE_DIR/${name}:/profiles" \
		-e DAPTIN_OUTBOX_CLAIM_TTL_SECONDS="$CLAIM_TTL_SECONDS" \
		-e DAPTIN_MAX_OPEN_CONNECTIONS="$NODE_MAX_OPEN_CONNECTIONS" \
		-e DAPTIN_MAX_IDLE_CONNECTIONS="$NODE_MAX_IDLE_CONNECTIONS" \
		-w /work \
		golang:1.25-bookworm \
		/work/daptin \
		-port ":6336" \
		-db_type postgres \
		-db_connection_string "host=postgres port=5432 user=${PG_USER} password=${PG_PASS} dbname=${PG_DB} sslmode=disable" \
		-olric_peers "$peers" \
		-olric_port "$olric_port" \
		-olric_env local \
		"${profile_args[@]}" >/dev/null
}

sample_once() {
	local phase="$1" stamp="$2"
	mkdir -p "$SAMPLE_DIR/$phase"

	pg_exec_file "$SAMPLE_DIR/$phase/${stamp}-pg-activity-summary.txt" \
		"select state, wait_event_type, wait_event, count(*) from pg_stat_activity where datname='${PG_DB}' group by state, wait_event_type, wait_event order by count(*) desc;"
	pg_exec_file "$SAMPLE_DIR/$phase/${stamp}-pg-activity-detail.txt" \
		"select pid, client_addr, state, wait_event_type, wait_event, now()-xact_start as xact_age, now()-query_start as query_age, left(query, 240) as query from pg_stat_activity where datname='${PG_DB}' order by xact_start nulls last limit 60;"
	pg_exec_file "$SAMPLE_DIR/$phase/${stamp}-pg-locks.txt" \
		"select locktype, mode, granted, relation::regclass, count(*) from pg_locks where database = (select oid from pg_database where datname='${PG_DB}') group by locktype, mode, granted, relation order by granted, count(*) desc;"
	pg_exec_file "$SAMPLE_DIR/$phase/${stamp}-pg-progress.txt" \
		"select 'users_load' as metric, count(*) from user_account where email like 'load-%@${DOMAIN}' union all select 'users_fail', count(*) from user_account where email like 'fail-%@${DOMAIN}' union all select 'outbox_total', count(*) from outbox union all select 'outbox_unsent', count(*) from outbox where sent=false;"

	docker stats --no-stream --format 'table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}\t{{.BlockIO}}' \
		"$NODE1" "$NODE2" "$NODE3" "$MAILPIT_CONTAINER" "$PG_CONTAINER" >"$SAMPLE_DIR/$phase/${stamp}-docker-stats.txt" 2>&1 || true

	for node in "$NODE1:$NODE1_HTTP_PORT" "$NODE2:$NODE2_HTTP_PORT" "$NODE3:$NODE3_HTTP_PORT"; do
		local node_name="${node%%:*}" node_port="${node##*:}"
		curl -s --max-time 5 "http://127.0.0.1:${node_port}/statistics" >"$SAMPLE_DIR/$phase/${stamp}-${node_name}-statistics.json" 2>&1 || true
	done
}

start_sampler() {
	mkdir -p "$SAMPLE_DIR" "$LOG_DIR" "$PROFILE_DIR"
	(
		while true; do
			phase="$(cat "$TMP_DIR/current-phase" 2>/dev/null || printf '%s' "$CURRENT_PHASE")"
			phase="$(printf "%s" "$phase" | tr -c '[:alnum:]_.-' '_')"
			stamp="$(date +%Y%m%d-%H%M%S)"
			sample_once "$phase" "$stamp"
			sleep "$SAMPLE_INTERVAL"
		done
	) &
	SAMPLER_PID="$!"
}

set_phase() {
	CURRENT_PHASE="$1"
	printf "%s" "$CURRENT_PHASE" > "$TMP_DIR/current-phase"
	log "Phase: $CURRENT_PHASE"
	if [ -n "$SAMPLER_PID" ]; then
		sample_once "$CURRENT_PHASE" "$(date +%Y%m%d-%H%M%S)-phase-start" || true
	fi
}

collect_artifacts() {
	mkdir -p "$LOG_DIR" "$PROFILE_DIR"
	for node in "$NODE1" "$NODE2" "$NODE3" "$MAILPIT_CONTAINER" "$PG_CONTAINER" "$DNS_CONTAINER"; do
		if docker ps -a --format '{{.Names}}' | grep -qx "$node"; then
			docker logs "$node" >"$LOG_DIR/${node}.log" 2>&1 || true
		fi
	done
}

analyze_profiles() {
	if [ "$PROFILE_ENABLED" != "1" ]; then
		return 0
	fi
	if ! command -v go >/dev/null 2>&1; then
		log "go not found; skipping pprof summary"
		return 0
	fi
	local summary_dir="$ARTIFACT_DIR/profile-summaries"
	mkdir -p "$summary_dir"
	find "$PROFILE_DIR" -type f -name '*_profile_cpu.*' | sort | while read -r profile_file; do
		go tool pprof -top "$profile_file" >"$summary_dir/$(basename "$profile_file").top.txt" 2>&1 || true
	done
	find "$PROFILE_DIR" -type f -name '*_profile_heap.*' | sort | while read -r profile_file; do
		go tool pprof -top -alloc_space "$profile_file" >"$summary_dir/$(basename "$profile_file").alloc_space.txt" 2>&1 || true
		go tool pprof -top -inuse_space "$profile_file" >"$summary_dir/$(basename "$profile_file").inuse_space.txt" 2>&1 || true
	done
}

bootstrap_admin() {
	curl -s --max-time 30 \
		-X POST "http://127.0.0.1:${NODE1_HTTP_PORT}/action/user_account/signup" \
		-H "Content-Type: application/json" \
		-d '{"attributes":{"name":"Admin","email":"admin@admin.com","password":"adminadmin","passwordConfirm":"adminadmin"}}' >/dev/null || true

	curl -s --max-time 30 \
		-X POST "http://127.0.0.1:${NODE1_HTTP_PORT}/action/user_account/signin" \
		-H "Content-Type: application/json" \
		-d '{"attributes":{"email":"admin@admin.com","password":"adminadmin"}}' \
		| jq -r '.[] | select(.ResponseType == "client.store.set") | .Attributes.value // empty' > "$TOKEN_FILE"

	if [ ! -s "$TOKEN_FILE" ]; then
		echo "ERROR: failed to get admin token" >&2
		docker logs "$NODE1" >&2 || true
		exit 1
	fi

	api_post "$NODE1_HTTP_PORT" "/action/world/become_an_administrator" '{"attributes":{}}' >/dev/null || true
}

create_mail_server() {
	local body response
	body="$(jq -n \
		--arg hostname "$MAIL_SERVER_HOSTNAME" \
		'{data:{type:"mail_server",attributes:{hostname:$hostname,is_enabled:false,listen_interface:"127.0.0.1:2525",max_size:1048576,max_clients:100,xclient_on:false,always_on_tls:false,authentication_required:false}}}')"
	response="$(api_post_jsonapi "$NODE1_HTTP_PORT" "/api/mail_server" "$body")"
	MAIL_SERVER_ID="$(echo "$response" | jq -r '.data.id // empty')"
	if [ -z "$MAIL_SERVER_ID" ]; then
		echo "ERROR: failed to create mail_server" >&2
		echo "$response" >&2
		exit 1
	fi
	log "Created mail_server ${MAIL_SERVER_ID} (${MAIL_SERVER_HOSTNAME})"
}

create_self_certificate() {
	local hostname="$1"
	local body response certificate_id action_response
	body="$(jq -n --arg hostname "$hostname" '{data:{type:"certificate",attributes:{hostname:$hostname,issuer:"self"}}}')"
	response="$(api_post_jsonapi "$NODE1_HTTP_PORT" "/api/certificate" "$body")"
	certificate_id="$(echo "$response" | jq -r '.data.id // empty')"
	if [ -z "$certificate_id" ]; then
		echo "ERROR: failed to create certificate row for ${hostname}" >&2
		echo "$response" >&2
		exit 1
	fi
	action_response="$(api_post "$NODE1_HTTP_PORT" "/action/certificate/generate_self_certificate" "$(jq -n --arg id "$certificate_id" '{attributes:{certificate_id:$id}}')")"
	if ! echo "$action_response" | jq -e '.[]? | select(.ResponseType == "client.notify")' >/dev/null; then
		echo "ERROR: failed to generate self certificate for ${hostname}" >&2
		echo "$action_response" >&2
		exit 1
	fi
	log "Generated self certificate for ${hostname}"
}

create_users() {
	local prefix="$1" count="$2"
	local failure_dir="$TMP_DIR/${prefix}-signup-failures"
	rm -rf "$failure_dir"
	mkdir -p "$failure_dir"
	seq 1 "$count" | xargs -P "$CONCURRENCY" -I{} bash -c '
		set -euo pipefail
		i="$1"
		prefix="$2"
		domain="$3"
		port1="$4"
		port2="$5"
		port3="$6"
		failure_dir="$7"
		case $(( (i - 1) % 3 )) in
			0) port="$port1" ;;
			1) port="$port2" ;;
			*) port="$port3" ;;
		esac
		email="$(printf "%s-%04d@%s" "$prefix" "$i" "$domain")"
		response_file="${failure_dir}/${prefix}-${i}.response"
		status="$(curl -s --max-time 30 -o "$response_file" -w "%{http_code}" \
			-X POST "http://127.0.0.1:${port}/action/user_account/signup" \
			-H "Content-Type: application/json" \
			-d "$(jq -n --arg email "$email" "{attributes:{name:\"Mail Flow User\",email:\$email,password:\"receiverpass\",passwordConfirm:\"receiverpass\"}}")" || true)"
		if [ "$status" != "200" ] && [ "$status" != "201" ]; then
			if [ -f "$response_file" ]; then
				mv "$response_file" "${failure_dir}/${prefix}-${i}-${status}.response"
			else
				printf "curl failed before writing a response body\n" > "${failure_dir}/${prefix}-${i}-${status}.response"
			fi
			exit 1
		fi
		rm -f "$response_file"
	' _ {} "$prefix" "$DOMAIN" "$NODE1_HTTP_PORT" "$NODE2_HTTP_PORT" "$NODE3_HTTP_PORT" "$failure_dir"
	if find "$failure_dir" -type f | grep -q .; then
		log "Signup failures for prefix ${prefix}:"
		find "$failure_dir" -type f | sort | head -20 | while read -r failure_file; do
			log "$(basename "$failure_file"): $(tr "\n" " " < "$failure_file" | cut -c1-500)"
		done
		return 1
	fi
}

trigger_reset_passwords() {
	local prefix="$1" count="$2"
	local failure_dir="$TMP_DIR/${prefix}-reset-failures"
	rm -rf "$failure_dir"
	mkdir -p "$failure_dir"
	seq 1 "$count" | xargs -P "$CONCURRENCY" -I{} bash -c '
		set -euo pipefail
		i="$1"
		prefix="$2"
		domain="$3"
		port1="$4"
		port2="$5"
		port3="$6"
		token="$7"
		failure_dir="$8"
		case $(( (i - 1) % 3 )) in
			0) port="$port1" ;;
			1) port="$port2" ;;
			*) port="$port3" ;;
		esac
		email="$(printf "%s-%04d@%s" "$prefix" "$i" "$domain")"
		response_file="${failure_dir}/${prefix}-${i}.response"
		status="$(curl -s --max-time 45 -o "$response_file" -w "%{http_code}" \
			-X POST "http://127.0.0.1:${port}/action/user_account/reset-password" \
			-H "Authorization: Bearer ${token}" \
			-H "Content-Type: application/json" \
			-d "$(jq -n --arg email "$email" "{attributes:{email:\$email}}")" || true)"
		if [ "$status" != "200" ]; then
			if [ -f "$response_file" ]; then
				mv "$response_file" "${failure_dir}/${prefix}-${i}-${status}.response"
			else
				printf "curl failed before writing a response body\n" > "${failure_dir}/${prefix}-${i}-${status}.response"
			fi
			exit 1
		fi
		rm -f "$response_file"
	' _ {} "$prefix" "$DOMAIN" "$NODE1_HTTP_PORT" "$NODE2_HTTP_PORT" "$NODE3_HTTP_PORT" "$(cat "$TOKEN_FILE")" "$failure_dir"
	if find "$failure_dir" -type f | grep -q .; then
		log "Reset-password failures for prefix ${prefix}:"
		find "$failure_dir" -type f | sort | head -20 | while read -r failure_file; do
			log "$(basename "$failure_file"): $(tr "\n" " " < "$failure_file" | cut -c1-500)"
		done
		return 1
	fi
}

wait_for_mailpit_count() {
	local expected="$1" max_wait="${2:-120}"
	for _ in $(seq 1 "$max_wait"); do
		local count
		count="$(mailpit_count)"
		if [ "$count" -ge "$expected" ]; then
			return 0
		fi
		sleep 1
	done
	return 1
}

set_mailpit_chaos() {
	local body="$1"
	curl -s --max-time 10 \
		-X PUT "http://127.0.0.1:${MAILPIT_HTTP_PORT}/api/v1/chaos" \
		-H "Content-Type: application/json" \
		-d "$body" >/dev/null
}

trigger_process_outbox_cluster() {
	local token
	token="$(cat "$TOKEN_FILE")"
	local pids=()
	for port in "$NODE1_HTTP_PORT" "$NODE2_HTTP_PORT" "$NODE3_HTTP_PORT"; do
		curl -s --max-time 60 \
			-X POST "http://127.0.0.1:${port}/action/outbox/process_outbox" \
			-H "Authorization: Bearer ${token}" \
			-H "Content-Type: application/json" \
			-d '{"attributes":{}}' >/dev/null &
		pids+=("$!")
	done
	for pid in "${pids[@]}"; do
		wait "$pid"
	done
}

assert_mailpit_recipients() {
	local prefix="$1" expected="$2"
	local messages_json unique_count duplicate_count
	messages_json="$(mailpit_api "/api/v1/messages?limit=$((LOAD_COUNT + FAIL_COUNT + 50))")"
	unique_count="$(echo "$messages_json" | jq -r --arg prefix "$prefix" --arg domain "$DOMAIN" '
		[.messages[].To[]?.Address | select(test("^" + $prefix + "-[0-9]{4}@" + $domain + "$"))] | unique | length
	')"
	duplicate_count="$(echo "$messages_json" | jq -r --arg prefix "$prefix" --arg domain "$DOMAIN" '
		[.messages[].To[]?.Address | select(test("^" + $prefix + "-[0-9]{4}@" + $domain + "$"))]
		| group_by(.) | map(select(length > 1)) | length
	')"
	[ "$unique_count" = "$expected" ] && pass "Mailpit received ${expected} unique ${prefix} reset-password messages" || fail "Mailpit unique ${prefix} recipients = ${unique_count}, want ${expected}"
	[ "$duplicate_count" = "0" ] && pass "Mailpit has no duplicate ${prefix} recipient deliveries" || fail "Mailpit duplicate ${prefix} recipient groups = ${duplicate_count}"
}

main() {
	require_cmd docker
	require_cmd curl
	require_cmd jq
	require_cmd python3
	require_cmd xargs
	mkdir -p "$ARTIFACT_DIR" "$PROFILE_DIR" "$SAMPLE_DIR" "$LOG_DIR"

	NODE1_HTTP_PORT="$(free_port)"
	NODE2_HTTP_PORT="$(free_port)"
	NODE3_HTTP_PORT="$(free_port)"
	MAILPIT_HTTP_PORT="$(free_port)"

	log "Building Daptin test binary in Docker..."
	docker run --rm \
		-v "$DAPTIN_DIR:/src:ro" \
		-v "$TMP_DIR:/work" \
		-v daptin-mail-flow-gomod:/go/pkg/mod \
		-v daptin-mail-flow-gocache:/root/.cache/go-build \
		-e GOTMPDIR=/work/gotmp \
		-e TMPDIR=/work/gotmp \
		golang:1.25-bookworm \
		sh -lc 'mkdir -p /work/gotmp && cd /src && /usr/local/go/bin/go build -o /work/daptin .' >/dev/null

	log "Creating Docker network ${NETWORK}..."
	docker network create "$NETWORK" >/dev/null

	log "Starting PostgreSQL..."
	docker run -d --name "$PG_CONTAINER" --network "$NETWORK" --network-alias postgres \
		-e POSTGRES_USER="$PG_USER" \
		-e POSTGRES_PASSWORD="$PG_PASS" \
		-e POSTGRES_DB="$PG_DB" \
		postgres:16 >/dev/null
	for _ in $(seq 1 60); do
		if docker exec "$PG_CONTAINER" pg_isready -U "$PG_USER" >/dev/null 2>&1; then
			break
		fi
		sleep 1
	done

	log "Starting Mailpit receiver (${MAILPIT_IMAGE})..."
	docker run -d --name "$MAILPIT_CONTAINER" --network "$NETWORK" \
		-p "127.0.0.1:${MAILPIT_HTTP_PORT}:8025" \
		-e MP_SMTP_BIND_ADDR="0.0.0.0:25" \
		-e MP_UI_BIND_ADDR="0.0.0.0:8025" \
		-e MP_ENABLE_CHAOS="true" \
		-e MP_MAX_MESSAGES="$MAILPIT_MAX_MESSAGES" \
		"$MAILPIT_IMAGE" >/dev/null
	if ! wait_for_mailpit; then
		echo "ERROR: Mailpit API did not become ready" >&2
		docker logs "$MAILPIT_CONTAINER" >&2 || true
		exit 1
	fi

	write_dns_config "$(container_ip "$MAILPIT_CONTAINER")"
	log "Starting CoreDNS..."
	docker run -d --name "$DNS_CONTAINER" --network "$NETWORK" \
		-v "$DNS_DIR:/zones:ro" \
		coredns/coredns:1.11.3 -conf /zones/Corefile >/dev/null

	log "Starting 3-node Daptin cluster..."
	start_node "$NODE1" "$NODE1_HTTP_PORT" 5336
	if ! wait_for_http "$NODE1_HTTP_PORT" "node1"; then
		docker logs "$NODE1" >&2 || true
		exit 1
	fi
	start_node "$NODE2" "$NODE2_HTTP_PORT" 5338
	start_node "$NODE3" "$NODE3_HTTP_PORT" 5340
	wait_for_http "$NODE2_HTTP_PORT" "node2" || { docker logs "$NODE2" >&2 || true; exit 1; }
	wait_for_http "$NODE3_HTTP_PORT" "node3" || { docker logs "$NODE3" >&2 || true; exit 1; }

	start_sampler
	set_phase "bootstrap"
	bootstrap_admin
	create_mail_server
	config_set "$NODE1_HTTP_PORT" "mail.default_server_hostname" "$MAIL_SERVER_HOSTNAME"
	create_self_certificate "localhost"
	curl -s --max-time 10 -X DELETE "http://127.0.0.1:${MAILPIT_HTTP_PORT}/api/v1/messages" -H "Content-Type: application/json" -d '{}' >/dev/null

	set_phase "load-signup"
	log "Creating ${LOAD_COUNT} users..."
	create_users "load" "$LOAD_COUNT"

	set_phase "load-reset-password"
	log "Triggering ${LOAD_COUNT} reset-password actions across 3 nodes with concurrency ${CONCURRENCY}..."
	trigger_reset_passwords "load" "$LOAD_COUNT"
	if wait_for_mailpit_count "$LOAD_COUNT" 180; then
		pass "Mailpit received at least ${LOAD_COUNT} messages after user-flow load"
	else
		fail "Mailpit did not receive ${LOAD_COUNT} messages; got $(mailpit_count)"
	fi
	assert_mailpit_recipients "load" "$LOAD_COUNT"

	local sent_load unsent_load retry_load
	sent_load="$(pg_exec "SELECT count(*) FROM outbox WHERE to_address LIKE 'load-%@${DOMAIN}' AND from_address='${MAIL_FROM}' AND sent = true;")"
	unsent_load="$(pg_exec "SELECT count(*) FROM outbox WHERE to_address LIKE 'load-%@${DOMAIN}' AND from_address='${MAIL_FROM}' AND sent = false;")"
	retry_load="$(pg_exec "SELECT count(*) FROM outbox WHERE to_address LIKE 'load-%@${DOMAIN}' AND from_address='${MAIL_FROM}' AND retry_count <> 0;")"
	[ "$sent_load" = "$LOAD_COUNT" ] && pass "all load outbox rows marked sent" || fail "load sent rows = ${sent_load}, want ${LOAD_COUNT}"
	[ "$unsent_load" = "0" ] && pass "no unsent load outbox rows remain" || fail "unsent load rows = ${unsent_load}"
	[ "$retry_load" = "0" ] && pass "load deliveries did not require retry bookkeeping" || fail "load rows with retry_count != 0 = ${retry_load}"

	set_phase "fail-signup"
	log "Enabling Mailpit recipient chaos for ${FAIL_COUNT} failure-path actions..."
	set_mailpit_chaos '{"Recipient":{"ErrorCode":451,"Probability":100}}'
	create_users "fail" "$FAIL_COUNT"
	local before_fail_count
	before_fail_count="$(mailpit_count)"
	set_phase "fail-reset-password"
	trigger_reset_passwords "fail" "$FAIL_COUNT"
	sleep 3
	local after_fail_count
	after_fail_count="$(mailpit_count)"
	[ "$after_fail_count" = "$before_fail_count" ] && pass "Mailpit chaos rejected failure batch without storing messages" || fail "Mailpit stored messages during chaos: before=${before_fail_count}, after=${after_fail_count}"

	local failed_unsent failed_retry failed_error
	failed_unsent="$(pg_exec "SELECT count(*) FROM outbox WHERE to_address LIKE 'fail-%@${DOMAIN}' AND from_address='${MAIL_FROM}' AND sent = false;")"
	failed_retry="$(pg_exec "SELECT count(*) FROM outbox WHERE to_address LIKE 'fail-%@${DOMAIN}' AND from_address='${MAIL_FROM}' AND retry_count = 1;")"
	failed_error="$(pg_exec "SELECT count(*) FROM outbox WHERE to_address LIKE 'fail-%@${DOMAIN}' AND from_address='${MAIL_FROM}' AND coalesce(last_error, '') <> '';")"
	[ "$failed_unsent" = "$FAIL_COUNT" ] && pass "chaos failure rows remain unsent" || fail "chaos unsent rows = ${failed_unsent}, want ${FAIL_COUNT}"
	[ "$failed_retry" = "$FAIL_COUNT" ] && pass "chaos failure rows increment retry_count to 1" || fail "chaos retry_count=1 rows = ${failed_retry}, want ${FAIL_COUNT}"
	[ "$failed_error" = "$FAIL_COUNT" ] && pass "chaos failure rows store last_error" || fail "chaos rows with last_error = ${failed_error}, want ${FAIL_COUNT}"

	set_phase "retry-drain"
	log "Disabling chaos and draining retry queue concurrently across the cluster..."
	set_mailpit_chaos '{}'
	pg_exec "UPDATE outbox SET next_retry_at = now() - interval '1 minute' WHERE to_address LIKE 'fail-%@${DOMAIN}' AND from_address='${MAIL_FROM}';" >/dev/null
	sleep "$((CLAIM_TTL_SECONDS + 1))"
	trigger_process_outbox_cluster
	if wait_for_mailpit_count "$((LOAD_COUNT + FAIL_COUNT))" 180; then
		pass "Mailpit received retry-drained failure batch"
	else
		fail "Mailpit did not receive retry-drained batch; got $(mailpit_count)"
	fi
	assert_mailpit_recipients "fail" "$FAIL_COUNT"

	local fail_sent fail_unsent_after
	fail_sent="$(pg_exec "SELECT count(*) FROM outbox WHERE to_address LIKE 'fail-%@${DOMAIN}' AND from_address='${MAIL_FROM}' AND sent = true;")"
	fail_unsent_after="$(pg_exec "SELECT count(*) FROM outbox WHERE to_address LIKE 'fail-%@${DOMAIN}' AND from_address='${MAIL_FROM}' AND sent = false;")"
	[ "$fail_sent" = "$FAIL_COUNT" ] && pass "retry-drained failure rows marked sent" || fail "retry-drained sent rows = ${fail_sent}, want ${FAIL_COUNT}"
	[ "$fail_unsent_after" = "0" ] && pass "no unsent failure rows remain after retry drain" || fail "unsent failure rows after retry drain = ${fail_unsent_after}"

	set_phase "complete"
	sample_once "complete" "$(date +%Y%m%d-%H%M%S)-final" || true
	collect_artifacts || true
	analyze_profiles || true
	log "Results: ${PASS} passed, ${FAIL} failed"
	log "Artifacts: ${ARTIFACT_DIR}"
	if [ "$FAIL" -ne 0 ]; then
		log "Outbox fail rows:"
		pg_exec "SELECT id, to_address, sent, retry_count, next_retry_at, coalesce(last_error, '') FROM outbox WHERE to_address LIKE 'fail-%@${DOMAIN}' ORDER BY id;" || true
		log "Outbox pending summary:"
		pg_exec "SELECT sent, retry_count, count(*) FROM outbox WHERE to_address LIKE '%@${DOMAIN}' GROUP BY sent, retry_count ORDER BY sent, retry_count;" || true
		log "Mailpit message summary:"
		mailpit_api "/api/v1/messages?limit=$((LOAD_COUNT + FAIL_COUNT + 50))" | jq -r '.messages[]? | [.To[0].Address, .Subject] | @tsv' || true
		log "Node1 tail:"
		docker logs --tail 80 "$NODE1" 2>&1 || true
		log "Node2 tail:"
		docker logs --tail 80 "$NODE2" 2>&1 || true
		log "Node3 tail:"
		docker logs --tail 80 "$NODE3" 2>&1 || true
		log "Mailpit tail:"
		docker logs --tail 80 "$MAILPIT_CONTAINER" 2>&1 || true
	fi
	[ "$FAIL" -eq 0 ]
}

main "$@"
