# Asset Columns

File and media handling in Daptin.

## Two Storage Modes

Daptin supports two ways to store files:

1. **Inline (Default)** - Files stored as base64 in database column
2. **Cloud Storage** - Files stored in external storage (S3, GCS, local filesystem)

## Asset Column Types

| Type | Description | Storage |
|------|-------------|---------|
| `file` | General file upload | Inline base64 |
| `image` | Image files | Inline base64 |
| `video` | Video files | Inline base64 |
| `blob` | Binary data | Inline base64 |

**Note:** For large files, use Cloud Storage instead of inline storage.

## Defining Asset Columns

```yaml
Tables:
  - TableName: product
    Columns:
      - Name: photo
        DataType: text
        ColumnType: image

      - Name: attachment
        DataType: text
        ColumnType: file
```

## Uploading Files (Inline Storage)

### Base64 Upload (Recommended)

```bash
# Encode file to base64
IMG_BASE64=$(base64 < /path/to/image.png | tr -d '\n')

curl -X POST http://localhost:6336/api/product \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d "{
    \"data\": {
      \"type\": \"product\",
      \"attributes\": {
        \"name\": \"Widget\",
        \"photo\": \"data:image/png;base64,$IMG_BASE64\"
      }
    }
  }"
```

**Response:**
```json
{
  "data": {
    "type": "product",
    "id": "abc123...",
    "attributes": {
      "name": "Widget",
      "photo": "data:image/png;base64,iVBOR..."
    }
  }
}
```

The file is stored directly in the database as a base64 string.

## Accessing Inline Files

For inline storage, files are returned as base64 strings in the API response:

```bash
curl http://localhost:6336/api/product/ID \
  -H "Authorization: Bearer $TOKEN"
```

```json
{
  "data": {
    "attributes": {
      "photo": "data:image/png;base64,iVBOR..."
    }
  }
}
```

Decode base64 to use the file:

```bash
# Extract and decode
echo "iVBOR..." | base64 -d > image.png
```

---

## Cloud Storage (For Large Files)

For files larger than a few KB, use cloud storage instead of inline.

See [Cloud Storage](Cloud-Storage.md) for setup instructions.

### How Cloud Storage Works

1. Create a `cloud_store` record (S3, GCS, local filesystem)
2. Link the cloud store to your table column
3. Files are uploaded to storage and only metadata is stored in DB

### Link Column to Cloud Storage

```bash
# First, create a cloud_store (see Cloud-Storage.md)
# Then link your table to use it

curl -X PATCH http://localhost:6336/api/world/PRODUCT_TABLE_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "world",
      "attributes": {
        "default_storage": "CLOUD_STORE_REFERENCE_ID"
      }
    }
  }'
```

### Cloud Storage Response Format

When cloud storage is configured, API returns metadata:

```json
{
  "data": {
    "attributes": {
      "photo": {
        "name": "image.jpg",
        "type": "image/jpeg",
        "size": 102400,
        "path": "/uploads/product/abc123/photo/image.jpg"
      }
    }
  }
}
```

### Asset Endpoint (Cloud Storage Only)

When using cloud storage, access files via:

```
http://localhost:6336/asset/{entity}/{id}/{column}
```

**Note:** This endpoint only works with cloud storage, not inline base64.

---

## Update Files

Replace file with new base64 content:

```bash
IMG_BASE64=$(base64 < /path/to/new-image.png | tr -d '\n')

curl -X PATCH http://localhost:6336/api/product/ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d "{
    \"data\": {
      \"type\": \"product\",
      \"id\": \"ID\",
      \"attributes\": {
        \"photo\": \"data:image/png;base64,$IMG_BASE64\"
      }
    }
  }"
```

## Delete Files

Clear file field by setting to null:

```bash
curl -X PATCH http://localhost:6336/api/product/ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "product",
      "id": "ID",
      "attributes": {
        "photo": null
      }
    }
  }'
```

## Multiple Files

For multiple files, use relation to file table:

```yaml
Tables:
  - TableName: product
    Columns:
      - Name: name
        DataType: varchar(255)

  - TableName: product_image
    Columns:
      - Name: image
        DataType: text
        ColumnType: image

    Relations:
      - Subject: product_image
        Object: product
        Relation: belongs_to
```

## File Metadata (Cloud Storage)

When using cloud storage, file metadata is tracked:

```json
{
  "name": "photo.jpg",
  "type": "image/jpeg",
  "size": 102400,
  "md5": "abc123...",
  "path": "/uploads/product/..."
}
```

**Note:** Inline storage (base64) does not generate separate metadata.

## Content Delivery

For production, use cloud storage with CDN:

1. Configure cloud storage (S3, GCS)
2. Set up CloudFront/Cloud CDN
3. Update asset URLs to use CDN

---

## Limitations

| Limitation | Details |
|------------|---------|
| **Inline storage size** | Base64 increases size by ~33%. Use cloud storage for files > 100KB |
| **No thumbnails** | Inline storage doesn't generate thumbnails |
| **Asset endpoint** | Only works with cloud storage, not inline base64 |

## See Also

- [Cloud Storage](Cloud-Storage.md) - For large file storage
- [Schema Definition](Schema-Definition.md) - Table and column setup
