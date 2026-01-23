# Data Exchange

Connect and sync data with external systems.

## Overview

Data Exchange enables:
- Syncing data with external APIs
- Pushing data to Google Sheets
- Triggering actions based on data changes
- Integration with OAuth-protected services

## Data Exchange Table

The `data_exchange` table stores exchange configurations:

| Column | Type | Description |
|--------|------|-------------|
| `name` | label | Unique identifier |
| `source_type` | label | Source system type |
| `source_attributes` | json | Source connection config |
| `target_type` | label | Target system type |
| `target_attributes` | json | Target connection config |
| `attributes` | json | Column mapping |
| `options` | json | Exchange options |

## Target Types

| Type | Description |
|------|-------------|
| `action` | Execute Daptin action |
| `rest` | HTTP REST API call |
| `gsheet-append` | Append to Google Sheet |
| `self` | Internal Daptin entity |

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
        "attributes": "{}"
      }
    }
  }'
```

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
        "target_attributes": "{\"action_name\": \"send_notification\", \"entity_name\": \"order\"}"
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
