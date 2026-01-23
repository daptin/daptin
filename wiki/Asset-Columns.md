# Asset Columns

File and media handling in Daptin.

## Asset Column Types

| Type | Description |
|------|-------------|
| `file` | General file upload |
| `image` | Image with thumbnails |
| `audio` | Audio files |
| `video` | Video files |
| `document` | Documents (PDF, DOC) |
| `blob` | Binary data |

## Defining Asset Columns

```yaml
Tables:
  - TableName: product
    Columns:
      - Name: photo
        DataType: text
        ColumnType: image

      - Name: manual
        DataType: text
        ColumnType: document

      - Name: attachment
        DataType: text
        ColumnType: file
```

## Uploading Files

### Multipart Upload

```bash
curl -X POST http://localhost:6336/api/product \
  -H "Authorization: Bearer $TOKEN" \
  -F "photo=@/path/to/image.jpg" \
  -F "data={\"type\":\"product\",\"attributes\":{\"name\":\"Widget\"}};type=application/json"
```

### Base64 Upload

```bash
curl -X POST http://localhost:6336/api/product \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "product",
      "attributes": {
        "name": "Widget",
        "photo": "data:image/jpeg;base64,/9j/4AAQSkZJRg..."
      }
    }
  }'
```

## File Storage

### Local Storage (Default)

Files stored in `./uploads/` directory.

### Cloud Storage

Link to cloud store:

```bash
curl -X PATCH http://localhost:6336/api/world/PRODUCT_TABLE_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "world",
      "attributes": {
        "default_storage": "CLOUD_STORE_ID"
      }
    }
  }'
```

## Accessing Files

### Direct URL

Files accessible via:

```
http://localhost:6336/asset/{entity}/{id}/{column}
```

Example:
```
http://localhost:6336/asset/product/abc123/photo
```

### In API Response

```json
{
  "data": {
    "type": "product",
    "attributes": {
      "photo": {
        "name": "image.jpg",
        "type": "image/jpeg",
        "size": 102400,
        "path": "/uploads/product/abc123/photo/image.jpg",
        "url": "http://localhost:6336/asset/product/abc123/photo"
      }
    }
  }
}
```

## Image Thumbnails

Image columns auto-generate thumbnails:

| Size | Dimension |
|------|-----------|
| original | Full size |
| large | 1024px |
| medium | 512px |
| small | 256px |
| thumbnail | 128px |

Access thumbnails:

```
http://localhost:6336/asset/product/abc123/photo?size=thumbnail
http://localhost:6336/asset/product/abc123/photo?size=medium
```

## File Validation

### Size Limits

```yaml
Columns:
  - Name: photo
    ColumnType: image
    MaxFileSize: 5242880  # 5MB
```

### Allowed Types

```yaml
Columns:
  - Name: document
    ColumnType: document
    AllowedMimeTypes:
      - application/pdf
      - application/msword
```

## Update Files

Replace file:

```bash
curl -X PATCH http://localhost:6336/api/product/ID \
  -H "Authorization: Bearer $TOKEN" \
  -F "photo=@/path/to/new-image.jpg" \
  -F "data={\"type\":\"product\",\"id\":\"ID\"};type=application/json"
```

## Delete Files

Clear file field:

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

## File Metadata

Asset columns store metadata:

```json
{
  "name": "photo.jpg",
  "type": "image/jpeg",
  "size": 102400,
  "width": 1920,
  "height": 1080,
  "checksum": "abc123...",
  "uploaded_at": "2024-01-15T10:00:00Z"
}
```

## Content Delivery

For production, use CDN:

1. Configure cloud storage (S3, GCS)
2. Set up CloudFront/Cloud CDN
3. Update asset URLs to use CDN
