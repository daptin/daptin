# Data Exchange

Connect and sync data with external systems.

## Overview

Data Exchange enables:
- Syncing data with external APIs
- Pushing data to Google Sheets
- Triggering actions based on data changes

An exchange attached to a Daptin resource runs from the resource lifecycle. A
`before` exchange runs synchronously before the row mutation. Its failure
is logged and the resource operation continues. An `after` exchange is always
recorded for background execution and is never attempted inside the source
request.

The source mutation and every matching `exchange_run` row use the
same database transaction. If an execution cannot be stored, the mutation is
rolled back. Once committed, the standard persisted task invokes the same
exchange executor for action and HTTP targets.

Every Daptin node may run the processor task. The configured SQL database owns
bounded attempts and expiring leases, and a conditional update ensures that
only one node owns an attempt. Olric is not queue authority. External effects
cannot be rolled back by SQL and may be delivered more than once after a
process or network failure. Targets should use stable domain identifiers for
idempotency.

## Data Exchange Table

The `data_exchange` table stores exchange configurations:

| Column | Type | Description |
|--------|------|-------------|
| `name` | label | Unique identifier |
| `source_type` | label | Source system type |
| `source_attributes` | json | Source connection config |
| `target_type` | label | Target system type |
| `target_attributes` | json | Target connection config |
| `attributes` | json | Source resource name, hook, methods, and result mapping |
| `options` | json | Exchange options |

## Target Types

| Type | Description |
|------|-------------|
| `action` | Execute Daptin action |
| `rest` | HTTP REST API call |
| `gsheet-append` | Append to Google Sheet |

For a lifecycle exchange, use `source_type: "self"` and set `attributes.name`
to the source resource. `attributes.hook` is `before` or `after`, and
`attributes.methods` is the list of lowercase mutation methods to observe.

Target configuration is exact:

| Target | Required `target_attributes` | Optional `target_attributes` |
|--------|------------------------------|------------------------------|
| `action` | `type`, `action` | `attributes` object |
| `rest` | `url`, `method` | `headers`, `body`, `query_params` objects |
| `gsheet-append` | `sheetUrl`, `appKey` | None |

## Create Data Exchange

### REST API Target

```bash
curl -X POST http://localhost:6336/api/data_exchange \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "data_exchange",
      "attributes": {
        "name": "send_to_webhook",
        "source_type": "self",
        "source_attributes": "{\"name\": \"order\"}",
        "target_type": "rest",
        "target_attributes": "{\"url\": \"https://api.example.com/webhook\", \"method\": \"POST\"}",
        "attributes": "{\"name\": \"order\", \"hook\": \"after\", \"methods\": [\"post\"]}"
      }
    }
  }'
```

Restart Daptin after creating or changing a data exchange so the runtime can
reload the exchange definitions.

### Google Sheets Integration

```bash
curl -X POST http://localhost:6336/api/data_exchange \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "data_exchange",
      "attributes": {
        "name": "export_to_gsheet",
        "source_type": "self",
        "source_attributes": "{\"name\": \"order\"}",
        "target_type": "gsheet-append",
        "target_attributes": "{\"sheetUrl\": \"https://sheets.googleapis.com/v4/spreadsheets/SHEET_ID/values/Sheet1:append\", \"appKey\": \"YOUR_API_KEY\"}",
        "attributes": "{\"name\": \"order\", \"hook\": \"after\", \"methods\": [\"post\"]}"
      }
    }
  }'
```

### Action Target

Execute Daptin action when data changes:

```bash
curl -X POST http://localhost:6336/api/data_exchange \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "data_exchange",
      "attributes": {
        "name": "trigger_notification",
        "source_type": "self",
        "source_attributes": "{\"name\": \"order\"}",
        "target_type": "action",
        "target_attributes": "{\"type\": \"order\", \"action\": \"send_notification\", \"attributes\": {}}",
        "attributes": "{\"name\": \"order\", \"hook\": \"after\", \"methods\": [\"post\", \"patch\"]}"
      }
    }
  }'
```

## OAuth-Protected Targets

The `rest` exchange target does not apply the `oauth_token_id` relationship to
outbound requests. For an OAuth-protected API, define the operation as a Daptin
integration and use an `action` exchange target to invoke that operation. This
keeps token ownership, provider matching, permissions, and execution identity
in the existing integration action path. See [[Integrations|Integrations]].

### Execute As User

Run exchange with specific user permissions:

```bash
curl -X PATCH http://localhost:6336/api/data_exchange/EXCHANGE_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "data_exchange",
      "id": "EXCHANGE_ID",
      "relationships": {
        "as_user_id": {
          "data": {"type": "user_account", "id": "USER_ID"}
        }
      }
    }
  }'
```

## Column Mapping

Map source columns to target columns:

```json
{
  "attributes": [
    {
      "SourceColumn": "order_id",
      "TargetColumn": "id"
    },
    {
      "SourceColumn": "customer_name",
      "TargetColumn": "name"
    },
    {
      "SourceColumn": "total_amount",
      "TargetColumn": "value"
    }
  ]
}
```

## REST Exchange Options

For REST target type:

```json
{
  "target_attributes": {
    "url": "https://api.example.com/data",
    "method": "POST",
    "headers": {
      "Content-Type": "application/json",
      "X-API-Key": "your-key"
    },
    "query_params": {
      "source": "daptin"
    },
    "body": {
      "data": "{{.}}"
    }
  }
}
```

## Supported HTTP Methods

- GET
- POST
- PUT
- PATCH
- DELETE

## Reliable Background Processing

Every mutation `after` exchange is stored as an administrator-only
`exchange_run` resource. The execution stores the related exchange,
configured execution account, source reference and version, method, bounded
attempt state, and lease. It does not copy the source payload or credentials.
The source is reloaded through Daptin resources for each attempt and the
configured account's current groups and read permission are checked before a
target is called.

Target failures use exponential backoff capped at one hour and stop after the
stored attempt limit. A missing exchange, execution identity, source row,
source permission, or a changed source version is terminal. Expired leases can
be reclaimed, and lease tokens prevent an older worker from completing a
reclaimed execution.

Administrators can invoke `retry_data_exchange_execution` on one terminal
execution. This grants a fresh bounded attempt budget and returns it to the
same scheduled claim path; it does not run the target inline. Completed rows
are removed in bounded batches after the retention period.

## List Data Exchanges

```bash
curl http://localhost:6336/api/data_exchange \
  -H "Authorization: Bearer $TOKEN"
```

## Troubleshooting

### Exchange Not Triggering

1. Check exchange is configured correctly
2. Verify OAuth token is valid (if used)
3. Check target URL is accessible
4. Review server logs for errors

### Authentication Errors

For OAuth-protected APIs, troubleshoot the integration action and its selected
token. A direct `rest` exchange does not select or refresh OAuth tokens.
