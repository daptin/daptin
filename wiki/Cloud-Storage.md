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

## Creating Cloud Store

### Amazon S3

```bash
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
        "root_path": "/daptin",
        "store_parameters": {
          "access_key_id": "AKIAXXXXXXXX",
          "secret_access_key": "your-secret-key",
          "region": "us-east-1",
          "bucket": "your-bucket-name",
          "acl": "private"
        }
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
        "root_path": "/",
        "store_parameters": {
          "access_key_id": "minio-key",
          "secret_access_key": "minio-secret",
          "endpoint": "http://minio:9000",
          "bucket": "daptin-files",
          "force_path_style": true
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
          "bucket": "your-gcs-bucket",
          "project": "your-project-id",
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

### Local Filesystem

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

## File Operations

### Upload File

```bash
curl -X POST http://localhost:6336/action/cloud_store/cloudstore_file_upload \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "attributes": {
      "cloud_store_id": "STORE_ID",
      "path": "/uploads/document.pdf",
      "file": [{
        "name": "document.pdf",
        "file": "data:application/pdf;base64,..."
      }]
    }
  }'
```

### List Files

```bash
curl -X POST http://localhost:6336/action/cloud_store/cloudstore_file_list \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "attributes": {
      "cloud_store_id": "STORE_ID",
      "path": "/uploads"
    }
  }'
```

### Download File

Files are accessed via presigned URLs or direct download actions.

### Delete File

```bash
curl -X POST http://localhost:6336/action/cloud_store/cloudstore_file_delete \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "attributes": {
      "cloud_store_id": "STORE_ID",
      "path": "/uploads/old-file.pdf"
    }
  }'
```

### Create Folder

```bash
curl -X POST http://localhost:6336/action/cloud_store/cloudstore_folder_create \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "attributes": {
      "cloud_store_id": "STORE_ID",
      "path": "/uploads/2024"
    }
  }'
```

### Move/Rename

```bash
curl -X POST http://localhost:6336/action/cloud_store/cloudstore_path_move \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "attributes": {
      "cloud_store_id": "STORE_ID",
      "source_path": "/uploads/old-name.pdf",
      "destination_path": "/archive/new-name.pdf"
    }
  }'
```

## Asset Columns

Link file columns to cloud storage:

### Schema Definition

```yaml
Tables:
  - TableName: product
    Columns:
      - Name: image
        DataType: text
        ColumnType: file.image
        CloudStoreId: STORE_ID  # Reference to cloud_store
```

### Sync Asset Column

```bash
curl -X POST http://localhost:6336/action/product/__column_sync_storage \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "attributes": {
      "column_name": "image"
    }
  }'
```

## Presigned URLs

Get temporary download URLs:

```bash
curl http://localhost:6336/asset/product/PRODUCT_ID/image \
  -H "Authorization: Bearer $TOKEN"
```

Returns signed URL valid for limited time.

## Credentials Management

### Store Credentials Separately

```bash
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
          "secret_access_key": "secret"
        }
      }
    }
  }'
```

### Link to Cloud Store

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

## Caching

Cloud storage files are cached:
- Default: 24 hours
- Images: 7 days
- Videos: 14 days
- Max file size: 2MB for cache

## Sync with External Storage

Periodic sync with external storage:

```bash
curl -X POST http://localhost:6336/action/cloud_store/cloudstore_sync \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "attributes": {
      "cloud_store_id": "STORE_ID",
      "direction": "both"
    }
  }'
```

## Static Site Hosting

### Create Site on Cloud Storage

Deploy a static website to cloud storage:

```bash
curl -X POST http://localhost:6336/action/cloud_store/cloudstore.site.create \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "attributes": {
      "cloud_store_id": "STORE_ID",
      "site_path": "/website",
      "files": [{
        "name": "index.html",
        "file": "data:text/html;base64,..."
      }]
    }
  }'
```

### Sync Site to Storage

```bash
curl -X POST http://localhost:6336/action/site/site.storage.sync \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "attributes": {
      "site_id": "SITE_REFERENCE_ID"
    }
  }'
```

## Site File Operations

### List Site Files

```bash
curl -X POST http://localhost:6336/action/site/site.file.list \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "attributes": {
      "site_id": "SITE_REFERENCE_ID",
      "path": "/"
    }
  }'
```

### Get Site File

```bash
curl -X POST http://localhost:6336/action/site/site.file.get \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "attributes": {
      "site_id": "SITE_REFERENCE_ID",
      "path": "/index.html"
    }
  }'
```

### Delete Site File

```bash
curl -X POST http://localhost:6336/action/site/site.file.delete \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "attributes": {
      "site_id": "SITE_REFERENCE_ID",
      "path": "/old-page.html"
    }
  }'
```

## Import Files from Cloud Storage

Import files from an external cloud storage into Daptin:

```bash
curl -X POST http://localhost:6336/action/cloud_store/cloud_store.files.import \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "attributes": {
      "cloud_store_id": "SOURCE_STORE_ID",
      "path": "/import",
      "target_table": "document"
    }
  }'
```
