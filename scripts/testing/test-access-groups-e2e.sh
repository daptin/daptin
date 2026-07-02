#!/usr/bin/env bash
set -euo pipefail

# Real black-box E2E for AccessGroups authorization patterns.
# Starts an independent Daptin server with isolated SQLite DB/schema/storage,
# then verifies scenarios through HTTP only.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
TMP_DIR="$(mktemp -d)"
SERVER_PID=""

cleanup() {
  if [[ -n "${SERVER_PID}" ]] && kill -0 "$SERVER_PID" >/dev/null 2>&1; then
    kill "$SERVER_PID" >/dev/null 2>&1 || true
    wait "$SERVER_PID" >/dev/null 2>&1 || true
  fi
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

require_command go
require_command curl
require_command python3

free_port() {
  python3 - <<'PY'
import socket
s = socket.socket()
s.bind(("127.0.0.1", 0))
print(s.getsockname()[1])
s.close()
PY
}

PORT="$(free_port)"
HTTPS_PORT="$(free_port)"
BASE_URL="http://127.0.0.1:${PORT}"
BIN_PATH="$TMP_DIR/daptin-access-groups-e2e"
LOG_PATH="$TMP_DIR/daptin.log"

cat > "$TMP_DIR/schema_access_groups_e2e.yaml" <<'YAML'
Tables:
  - TableName: public_page
    Permission: 3
    DefaultPermission: 2
    Columns:
      - Name: title
        DataType: varchar(100)
        ColumnType: label

  - TableName: private_note
    Permission: 16384
    DefaultPermission: 1
    AccessGroups:
      - Name: users
        Permission: 114688
    DefaultGroups:
      - Name: users
        Permission: 49152
    Columns:
      - Name: title
        DataType: varchar(100)
        ColumnType: label

  - TableName: owner_note
    Permission: 16384
    DefaultPermission: 256
    AccessGroups:
      - Name: users
        Permission: 114688
    Columns:
      - Name: title
        DataType: varchar(100)
        ColumnType: label

  - TableName: mixed_article
    Permission: 3
    DefaultPermission: 2
    Columns:
      - Name: title
        DataType: varchar(100)
        ColumnType: label

  - TableName: workspace_item
    Permission: 16384
    DefaultPermission: 1
    AccessGroups:
      - Name: users
        Permission: 245760
    Columns:
      - Name: title
        DataType: varchar(100)
        ColumnType: label

  - TableName: action_doc
    Permission: 524288
    AccessGroups:
      - Name: users
        Permission: 524288
    Columns:
      - Name: title
        DataType: varchar(100)
        ColumnType: label

Actions:
  - Name: allowed_action
    Label: Allowed action
    OnType: action_doc
    InstanceOptional: true
    Permission: 0
    AccessGroups:
      - Name: users
        Permission: 524288
    OutFields:
      - Method: ACTIONRESPONSE
        Type: client.notify
        Attributes:
          type: success
          title: Access
          message: allowed

  - Name: denied_action
    Label: Denied action
    OnType: action_doc
    InstanceOptional: true
    Permission: 0
    OutFields:
      - Method: ACTIONRESPONSE
        Type: client.notify
        Attributes:
          type: success
          title: Access
          message: denied
YAML

echo "Building Daptin binary..."
(cd "$PROJECT_ROOT" && go build -o "$BIN_PATH" .)

mkdir -p "$TMP_DIR/storage"
echo "Starting isolated Daptin on ${BASE_URL}..."
(
  cd "$PROJECT_ROOT"
  DAPTIN_SCHEMA_FOLDER="$TMP_DIR" "$BIN_PATH" \
    -port ":$PORT" \
    -https_port ":$HTTPS_PORT" \
    -db_type sqlite3 \
    -db_connection_string "$TMP_DIR/daptin.db" \
    -local_storage_path "$TMP_DIR/storage" \
    -runtime test \
    -log_level error
) >"$LOG_PATH" 2>&1 &
SERVER_PID="$!"

for _ in $(seq 1 120); do
  if curl -fsS --max-time 2 "$BASE_URL/api/world" >/dev/null 2>&1; then
    break
  fi
  if ! kill -0 "$SERVER_PID" >/dev/null 2>&1; then
    echo "Daptin exited before readiness" >&2
    tail -80 "$LOG_PATH" >&2 || true
    exit 1
  fi
  sleep 1
done

if ! curl -fsS --max-time 2 "$BASE_URL/api/world" >/dev/null 2>&1; then
  echo "Daptin did not become ready" >&2
  tail -80 "$LOG_PATH" >&2 || true
  exit 1
fi

BODY_FILE="$TMP_DIR/response.json"

request() {
  local method="$1"
  local path="$2"
  local token="$3"
  local payload="${4:-}"
  local expected="$5"
  local content_type="application/json"
  [[ "$path" == /api/* ]] && content_type="application/vnd.api+json"

  local args=(-sS --max-time 20 -o "$BODY_FILE" -w "%{http_code}" -X "$method" "$BASE_URL$path")
  if [[ -n "$token" ]]; then
    args+=(-H "Authorization: Bearer $token")
  fi
  if [[ -n "$payload" ]]; then
    args+=(-H "Content-Type: $content_type" -d "$payload")
  fi

  local code
  code="$(curl "${args[@]}")"
  if [[ "$code" != "$expected" ]]; then
    echo "FAILED: $method $path returned $code, expected $expected" >&2
    cat "$BODY_FILE" >&2 || true
    echo "" >&2
    tail -80 "$LOG_PATH" >&2 || true
    exit 1
  fi
}

json_value() {
  python3 - "$1" "$BODY_FILE" <<'PY'
import json, sys
key = sys.argv[1]
with open(sys.argv[2]) as f:
    data = json.load(f)

def walk(v):
    if isinstance(v, dict):
        for k, val in v.items():
            if k.lower() == key.lower() and isinstance(val, str):
                print(val)
                return True
            if walk(val):
                return True
    if isinstance(v, list):
        for item in v:
            if walk(item):
                return True
    return False

if not walk(data):
    sys.exit(1)
PY
}

resource_id() {
  python3 - "$BODY_FILE" <<'PY'
import json, sys
with open(sys.argv[1]) as f:
    data = json.load(f)
print(data["data"]["id"])
PY
}

data_count() {
  python3 - "$BODY_FILE" <<'PY'
import json, sys
with open(sys.argv[1]) as f:
    data = json.load(f)
print(len(data["data"]))
PY
}

find_id_by_attr() {
  local entity="$1"
  local attr="$2"
  local value="$3"
  local token="$4"
  request GET "/api/${entity}?page%5Bsize%5D=200" "$token" "" 200
  python3 - "$BODY_FILE" "$attr" "$value" <<'PY'
import json, sys
with open(sys.argv[1]) as f:
    data = json.load(f)
attr, value = sys.argv[2], sys.argv[3]
for item in data["data"]:
    if item.get("attributes", {}).get(attr) == value:
        print(item["id"])
        break
else:
    raise SystemExit(f"not found: {attr}={value}")
PY
}

record_payload() {
  local entity="$1"
  local id="$2"
  local attrs="$3"
  if [[ -n "$id" ]]; then
    printf '{"data":{"type":"%s","id":"%s","attributes":%s}}' "$entity" "$id" "$attrs"
  else
    printf '{"data":{"type":"%s","attributes":%s}}' "$entity" "$attrs"
  fi
}

signup_signin() {
  local local_part="$1"
  local admin_token="${2:-}"
  local email="${local_part}@test.local"
  local password="testpass123"
  request POST "/action/user_account/signup" "$admin_token" \
    "{\"attributes\":{\"email\":\"${email}\",\"password\":\"${password}\",\"passwordConfirm\":\"${password}\",\"name\":\"${local_part}\"}}" 200
  request POST "/action/user_account/signin" "" \
    "{\"attributes\":{\"email\":\"${email}\",\"password\":\"${password}\"}}" 200
  json_value value
}

create_record() {
  local entity="$1"
  local token="$2"
  local attrs="$3"
  request POST "/api/${entity}" "$token" "$(record_payload "$entity" "" "$attrs")" 201
  resource_id
}

patch_record() {
  local entity="$1"
  local id="$2"
  local token="$3"
  local attrs="$4"
  request PATCH "/api/${entity}/${id}" "$token" "$(record_payload "$entity" "$id" "$attrs")" 200
}

create_join() {
  local entity="$1"
  local token="$2"
  local attrs="$3"
  local permission="${4:-0}"
  request POST "/api/${entity}" "$token" "$(record_payload "$entity" "" "$attrs")" 201
  local id
  id="$(resource_id)"
  if [[ "$permission" != "0" ]]; then
    patch_record "$entity" "$id" "$token" "{\"permission\":${permission}}"
  fi
  echo "$id"
}

assert_list_count() {
  local entity="$1"
  local token="$2"
  local expected="$3"
  request GET "/api/${entity}?page%5Bsize%5D=100" "$token" "" 200
  local actual
  actual="$(data_count)"
  if [[ "$actual" != "$expected" ]]; then
    echo "FAILED: expected ${entity} count ${expected}, got ${actual}" >&2
    cat "$BODY_FILE" >&2
    exit 1
  fi
}

echo "Creating users..."
ADMIN_TOKEN="$(signup_signin admin)"
request POST "/action/world/become_an_administrator" "$ADMIN_TOKEN" '{"attributes":{}}' 200
USER_TOKEN="$(signup_signin user "$ADMIN_TOKEN")"
OTHER_TOKEN="$(signup_signin other "$ADMIN_TOKEN")"
EDITOR_TOKEN="$(signup_signin editor "$ADMIN_TOKEN")"
MEMBER_TOKEN="$(signup_signin member "$ADMIN_TOKEN")"

echo "Scenario: public site"
create_record public_page "$ADMIN_TOKEN" '{"title":"public"}' >/dev/null
assert_list_count public_page "" 1
request POST "/api/public_page" "" "$(record_payload public_page "" '{"title":"guest write"}')" 403

echo "Scenario: private site"
create_record private_note "$ADMIN_TOKEN" '{"title":"private"}' >/dev/null
request GET "/api/private_note" "" "" 403
assert_list_count private_note "$USER_TOKEN" 1
request POST "/api/private_note" "$USER_TOKEN" "$(record_payload private_note "" '{"title":"user private"}')" 201

echo "Scenario: semi-private owner rows"
create_record owner_note "$USER_TOKEN" '{"title":"owned by user"}' >/dev/null
assert_list_count owner_note "$USER_TOKEN" 1
assert_list_count owner_note "$OTHER_TOKEN" 0

echo "Scenario: mixed public/private rows"
create_record mixed_article "$ADMIN_TOKEN" '{"title":"public article"}' >/dev/null
PRIVATE_ARTICLE_ID="$(create_record mixed_article "$ADMIN_TOKEN" '{"title":"private article"}')"
patch_record mixed_article "$PRIVATE_ARTICLE_ID" "$ADMIN_TOKEN" '{"permission":0}'
assert_list_count mixed_article "" 1
assert_list_count mixed_article "$ADMIN_TOKEN" 2

echo "Scenario: shared group workspace"
EDITORS_GROUP_ID="$(create_record usergroup "$ADMIN_TOKEN" '{"name":"e2e_shell_editors"}')"
MEMBERS_GROUP_ID="$(create_record usergroup "$ADMIN_TOKEN" '{"name":"e2e_shell_members"}')"
EDITOR_USER_ID="$(find_id_by_attr user_account email editor@test.local "$ADMIN_TOKEN")"
MEMBER_USER_ID="$(find_id_by_attr user_account email member@test.local "$ADMIN_TOKEN")"
create_join user_account_user_account_id_has_usergroup_usergroup_id "$ADMIN_TOKEN" "{\"user_account_id\":\"${EDITOR_USER_ID}\",\"usergroup_id\":\"${EDITORS_GROUP_ID}\"}" >/dev/null
create_join user_account_user_account_id_has_usergroup_usergroup_id "$ADMIN_TOKEN" "{\"user_account_id\":\"${MEMBER_USER_ID}\",\"usergroup_id\":\"${MEMBERS_GROUP_ID}\"}" >/dev/null
WORKSPACE_ITEM_ID="$(create_record workspace_item "$ADMIN_TOKEN" '{"title":"shared"}')"
patch_record workspace_item "$WORKSPACE_ITEM_ID" "$ADMIN_TOKEN" '{"permission":1}'
create_join workspace_item_workspace_item_id_has_usergroup_usergroup_id "$ADMIN_TOKEN" "{\"workspace_item_id\":\"${WORKSPACE_ITEM_ID}\",\"usergroup_id\":\"${EDITORS_GROUP_ID}\"}" 180224 >/dev/null
create_join workspace_item_workspace_item_id_has_usergroup_usergroup_id "$ADMIN_TOKEN" "{\"workspace_item_id\":\"${WORKSPACE_ITEM_ID}\",\"usergroup_id\":\"${MEMBERS_GROUP_ID}\"}" 49152 >/dev/null
assert_list_count workspace_item "$EDITOR_TOKEN" 1
assert_list_count workspace_item "$MEMBER_TOKEN" 1
patch_record workspace_item "$WORKSPACE_ITEM_ID" "$EDITOR_TOKEN" '{"title":"edited"}'
request PATCH "/api/workspace_item/${WORKSPACE_ITEM_ID}" "$MEMBER_TOKEN" "$(record_payload workspace_item "$WORKSPACE_ITEM_ID" '{"title":"member edit"}')" 403

echo "Scenario: action two-gate access"
request POST "/action/action_doc/allowed_action" "$USER_TOKEN" '{"attributes":{}}' 200
request POST "/action/action_doc/denied_action" "$USER_TOKEN" '{"attributes":{}}' 403
request POST "/action/action_doc/allowed_action" "" '{"attributes":{}}' 403

echo ""
echo "✅ AccessGroups real shell E2E passed"
