# Schema Definition

Define your data model using JSON, YAML, or TOML configuration files.

## Table Definition

```yaml
Tables:
  - TableName: product
    TableName: product
    Icon: shopping-cart
    Columns:
      - Name: name
        DataType: varchar(500)
        ColumnType: label
        IsIndexed: true
        IsNullable: false
      - Name: price
        DataType: float(8)
        ColumnType: measurement
      - Name: description
        DataType: text
        ColumnType: content
      - Name: image
        DataType: text
        ColumnType: file.image
```

## Column Properties

| Property | Type | Description |
|----------|------|-------------|
| `Name` | string | Column name |
| `DataType` | string | SQL data type |
| `ColumnType` | string | Daptin column type |
| `IsNullable` | bool | Allow NULL values |
| `IsIndexed` | bool | Create database index |
| `IsUnique` | bool | Unique constraint |
| `DefaultValue` | string | Default value |
| `IsForeignKey` | bool | Foreign key reference |
| `ForeignKeyData` | object | FK configuration |

## DataType Options

SQL data types supported:

| DataType | Description |
|----------|-------------|
| `varchar(n)` | Variable length string |
| `text` | Unlimited text |
| `int(11)` | Integer |
| `bigint(20)` | Large integer |
| `float(8)` | Floating point |
| `double` | Double precision |
| `bool` | Boolean |
| `date` | Date only |
| `datetime` | Date and time |
| `timestamp` | Unix timestamp |
| `blob` | Binary data |

## Relationship Definition

```yaml
Relations:
  - Subject: order
    Object: user_account
    Relation: belongs_to

  - Subject: order
    Object: product
    Relation: has_many

  - Subject: product
    Object: category
    Relation: has_many
    ObjectName: categories
    SubjectName: products
```

## Relation Types

| Relation | Description | Example |
|----------|-------------|---------|
| `belongs_to` | Many-to-one | Order belongs_to User |
| `has_one` | One-to-one | User has_one Profile |
| `has_many` | One-to-many | User has_many Orders |
| `has_many_and_belongs_to_many` | Many-to-many | Product has_many Categories |

## Table Properties

```yaml
Tables:
  - TableName: audit_log
    IsHidden: true           # Hide from API listing
    IsStateTrackingEnabled: true  # Track changes
    IsAuditEnabled: true     # Enable audit trail
    DefaultPermission: 262142     # Permission bitmask
    Icon: clipboard-list     # UI icon
    Validations:
      - ColumnName: email
        Regex: "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$"
```

## Column Validations

```yaml
Columns:
  - Name: email
    DataType: varchar(200)
    ColumnType: email
    Validations:
      - Regex: "^[a-z0-9._%+-]+@[a-z0-9.-]+\\.[a-z]{2,}$"
        Message: "Invalid email format"

  - Name: age
    DataType: int(11)
    ColumnType: measurement
    Validations:
      - Min: 0
        Max: 150
        Message: "Age must be between 0 and 150"
```

## Default System Columns

Every table automatically includes:

| Column | Type | Description |
|--------|------|-------------|
| `id` | bigint | Auto-increment ID |
| `reference_id` | varchar(40) | UUID reference |
| `created_at` | datetime | Creation timestamp |
| `updated_at` | datetime | Last update timestamp |
| `permission` | int | Row permission bitmask |
| `user_account_id` | varchar(40) | Owner reference |
| `version` | int | Version number |

## Import Initial Data

```yaml
Imports:
  - FilePath: ./data/products.csv
    Entity: product
    TruncateBeforeInsert: false
```

## Complete Example

```json
{
  "Tables": [
    {
      "TableName": "blog_post",
      "Icon": "file-text",
      "Columns": [
        {
          "Name": "title",
          "DataType": "varchar(500)",
          "ColumnType": "label",
          "IsNullable": false,
          "IsIndexed": true
        },
        {
          "Name": "slug",
          "DataType": "varchar(200)",
          "ColumnType": "alias",
          "IsUnique": true
        },
        {
          "Name": "content",
          "DataType": "text",
          "ColumnType": "content"
        },
        {
          "Name": "published",
          "DataType": "bool",
          "ColumnType": "truefalse",
          "DefaultValue": "false"
        },
        {
          "Name": "published_at",
          "DataType": "datetime",
          "ColumnType": "datetime"
        }
      ]
    },
    {
      "TableName": "category",
      "Columns": [
        {
          "Name": "name",
          "DataType": "varchar(200)",
          "ColumnType": "label",
          "IsUnique": true
        }
      ]
    }
  ],
  "Relations": [
    {
      "Subject": "blog_post",
      "Object": "user_account",
      "Relation": "belongs_to",
      "SubjectName": "posts",
      "ObjectName": "author"
    },
    {
      "Subject": "blog_post",
      "Object": "category",
      "Relation": "has_many_and_belongs_to_many"
    }
  ]
}
```
