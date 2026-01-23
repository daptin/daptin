# Data Actions

Actions for importing, exporting, and managing data.

## import_data

Import data from JSON, CSV, or XLSX files.

```bash
curl -X POST http://localhost:6336/action/todo/__data_import \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "table_name": "todo",
      "dump_file": [{
        "name": "todos.csv",
        "file": "data:text/csv;base64,dGl0bGUsY29tcGxldGVkClRhc2sgMSxmYWxzZQpUYXNrIDIsdHJ1ZQ=="
      }],
      "truncate_before_insert": false,
      "batch_size": 500
    }
  }'
```

**Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| table_name | string | Target table |
| dump_file | array | File(s) to import |
| truncate_before_insert | bool | Clear table first |
| batch_size | int | Records per batch |

**Supported formats:**
- CSV (`.csv`)
- JSON (`.json`)
- Excel (`.xlsx`)

### JSON Format

```json
[
  {"title": "Task 1", "completed": false},
  {"title": "Task 2", "completed": true}
]
```

### CSV Format

```csv
title,completed
Task 1,false
Task 2,true
```

## export_data

Export table data in multiple formats.

```bash
curl -X POST http://localhost:6336/action/todo/__data_export \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "table_name": "todo",
      "format": "json"
    }
  }'
```

**Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| table_name | string | Source table |
| format | string | json, csv, xlsx, pdf, html |
| columns | array | Specific columns (optional) |
| include_headers | bool | Include column headers |
| page_size | int | Records per page (streaming) |

**Response:**
```json
[
  {
    "ResponseType": "client.file.download",
    "Attributes": {
      "content": "base64-encoded-data",
      "name": "todo_export.json",
      "contentType": "application/json"
    }
  }
]
```

### Export Formats

| Format | Content-Type | Description |
|--------|--------------|-------------|
| json | application/json | JSON array |
| csv | text/csv | Comma-separated |
| xlsx | application/vnd.openxmlformats-officedocument.spreadsheetml.sheet | Excel |
| pdf | application/pdf | PDF table |
| html | text/html | HTML table |

## export_csv_data

Export specifically as CSV.

```bash
curl -X POST http://localhost:6336/action/todo/__export_csv_data \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "table_name": "todo",
      "include_headers": true
    }
  }'
```

## csv_to_entity

Create a new table from CSV file.

```bash
curl -X POST http://localhost:6336/action/world/csv_to_entity \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "dump_file": [{
        "name": "products.csv",
        "file": "data:text/csv;base64,..."
      }],
      "entity_name": "product"
    }
  }'
```

Daptin automatically:
1. Analyzes CSV headers
2. Detects column types
3. Creates table schema
4. Imports data

**Type detection:**
- Email patterns → `email` type
- URLs → `url` type
- Dates → `datetime` type
- Numbers → `measurement` type
- Booleans → `truefalse` type

## xls_to_entity

Create table from Excel file.

```bash
curl -X POST http://localhost:6336/action/world/xls_to_entity \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "dump_file": [{
        "name": "data.xlsx",
        "file": "data:application/vnd.openxmlformats-officedocument.spreadsheetml.sheet;base64,..."
      }],
      "entity_name": "imported_data"
    }
  }'
```

## import_cloudstore_files

Import files from cloud storage.

```bash
curl -X POST http://localhost:6336/action/cloud_store/import_cloudstore_files \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "cloud_store_id": "CLOUDSTORE_ID",
      "path": "/imports/data.csv",
      "table_name": "imported_table"
    }
  }'
```

## generate_random_data

Generate test data for a table.

```bash
curl -X POST http://localhost:6336/action/todo/__generate_random_data \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "count": 100
    }
  }'
```

Uses faker library to generate realistic data based on column types.

## random_value_generate

Generate a single random value.

```bash
curl -X POST http://localhost:6336/action/world/random_value_generate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "type": "email"
    }
  }'
```

**Supported types:**
- email
- name
- phone
- address
- uuid
- number
- date
- text

## Streaming Export

For large datasets, use streaming:

```bash
curl -X POST http://localhost:6336/action/large_table/__data_export \
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

Exports in chunks to avoid memory issues.

## Batch Import

For large imports, use batching:

```bash
curl -X POST http://localhost:6336/action/world/__data_import \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "attributes": {
      "table_name": "large_table",
      "batch_size": 1000,
      "dump_file": [...]
    }
  }'
```

Inserts records in batches of 1000.
