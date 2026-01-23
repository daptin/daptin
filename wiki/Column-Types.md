# Column Types

Daptin supports **41 built-in column types** for automatic validation, storage, and API type mapping.

**Related**: [Core Concepts](Core-Concepts.md) | [Column Type Reference](Column-Type-Reference.md) | [Schema Definition](Schema-Definition.md)

**Source of truth**: `server/resource/column_types.go`

## Basic Types

| ColumnType | DataType | Description |
|------------|----------|-------------|
| `label` | varchar(500) | Short text, single line |
| `name` | varchar(200) | Name field |
| `text` | varchar(1000) | Medium text |
| `textarea` | text | Long text, multiline |
| `content` | text | Rich content |
| `hidden` | varchar(500) | Hidden field |

## Numeric Types

| ColumnType | DataType | Description |
|------------|----------|-------------|
| `measurement` | float(8) | Numeric measurement |
| `decimal` | double | Decimal number |
| `percent` | float(8) | Percentage (0-100) |
| `rating5` | int(11) | Rating 1-5 |
| `rating10` | int(11) | Rating 1-10 |
| `rating100` | int(11) | Rating 1-100 |

## Boolean Types

| ColumnType | DataType | Description |
|------------|----------|-------------|
| `truefalse` | bool | True/false toggle |

## Date/Time Types

| ColumnType | DataType | Description |
|------------|----------|-------------|
| `datetime` | datetime | Full date and time |
| `date` | date | Date only |
| `time` | time | Time only |
| `day` | int(11) | Day of month (1-31) |
| `month` | int(11) | Month (1-12) |
| `year` | int(11) | Year |
| `timestamp` | timestamp | Unix timestamp |

## Identity Types

| ColumnType | DataType | Description |
|------------|----------|-------------|
| `email` | varchar(200) | Email address |
| `url` | varchar(500) | Web URL |
| `image` | varchar(500) | Image URL |
| `id` | varchar(40) | UUID reference |
| `alias` | varchar(200) | URL-safe slug |

## Security Types

| ColumnType | DataType | Description |
|------------|----------|-------------|
| `password` | varchar(200) | Hashed password |
| `bcrypt` | varchar(200) | Bcrypt hash |
| `encrypted` | text | AES encrypted |

## Geographic Types

| ColumnType | DataType | Description |
|------------|----------|-------------|
| `location` | varchar(100) | Lat/long coordinates |
| `location.latitude` | float(8) | Latitude |
| `location.longitude` | float(8) | Longitude |

## File Types

| ColumnType | DataType | Description |
|------------|----------|-------------|
| `file` | text | Generic file |
| `file.image` | text | Image file |
| `file.video` | text | Video file |
| `file.audio` | text | Audio file |
| `file.document` | text | Document file |
| `file.spreadsheet` | text | Spreadsheet file |
| `file.pdf` | text | PDF file |

File columns support:
- Base64 encoding
- Cloud storage sync
- YJS collaborative editing
- Presigned URL generation

## Data Types

| ColumnType | DataType | Description |
|------------|----------|-------------|
| `json` | text | JSON object |
| `markdown` | text | Markdown content |
| `html` | text | HTML content |

## Selection Types

| ColumnType | DataType | Description |
|------------|----------|-------------|
| `enum` | varchar(100) | Enumerated values |
| `select` | varchar(100) | Single select |
| `multiselect` | text | Multiple select (JSON) |

### Enum Configuration

```yaml
Columns:
  - Name: status
    DataType: varchar(50)
    ColumnType: enum
    Values:
      - draft
      - published
      - archived
```

## Color Types

| ColumnType | DataType | Description |
|------------|----------|-------------|
| `color` | varchar(20) | Hex color (#RRGGBB) |

## Network Types

| ColumnType | DataType | Description |
|------------|----------|-------------|
| `ipaddress` | varchar(50) | IPv4/IPv6 address |

## Special Types

| ColumnType | DataType | Description |
|------------|----------|-------------|
| `uuid` | varchar(40) | UUID v7 |
| `namespace` | varchar(200) | Namespace path |

## Type Detection

When importing data, Daptin automatically detects types:

```go
// Detection priority
1. UUID pattern -> id
2. Email pattern -> email
3. URL pattern -> url
4. DateTime pattern -> datetime
5. Date pattern -> date
6. Time pattern -> time
7. Boolean -> truefalse
8. Integer -> measurement
9. Float -> decimal
10. JSON -> json
11. Default -> label
```

## Column Definition Example

```yaml
Columns:
  # Text
  - Name: title
    DataType: varchar(500)
    ColumnType: label
    IsNullable: false

  # Numeric
  - Name: price
    DataType: float(8)
    ColumnType: measurement

  # Boolean
  - Name: active
    DataType: bool
    ColumnType: truefalse
    DefaultValue: "true"

  # DateTime
  - Name: published_at
    DataType: datetime
    ColumnType: datetime

  # Email
  - Name: contact_email
    DataType: varchar(200)
    ColumnType: email

  # File
  - Name: attachment
    DataType: text
    ColumnType: file.document

  # JSON
  - Name: metadata
    DataType: text
    ColumnType: json

  # Enum
  - Name: status
    DataType: varchar(50)
    ColumnType: enum
    Values: ["draft", "review", "published"]

  # Location
  - Name: coordinates
    DataType: varchar(100)
    ColumnType: location

  # Encrypted
  - Name: secret_data
    DataType: text
    ColumnType: encrypted
```

## Validation

Types include automatic validation:

| Type | Validation |
|------|------------|
| `email` | RFC 5322 email format |
| `url` | Valid URL format |
| `datetime` | ISO 8601 format |
| `json` | Valid JSON |
| `rating5` | Range 1-5 |
| `percent` | Range 0-100 |
