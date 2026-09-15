# Data Exchange

Connect and sync data with external systems.

## Overview

Data Exchange enables:
- Syncing data with external APIs
- Pushing data to Google Sheets
- Triggering actions based on data changes
- Integration with OAuth-protected services

An exchange attached to a Daptin resource runs from the resource lifecycle. A
`before` exchange runs synchronously before the row mutation. Its failure is
logged without rejecting the resource operation by default. An `after`
exchange is attempted after the row mutation and has the same default.
This preserves the resource API behavior of existing exchange definitions.

Set `options.on_error` on an exchange to choose another failure policy:

| Value | Behavior |
|-------|----------|
| `continue` | Log the failure and continue the resource operation. This is the default. |
| `retry` | Continue the resource operation and durably retry the failed exchange. Mutation methods only. |
| `error` | Return the exchange failure to the resource lifecycle. |

These are exact values. An invalid policy is a configuration error.

`error` rolls back the owning resource transaction when the exchange reports
failure. It cannot undo an external effect that completed before the failure
was observed, so it is not distributed atomicity.

The source mutation and a failed exchange's retry record use the same database
transaction. If the retry record cannot be stored, the resource operation
fails instead of acknowledging work that cannot be retried. Every Daptin node
may run the processor task, while a conditional database claim ensures that
only one node owns an attempt. External effects
such as HTTP requests, email delivery, object storage writes, and live
publications cannot be rolled back by SQL and may be delivered more than once
after a process or network failure. Targets must therefore be safe to retry,
normally by using stable domain identifiers from the event.

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
        "attributes": "{\"name\": \"order\", \"hook\": \"after\", \"methods\": [\"post\"]}",
        "options": "{\"on_error\": \"retry\"}"
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
        "target_attributes": "{\"sheetUrl\": \"https://sheets.googleapis.com/v4/spreadsheets/SHEET_ID/values/Sheet1:append\", \"appKey\": \"YOUR_API_KEY\"}"
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

## OAuth Integration

Data exchanges can use OAuth tokens for authenticated APIs.

### Link OAuth Token

```bash
curl -X PATCH http://localhost:6336/api/data_exchange/EXCHANGE_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "data_exchange",
      "id": "EXCHANGE_ID",
      "relationships": {
        "oauth_token_id": {
          "data": {"type": "oauth_token", "id": "TOKEN_ID"}
        }
      }
    }
  }'
```

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

Failed mutation exchanges configured with `"on_error": "retry"` are stored as permissioned
`data_exchange_execution` resources. Each execution contains the immutable
event envelope, retry state, attempt count, and next-attempt time. The
standard `process_data_exchange_executions` action invokes the same exchange
executor used by the initial attempt.

Retries use exponential backoff capped at one hour. In a cluster, every node
may invoke the processor; the configured SQL database owns claims and retry
state. Olric is not the durable queue or claim authority.

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

1. Verify OAuth token exists and is valid
2. Check token has required scopes
3. Refresh expired tokens
