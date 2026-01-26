# Daptin Testing Guide

When testing Daptin features, follow this setup guide to avoid repeating mistakes.

---

## CHECK WIKI DOCS FIRST

Before searching codebase, check wiki documentation at `wiki/`:
- `wiki/Action-Reference.md` - Complete list of all actions with parameters
- `wiki/Schema-Definition.md` - How to define tables/schemas
- `wiki/Admin-Actions.md` - Admin operations
- `wiki/Data-Actions.md` - Import/export actions
- `wiki/Cloud-Storage.md` - Cloud storage setup
- `wiki/Asset-Columns.md` - File column configuration

---

## USE THESE TOOLS FIRST

### test-runner.sh (Primary Tool)
Located at `scripts/testing/test-runner.sh` - use for ALL server lifecycle:

```bash
./scripts/testing/test-runner.sh check     # Check if running
./scripts/testing/test-runner.sh start     # Start server (kills old first)
./scripts/testing/test-runner.sh stop      # Stop server
./scripts/testing/test-runner.sh token     # Get auth token
./scripts/testing/test-runner.sh get /api/world    # GET request
./scripts/testing/test-runner.sh post /api/x '{}'  # POST request
./scripts/testing/test-runner.sh action entity action '{}'  # Call action
./scripts/testing/test-runner.sh logs      # Show logs
./scripts/testing/test-runner.sh errors    # Show errors only
```

Token file: `/tmp/daptin-token.txt`
Default creds: `admin@admin.com` / `adminadmin`

### Makefile Commands
```bash
make daptin      # Build binary
make test        # Run tests
make quicktest   # Quick tests
```

---

## CRITICAL: Kill Stale Processes First

**The #1 cause of "403 forbidden" and "Unauthorized" errors is stale Olric cache from old processes.**

### Understanding Daptin's Two Ports

Daptin uses TWO ports that must BOTH be killed:

1. **Port 6336**: HTTP API
2. **Port 5336**: Olric distributed cache ⚠️ **THIS IS THE CRITICAL ONE**

### Why Port 5336 Matters

The Olric distributed cache on port 5336 stores:
- Administrator reference IDs (60-minute TTL)
- Permission data
- User-group memberships

**If an old process holds port 5336**, its cache will:
- ❌ Make `become_an_administrator` return "Unauthorized" EVEN WITH FRESH DATABASE
- ❌ Cause 403 errors due to stale permission data
- ❌ Show wrong user-group relationships

### Always Kill Both Ports

```bash
# PREFERRED: Use test-runner.sh (now kills both ports)
./scripts/testing/test-runner.sh stop

# MANUAL: Kill both ports explicitly
lsof -i :6336 -t | xargs kill -9 2>/dev/null || true  # HTTP API
lsof -i :5336 -t | xargs kill -9 2>/dev/null || true  # Olric cache ⚠️ CRITICAL!

# Also kill by process name
pkill -9 -f daptin 2>/dev/null || true
pkill -9 -f "go run main" 2>/dev/null || true

sleep 2

# VERIFY both ports are free
lsof -i :6336 || echo "✓ Port 6336 free"
lsof -i :5336 || echo "✓ Port 5336 free (CRITICAL!)"
```

### Symptoms of Stale Olric Cache

If you forget to kill port 5336, you'll see:

**Symptom 1: "Unauthorized" on become_an_administrator**
```json
{"ResponseType": "client.notify", "Attributes": {"message": "Unauthorized"}}
```

**Symptom 2: 403 Forbidden on fresh database**
```json
{"errors": [{"status": "403", "title": "Forbidden"}]}
```

**Symptom 3: Wrong permissions**
- User should have access but gets 403
- Changes to permissions don't take effect

**Solution**: Kill port 5336 and restart

---

## Quick Start: Fresh Database Testing

```bash
# PREFERRED: Use test-runner.sh
./scripts/testing/test-runner.sh stop
rm -f daptin.db
./scripts/testing/test-runner.sh start

# Signup
curl -s -X POST http://localhost:6336/action/user_account/signup \
  -H "Content-Type: application/json" \
  -d '{"attributes":{"name":"Admin","email":"admin@admin.com","password":"adminadmin","passwordConfirm":"adminadmin"}}'

# Get token
./scripts/testing/test-runner.sh token

# OR MANUAL PROCESS:
# 1. KILL all old processes (CRITICAL!)
pkill -9 -f daptin 2>/dev/null || true
pkill -9 -f "go run main" 2>/dev/null || true
sleep 2

# 2. Remove old database for clean start
cd /Users/artpar/workspace/code/github.com/daptin/daptin
rm -f daptin.db

# 3. Start Daptin
nohup go run main.go > /tmp/daptin.log 2>&1 &
sleep 10

# 4. Signup - uses simpler password (adminadmin)
curl -s -X POST http://localhost:6336/action/user_account/signup \
  -H "Content-Type: application/json" \
  -d '{"attributes":{"name":"Admin","email":"admin@admin.com","password":"adminadmin","passwordConfirm":"adminadmin"}}'

# 5. Sign in - extract token correctly
TOKEN=$(curl -s -X POST http://localhost:6336/action/user_account/signin \
  -H "Content-Type: application/json" \
  -d '{"attributes":{"email":"admin@admin.com","password":"adminadmin"}}' | \
  jq -r '.[] | select(.ResponseType == "client.store.set") | .Attributes.value')

echo "$TOKEN" > /tmp/daptin-token.txt
```

---

## Using Existing Database

If database already has users:

```bash
# Check existing users
sqlite3 daptin.db "SELECT id, name, email FROM user_account;"

# To reset a password, need to use Go bcrypt (htpasswd won't work)
# INSTEAD: Create new admin via direct DB insert
sqlite3 daptin.db "INSERT INTO user_account (reference_id, name, email, password, permission) VALUES (lower(hex(randomblob(16))), 'TestAdmin', 'testadmin@test.com', '\$2a\$10\$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', 2097151);"

# Note: That hash = "password123" - use for testing only
```

---

## Token File Convention

Token file: `/tmp/daptin-token.txt` (used by test-runner.sh)

```bash
# Get token via test-runner.sh (PREFERRED)
./scripts/testing/test-runner.sh token

# Use with test-runner.sh
./scripts/testing/test-runner.sh get /api/world

# OR manual with TOKEN variable
TOKEN=$(cat /tmp/daptin-token.txt)
curl -H "Authorization: Bearer $TOKEN" http://localhost:6336/api/world
```

---

## Common Issues

### 403 Permission Denied
- Admin user must be in "administrators" usergroup (id=2)
- Check: `sqlite3 daptin.db "SELECT * FROM user_account_user_account_id_has_usergroup_usergroup_id WHERE usergroup_id=2;"`
- Fix: Insert join record if missing

### Password Validation Fails
- Password must be: 8+ chars, uppercase, lowercase, number, special char
- Example: `Test123!@#Secure`

### Signup Fails on Existing Database
- First admin locks signup by default
- Either use existing admin or start fresh database

---

## Cloud Storage Testing

### Local Filesystem Cloud Store

```bash
# Create test directory
mkdir -p /tmp/cloud-test-storage

# Create cloud_store record (use test-runner.sh for token)
TOKEN=$(cat /tmp/daptin-token.txt)
curl -X POST http://localhost:6336/api/cloud_store \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "cloud_store",
      "attributes": {
        "name": "local-test",
        "store_type": "local",
        "store_provider": "localstore",
        "root_path": "/tmp/cloud-test-storage"
      }
    }
  }'
```

### Minio (S3-compatible)

```bash
# Start Minio locally
docker run -d --name minio \
  -p 9000:9000 -p 9001:9001 \
  -e MINIO_ROOT_USER=minioadmin \
  -e MINIO_ROOT_PASSWORD=minioadmin123 \
  minio/minio server /data --console-address ":9001"

# Create bucket using mc inside container
docker exec minio mc alias set local http://localhost:9000 minioadmin minioadmin123
docker exec minio mc mb local/daptin-test

# CRITICAL: Credential content MUST include rclone fields: type, provider
# This is the format rclone expects - without "type":"s3" it will panic!
TOKEN=$(cat /tmp/daptin-token.txt)
curl -X POST http://localhost:6336/api/credential \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "credential",
      "attributes": {
        "name": "minio-creds",
        "content": "{\"type\":\"s3\",\"provider\":\"Minio\",\"env_auth\":\"false\",\"access_key_id\":\"minioadmin\",\"secret_access_key\":\"minioadmin123\",\"endpoint\":\"http://localhost:9000\",\"region\":\"us-east-1\"}"
      }
    }
  }'

# Create cloud_store
curl -X POST http://localhost:6336/api/cloud_store \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "cloud_store",
      "attributes": {
        "name": "minio-store",
        "store_type": "s3",
        "store_provider": "s3",
        "root_path": "minio-store:daptin-test",
        "credential_name": "minio-creds",
        "store_parameters": "{}"
      }
    }
  }'

# IMPORTANT: Link credential to cloud_store via relationship PATCH
CRED_ID=$(curl -s -H "Authorization: Bearer $TOKEN" http://localhost:6336/api/credential | jq -r '.data[0].id')
STORE_ID=$(curl -s -H "Authorization: Bearer $TOKEN" http://localhost:6336/api/cloud_store | jq -r '.data[] | select(.attributes.name == "minio-store") | .id')

curl -X PATCH "http://localhost:6336/api/cloud_store/$STORE_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d "{\"data\":{\"type\":\"cloud_store\",\"id\":\"$STORE_ID\",\"relationships\":{\"credential_id\":{\"data\":{\"type\":\"credential\",\"id\":\"$CRED_ID\"}}}}}"

# Restart server to pick up config
./scripts/testing/test-runner.sh stop && ./scripts/testing/test-runner.sh start
```

### AWS S3

```bash
# CRITICAL: Credential content MUST include rclone fields: type, provider
TOKEN=$(cat /tmp/daptin-token.txt)
curl -X POST http://localhost:6336/api/credential \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "credential",
      "attributes": {
        "name": "aws-s3-creds",
        "content": "{\"type\":\"s3\",\"provider\":\"AWS\",\"env_auth\":\"false\",\"access_key_id\":\"YOUR_KEY\",\"secret_access_key\":\"YOUR_SECRET\",\"region\":\"us-west-2\"}"
      }
    }
  }'

# Create cloud_store
curl -X POST http://localhost:6336/api/cloud_store \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "cloud_store",
      "attributes": {
        "name": "aws-s3-store",
        "store_type": "s3",
        "store_provider": "s3",
        "root_path": "aws-s3-store:your-bucket-name/optional-prefix",
        "credential_name": "aws-s3-creds",
        "store_parameters": "{}"
      }
    }
  }'

# Link credential via relationship (see Minio section for PATCH example)
```

### Google Cloud Storage

```bash
# CRITICAL: Credential content MUST include rclone fields: type, service_account_credentials
# Get your service account JSON from Google Cloud Console and escape it properly
TOKEN=$(cat /tmp/daptin-token.txt)
curl -X POST http://localhost:6336/api/credential \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "credential",
      "attributes": {
        "name": "gcs-creds",
        "content": "{\"type\":\"google cloud storage\",\"service_account_credentials\":\"{...SERVICE_ACCOUNT_JSON_ESCAPED...}\"}"
      }
    }
  }'

# Create cloud_store
curl -X POST http://localhost:6336/api/cloud_store \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "cloud_store",
      "attributes": {
        "name": "gcs-store",
        "store_type": "gcs",
        "store_provider": "gcs",
        "root_path": "gcs-store:your-bucket-name/optional-prefix",
        "credential_name": "gcs-creds",
        "store_parameters": "{}"
      }
    }
  }'

# IMPORTANT: Link credential to cloud_store via relationship PATCH (see Minio section)
```

---

## Asset Column Testing

### File Format (CRITICAL)

Files MUST be sent as array of objects:

```json
{
  "photo": [
    {
      "name": "image.png",
      "file": "data:image/png;base64,iVBORw0KGgo...",
      "type": "image/png"
    }
  ]
}
```

NOT as a string. NOT as a single object. ARRAY of objects.

### Schema for Cloud Storage Column

```json
{
  "Tables": [{
    "TableName": "product",
    "Columns": [{
      "Name": "photo",
      "DataType": "text",
      "ColumnType": "file",
      "IsForeignKey": true,
      "ForeignKeyData": {
        "DataSource": "cloud_store",
        "Namespace": "local-test",
        "KeyName": "products"
      }
    }]
  }]
}
```

- `Namespace`: Must match cloud_store `name` field exactly
- `KeyName`: Subfolder within root_path

---

## Restart Server After

Always restart Daptin after:
1. Creating new tables via schema API
2. Creating cloud_store records
3. Changes to world/column definitions

```bash
# PREFERRED: Use test-runner.sh
./scripts/testing/test-runner.sh stop && ./scripts/testing/test-runner.sh start

# OR manual
pkill -f "daptin" && sleep 2 && nohup go run main.go > /tmp/daptin.log 2>&1 &
```

---

## Port

Default port: 6336 (not 8181)

Check with: `curl http://localhost:6336/api/world | head -c 50`

---

## Complete Documentation

See `wiki/Cloud-Storage-Complete-Guide.md` for comprehensive end-user documentation covering:
- Core concepts and architecture
- Step-by-step setup for all cloud providers
- Credential format reference (rclone-compatible)
- File upload format requirements
- Common pitfalls and solutions
- Complete working examples

---

## Key Learnings Summary

### Credential Content Format
The `content` field is passed directly to rclone. It MUST include:
- `type` - The rclone remote type (e.g., "s3", "google cloud storage")
- Provider-specific fields (access keys, endpoints, etc.)

### The Relationship Link is Critical
`credential_name` field does NOT automatically link the credential.
You MUST use a relationship PATCH to link `credential_id`:
```bash
curl -X PATCH "/api/cloud_store/$STORE_ID" \
  -d '{"data":{"relationships":{"credential_id":{"data":{"type":"credential","id":"$CRED_ID"}}}}}'
```

### Server Restart Required
Restart after:
1. Creating cloud_store
2. Creating tables via schema API
3. Linking credential to cloud_store

### File Upload Format
Always array of objects: `[{name, file, type}]`
- `file`: Base64 data URI (`data:mime/type;base64,...`)
- `name`: Filename with extension
- `type`: MIME type

### Ports to Clear
- 6336: HTTP API
- 5336: Olric distributed cache (kills old process or 403 errors occur)
