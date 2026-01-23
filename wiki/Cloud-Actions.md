# Cloud Actions

Actions for cloud storage operations using rclone-compatible backends.

## Supported Providers

- Amazon S3
- Google Cloud Storage
- Microsoft Azure Blob
- Dropbox
- Box.com
- Backblaze B2
- OpenStack Swift
- FTP/SFTP
- Local filesystem
- 30+ more via rclone

## cloudstore_file_upload

Upload file to cloud storage.

```bash
curl -X POST http://localhost:6336/action/cloud_store/cloudstore_file_upload \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "cloud_store_id": "CLOUDSTORE_ID",
      "path": "/uploads/document.pdf",
      "file": [{
        "name": "document.pdf",
        "file": "data:application/pdf;base64,..."
      }]
    }
  }'
```

**Parameters:**

| Parameter | Description |
|-----------|-------------|
| cloud_store_id | Reference ID of cloud_store |
| path | Destination path |
| file | File data (base64) |

## cloudstore_file_delete

Delete file from cloud storage.

```bash
curl -X POST http://localhost:6336/action/cloud_store/cloudstore_file_delete \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "cloud_store_id": "CLOUDSTORE_ID",
      "path": "/uploads/old-file.pdf"
    }
  }'
```

## cloudstore_folder_create

Create folder in cloud storage.

```bash
curl -X POST http://localhost:6336/action/cloud_store/cloudstore_folder_create \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "cloud_store_id": "CLOUDSTORE_ID",
      "path": "/uploads/2024/january"
    }
  }'
```

## cloudstore_path_move

Move or rename file/folder.

```bash
curl -X POST http://localhost:6336/action/cloud_store/cloudstore_path_move \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "cloud_store_id": "CLOUDSTORE_ID",
      "source_path": "/uploads/old-name.pdf",
      "destination_path": "/archive/new-name.pdf"
    }
  }'
```

## cloudstore_site_create

Create a subsite backed by cloud storage.

```bash
curl -X POST http://localhost:6336/action/cloud_store/cloudstore_site_create \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "cloud_store_id": "CLOUDSTORE_ID",
      "site_name": "static-website",
      "hostname": "static.example.com"
    }
  }'
```

## column_sync_storage

Sync asset column files to cloud storage.

```bash
curl -X POST http://localhost:6336/action/product/__column_sync_storage \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "column_name": "image",
      "cloud_store_id": "CLOUDSTORE_ID"
    }
  }'
```

Syncs all files in the specified asset column to cloud storage.

## site_file_get

Get file from subsite storage.

```bash
curl -X POST http://localhost:6336/action/site/site_file_get \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "site_id": "SITE_ID",
      "path": "/index.html"
    }
  }'
```

## site_file_list

List files in subsite storage.

```bash
curl -X POST http://localhost:6336/action/site/site_file_list \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "site_id": "SITE_ID",
      "path": "/assets"
    }
  }'
```

## site_sync_storage

Sync subsite files with cloud storage.

```bash
curl -X POST http://localhost:6336/action/site/site_sync_storage \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "site_id": "SITE_ID"
    }
  }'
```

## Creating Cloud Store

### S3 Configuration

```bash
curl -X POST http://localhost:6336/api/cloud_store \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "cloud_store",
      "attributes": {
        "name": "my-s3-bucket",
        "store_type": "s3",
        "store_provider": "AWS",
        "root_path": "/daptin-files",
        "store_parameters": {
          "access_key_id": "AKIAXXXXXXXX",
          "secret_access_key": "secret",
          "region": "us-east-1",
          "bucket": "my-bucket"
        }
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
        "root_path": "/",
        "store_parameters": {
          "bucket": "my-gcs-bucket",
          "service_account_json": "{...}"
        }
      }
    }
  }'
```

### Dropbox

```bash
curl -X POST http://localhost:6336/api/cloud_store \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "cloud_store",
      "attributes": {
        "name": "dropbox-storage",
        "store_type": "dropbox",
        "root_path": "/Apps/Daptin",
        "store_parameters": {
          "token": "dropbox-oauth-token"
        }
      }
    }
  }'
```

### Local Storage

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
        "root_path": "/var/daptin/files"
      }
    }
  }'
```

## Credentials Management

Store credentials securely:

```bash
curl -X POST http://localhost:6336/api/credential \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "credential",
      "attributes": {
        "name": "aws-credentials",
        "credential_type": "aws",
        "credential_value": {
          "access_key_id": "AKIAXXXXXXXX",
          "secret_access_key": "secret"
        }
      }
    }
  }'
```

Link credential to cloud store:

```bash
curl -X PATCH http://localhost:6336/api/cloud_store/STORE_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "cloud_store",
      "id": "STORE_ID",
      "relationships": {
        "credential": {"data": {"type": "credential", "id": "CRED_ID"}}
      }
    }
  }'
```
