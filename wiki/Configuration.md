# Configuration

Daptin supports configuration via command-line flags, environment variables, and a runtime configuration API.

## Configuration API

The `/_config` API allows runtime configuration changes stored in the database.

### Set Configuration

```bash
curl -X POST http://localhost:6336/_config/backend/graphql.enable \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: text/plain' \
  --data-binary 'true'
```

The config handler stores the raw request body. Do not JSON-quote scalar
strings: posting `"127.0.0.1:22121"` stores the quote characters and can later
produce an invalid-port startup failure.

| Value kind | Request body | Content-Type | Stored text |
|---|---|---|---|
| String | `127.0.0.1:22121` | `text/plain` | `127.0.0.1:22121` |
| Integer | `100` | `text/plain` | `100` |
| Boolean | `true` | `text/plain` | `true` |
| JSON object | `{"version":"1","limits":{"/statistics":2}}` | `application/json` | The JSON object text |

`GET /_config/backend/{key}` returns the stored text, not a typed JSON
envelope. Read it back and compare exact bytes before restarting a listener.

### Get Configuration

```bash
curl http://localhost:6336/_config/backend/graphql.enable \
  -H "Authorization: Bearer $TOKEN"
```

## Backend Configuration Parameters

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `graphql.enable` | bool | false | Enable GraphQL endpoint |
| `graphql.max_request_bytes` | integer | 10485760 | Maximum GraphQL POST body in bytes (1–67108864); restart after changing |
| `http.max_buffered_request_bytes` | integer | 10485760 | Maximum body for actions, JSON:API writes, integration operations, and event starts in bytes (1–16777216); restart after changing. Oversized requests receive HTTP 413. Streaming asset uploads are unaffected. |
| `gzip.enable` | bool | true | Enable negotiated GZIP compression for API, dashboard, and hosted-site responses; restart after changing |
| `limit.rate` | JSON object | `{"version":"1","limits":{}}` | Per-path requests in a one-second UTC window; see [[Rate-Limiting]] |
| `yjs.enabled` | bool | true | Enable YJS collaborative editing |
| `yjs.storage.path` | string | ./yjs | YJS document storage path |
| `caldav.enable` | bool | false | Enable CalDAV server |
| `ftp.enable` | bool | false | Enable FTP server |
| `ftp.listen_interface` | string | 0.0.0.0:2121 | FTP bind address |
| `imap.enabled` | bool | false | Enable IMAP server |
| `imap.listen_interface` | string | :1143 | IMAP bind address |
| `imap.hostname` | string | imap.{hostname} | IMAP/IMAPS TLS hostname |
| `jwt.secret` | string | auto | JWT signing secret |
| `jwt.token.issuer` | string | daptin | JWT issuer name |
| `language.default` | string | en | Default language |
| `hostname` | string | auto | Server hostname |
| `encryption.secret` | string | - | Data encryption key |
| `totp.secret` | string | auto | 2FA TOTP secret |
| `password.reset.email.from` | string | - | Password reset sender |
| `enable_https` | bool | true | Enable HTTPS |

### Buffered request bodies

The limit applies to the complete HTTP request body before an action, JSON:API
write, integration operation, or event start is processed. It includes JSON
syntax and base64 data inside an action request. A request above the limit
receives HTTP 413 before the operation runs. GraphQL POST bodies use the
separate `graphql.max_request_bytes` setting. Asset uploads and other streaming
protocols retain their own handling.

An administrator can raise the limit to 16 MiB at most, then restart Daptin:

```bash
curl -X POST http://localhost:6336/_config/backend/http.max_buffered_request_bytes \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: text/plain' \
  --data-binary '16777216'
```

The value must be a whole number from 1 through 16777216. An invalid stored
value prevents the server from starting; check it with a GET request to
`/_config/backend/http.max_buffered_request_bytes` before restarting.

## Schema Configuration Files

Define your data model using JSON or YAML files whose basename starts with
`schema_`. A neighboring `schema.yaml` is ignored.

### File Naming

```
schema_*.json
schema_*.yaml
schema_*.yml
```

Daptin always scans `schema_*.*` in the process working directory. If
`DAPTIN_SCHEMA_FOLDER` is set, it also scans that directory and appends those
matches. Unsupported extensions are logged and skipped; TOML files are not
loaded. Schema files are loaded before stored
`world` rows are merged. Use `DAPTIN_SKIP_CONFIG_FROM_DATABASE=true` only when
you intentionally do not want stored world definitions merged, and
`DAPTIN_SKIP_INITIALISE_RESOURCES=true` only for controlled diagnostics because
it skips normal resource initialization.

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

## Schema File Sections

Schema files can contain these top-level sections:

| Section | Defines |
|---|---|
| `Tables` | Resources and their columns |
| `Relations` | Connections between resources |
| `Actions` | Named operations and their outcomes |
| `StateMachineDescriptions` | State machine definitions |
| `Streams` | Data streams |
| `ExchangeContracts` | Data exchange definitions |
| `Tasks` | Scheduled actions |
| `Imports` | Initial data |
| `EnableGraphQL` | Enable the GraphQL endpoint at startup |

You can also set `graphql.enable` through the configuration API shown above.

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

Do not assume a stored value hot-reloads its owning component. The following
are composed at startup and require a process-supervisor restart:

- GraphQL enable/disable
- Global `limit.rate` middleware
- FTP, IMAP, SMTP, CalDAV/CardDAV, and HTTPS listeners/settings
- Feed and stream maps
- Cloud stores and site routes
- Schema changes (new tables/columns)
- State machine definitions
- New actions and scheduled tasks

```bash
# Restart through your process supervisor
docker restart daptin
```

## Export Configuration

```bash
# Export full system configuration
curl -X POST http://localhost:6336/action/world/download_system_schema \
  -H "Authorization: Bearer $TOKEN"
```
