# Custom Actions

Define custom actions for your entities.

## Overview

Custom actions allow you to:
- Execute business logic
- Integrate external services
- Chain multiple operations
- Trigger workflows

## Defining Actions

Actions defined in schema:

```yaml
Tables:
  - TableName: order
    Columns:
      - Name: status
        DataType: varchar(100)
      - Name: total
        DataType: decimal

    Actions:
      - Name: process_payment
        Label: Process Payment
        OnType: order
        InstanceOptional: false
        InFields:
          - Name: payment_method
            ColumnType: label
            IsNullable: false
          - Name: card_token
            ColumnType: encrypted
        OutFields:
          - Name: transaction_id
            ColumnType: label
          - Name: success
            ColumnType: truefalse
        Conformations:
          - Name: status
            Value: "processing"
        Validations:
          - ColumnName: payment_method
            Tags: required
```

## Action Properties

| Property | Type | Description |
|----------|------|-------------|
| Name | string | Internal name |
| Label | string | Display name |
| OnType | string | Entity type |
| InstanceOptional | bool | Requires specific record |
| InFields | array | Input parameters |
| OutFields | array | Return values |
| Conformations | array | Auto-set values |
| Validations | array | Input validation |

## Input Fields (InFields)

Define what users provide:

```yaml
InFields:
  - Name: amount
    ColumnType: money
    IsNullable: false

  - Name: notes
    ColumnType: content
    IsNullable: true

  - Name: priority
    ColumnType: label
    DataType: enum('low','medium','high')
```

## Output Fields (OutFields)

Define what action returns:

```yaml
OutFields:
  - Name: result_id
    ColumnType: alias

  - Name: message
    ColumnType: content

  - Name: success
    ColumnType: truefalse
```

## Conformations

Auto-set values when action runs:

```yaml
Conformations:
  - Name: status
    Value: "completed"

  - Name: processed_at
    Value: "~now"

  - Name: processed_by
    AttributeName: user_id
```

Special values:
- `~now` - Current timestamp
- `~user_id` - Current user ID
- `~uuid` - Generate UUID

## Executing Actions

### Instance Action

Operates on specific record:

```bash
curl -X POST http://localhost:6336/action/order/{id}/process_payment \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "attributes": {
      "payment_method": "card",
      "card_token": "tok_xxx"
    }
  }'
```

### Collection Action

Operates on entity (no specific record):

```bash
curl -X POST http://localhost:6336/action/order/bulk_export \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "attributes": {
      "format": "csv"
    }
  }'
```

## Performers (OutFields)

Actions execute through **performers** defined in `OutFields`. Each performer type handles a specific operation.

### $network.request

Make HTTP requests to external APIs.

**Performer**: `$network.request`

**Attributes**:
| Attribute | Type | Description |
|-----------|------|-------------|
| `Url` | string | Target URL (required) |
| `Method` | string | HTTP method: GET, POST, PUT, PATCH, DELETE (default: GET) |
| `Headers` | object | Request headers |
| `Body` | object/array | Request body (for POST/PUT/PATCH) |
| `FormData` | object | Form data (x-www-form-urlencoded) |
| `Query` | object | URL query parameters |

**Example - Webhook notification**:
```yaml
Actions:
  - Name: send_webhook
    Label: Send Webhook
    OnType: order
    InstanceOptional: false
    InFields:
      - Name: webhook_url
        ColumnType: url
        IsNullable: false
    OutFields:
      - Type: $network.request
        Method: EXECUTE
        Attributes:
          Url: "~webhook_url"
          Method: "POST"
          Headers:
            Content-Type: "application/json"
            X-Webhook-Event: "order.created"
          Body:
            order_id: "$.reference_id"
            total: "$.total"
            created_at: "$.created_at"
```

**Response structure**:
```json
{
  "body": {...},           // Parsed JSON or string
  "bodyPlainText": "...",  // Raw response text
  "base32EncodedBody": "...", // Base64-encoded binary
  "headers": {...}         // Response headers
}
```

**Using input fields** (prefix with `~`):
- `~field_name` - Input field value from action call
- `$.column_name` - Value from the target entity instance

### Integration Performers

For complex API integrations, use [Integrations](Integrations.md) which parse OpenAPI specs and create dynamic performers.

---

## Action Chains

Execute multiple operations sequentially using multiple OutFields:

```yaml
Actions:
  - Name: complete_order
    Label: Complete Order
    OnType: order
    OutFields:
      # First: Send notification
      - Type: $network.request
        Method: EXECUTE
        Attributes:
          Url: "https://api.notifications.com/send"
          Method: "POST"
          Body:
            event: "order_completed"
            order_id: "$.reference_id"

      # Second: Update status via column conformation
      # (Conformations handle this, not OutFields)
    Conformations:
      - Name: status
        Value: "completed"
      - Name: completed_at
        Value: "~now"
```

---

## JavaScript Expressions

Use JavaScript expressions for dynamic values:

```yaml
OutFields:
  - Type: $network.request
    Method: EXECUTE
    Attributes:
      Url: "!subject.webhook_url"  # JavaScript expression (prefix with !)
      Body:
        calculated_total: "!subject.price * subject.quantity"
        timestamp: "!new Date().toISOString()"
```

**Expression prefixes**:
- `!expression` - JavaScript expression
- `~field_name` - Input field from action call
- `$.column_name` - Entity column value

## Action Permissions

Actions inherit permissions from their OnType table. The Execute permission bit must be set.

See [Permissions](Permissions.md) for permission calculation.

---

## Other Built-in Performers

| Performer | Purpose |
|-----------|---------|
| `$network.request` | HTTP requests to external APIs |
| `__data_export` | Export table data |
| `__data_import` | Import data to table |
| `mail.send` | Send email (internal, not REST) |
| `otp.generate` | Generate 2FA code |
| `integration.install` | Install OpenAPI integration |
| `self.tls.generate` | Generate self-signed certificate |
| `acme.tls.generate` | Generate Let's Encrypt certificate |

See [Documentation-TODO](Documentation-TODO.md) for full performer list.

## Error Handling

Actions return errors in response:

```json
{
  "errors": [{
    "status": "400",
    "title": "Validation failed",
    "detail": "payment_method is required"
  }]
}
```

## List Custom Actions

```bash
curl 'http://localhost:6336/api/action?query=[{"column":"world_id","operator":"is","value":"TABLE_ID"}]' \
  -H "Authorization: Bearer $TOKEN"
```
