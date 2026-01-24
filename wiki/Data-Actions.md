# Data Actions

Actions for importing, exporting, and managing data.

**Related**: [Data Exchange](Data-Exchange.md) | [Cloud Storage](Cloud-Storage.md) | [Actions Overview](Actions-Overview.md)

**Source of truth**: `server/resource/columns.go` (SystemActions), `server/actions/action_*_data.go` (performers)

---

## export_data

Export table data in multiple formats.

**Action**: `export_data`
**OnType**: `world`
**InstanceOptional**: true (no instance ID required)

```bash
curl -X POST http://localhost:6336/action/world/export_data \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "table_name": "todo",
      "format": "json"
    }
  }'
```

**Parameters**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `table_name` | string | Table to export (optional - exports all if omitted) |
| `format` | string | Output format: `json`, `csv`, `xlsx`, `pdf`, `html` (default: `json`) |
| `columns` | array | Specific columns to export (optional - all if omitted) |
| `include_headers` | bool | Include column headers (default: true) |
| `page_size` | int | Records per batch for streaming (default: 1000) |

**Response**:
```json
[
  {
    "ResponseType": "client.file.download",
    "Attributes": {
      "content": "base64-encoded-data",
      "name": "daptin_export_todo.json",
      "contentType": "application/json",
      "message": "Downloading data as json"
    }
  }
]
```

### Export Formats

| Format | Content-Type | Extension |
|--------|--------------|-----------|
| `json` | application/json | .json |
| `csv` | text/csv | .csv |
| `xlsx` | application/vnd.openxmlformats-officedocument.spreadsheetml.sheet | .xlsx |
| `pdf` | application/pdf | .pdf |
| `html` | text/html | .html |

### Export Specific Columns

```bash
curl -X POST http://localhost:6336/action/world/export_data \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "table_name": "todo",
      "format": "csv",
      "columns": ["title", "completed", "created_at"]
    }
  }'
```

---

## export_csv_data

Export specifically as CSV (shortcut for export_data with format=csv).

**Action**: `export_csv_data`
**OnType**: `world`
**InstanceOptional**: true

```bash
curl -X POST http://localhost:6336/action/world/export_csv_data \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "table_name": "todo"
    }
  }'
```

---

## import_data

Import data from files into a table.

**Action**: `import_data`
**OnType**: `world`
**InstanceOptional**: false (requires table's world reference_id)

```bash
# First get the table's reference_id from the world table
TABLE_REF=$(curl -s "http://localhost:6336/api/world?filter[table_name]=todo" \
  -H "Authorization: Bearer $TOKEN" | jq -r '.data[0].id')

# Then import data
curl -X POST "http://localhost:6336/action/world/$TABLE_REF/import_data" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "dump_file": [{
        "name": "todos.csv",
        "file": "data:text/csv;base64,dGl0bGUsY29tcGxldGVkClRhc2sgMSxmYWxzZQpUYXNrIDIsdHJ1ZQ=="
      }],
      "truncate_before_insert": false,
      "batch_size": 100
    }
  }'
```

**Parameters**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `dump_file` | array | Files to import (base64-encoded with name) |
| `truncate_before_insert` | bool | Clear table before import (default: false) |
| `batch_size` | int | Records per insert batch (default: 100) |

**File format**: Each file in `dump_file` array:
```json
{
  "name": "filename.csv",
  "file": "data:text/csv;base64,BASE64_CONTENT"
}
```

**Supported file formats**:
- CSV (`.csv`)
- JSON (`.json`)
- YAML (`.yaml`, `.yml`)
- TOML (`.toml`)
- HCL (`.hcl`)
- Excel (`.xlsx`)
- PDF (`.pdf`) - text extraction
- HTML (`.html`) - table extraction
- Word (`.docx`) - table extraction

**Response**:
```json
[
  {
    "ResponseType": "client.notify",
    "Attributes": {
      "message": "Import completed in 123ms. 50 rows imported successfully across 1 tables.",
      "rows_imported": 50,
      "successful_tables": 1,
      "failed_tables": 0
    }
  }
]
```

### JSON Import Format

```json
[
  {"title": "Task 1", "completed": false},
  {"title": "Task 2", "completed": true}
]
```

### CSV Import Format

```csv
title,completed
Task 1,false
Task 2,true
```

---

## import_files_from_store

Import files from cloud storage into a table.

**Action**: `import_files_from_store`
**OnType**: `world`
**InstanceOptional**: false (requires table's world reference_id)

```bash
curl -X POST "http://localhost:6336/action/world/$TABLE_REF/import_files_from_store" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "table_name": "todo"
    }
  }'
```

**Prerequisites**: Table must have a cloud_store relationship configured.

---

## generate_random_data

Generate test data for a table using faker library.

**Action**: `generate_random_data`
**OnType**: `world`
**InstanceOptional**: true

```bash
curl -X POST http://localhost:6336/action/world/generate_random_data \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "table_name": "todo",
      "count": 100
    }
  }'
```

**Parameters**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `table_name` | string | Table to populate |
| `count` | int | Number of records to generate (must be > 0) |

**Faker type detection** based on column types:
- `email` → email address
- `name`, `label` → person name or lorem words
- `content` → paragraph text
- `datetime`, `date` → random date
- `measurement`, `rating` → random number
- `truefalse` → random boolean
- `url` → URL

---

## upload_file

Upload file to cloud storage.

**Action**: `upload_file`
**OnType**: `cloud_store`
**InstanceOptional**: false (requires cloud_store reference_id)

```bash
curl -X POST "http://localhost:6336/action/cloud_store/$CLOUDSTORE_REF/upload_file" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "file": [{
        "name": "document.pdf",
        "file": "data:application/pdf;base64,BASE64_CONTENT"
      }],
      "path": "/uploads/"
    }
  }'
```

**Parameters**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `file` | array | File(s) to upload (base64-encoded) |
| `path` | string | Destination path in cloud store |

See [Cloud Storage](Cloud-Storage.md) for cloud_store setup.

---

## Streaming Export for Large Data

For large datasets, use streaming with page_size:

```bash
curl -X POST http://localhost:6336/action/world/export_data \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "table_name": "large_table",
      "format": "csv",
      "page_size": 10000
    }
  }'
```

Data is streamed in chunks to avoid memory issues.

---

## Batch Import for Large Data

For large imports, adjust batch_size:

```bash
curl -X POST "http://localhost:6336/action/world/$TABLE_REF/import_data" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "dump_file": [...],
      "batch_size": 1000
    }
  }'
```

Records are inserted in batches for better performance.

---

## Complete Import/Export Workflow

### Export Data for Backup

```bash
# Export all tables as JSON
curl -X POST http://localhost:6336/action/world/export_data \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"attributes": {"format": "json"}}'

# Export specific table as CSV
curl -X POST http://localhost:6336/action/world/export_data \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"attributes": {"table_name": "todo", "format": "csv"}}'
```

### Import Data from Backup

```bash
# Get table reference_id
TABLE_REF=$(curl -s "http://localhost:6336/api/world?filter[table_name]=todo" \
  -H "Authorization: Bearer $TOKEN" | jq -r '.data[0].id')

# Import (truncate first for clean restore)
curl -X POST "http://localhost:6336/action/world/$TABLE_REF/import_data" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "dump_file": [{"name": "backup.json", "file": "..."}],
      "truncate_before_insert": true
    }
  }'
```

---

## Troubleshooting

### "no reference id" on import_data

Import requires the table's world record reference_id:
```bash
# Get the reference_id first
curl "http://localhost:6336/api/world?filter[table_name]=todo" \
  -H "Authorization: Bearer $TOKEN"
```

### Import fails with "no files provided"

Ensure `dump_file` is an array of file objects with `name` and `file` keys:
```json
{
  "dump_file": [{
    "name": "data.csv",
    "file": "data:text/csv;base64,..."
  }]
}
```

### Export returns empty content

1. Verify table exists and has data
2. Check you have read permission on the table
3. Verify column names if using `columns` parameter

---

## See Also

- [Data Exchange](Data-Exchange.md) - External API syncing
- [Cloud Storage](Cloud-Storage.md) - File storage setup
- [Actions Overview](Actions-Overview.md) - Action system details
