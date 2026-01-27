# Cloud Storage

**Tested ✓** - Cloud store creation, listing, and file operations verified on 2026-01-27.

**Actions Status:**
- ✅ **create_folder** - Working (correct URL format documented below)
- ✅ **upload_file** - Working (correct URL format documented below)
- ⚠️ **move_path** - Partially working (has bug: creates directory instead of renaming)
- ❌ **delete_path** - Returns success but doesn't actually delete

**Critical**: GitHub Issue #166 was about wrong URL format in documentation, not broken actions. The correct format is `/action/{type}/{action_name}?{type}_id={id}` NOT `/action/{type}/{id}/{action_name}`.

Integrate with cloud storage providers via rclone.

## Supported Providers

| Provider | Type | Description |
|----------|------|-------------|
| Amazon S3 | s3 | AWS S3 and compatible |
| Google Cloud | gcs | Google Cloud Storage |
| Azure Blob | azureblob | Microsoft Azure |
| Dropbox | dropbox | Dropbox |
| Google Drive | drive | Google Drive |
| Box | box | Box.com |
| Backblaze B2 | b2 | Backblaze B2 |
| OpenStack Swift | swift | OpenStack |
| FTP | ftp | FTP servers |
| SFTP | sftp | SFTP servers |
| Local | local | Local filesystem |
| WebDAV | webdav | WebDAV servers |

---

## Creating Cloud Store

### Local Filesystem

**Tested ✓** - For development and simple deployments:

```bash
# First create the storage directory
mkdir -p /tmp/my-storage

curl -X POST http://localhost:6336/api/cloud_store \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "cloud_store",
      "attributes": {
        "name": "local-storage",
        "store_type": "local",
        "store_provider": "local",
        "root_path": "/tmp/my-storage",
        "store_parameters": "{}"
      }
    }
  }'
```

**Response (tested):**
```json
{
  "data": {
    "type": "cloud_store",
    "id": "019bf49e-b032-75fb-9f52-edfe1b7ddae2",
    "attributes": {
      "name": "local-storage",
      "root_path": "/tmp/my-storage",
      "store_provider": "local",
      "store_type": "local",
      "store_parameters": "{}"
    }
  }
}
```

**Note:** `store_parameters` must be provided as a JSON string (e.g., `"{}"`) even if empty, otherwise you may get a database constraint error.

### Amazon S3

**CRITICAL**: The credential `content` field must be a JSON string in rclone format.

```bash
# Step 1: Create credential with rclone-format content
curl -X POST http://localhost:6336/api/credential \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "credential",
      "attributes": {
        "name": "aws-creds",
        "content": "{\"type\":\"s3\",\"provider\":\"AWS\",\"env_auth\":\"false\",\"access_key_id\":\"AKIAXXXXXXXX\",\"secret_access_key\":\"your-secret-key\",\"region\":\"us-east-1\"}"
      }
    }
  }'

# Step 2: Create the cloud store
curl -X POST http://localhost:6336/api/cloud_store \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "cloud_store",
      "attributes": {
        "name": "aws-storage",
        "store_type": "s3",
        "store_provider": "s3",
        "root_path": "aws-storage:your-bucket-name",
        "store_parameters": "{}"
      }
    }
  }'

# Step 3: Link credential to cloud store via relationship
CRED_ID=$(curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:6336/api/credential | \
  jq -r '.data[] | select(.attributes.name == "aws-creds") | .id')

STORE_ID=$(curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:6336/api/cloud_store | \
  jq -r '.data[] | select(.attributes.name == "aws-storage") | .id')

curl -X PATCH "http://localhost:6336/api/cloud_store/$STORE_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d "{
    \"data\": {
      \"type\": \"cloud_store\",
      \"id\": \"$STORE_ID\",
      \"relationships\": {
        \"credential_id\": {
          \"data\": {\"type\": \"credential\", \"id\": \"$CRED_ID\"}
        }
      }
    }
  }"

# Step 4: Restart server to load cloud storage
pkill -9 -f daptin
sleep 2
./daptin &
sleep 10
```

**Important Notes**:
- The `content` field is passed directly to rclone - it must include `type`, `provider`, and provider-specific fields
- The `credential_name` attribute does NOT automatically link - you must use a relationship PATCH
- Server restart is REQUIRED after creating/linking cloud storage

### S3 Compatible (MinIO, DigitalOcean, etc.)

```bash
# Create credential with Minio endpoint
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

# Create cloud store
curl -X POST http://localhost:6336/api/cloud_store \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "cloud_store",
      "attributes": {
        "name": "minio-storage",
        "store_type": "s3",
        "store_provider": "s3",
        "root_path": "minio-storage:bucket-name",
        "store_parameters": "{}"
      }
    }
  }'

# Link credential (see AWS S3 example above for full linking steps)
```

**Minio-specific rclone fields**:
- `endpoint`: Minio server URL (e.g., `http://localhost:9000`)
- `provider`: Must be `"Minio"` for Minio

### Google Cloud Storage

```bash
# Create credential with service account JSON
# CRITICAL: Escape the service account JSON properly
curl -X POST http://localhost:6336/api/credential \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "credential",
      "attributes": {
        "name": "gcs-creds",
        "content": "{\"type\":\"google cloud storage\",\"service_account_credentials\":\"{...ESCAPED_SERVICE_ACCOUNT_JSON...}\"}"
      }
    }
  }'

# Create cloud store
curl -X POST http://localhost:6336/api/cloud_store \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "cloud_store",
      "attributes": {
        "name": "gcs-storage",
        "store_type": "gcs",
        "store_provider": "gcs",
        "root_path": "gcs-storage:your-bucket-name",
        "store_parameters": "{}"
      }
    }
  }'

# Link credential (see AWS S3 example above for full linking steps)
```

**GCS-specific rclone fields**:
- `type`: Must be `"google cloud storage"` exactly
- `service_account_credentials`: JSON string of your GCS service account key

---

## CRUD Operations

**Tested ✓** - Standard CRUD operations on cloud_store work correctly.

### List All Cloud Stores

```bash
curl http://localhost:6336/api/cloud_store \
  -H "Authorization: Bearer $TOKEN" | jq '.data'
```

### Get Single Cloud Store

```bash
curl http://localhost:6336/api/cloud_store/$STORE_ID \
  -H "Authorization: Bearer $TOKEN"
```

### Update Cloud Store

```bash
curl -X PATCH http://localhost:6336/api/cloud_store/$STORE_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "cloud_store",
      "id": "YOUR_STORE_ID",
      "attributes": {
        "name": "new-name"
      }
    }
  }'
```

### Delete Cloud Store

```bash
curl -X DELETE http://localhost:6336/api/cloud_store/$STORE_ID \
  -H "Authorization: Bearer $TOKEN"
```

---

## File Operations

**Status**: Most operations working with correct URL format (2026-01-27 testing).

**CRITICAL**: All cloud store actions require the `cloud_store_id` as a **query parameter**, NOT in the request body or URL path:

```bash
/action/cloud_store/{action_name}?cloud_store_id={STORE_ID}
```

### Get Cloud Store ID

```bash
# Get the reference ID of your cloud store
curl http://localhost:6336/api/cloud_store \
  -H "Authorization: Bearer $TOKEN" | jq '.data[0].id'
```

### Create Folder

**Tested ✓** - Creates folders successfully in cloud storage.

**CRITICAL**: The instance ID must be passed as a query parameter, not in the URL path.

```bash
# Correct format - instance ID as query parameter
curl -X POST "http://localhost:6336/action/cloud_store/create_folder?cloud_store_id=$STORE_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "name": "my-folder",
      "path": ""
    }
  }'
```

**Response:**
```json
[{
  "ResponseType": "client.notify",
  "Attributes": {
    "message": "Cloud storage file upload queued",
    "type": "success"
  }
}]
```

**Parameters:**
- `name` (required) - Folder name to create
- `path` (optional) - Parent path where folder will be created (use empty string for root)

### Upload File

**Tested ✓** - Uploads files successfully to cloud storage.

**CRITICAL**: The instance ID must be passed as a query parameter, not in the body.

```bash
# Encode your file to base64
FILE_BASE64=$(base64 < /path/to/file.txt | tr -d '\n')

# Correct format - instance ID as query parameter
curl -X POST "http://localhost:6336/action/cloud_store/upload_file?cloud_store_id=$STORE_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "path": "",
      "file": [
        {
          "name": "file.txt",
          "file": "data:text/plain;base64,'"$FILE_BASE64"'",
          "type": "text/plain"
        }
      ]
    }
  }'
```

**Response:**
```json
[{
  "ResponseType": "client.notify",
  "Attributes": {
    "message": "Cloud storage file upload queued",
    "type": "success"
  }
}]
```

**Parameters:**
- `path` (optional) - Target folder path (use empty string for root)
- `file` (required) - Array of file objects with `name`, `file` (base64 data URI), and `type` (MIME type)

**Note:** Multiple files can be uploaded in a single request. Files are uploaded asynchronously.

### Delete Path

**Not Working** - Returns success but doesn't actually delete files/folders.

Deletes a file or folder from the cloud store.

```bash
# Correct URL format (but doesn't actually delete)
curl -X POST "http://localhost:6336/action/cloud_store/delete_path?cloud_store_id=$STORE_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "path": "uploads/old-file.pdf"
    }
  }'
```

**Known Issue**: Action returns success message but files/folders remain in storage. This is a bug in the delete performer.

### Move/Rename Path

**Partially Working** - Returns success but has incorrect behavior.

Move or rename a file or folder.

```bash
# Correct URL format
curl -X POST "http://localhost:6336/action/cloud_store/move_path?cloud_store_id=$STORE_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "source": "uploads/old-name.pdf",
      "destination": "archive/new-name.pdf"
    }
  }'
```

**Known Issue**: Instead of renaming the file, this action creates a directory with the destination name and moves the source file inside it. For example, moving `test.txt` to `renamed.txt` creates `renamed.txt/` directory containing `test.txt`.

---

## Sites (Static Website Hosting)

Sites allow you to host static websites on cloud storage. See [Subsites.md](Subsites.md) for detailed site management documentation.

### Create Site

**Status Unknown** - Not tested yet, but likely requires correct URL format like other actions.

```bash
# Expected correct format (not yet verified)
curl -X POST "http://localhost:6336/action/cloud_store/create_site?cloud_store_id=$STORE_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "hostname": "mysite.example.com",
      "path": "mysite",
      "site_type": "static"
    }
  }'
```

**Parameters:**
- `hostname` (required) - Domain name for the site
- `path` (required) - Folder path within cloud store
- `site_type` - Site type: `static`, `hugo`

**Note**: Instance ID passed as query parameter, not in body.

### List Site Files

```bash
curl -X POST http://localhost:6336/action/site/list_files \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "site_id": "SITE_REFERENCE_ID",
      "path": "/"
    }
  }'
```

### Get Site File

```bash
curl -X POST http://localhost:6336/action/site/get_file \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "site_id": "SITE_REFERENCE_ID",
      "path": "/index.html"
    }
  }'
```

Returns file content as base64.

---

## Link Cloud Storage to Table Columns

**Tested ✓** - For automatic file storage when uploading via API.

There are TWO ways to link cloud storage to tables:

### Method 1: Schema File (Recommended)

Define cloud storage in your schema YAML file before creating the table:

```yaml
Tables:
  - TableName: product
    Columns:
      - Name: photo
        DataType: text
        ColumnType: file
        IsNullable: true
        IsForeignKey: true
        ForeignKeyData:
          DataSource: cloud_store
          Namespace: product-images    # Must match cloud_store "name" field
          KeyName: photos               # Subfolder within root_path
```

**How it works**:
1. Create the cloud_store with `name: "product-images"` (see examples above)
2. Restart server to load the schema
3. Files uploaded to the `photo` column automatically go to cloud storage at `{root_path}/photos/`

**File upload format** (CRITICAL):

```bash
# Files must be sent as an ARRAY of objects
FILE_BASE64=$(base64 < /path/to/image.jpg | tr -d '\n')

curl -X POST http://localhost:6336/api/product \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "product",
      "attributes": {
        "name": "Widget",
        "photo": [
          {
            "name": "widget.jpg",
            "file": "data:image/jpeg;base64,'$FILE_BASE64'",
            "type": "image/jpeg"
          }
        ]
      }
    }
  }'
```

**Common mistakes**:
- ❌ Sending photo as a string
- ❌ Sending photo as a single object
- ✅ **Must be an array of objects**: `[{name, file, type}]`

### Method 2: API (Runtime)

Link cloud storage to existing table via default_storage:

```bash
# Get your table's world record ID
TABLE_WORLD_ID=$(curl --get \
  --data-urlencode 'query=[{"column":"table_name","operator":"is","value":"product"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/world" | jq -r '.data[0].id')

# Get cloud store ID
STORE_ID=$(curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:6336/api/cloud_store | \
  jq -r '.data[] | select(.attributes.name == "product-images") | .id')

# Link via relationship
curl -X PATCH "http://localhost:6336/api/world/$TABLE_WORLD_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "world",
      "id": "'$TABLE_WORLD_ID'",
      "relationships": {
        "default_storage": {
          "data": {"type": "cloud_store", "id": "'$STORE_ID'"}
        }
      }
    }
  }'

# Restart server
pkill -9 -f daptin && sleep 2 && ./daptin &
```

---

## Asset Endpoint

Access files stored in cloud storage:

```
GET /asset/{table}/{record_id}/{column}
```

Example:
```bash
curl http://localhost:6336/asset/product/PRODUCT_ID/image \
  -H "Authorization: Bearer $TOKEN"
```

**Note:** This endpoint only works when the table is linked to cloud storage. For inline base64 storage, files are returned directly in the API response.

---

## Credential Format (CRITICAL)

**Important**: Daptin uses rclone internally for cloud storage. The credential `content` field must be valid rclone configuration JSON.

### Credential Structure

```bash
curl -X POST http://localhost:6336/api/credential \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "credential",
      "attributes": {
        "name": "my-creds",
        "content": "{RCLONE_JSON_CONFIG}"
      }
    }
  }'
```

The `content` field must include:
- `"type"`: The rclone remote type (`"s3"`, `"google cloud storage"`, etc.)
- `"provider"`: Provider name (`"AWS"`, `"Minio"`, `"GCS"`, etc.)
- Provider-specific fields (access keys, endpoints, etc.)

**Common credential formats**:

**S3/Minio**:
```json
{
  "type": "s3",
  "provider": "AWS",
  "env_auth": "false",
  "access_key_id": "YOUR_KEY",
  "secret_access_key": "YOUR_SECRET",
  "region": "us-east-1"
}
```

**Minio with custom endpoint**:
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

**Google Cloud Storage**:
```json
{
  "type": "google cloud storage",
  "service_account_credentials": "{...SERVICE_ACCOUNT_JSON...}"
}
```

**References**: See [rclone documentation](https://rclone.org/docs/) for all provider configurations.

### Link Credential to Cloud Store

**CRITICAL**: The `credential_name` field does NOT automatically link the credential. You must use a relationship PATCH:

```bash
# Get credential ID
CRED_ID=$(curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:6336/api/credential | \
  jq -r '.data[] | select(.attributes.name == "my-creds") | .id')

# Get cloud store ID
STORE_ID=$(curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:6336/api/cloud_store | \
  jq -r '.data[] | select(.attributes.name == "my-storage") | .id')

# Link via relationship PATCH
curl -X PATCH "http://localhost:6336/api/cloud_store/$STORE_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d "{
    \"data\": {
      \"type\": \"cloud_store\",
      \"id\": \"$STORE_ID\",
      \"relationships\": {
        \"credential_id\": {
          \"data\": {\"type\": \"credential\", \"id\": \"$CRED_ID\"}
        }
      }
    }
  }"
```

**After linking**: Restart the server for the credential link to take effect.

---

## Cloud Store Fields

| Field | Description |
|-------|-------------|
| `name` | Display name for the store |
| `store_type` | Provider type: `local`, `s3`, `gcs`, `dropbox`, etc. |
| `store_provider` | Provider identifier |
| `root_path` | Base path (e.g., `./storage`, `bucket-name:`) |
| `store_parameters` | JSON string with provider-specific config |
| `credential_name` | Name of linked credential |

---

## Troubleshooting

### Files Not Uploading to Cloud Storage

**Check 1**: Verify cloud_store was created correctly
```bash
curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:6336/api/cloud_store | \
  jq '.data[] | {name: .attributes.name, store_type: .attributes.store_type, root_path: .attributes.root_path}'
```

**Check 2**: Verify credential is linked
```bash
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/cloud_store/$STORE_ID" | \
  jq '.data.relationships.credential_id'
```

If `null`, the credential is not linked. Use the relationship PATCH from the section above.

**Check 3**: Check server logs for cloud storage initialization
```bash
grep -i "Sync table column" /tmp/daptin.log
# Expected: [71] Sync table column [product][photo] at /path/to/storage
```

**Check 4**: Verify file upload format is an array
```json
// CORRECT
{"photo": [{"name": "file.jpg", "file": "data:image/jpeg;base64,...", "type": "image/jpeg"}]}

// WRONG
{"photo": "data:image/jpeg;base64,..."}
{"photo": {"name": "file.jpg", "file": "data:..."}}
```

### "content field is required" Error

The credential `content` field cannot be empty. Provide valid rclone JSON configuration.

### Files Go to Wrong Location

**Problem**: Files are stored at the wrong path in cloud storage.

**Cause**: The `KeyName` in ForeignKeyData creates a subfolder within `root_path`.

**Example**:
- `root_path`: `/tmp/product-images`
- `KeyName`: `photos`
- **Actual storage path**: `/tmp/product-images/photos/filename.jpg`

### Credential Not Found After Linking

**Cause**: Server hasn't reloaded the cloud_store configuration.

**Solution**: Restart Daptin:
```bash
pkill -9 -f daptin
sleep 2
./daptin &
sleep 10
```

### rclone Panic or Errors

**Cause**: The `content` JSON is missing required rclone fields like `"type"` or `"provider"`.

**Solution**: Ensure credential content includes all required rclone fields for your provider. Check [rclone docs](https://rclone.org/) for exact field names.

---

## See Also

- [Asset Columns](Asset-Columns.md) - Inline vs cloud file storage configuration
- [Subsites](Subsites.md) - Static site hosting details
- [Credentials](Credentials.md) - Credential management
- [[Walkthrough-Product-Catalog]] - Complete cloud storage setup example
