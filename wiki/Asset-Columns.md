# Asset Columns

**Tested** - Inline and cloud storage verified on 2026-01-25

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

### File Data Format

Asset columns expect an **array of file objects**, each containing:

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Filename with extension |
| `file` | Yes | Data URL: `data:mimetype;base64,CONTENT` |
| `type` | Optional | MIME type (e.g., `image/png`) |

### Upload Example

```bash
# Encode file to base64
IMG_BASE64=$(base64 < /path/to/image.png | tr -d '\n')

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
            "name": "product-photo.png",
            "file": "data:image/png;base64,'"$IMG_BASE64"'",
            "type": "image/png"
          }
        ]
      }
    }
  }'
```

**Response:**
```json
{
  "data": {
    "type": "product",
    "id": "019bf4b2-f2b4-767c-b174-825880ac0fac",
    "attributes": {
      "name": "Widget",
      "photo": "[{\"file\":\"data:image/png;base64,iVBOR...\",\"name\":\"product-photo.png\",\"type\":\"image/png\"}]"
    }
  }
}
```

**Note:** The response stores files as a JSON string. Parse it to access individual files.

## Accessing Inline Files

For inline storage, files are returned as a JSON string containing the array:

```bash
curl http://localhost:6336/api/product/ID \
  -H "Authorization: Bearer $TOKEN"
```

```json
{
  "data": {
    "attributes": {
      "photo": "[{\"file\":\"data:image/png;base64,iVBOR...\",\"name\":\"product-photo.png\",\"type\":\"image/png\"}]"
    }
  }
}
```

Parse and decode to use the file:

```javascript
// JavaScript example
const photoData = JSON.parse(response.data.attributes.photo);
const base64Content = photoData[0].file.split(',')[1];
// Use base64Content as needed
```

---

## Cloud Storage (For Large Files)

For files larger than a few KB, use cloud storage instead of inline.

See [Cloud Storage](Cloud-Storage.md) for setup instructions.

### How Cloud Storage Works

1. Create a `cloud_store` record (S3, GCS, local filesystem)
2. Define column with `ForeignKeyData` pointing to cloud store
3. Files are uploaded to storage; only metadata is stored in DB

### Define Cloud Storage Column

Configure cloud storage at schema level using `ForeignKeyData`:

```json
{
  "Tables": [
    {
      "TableName": "product",
      "Columns": [
        {
          "Name": "photo",
          "DataType": "text",
          "ColumnType": "file",
          "IsForeignKey": true,
          "ForeignKeyData": {
            "DataSource": "cloud_store",
            "Namespace": "my-storage",
            "KeyName": "photo"
          }
        }
      ]
    }
  ]
}
```

#### ForeignKeyData Configuration

| Field | Required | Description |
|-------|----------|-------------|
| `DataSource` | Yes | Must be `"cloud_store"` to trigger cloud storage logic |
| `Namespace` | Yes | Name of the `cloud_store` record (matches `name` column in cloud_store table) |
| `KeyName` | Yes | Subfolder within cloud storage root path. Files stored at `{root_path}/{KeyName}/` |

**Path Construction:**
```
Final path = cloud_store.root_path + "/" + ForeignKeyData.KeyName + "/" + [file.path] + "/" + file.name
```

Example with `root_path="/tmp/storage"` and `KeyName="photo"`:
- `/tmp/storage/photo/image.png`
- `/tmp/storage/photo/thumbnails/thumb.png` (if `path: "thumbnails"` is set)

### File Object Fields

#### Input Fields (when creating/updating)

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Filename with extension (e.g., `"image.png"`) |
| `file` | Yes | Data URL containing base64 content: `data:mimetype;base64,CONTENT` |
| `contents` | Alt | Alternative to `file` - just the base64 content without data URL prefix |
| `type` | No | MIME type (e.g., `"image/png"`). Optional but recommended. |
| `path` | No | Subdirectory within cloud storage. Allows organizing files in folders. |

#### Output Fields (in API response)

| Field | Description |
|-------|-------------|
| `name` | Filename |
| `type` | MIME type |
| `size` | File size in bytes (computed from decoded content) |
| `md5` | MD5 hash of file content (for integrity/deduplication) |
| `path` | Subdirectory path (empty string if not specified) |
| `src` | Same as `name` - used by frontend for display |

**Note:** The `file`/`contents` fields are **removed** from the stored data. Only metadata is kept in the database.

### Upload Example

```bash
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
            "name": "product-photo.png",
            "file": "data:image/png;base64,'"$IMG_BASE64"'",
            "type": "image/png"
          }
        ]
      }
    }
  }'
```

### Using the Path Field

Organize files into subdirectories:

```json
{
  "photo": [
    {
      "name": "original.png",
      "file": "data:image/png;base64,...",
      "path": "originals"
    },
    {
      "name": "thumb.png",
      "file": "data:image/png;base64,...",
      "path": "thumbnails"
    }
  ]
}
```

Results in cloud storage:
```
/root_path/KeyName/originals/original.png
/root_path/KeyName/thumbnails/thumb.png
```

### Cloud Storage Response Format

When cloud storage is configured, API returns metadata (not base64):

```json
{
  "data": {
    "attributes": {
      "photo": [
        {
          "name": "product-photo.png",
          "type": "image/png",
          "size": 70,
          "md5": "02c4278e5dc76862c17c04b3bd51946d",
          "src": "product-photo.png"
        }
      ]
    }
  }
}
```

**Note:** The `file` field (base64 content) is removed from the response.

### Asset Endpoint (Cloud Storage Only)

Access files directly via the asset endpoint:

```
GET http://localhost:6336/asset/{table}/{record_id}/{column}
```

Example:
```bash
curl -o photo.png "http://localhost:6336/asset/product/019bf4b7-3cb9-7c11-9b75-fdddd94daeec/photo" \
  -H "Authorization: Bearer $TOKEN"
```

**Note:** This endpoint only works with cloud storage, not inline base64.

---

## Update Files

Replace file with new content (same array format):

```bash
IMG_BASE64=$(base64 < /path/to/new-image.png | tr -d '\n')

curl -X PATCH http://localhost:6336/api/product/ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "product",
      "id": "ID",
      "attributes": {
        "photo": [
          {
            "name": "new-photo.png",
            "file": "data:image/png;base64,'"$IMG_BASE64"'",
            "type": "image/png"
          }
        ]
      }
    }
  }'
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

## Inline vs Cloud Storage Comparison

| Feature | Inline | Cloud Storage |
|---------|--------|---------------|
| Storage location | Database column | External storage (S3, GCS, local) |
| Response content | Full base64 data | Metadata only (md5, size, name) |
| Asset endpoint | Not available | Available |
| File size limit | Limited by DB column size | Unlimited |
| Use case | Small files (<100KB) | Large files, CDN integration |

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
