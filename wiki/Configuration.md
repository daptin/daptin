# Configuration

Daptin supports configuration via command-line flags, environment variables, and a runtime configuration API.

## Configuration API

The `/_config` API allows runtime configuration changes stored in the database.

### Set Configuration

```bash
curl -X POST http://localhost:6336/_config/backend/graphql.enable \
  -H "Authorization: Bearer $TOKEN" \
  -d 'true'
```

### Get Configuration

```bash
curl http://localhost:6336/_config/backend/graphql.enable \
  -H "Authorization: Bearer $TOKEN"
```

## Backend Configuration Parameters

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `graphql.enable` | bool | false | Enable GraphQL endpoint |
| `gzip.enable` | bool | true | Enable GZIP compression |
| `limit.rate` | int | 500 | Rate limit (requests/second) |
| `yjs.enabled` | bool | true | Enable YJS collaborative editing |
| `yjs.storage.path` | string | ./yjs | YJS document storage path |
| `caldav.enable` | bool | false | Enable CalDAV server |
| `ftp.enable` | bool | false | Enable FTP server |
| `ftp.listen_interface` | string | 0.0.0.0:21 | FTP bind address |
| `imap.enabled` | bool | false | Enable IMAP server |
| `imap.listen_interface` | string | 0.0.0.0:993 | IMAP bind address |
| `jwt.secret` | string | auto | JWT signing secret |
| `jwt.token.issuer` | string | daptin | JWT issuer name |
| `language.default` | string | en | Default language |
| `hostname` | string | auto | Server hostname |
| `encryption.secret` | string | - | Data encryption key |
| `totp.secret` | string | auto | 2FA TOTP secret |
| `password.reset.email.from` | string | - | Password reset sender |
| `enable_https` | bool | true | Enable HTTPS |

## Schema Configuration Files

Define your data model using JSON, YAML, or TOML files in the schema folder.

### File Naming

```
schema_*.json
schema_*.yaml
schema_*.toml
```

### JSON Schema Example

```json
{
  "Tables": [
    {
      "TableName": "todo",
      "Columns": [
        {"Name": "title", "DataType": "varchar(500)", "ColumnType": "label"},
        {"Name": "completed", "DataType": "bool", "ColumnType": "truefalse", "DefaultValue": "false"}
      ]
    }
  ],
  "Relations": [
    {
      "Subject": "todo",
      "Object": "user_account",
      "Relation": "belongs_to"
    }
  ]
}
```

### YAML Schema Example

```yaml
Tables:
  - TableName: todo
    Columns:
      - Name: title
        DataType: varchar(500)
        ColumnType: label
      - Name: completed
        DataType: bool
        ColumnType: truefalse
        DefaultValue: "false"

Relations:
  - Subject: todo
    Object: user_account
    Relation: belongs_to
```

## CmsConfig Structure

Full configuration schema:

```go
type CmsConfig struct {
    Tables        []TableInfo       // Entity definitions
    Relations     []RelationInfo    // Relationships
    Actions       []ActionInfo      // Business logic
    StateMachines []StateMachine    // FSM definitions
    Streams       []StreamInfo      // Data streams
    Exchanges     []ExchangeInfo    // External integrations
    Tasks         []TaskInfo        // Scheduled jobs
    Imports       []ImportInfo      // Initial data
    EnableGraphQL bool              // GraphQL toggle
}
```

## Environment-Specific Configuration

Configuration values are environment-aware:

```bash
# Debug mode
./daptin -runtime=debug

# Release mode (default)
./daptin -runtime=release

# Test mode
./daptin -runtime=test
```

## Configuration Precedence

1. Command-line flags (highest)
2. Environment variables
3. Database configuration (`_config` table)
4. Schema files
5. Default values (lowest)

## Restart Requirements

Most configuration changes take effect immediately. These require a restart:

- GraphQL enable/disable
- Schema changes (new tables/columns)
- State machine definitions
- New actions

```bash
# Restart via API
curl -X POST http://localhost:6336/action/world/restart_daptin \
  -H "Authorization: Bearer $TOKEN"
```

## Export Configuration

```bash
# Export full system configuration
curl -X POST http://localhost:6336/action/world/download_cms_config \
  -H "Authorization: Bearer $TOKEN"
```
