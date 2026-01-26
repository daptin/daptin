# Daptin Cloud Storage - Complete End-User Guide

This guide documents everything you need to know to use Daptin's cloud storage feature, based on hands-on testing with local filesystem, Minio (S3-compatible), and AWS S3.

---

## Table of Contents

1. [Core Concepts](#core-concepts)
2. [Architecture Overview](#architecture-overview)
3. [Step-by-Step Setup](#step-by-step-setup)
4. [Credential Format Reference](#credential-format-reference)
5. [Creating Tables with File Columns](#creating-tables-with-file-columns)
6. [Uploading Files](#uploading-files)
7. [Common Pitfalls and Solutions](#common-pitfalls-and-solutions)
8. [Complete Working Examples](#complete-working-examples)

---

## Core Concepts

### What is Cloud Storage in Daptin?

Daptin allows you to store files (images, documents, etc.) in external storage backends instead of the database. Files are uploaded through Daptin's API and automatically synced to your configured storage.

### Key Entities

| Entity | Purpose |
|--------|---------|
| `credential` | Stores authentication details for cloud providers (encrypted) |
| `cloud_store` | Defines a storage location with provider type and root path |
| `file column` | A table column that stores files in a linked cloud_store |

### How It Works

```
User uploads file via API
        ↓
Daptin receives file + metadata
        ↓
Looks up cloud_store by name (from column's ForeignKeyData.Namespace)
        ↓
Fetches linked credential
        ↓
Configures rclone with credential data
        ↓
rclone copies file to destination
        ↓
Returns file metadata (md5, size, path)
```

---

## Architecture Overview

### The Three-Layer Model

```
┌─────────────────────────────────────────────────────┐
│                    YOUR TABLE                        │
│  ┌─────────┬──────────┬─────────────────────────┐   │
│  │  name   │  price   │  photo (file column)    │   │
│  └─────────┴──────────┴────────────┬────────────┘   │
│                                    │                 │
│            ForeignKeyData.Namespace = "my-store"    │
└────────────────────────────────────┼────────────────┘
                                     │
┌────────────────────────────────────▼────────────────┐
│                   CLOUD_STORE                        │
│  name: "my-store"                                    │
│  store_type: "s3"                                    │
│  root_path: "my-store:bucket-name/prefix"           │
│  credential_id: → (linked via relationship)         │
└────────────────────────────────────┬────────────────┘
                                     │
┌────────────────────────────────────▼────────────────┐
│                   CREDENTIAL                         │
│  name: "my-creds"                                    │
│  content: "{\"type\":\"s3\",\"provider\":\"AWS\"}"  │
│  (encrypted at rest)                                 │
└─────────────────────────────────────────────────────┘
```

### Important: The Relationship Link

The `cloud_store.credential_id` field must be linked via a **relationship PATCH request**. Setting `credential_name` alone is NOT sufficient - you must explicitly link the credential using the JSON:API relationships endpoint.

---

## Step-by-Step Setup

### Prerequisites

- Daptin server running (default port: 6336)
- Admin user authenticated
- JWT token saved to `/tmp/daptin-token.txt`

### Step 1: Create a Credential

The credential stores your cloud provider authentication. **Critical**: The `content` field must be a JSON string with rclone-compatible fields.

```bash
TOKEN=$(cat /tmp/daptin-token.txt)

curl -X POST http://localhost:6336/api/credential \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "credential",
      "attributes": {
        "name": "my-s3-creds",
        "content": "{\"type\":\"s3\",\"provider\":\"AWS\",\"env_auth\":\"false\",\"access_key_id\":\"AKIAIOSFODNN7EXAMPLE\",\"secret_access_key\":\"wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY\",\"region\":\"us-east-1\"}"
      }
    }
  }'
```

### Step 2: Create a Cloud Store

The cloud store defines where files are stored.

```bash
curl -X POST http://localhost:6336/api/cloud_store \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "cloud_store",
      "attributes": {
        "name": "my-store",
        "store_type": "s3",
        "store_provider": "s3",
        "root_path": "my-store:my-bucket-name",
        "store_parameters": "{}"
      }
    }
  }'
```

### Step 3: Link Credential to Cloud Store (CRITICAL)

This step is often missed! You must link the credential via a relationship PATCH:

```bash
# Get the credential ID
CRED_ID=$(curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:6336/api/credential | \
  jq -r '.data[] | select(.attributes.name == "my-s3-creds") | .id')

# Get the cloud store ID
STORE_ID=$(curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:6336/api/cloud_store | \
  jq -r '.data[] | select(.attributes.name == "my-store") | .id')

# Link them via relationship PATCH
curl -X PATCH "http://localhost:6336/api/cloud_store/$STORE_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d "{
    \"data\": {
      \"type\": \"cloud_store\",
      \"id\": \"$STORE_ID\",
      \"relationships\": {
        \"credential_id\": {
          \"data\": {
            \"type\": \"credential\",
            \"id\": \"$CRED_ID\"
          }
        }
      }
    }
  }"
```

### Step 4: Restart Server

**Important**: After creating cloud stores, restart the server to pick up the configuration:

```bash
# If using test-runner.sh
./scripts/testing/test-runner.sh stop && ./scripts/testing/test-runner.sh start

# Or manually
pkill -f daptin && sleep 2 && ./daptin
```

### Step 5: Create a Table with File Column

Upload a schema that references your cloud store:

```bash
curl -X POST "http://localhost:6336/api/world/action/upload_system_schema" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "schema_json": "{\"Tables\":[{\"TableName\":\"product\",\"Columns\":[{\"Name\":\"name\",\"DataType\":\"varchar(500)\",\"ColumnType\":\"name\"},{\"Name\":\"photo\",\"DataType\":\"text\",\"ColumnType\":\"file\",\"IsForeignKey\":true,\"ForeignKeyData\":{\"DataSource\":\"cloud_store\",\"Namespace\":\"my-store\",\"KeyName\":\"products\"}}]}]}"
    }
  }'
```

### Step 6: Restart Again

Restart after creating new tables:

```bash
./scripts/testing/test-runner.sh stop && ./scripts/testing/test-runner.sh start
```

---

## Credential Format Reference

### The Critical Rule

**Credentials MUST include rclone-compatible fields.** The `content` field is passed directly to rclone's configuration. If the format is wrong, you will get a nil pointer panic.

### S3 / Minio

```json
{
  "type": "s3",
  "provider": "AWS",
  "env_auth": "false",
  "access_key_id": "YOUR_ACCESS_KEY",
  "secret_access_key": "YOUR_SECRET_KEY",
  "region": "us-east-1"
}
```

For Minio, add endpoint:

```json
{
  "type": "s3",
  "provider": "Minio",
  "env_auth": "false",
  "access_key_id": "minioadmin",
  "secret_access_key": "minioadmin123",
  "endpoint": "http://localhost:9000",
  "region": "us-east-1"
}
```

### Google Cloud Storage

```json
{
  "type": "google cloud storage",
  "service_account_credentials": "{...escaped service account JSON...}"
}
```

### Azure Blob Storage

```json
{
  "type": "azureblob",
  "account": "your-storage-account",
  "key": "your-storage-key"
}
```

### Local Filesystem

No credential needed. Use these cloud_store settings:

```json
{
  "name": "local-store",
  "store_type": "local",
  "store_provider": "localstore",
  "root_path": "/path/to/storage"
}
```

---

## Creating Tables with File Columns

### Schema Structure

A file column uses `ForeignKeyData` to specify where files are stored:

```json
{
  "Name": "photo",
  "DataType": "text",
  "ColumnType": "file",
  "IsForeignKey": true,
  "ForeignKeyData": {
    "DataSource": "cloud_store",
    "Namespace": "my-store",
    "KeyName": "photos"
  }
}
```

| Field | Description |
|-------|-------------|
| `DataSource` | Always `"cloud_store"` for file columns |
| `Namespace` | Must match the `name` field of your cloud_store exactly |
| `KeyName` | Subfolder within the cloud store's root_path |

### File Storage Path

Files are stored at: `{root_path}/{KeyName}/{filename}`

Example:
- root_path: `my-store:my-bucket`
- KeyName: `photos`
- Uploaded file: `image.jpg`
- Final path: `my-bucket/photos/image.jpg`

---

## Uploading Files

### File Format (CRITICAL)

Files must be sent as an **array of objects** with base64-encoded content:

```json
{
  "photo": [
    {
      "name": "image.png",
      "file": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
      "type": "image/png"
    }
  ]
}
```

### Common Mistakes

| Wrong | Right |
|-------|-------|
| `"photo": "base64..."` | `"photo": [{...}]` |
| `"photo": {"name": "..."}` | `"photo": [{"name": "..."}]` |
| Missing `type` field | Include MIME type |

### Full Upload Example

```bash
TOKEN=$(cat /tmp/daptin-token.txt)

# Create base64 content
FILE_CONTENT=$(echo -n "Hello World" | base64)

curl -X POST http://localhost:6336/api/product \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "product",
      "attributes": {
        "name": "Test Product",
        "photo": [
          {
            "name": "test.txt",
            "file": "data:text/plain;base64,'"$FILE_CONTENT"'",
            "type": "text/plain"
          }
        ]
      }
    }
  }'
```

### Response Format

Successful upload returns file metadata:

```json
{
  "photo": [
    {
      "md5": "b10a8db164e0754105b7a99be72e3fe5",
      "name": "test.txt",
      "path": "",
      "size": 11,
      "src": "test.txt",
      "type": "text/plain"
    }
  ]
}
```

---

## Common Pitfalls and Solutions

### 1. Nil Pointer Panic on Upload

**Symptom**: Server crashes with `runtime error: invalid memory address or nil pointer dereference`

**Cause**: Credential not properly linked or credential content missing rclone fields

**Solution**:
1. Verify credential content includes `type` field
2. Verify credential is linked via relationship PATCH
3. Check with:
   ```bash
   curl -s -H "Authorization: Bearer $TOKEN" \
     "http://localhost:6336/api/cloud_store?include=credential_id" | jq
   ```

### 2. 403 Forbidden on Fresh Database

**Symptom**: All API calls return 403 even with valid token

**Cause**: Stale Olric cache from previous server process

**Solution**:
```bash
pkill -9 -f daptin
lsof -i :5336 -t | xargs kill -9 2>/dev/null
lsof -i :6336 -t | xargs kill -9 2>/dev/null
sleep 2
# Start fresh
```

### 3. New Table Returns HTML Instead of JSON

**Symptom**: GET /api/new_table returns HTML page

**Cause**: Routes not registered until server restart

**Solution**: Restart server after creating tables via schema API

### 4. File Upload Returns Success But File Not in Storage

**Symptom**: API returns 201 but file not found in bucket

**Cause**:
- Incorrect root_path format
- Bucket doesn't exist
- Credential permissions insufficient

**Solution**:
1. Verify bucket exists
2. Check root_path format: `{store_name}:{bucket}/{prefix}`
3. Verify credentials have write permissions

### 5. UNIQUE Constraint Failed on Credential

**Symptom**: Error creating credential or cloud_store

**Cause**: Stale join table entries from deleted records

**Solution**:
```bash
sqlite3 daptin.db "DELETE FROM credential_credential_id_has_usergroup_usergroup_id WHERE credential_id NOT IN (SELECT id FROM credential);"
```

---

## Complete Working Examples

### Example 1: Local Filesystem Storage

```bash
TOKEN=$(cat /tmp/daptin-token.txt)

# Daptin includes a default "localstore" cloud_store at ./storage
# Just create a table that references it:

curl -X POST "http://localhost:6336/api/world/action/upload_system_schema" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "schema_json": "{\"Tables\":[{\"TableName\":\"local_product\",\"Columns\":[{\"Name\":\"name\",\"DataType\":\"varchar(500)\",\"ColumnType\":\"name\"},{\"Name\":\"document\",\"DataType\":\"text\",\"ColumnType\":\"file\",\"IsForeignKey\":true,\"ForeignKeyData\":{\"DataSource\":\"cloud_store\",\"Namespace\":\"localstore\",\"KeyName\":\"documents\"}}]}]}"
    }
  }'

# Restart server
./scripts/testing/test-runner.sh stop && ./scripts/testing/test-runner.sh start

# Get fresh token
./scripts/testing/test-runner.sh token

# Upload a file
TOKEN=$(cat /tmp/daptin-token.txt)
curl -X POST http://localhost:6336/api/local_product \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "local_product",
      "attributes": {
        "name": "My Document",
        "document": [{"name":"readme.txt","file":"data:text/plain;base64,SGVsbG8gV29ybGQh","type":"text/plain"}]
      }
    }
  }'

# File is now at: ./storage/documents/readme.txt
```

### Example 2: Minio S3-Compatible Storage

```bash
# Start Minio
docker run -d --name minio \
  -p 9000:9000 -p 9001:9001 \
  -e MINIO_ROOT_USER=minioadmin \
  -e MINIO_ROOT_PASSWORD=minioadmin123 \
  minio/minio server /data --console-address ":9001"

# Create bucket
docker exec minio mc alias set local http://localhost:9000 minioadmin minioadmin123
docker exec minio mc mb local/my-bucket

TOKEN=$(cat /tmp/daptin-token.txt)

# 1. Create credential
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

# 2. Create cloud_store
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
        "root_path": "minio-store:my-bucket",
        "store_parameters": "{}"
      }
    }
  }'

# 3. Link credential (CRITICAL!)
CRED_ID=$(curl -s -H "Authorization: Bearer $TOKEN" http://localhost:6336/api/credential | jq -r '.data[] | select(.attributes.name == "minio-creds") | .id')
STORE_ID=$(curl -s -H "Authorization: Bearer $TOKEN" http://localhost:6336/api/cloud_store | jq -r '.data[] | select(.attributes.name == "minio-store") | .id')

curl -X PATCH "http://localhost:6336/api/cloud_store/$STORE_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d "{\"data\":{\"type\":\"cloud_store\",\"id\":\"$STORE_ID\",\"relationships\":{\"credential_id\":{\"data\":{\"type\":\"credential\",\"id\":\"$CRED_ID\"}}}}}"

# 4. Restart server
./scripts/testing/test-runner.sh stop && ./scripts/testing/test-runner.sh start
./scripts/testing/test-runner.sh token

# 5. Create table
TOKEN=$(cat /tmp/daptin-token.txt)
curl -X POST "http://localhost:6336/api/world/action/upload_system_schema" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "schema_json": "{\"Tables\":[{\"TableName\":\"minio_product\",\"Columns\":[{\"Name\":\"name\",\"DataType\":\"varchar(500)\",\"ColumnType\":\"name\"},{\"Name\":\"photo\",\"DataType\":\"text\",\"ColumnType\":\"file\",\"IsForeignKey\":true,\"ForeignKeyData\":{\"DataSource\":\"cloud_store\",\"Namespace\":\"minio-store\",\"KeyName\":\"photos\"}}]}]}"
    }
  }'

# 6. Restart again
./scripts/testing/test-runner.sh stop && ./scripts/testing/test-runner.sh start
./scripts/testing/test-runner.sh token

# 7. Upload file
TOKEN=$(cat /tmp/daptin-token.txt)
curl -X POST http://localhost:6336/api/minio_product \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "minio_product",
      "attributes": {
        "name": "Product with Photo",
        "photo": [{"name":"photo.txt","file":"data:text/plain;base64,UGhvdG8gY29udGVudA==","type":"text/plain"}]
      }
    }
  }'

# Verify in Minio
docker exec minio mc ls local/my-bucket/photos/
```

---

## Troubleshooting Checklist

Before reporting an issue, verify:

- [ ] Credential content includes `type` field (e.g., `"type":"s3"`)
- [ ] Credential is linked to cloud_store via relationship PATCH
- [ ] Server was restarted after creating cloud_store
- [ ] Server was restarted after creating table with file column
- [ ] Bucket/directory exists and is accessible
- [ ] File is sent as array: `[{name, file, type}]`
- [ ] No stale processes on ports 5336 and 6336
- [ ] Token is valid and user has permissions

---

## API Quick Reference

| Operation | Method | Endpoint |
|-----------|--------|----------|
| Create credential | POST | `/api/credential` |
| Create cloud_store | POST | `/api/cloud_store` |
| Link credential | PATCH | `/api/cloud_store/{id}` |
| Upload schema | POST | `/api/world/action/upload_system_schema` |
| Upload file | POST | `/api/{table_name}` |
| List cloud_stores | GET | `/api/cloud_store` |
| List credentials | GET | `/api/credential` |

---

## Version Information

- Tested with: Daptin (latest from source)
- rclone version: 1.71.2
- Database: SQLite3
- Date: January 2026
