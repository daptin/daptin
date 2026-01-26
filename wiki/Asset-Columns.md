# Asset Columns

**Tested ✓** - Inline, cloud storage, and image processing verified on 2026-01-26

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

#### Authorization & Permissions

The asset endpoint checks permissions before serving files:

**Permission Model:**
- Based on table's `default_permission` setting
- Respects owner/usergroup/others permission levels
- Requires `Authorization` header for restricted content

**Permission Scenarios:**

| Table Permission | User Type | API Access | Asset Access |
|-----------------|-----------|------------|--------------|
| Public (default_permission > 0) | Anyone | ✅ Allowed | ✅ Allowed |
| Private (default_permission = 0) | Unauthenticated | ❌ 403 | ❌ 500 |
| Private (default_permission = 0) | Owner | ✅ Allowed | ✅ Allowed |
| Private (default_permission = 0) | Non-owner | ❌ 403 | ⚠️ See Note |

**⚠️ Security Note (2026-01-26):** Asset endpoint permission checking has been verified to work for unauthenticated users and table owners. Non-owner access behavior should be tested in your specific deployment. For sensitive files, verify permissions work as expected before production use.

**Best Practices:**
1. Always use `Authorization` header for non-public assets
2. Set `default_permission=0` for tables with sensitive files
3. Test permission behavior with your security model
4. Consider using API endpoints (inline storage) for highly sensitive files

**Example: Restricted Asset Column**
```yaml
Tables:
  - TableName: private_document
    DefaultPermission: 0  # No public access
    Columns:
      - Name: confidential_file
        DataType: text
        ColumnType: file
        IsForeignKey: true
        ForeignKeyData:
          DataSource: cloud_store
          Namespace: private-storage
          KeyName: documents
```

Access attempt without authorization:
```bash
# Without auth token
curl http://localhost:6336/asset/private_document/{id}/confidential_file
# Response: 500 Internal Server Error

# With valid owner token
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:6336/asset/private_document/{id}/confidential_file
# Response: 200 OK - File served
```

### Image Processing & Transformations

For images accessed via the asset endpoint, Daptin provides on-the-fly image transformations using query parameters. Enable processing by adding `processImage=true` to the URL.

**Format:**
```
GET /asset/{table}/{record_id}/{column}?processImage=true&{filter}={value}
```

#### Available Filters

**Resize & Crop**

| Filter | Parameters | Description | Example |
|--------|------------|-------------|---------|
| `resize` | `height,width,method` | Resize image | `resize=400,300,Linear` |
| `crop` | `minX,minY,maxX,maxY` | Crop to rectangle | `crop=0,0,200,200` |
| `cropToSize` | `height,width,anchor` | Crop to size with anchor | `cropToSize=200,200,Center` |

**Resize methods:** `NearestNeighbor`, `Box`, `Linear`, `Cubic`, `Lanczos`

**Crop anchors:** `Center`, `TopLeft`, `Top`, `TopRight`, `Left`, `Right`, `BottomLeft`, `Bottom`, `BottomRight`

**Color Adjustments**

| Filter | Parameters | Description | Example |
|--------|------------|-------------|---------|
| `brightness` | `value` | Adjust brightness (-100 to 100) | `brightness=30` |
| `contrast` | `value` | Adjust contrast (-100 to 100) | `contrast=20` |
| `saturation` | `value` | Adjust saturation (-100 to 500) | `saturation=150` |
| `hue` | `value` | Adjust hue (-180 to 180) | `hue=45` |
| `gamma` | `value` | Gamma correction (0.1 to 10) | `gamma=1.5` |
| `colorBalance` | `red,green,blue` | Adjust RGB balance | `colorBalance=10,-5,0` |
| `colorize` | `hue,sat,percent` | Colorize image | `colorize=120,50,80` |

**Effects**

| Filter | Parameters | Description | Example |
|--------|------------|-------------|---------|
| `grayscale` | `true` or `1` | Convert to grayscale | `grayscale=true` |
| `sepia` | `value` | Apply sepia tone (0-100) | `sepia=80` |
| `invert` | `true` or `1` | Invert colors | `invert=true` |
| `emboss` | (no params) | Emboss effect | `emboss=true` |
| `sobel` | `true` or `1` | Sobel edge detection | `sobel=true` |
| `edgedetection` | `radius` | Edge detection filter | `edgedetection=1.5` |

**Blur & Sharpen**

| Filter | Parameters | Description | Example |
|--------|------------|-------------|---------|
| `gaussianBlur` | `radius` | Gaussian blur | `gaussianBlur=2.5` |
| `boxblur` | `radius` | Box blur | `boxblur=3.0` |
| `median` | `radius` | Median filter (noise reduction) | `median=2.0` |
| `sharpen` | (no params) | Sharpen image | `sharpen=true` |

**Rotate & Flip**

| Filter | Parameters | Description | Example |
|--------|------------|-------------|---------|
| `rotate` | `angle,bgColor,method` | Rotate by angle | `rotate=45,ffffff,Linear` |
| `rotate90` | `true` or `1` | Rotate 90° clockwise | `rotate90=true` |
| `rotate180` | `true` or `1` | Rotate 180° | `rotate180=true` |
| `rotate270` | `true` or `1` | Rotate 270° clockwise | `rotate270=true` |
| `flipHorizontal` | `true` or `1` | Flip horizontally | `flipHorizontal=true` |
| `flipVertical` | `true` or `1` | Flip vertically | `flipVertical=true` |
| `transpose` | `true` or `1` | Transpose image | `transpose=true` |
| `transverse` | `true` or `1` | Transverse image | `transverse=true` |

**Rotate interpolation methods:** `NearestNeighbor`, `Linear`, `Cubic`

**Morphological Operations**

| Filter | Parameters | Description | Example |
|--------|------------|-------------|---------|
| `dilate` | `radius` | Morphological dilation | `dilate=2.0` |
| `erode` | `radius` | Morphological erosion | `erode=2.0` |
| `threshold` | `value` | Binary threshold (0-100) | `threshold=50` |

**Color Space**

| Filter | Parameters | Description | Example |
|--------|------------|-------------|---------|
| `colorspaceLinearToSRGB` | `true` or `1` | Convert Linear to sRGB | `colorspaceLinearToSRGB=true` |
| `colorspaceSRGBToLinear` | `true` or `1` | Convert sRGB to Linear | `colorspaceSRGBToLinear=true` |

#### Usage Examples

**Basic transformations:**
```bash
TOKEN=$(cat /tmp/daptin-token.txt)

# Grayscale thumbnail
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/asset/product/ID/photo?processImage=true&resize=200,200,Linear&grayscale=true" \
  -o thumbnail.png

# Sepia with increased brightness
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/asset/product/ID/photo?processImage=true&sepia=80&brightness=20" \
  -o sepia-photo.png

# Crop and rotate
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/asset/product/ID/photo?processImage=true&cropToSize=300,300,Center&rotate90=true" \
  -o cropped-rotated.png
```

**Multiple filters (applied in order):**
```bash
# Resize, blur, and adjust brightness
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/asset/product/ID/photo?processImage=true&resize=800,600,Lanczos&gaussianBlur=1.5&brightness=15&contrast=10" \
  -o processed-photo.png
```

**Edge detection for analysis:**
```bash
# Extract edges from image
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/asset/product/ID/photo?processImage=true&edgedetection=2.0&grayscale=true" \
  -o edges.png
```

#### Use Cases

1. **Thumbnails**: `resize=150,150,Linear&cropToSize=150,150,Center`
2. **Profile Pictures**: `cropToSize=200,200,Center&grayscale=true`
3. **Watermarked Preview**: `brightness=-30&contrast=20`
4. **Mobile Optimization**: `resize=800,600,Cubic&gaussianBlur=0.5`
5. **Artistic Effects**: `sepia=90&contrast=15&saturation=80`
6. **Image Analysis**: `sobel=true` or `edgedetection=1.5`

#### Performance Notes

- Transformations are applied **on-the-fly** (not cached)
- For frequently accessed transformed images, consider pre-generating versions
- Complex filter chains may increase response time
- Filters are applied in the order they appear in the URL

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

---

## Troubleshooting

### Files Upload But Column Returns Null

**Symptom**: File upload appears successful but column returns `null` instead of metadata.

**Cause**: The cloud_store referenced in `Namespace` doesn't exist.

**Example**:
```yaml
# Schema references "product-images"
ForeignKeyData:
  Namespace: product-images  # ← Must match cloud_store name
```

**Solution**:
1. Check if cloud_store exists:
   ```bash
   curl -s -H "Authorization: Bearer $TOKEN" \
     "http://localhost:6336/api/cloud_store" | \
     jq '.data[] | .attributes.name'
   ```

2. Create the cloud_store with matching name:
   ```bash
   curl -X POST http://localhost:6336/api/cloud_store \
     -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/vnd.api+json" \
     -d '{
       "data": {
         "type": "cloud_store",
         "attributes": {
           "name": "product-images",
           "store_type": "local",
           "store_provider": "local",
           "root_path": "./storage",
           "store_parameters": "{}"
         }
       }
     }'
   ```

3. **Restart server** (CRITICAL):
   ```bash
   ./scripts/testing/test-runner.sh stop
   ./scripts/testing/test-runner.sh start
   ```

4. Retry file upload

**Key Point**: The `Namespace` field in schema MUST exactly match the `name` field of an existing cloud_store record.

### Asset Endpoint Returns 404

**Symptom**: `/asset/{table}/{id}/{column}` returns 404 or empty response.

**Possible Causes**:

1. **Using inline storage instead of cloud storage**
   - Asset endpoint only works with cloud storage
   - Check if column has `IsForeignKey: true` and `ForeignKeyData` in schema

2. **File not uploaded yet**
   - Verify file metadata exists:
     ```bash
     curl -s -H "Authorization: Bearer $TOKEN" \
       "http://localhost:6336/api/{table}/{id}" | \
       jq '.data.attributes.{column}'
     ```
   - Should return metadata array (md5, size, name), not null

3. **File missing from storage**
   - Check file exists on disk:
     ```bash
     ls -la {root_path}/{KeyName}/
     ```

4. **Authorization required**
   - Asset endpoint requires Bearer token:
     ```bash
     curl -H "Authorization: Bearer $TOKEN" \
       "http://localhost:6336/asset/{table}/{id}/{column}"
     ```

### Server Restart Required

**When to restart**:
1. After creating new cloud_store records
2. After linking cloud_store to tables (via relationship PATCH)
3. After changing schema with new ForeignKeyData
4. When cloud storage not being recognized

**How to restart**:
```bash
./scripts/testing/test-runner.sh stop
./scripts/testing/test-runner.sh start
# Wait ~5 seconds for initialization
```

**Why**: Daptin loads cloud storage configuration and table-to-storage mappings at startup.

### Files Go to Wrong Directory

**Symptom**: Files stored at unexpected path.

**Path construction**:
```
{cloud_store.root_path}/{ForeignKeyData.KeyName}/{file.path}/{file.name}
```

**Example**:
- `root_path`: `./storage`
- `KeyName`: `photos`
- `file.path`: `thumbnails` (from upload)
- `file.name`: `image.png`
- **Result**: `./storage/photos/thumbnails/image.png`

**Common mistakes**:
- Expecting files in `{root_path}` directly → They go in `{root_path}/{KeyName}/`
- Forgetting `KeyName` adds a subfolder
- Not accounting for optional `file.path` parameter

### Wrong File Format Error

**Symptom**: File doesn't upload, or error about format.

**Requirements**:
1. **Must be array**: `[{name, file, type}]` not `{name, file, type}`
2. **Data URI format**: `data:mimetype;base64,CONTENT`
3. **All required fields**: `name` and `file` are mandatory

**Correct**:
```json
{
  "photo": [{
    "name": "image.png",
    "file": "data:image/png;base64,iVBORw0KG...",
    "type": "image/png"
  }]
}
```

**Wrong**:
```json
// ❌ Not an array
{"photo": {"name": "image.png", "file": "..."}}

// ❌ Missing data URI prefix
{"photo": [{"name": "image.png", "file": "iVBORw0KG..."}]}

// ❌ Plain string
{"photo": "data:image/png;base64,..."}
```

---

## See Also

- [Cloud Storage](Cloud-Storage.md) - For large file storage
- [Schema Definition](Schema-Definition.md) - Table and column setup
