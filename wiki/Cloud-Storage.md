# Cloud Storage

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

For development and simple deployments:

```bash
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
        "root_path": "./storage"
      }
    }
  }'
```

### Amazon S3

```bash
# First, create a credential
curl -X POST http://localhost:6336/api/credential \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "credential",
      "attributes": {
        "name": "aws-creds",
        "credential_type": "aws",
        "credential_value": {
          "access_key_id": "AKIAXXXXXXXX",
          "secret_access_key": "your-secret-key",
          "region": "us-east-1"
        }
      }
    }
  }'

# Then create the cloud store linked to the credential
curl -X POST http://localhost:6336/api/cloud_store \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "cloud_store",
      "attributes": {
        "name": "aws-storage",
        "store_type": "s3",
        "store_provider": "AWS",
        "root_path": "your-bucket-name:",
        "credential_name": "aws-creds"
      }
    }
  }'
```

### S3 Compatible (MinIO, DigitalOcean, etc.)

```bash
curl -X POST http://localhost:6336/api/cloud_store \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "cloud_store",
      "attributes": {
        "name": "minio-storage",
        "store_type": "s3",
        "store_provider": "Other",
        "root_path": "bucket-name:",
        "store_parameters": "{\"endpoint\": \"http://minio:9000\", \"force_path_style\": true}"
      }
    }
  }'
```

### Google Cloud Storage

```bash
curl -X POST http://localhost:6336/api/cloud_store \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "cloud_store",
      "attributes": {
        "name": "gcs-storage",
        "store_type": "gcs",
        "store_provider": "Google",
        "root_path": "your-bucket-name:"
      }
    }
  }'
```

---

## File Operations

All cloud store actions require `cloud_store_id` in the attributes.

### Get Cloud Store ID

```bash
# Get the reference ID of your cloud store
curl http://localhost:6336/api/cloud_store \
  -H "Authorization: Bearer $TOKEN" | jq '.data[0].id'
```

### Create Folder

**Tested ✓** - Creates folders on the cloud store.

```bash
curl -X POST http://localhost:6336/action/cloud_store/create_folder \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "cloud_store_id": "YOUR_CLOUD_STORE_ID",
      "name": "my-folder",
      "path": ""
    }
  }'
```

**Parameters:**
- `cloud_store_id` (required) - Reference ID of the cloud store
- `name` (required) - Folder name to create
- `path` (optional) - Parent path where folder will be created

### Upload File

Uploads files to the cloud store.

```bash
# Encode your file to base64
FILE_BASE64=$(base64 < /path/to/file.txt | tr -d '\n')

curl -X POST http://localhost:6336/action/cloud_store/upload_file \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"attributes\": {
      \"cloud_store_id\": \"YOUR_CLOUD_STORE_ID\",
      \"path\": \"uploads\",
      \"file\": [
        {
          \"name\": \"file.txt\",
          \"file\": \"data:text/plain;base64,$FILE_BASE64\"
        }
      ]
    }
  }"
```

**Parameters:**
- `cloud_store_id` (required) - Reference ID of the cloud store
- `path` (optional) - Target folder path
- `file` (required) - Array of file objects with `name` and `file` (base64 data URI)

**Note:** Multiple files can be uploaded in a single request.

### Delete Path

Deletes a file or folder from the cloud store.

```bash
curl -X POST http://localhost:6336/action/cloud_store/delete_path \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "cloud_store_id": "YOUR_CLOUD_STORE_ID",
      "path": "uploads/old-file.pdf"
    }
  }'
```

### Move/Rename Path

Move or rename a file or folder.

```bash
curl -X POST http://localhost:6336/action/cloud_store/move_path \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "cloud_store_id": "YOUR_CLOUD_STORE_ID",
      "source": "uploads/old-name.pdf",
      "destination": "archive/new-name.pdf"
    }
  }'
```

---

## Sites (Static Website Hosting)

Sites allow you to host static websites on cloud storage.

### Create Site

```bash
curl -X POST http://localhost:6336/action/cloud_store/create_site \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "cloud_store_id": "YOUR_CLOUD_STORE_ID",
      "hostname": "mysite.example.com",
      "path": "mysite",
      "site_type": "static"
    }
  }'
```

**Parameters:**
- `cloud_store_id` (required) - Reference ID of the cloud store
- `hostname` (required) - Domain name for the site
- `path` (required) - Folder path within cloud store
- `site_type` - Site type: `static`, `hugo`

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

For automatic file storage to cloud when uploading via API.

### 1. Create Cloud Store

See examples above.

### 2. Link Table to Cloud Store

```bash
# Get your table's world record ID
TABLE_WORLD_ID=$(curl -s http://localhost:6336/api/world \
  -H "Authorization: Bearer $TOKEN" | \
  jq -r '.data[] | select(.attributes.table_name == "product") | .id')

# Link the cloud store as default storage
curl -X PATCH "http://localhost:6336/api/world/$TABLE_WORLD_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "world",
      "attributes": {
        "default_storage": "YOUR_CLOUD_STORE_ID"
      }
    }
  }'
```

### 3. Upload Files via Table API

When cloud storage is linked, file columns store metadata and files go to cloud:

```bash
FILE_BASE64=$(base64 < /path/to/image.jpg | tr -d '\n')

curl -X POST http://localhost:6336/api/product \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d "{
    \"data\": {
      \"type\": \"product\",
      \"attributes\": {
        \"name\": \"Widget\",
        \"image\": \"data:image/jpeg;base64,$FILE_BASE64\"
      }
    }
  }"
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

## Credentials

Store credentials separately for security.

### Create Credential

```bash
curl -X POST http://localhost:6336/api/credential \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "credential",
      "attributes": {
        "name": "my-aws-creds",
        "credential_type": "aws",
        "credential_value": "{\"access_key_id\": \"AKIA...\", \"secret_access_key\": \"...\"}"
      }
    }
  }'
```

### Link Credential to Cloud Store

Update the cloud store to use the credential:

```bash
curl -X PATCH http://localhost:6336/api/cloud_store/STORE_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "cloud_store",
      "id": "STORE_ID",
      "attributes": {
        "credential_name": "my-aws-creds"
      }
    }
  }'
```

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

## See Also

- [Asset Columns](Asset-Columns.md) - Inline vs cloud file storage
- [Subsites](Subsites.md) - Static site hosting details
- [Credentials](Credentials.md) - Credential management
