# Column Type Reference

Complete reference of all column types supported by Daptin.

**Related**: [Core Concepts](Core-Concepts.md) | [Column Types](Column-Types.md) | [Schema Definition](Schema-Definition.md)

**Source of truth**: `server/resource/column_types.go`

## Overview

Column types define data storage, validation, and GraphQL type mapping. Each type has:
- **BlueprintType**: JSON Schema type
- **ReclineType**: Data grid display type
- **DataTypes**: SQL data types used
- **GraphqlType**: GraphQL schema type
- **Validations**: Automatic validation rules
- **Conformations**: Automatic data transformations

## String Types

### label

Short text field for names, identifiers, titles.

| Property | Value |
|----------|-------|
| SQL Type | `varchar(100)` |
| GraphQL Type | `String` |
| Max Length | 100 characters |
| Fake Data | Product name |

```yaml
Columns:
  - ColumnName: title
    ColumnType: label
```

### content

Long text field for descriptions, body content.

| Property | Value |
|----------|-------|
| SQL Type | `text` |
| GraphQL Type | `String` |
| Max Length | Unlimited |
| Fake Data | Multiple sentences |

```yaml
Columns:
  - ColumnName: description
    ColumnType: content
```

### html

HTML content field.

| Property | Value |
|----------|-------|
| SQL Type | `text` |
| GraphQL Type | `String` |
| Max Length | Unlimited |

```yaml
Columns:
  - ColumnName: body_html
    ColumnType: html
```

### markdown

Markdown formatted text.

| Property | Value |
|----------|-------|
| SQL Type | `text` |
| GraphQL Type | `String` |
| Max Length | Unlimited |

```yaml
Columns:
  - ColumnName: readme
    ColumnType: markdown
```

### name

Name field with required validation.

| Property | Value |
|----------|-------|
| SQL Type | `varchar(100)` |
| GraphQL Type | `String` |
| Validation | Required |
| Conformation | Name format |
| Fake Data | Full name |

### alias

Short identifier or reference.

| Property | Value |
|----------|-------|
| SQL Type | `varchar(100)` |
| GraphQL Type | `String` |
| Fake Data | UUID |

### id

Unique identifier field.

| Property | Value |
|----------|-------|
| SQL Type | `varchar(20)` |
| GraphQL Type | `ID` |
| Fake Data | UUID |

### namespace

Namespace identifier for hierarchical organization.

| Property | Value |
|----------|-------|
| SQL Type | `varchar(200)` |
| GraphQL Type | `String` |

### value

Generic value field.

| Property | Value |
|----------|-------|
| SQL Type | `varchar(100)` |
| GraphQL Type | `String` |
| Fake Data | Random number 0-1000 |

### hidden

Hidden field (not displayed in forms).

| Property | Value |
|----------|-------|
| SQL Type | `varchar(100)` |
| GraphQL Type | `String` |

## Email Type

### email

Email address with validation.

| Property | Value |
|----------|-------|
| SQL Type | `varchar(100)` |
| GraphQL Type | `String` |
| Validation | Email format |
| Conformation | Email normalization |
| Fake Data | Random email address |

```yaml
Columns:
  - ColumnName: email
    ColumnType: email
```

## Password Types

### password

Password field with required validation. Stored hashed.

| Property | Value |
|----------|-------|
| SQL Type | `varchar(200)` |
| GraphQL Type | `String` |
| Validation | Required |
| Fake Data | Simple password |

### bcrypt

BCrypt hashed password.

| Property | Value |
|----------|-------|
| SQL Type | `varchar(200)` |
| GraphQL Type | `String` |
| Validation | Required |

### md5

MD5 hashed value.

| Property | Value |
|----------|-------|
| SQL Type | `varchar(200)` |
| GraphQL Type | `String` |
| Validation | Required |

### md5-bcrypt

MD5 hash of BCrypt hash (double hashed).

| Property | Value |
|----------|-------|
| SQL Type | `varchar(200)` |
| GraphQL Type | `String` |
| Validation | Required |

## Encrypted Type

### encrypted

Encrypted field for sensitive data.

| Property | Value |
|----------|-------|
| SQL Type | `text` |
| GraphQL Type | `String` |

Data is encrypted at rest using the system encryption key.

```yaml
Columns:
  - ColumnName: api_key
    ColumnType: encrypted
```

## Numeric Types

### measurement

Integer measurement value.

| Property | Value |
|----------|-------|
| SQL Type | `int(10)` |
| GraphQL Type | `Int` |
| Fake Data | Random 0-5000 |

```yaml
Columns:
  - ColumnName: quantity
    ColumnType: measurement
```

### float

Floating point number.

| Property | Value |
|----------|-------|
| SQL Type | `float(7,4)` |
| GraphQL Type | `Float` |

### rating

Rating value 0-10.

| Property | Value |
|----------|-------|
| SQL Type | `int(4)` |
| GraphQL Type | `Int` |
| Validation | min=0, max=10 |
| Fake Data | Random 0-10 |

## Boolean Type

### truefalse

Boolean true/false value.

| Property | Value |
|----------|-------|
| SQL Type | `boolean` |
| GraphQL Type | `Boolean` |
| Fake Data | Random 0 or 1 |

```yaml
Columns:
  - ColumnName: is_active
    ColumnType: truefalse
```

## Date/Time Types

### datetime

Full date and time.

| Property | Value |
|----------|-------|
| SQL Type | `timestamp` |
| GraphQL Type | `DateTime` |
| Format | RFC3339 |
| Fake Data | Random date 1980-2050 |

```yaml
Columns:
  - ColumnName: created_at
    ColumnType: datetime
```

### date

Date only (no time).

| Property | Value |
|----------|-------|
| SQL Type | `timestamp` |
| GraphQL Type | `DateTime` |
| Format | `2006-01-02` |
| Fake Data | Random date |

### time

Time only (no date).

| Property | Value |
|----------|-------|
| SQL Type | `timestamp` |
| GraphQL Type | `String` |
| Format | `15:04:05` |
| Fake Data | Random time |

### timestamp

Unix timestamp.

| Property | Value |
|----------|-------|
| SQL Type | `timestamp` |
| GraphQL Type | `DateTime` |
| Fake Data | Random unix timestamp |

### year

Year value.

| Property | Value |
|----------|-------|
| SQL Type | `int(4)` |
| GraphQL Type | `Int` |
| Validation | min=100, max=2100 |
| Fake Data | Random 1990-2018 |

### month

Month value 1-12.

| Property | Value |
|----------|-------|
| SQL Type | `int(4)` |
| GraphQL Type | `Int` |
| Validation | min=1, max=12 |
| Fake Data | Random month name |

### day

Day name.

| Property | Value |
|----------|-------|
| SQL Type | `varchar(10)` |
| GraphQL Type | `String` |
| Fake Data | Random day name |

### hour

Hour value 0-23.

| Property | Value |
|----------|-------|
| SQL Type | `int(4)` |
| GraphQL Type | `Int` |
| Fake Data | Random 0-23 |

### minute

Minute value 0-59.

| Property | Value |
|----------|-------|
| SQL Type | `int(4)` |
| GraphQL Type | `Int` |
| Validation | min=0, max=59 |
| Fake Data | Random 0-59 |

## Location Types

### location

Combined latitude/longitude as JSON array.

| Property | Value |
|----------|-------|
| SQL Type | `varchar(50)` |
| GraphQL Type | `String` |
| Format | `[lat, lng]` |
| Fake Data | Random coordinates |

```yaml
Columns:
  - ColumnName: coordinates
    ColumnType: location
```

### location.latitude

Latitude coordinate.

| Property | Value |
|----------|-------|
| SQL Type | `float(7,4)` |
| GraphQL Type | `Float` |
| Validation | Valid latitude |
| Fake Data | Random latitude |

### location.longitude

Longitude coordinate.

| Property | Value |
|----------|-------|
| SQL Type | `float(7,4)` |
| GraphQL Type | `Float` |
| Validation | Valid longitude |
| Fake Data | Random longitude |

### location.altitude

Altitude in meters.

| Property | Value |
|----------|-------|
| SQL Type | `float(7,4)` |
| GraphQL Type | `Float` |
| Fake Data | Random 0-10000 |

## URL Type

### url

URL with validation.

| Property | Value |
|----------|-------|
| SQL Type | `varchar(500)` |
| GraphQL Type | `String` |
| Validation | URL format |
| Fake Data | Example URL |

```yaml
Columns:
  - ColumnName: website
    ColumnType: url
```

## Color Type

### color

Color value (hex format).

| Property | Value |
|----------|-------|
| SQL Type | `varchar(50)` |
| GraphQL Type | `String` |
| Validation | Valid color |
| Fake Data | Random hex color |

```yaml
Columns:
  - ColumnName: theme_color
    ColumnType: color
```

## Enum Type

### enum

Enumerated value.

| Property | Value |
|----------|-------|
| SQL Type | `varchar(50)` |
| GraphQL Type | `String` |
| Fake Data | Random day name |

Used with `Options` to restrict values:

```yaml
Columns:
  - ColumnName: status
    ColumnType: enum
    Options:
      - label: Active
        value: active
      - label: Inactive
        value: inactive
```

## JSON Type

### json

JSON data structure.

| Property | Value |
|----------|-------|
| SQL Type | `text` or `JSON` |
| GraphQL Type | `String` |
| Fake Data | `{}` |

```yaml
Columns:
  - ColumnName: metadata
    ColumnType: json
```

## Binary/File Types

### file

Generic file storage.

| Property | Value |
|----------|-------|
| SQL Type | `blob` |
| GraphQL Type | `String` |
| Validation | Base64 encoded |
| Storage | Base64 in database or cloud storage |

Supports file type restrictions:

```yaml
Columns:
  - ColumnName: document
    ColumnType: file.pdf|doc|docx
```

### image

Image file storage.

| Property | Value |
|----------|-------|
| SQL Type | `blob` |
| GraphQL Type | `String` |
| Validation | Base64 encoded |

```yaml
Columns:
  - ColumnName: avatar
    ColumnType: image
```

### video

Video file storage.

| Property | Value |
|----------|-------|
| SQL Type | `blob` |
| GraphQL Type | `String` |
| Validation | Base64 encoded |

### gzip

Gzip compressed data.

| Property | Value |
|----------|-------|
| SQL Type | `blob` |
| GraphQL Type | `String` |
| Validation | Base64 encoded |

## File Type Patterns

File columns support extension filtering:

| Pattern | Description |
|---------|-------------|
| `file.*` | Any file type |
| `file.pdf` | PDF only |
| `file.csv` | CSV only |
| `file.json\|yaml\|toml` | JSON, YAML, or TOML |
| `file.xls\|xlsx` | Excel files |
| `image` | Image files |
| `video` | Video files |

## Column Properties

### IsNullable

Whether the column accepts NULL values.

```yaml
Columns:
  - ColumnName: optional_field
    ColumnType: label
    IsNullable: true
```

### DefaultValue

Default value when not provided.

```yaml
Columns:
  - ColumnName: status
    ColumnType: label
    DefaultValue: "pending"
```

### IsUnique

Enforce unique values.

```yaml
Columns:
  - ColumnName: code
    ColumnType: label
    IsUnique: true
```

### IsIndexed

Create database index.

```yaml
Columns:
  - ColumnName: email
    ColumnType: email
    IsIndexed: true
```

### IsAutoIncrement

Auto-increment integer.

```yaml
Columns:
  - ColumnName: sequence
    ColumnType: measurement
    IsAutoIncrement: true
```

## Type Mapping Summary

| Column Type | SQL | GraphQL | Blueprint |
|-------------|-----|---------|-----------|
| id | varchar(20) | ID | string |
| alias | varchar(100) | String | string |
| label | varchar(100) | String | string |
| name | varchar(100) | String | string |
| content | text | String | string |
| html | text | String | string |
| markdown | text | String | string |
| email | varchar(100) | String | string |
| password | varchar(200) | String | string |
| encrypted | text | String | string |
| value | varchar(100) | String | string |
| url | varchar(500) | String | string |
| color | varchar(50) | String | string |
| enum | varchar(50) | String | string |
| json | text/JSON | String | string |
| measurement | int(10) | Int | number |
| float | float(7,4) | Float | number |
| rating | int(4) | Int | number |
| year | int(4) | Int | number |
| month | int(4) | Int | number |
| hour | int(4) | Int | number |
| minute | int(4) | Int | number |
| truefalse | boolean | Boolean | boolean |
| datetime | timestamp | DateTime | string |
| date | timestamp | DateTime | string |
| time | timestamp | String | string |
| timestamp | timestamp | DateTime | string |
| location | varchar(50) | String | string |
| location.latitude | float(7,4) | Float | number |
| location.longitude | float(7,4) | Float | number |
| location.altitude | float(7,4) | Float | number |
| file | blob | String | string |
| image | blob | String | string |
| video | blob | String | string |
| gzip | blob | String | string |
