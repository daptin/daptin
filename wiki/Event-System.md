# Event System

Hooks and event handlers for data changes.

## Overview

Daptin triggers events on:
- Data creation
- Data updates
- Data deletion
- Action execution

## Event Types

| Event | Timing | Description |
|-------|--------|-------------|
| `before:create` | Pre-save | Before record created |
| `after:create` | Post-save | After record created |
| `before:update` | Pre-save | Before record updated |
| `after:update` | Post-save | After record updated |
| `before:delete` | Pre-delete | Before record deleted |
| `after:delete` | Post-delete | After record deleted |

## Defining Event Handlers

In schema YAML:

```yaml
Tables:
  - TableName: order
    Columns:
      - Name: status
        DataType: varchar(100)
      - Name: total
        DataType: decimal

    EventHandlers:
      - Event: after:create
        Handler: http.post
        Attributes:
          Url: https://webhook.example.com/order-created
          Headers:
            Authorization: "Bearer ${env.WEBHOOK_SECRET}"
          Body:
            order_id: "{{.reference_id}}"
            total: "{{.total}}"
            created_at: "{{.created_at}}"

      - Event: before:update
        Handler: validation
        Attributes:
          Condition: "{{.status}} != 'cancelled'"
          Message: "Cannot modify cancelled orders"

      - Event: after:delete
        Handler: action.execute
        Attributes:
          ActionName: cleanup_order_files
          EntityName: order
```

## Handler Types

### HTTP Webhook

```yaml
EventHandlers:
  - Event: after:create
    Handler: http.post
    Attributes:
      Url: https://api.example.com/webhook
      Headers:
        Content-Type: application/json
        X-API-Key: "{{env.API_KEY}}"
      Body:
        event: created
        data: "{{.}}"
```

### Execute Action

```yaml
EventHandlers:
  - Event: after:create
    Handler: action.execute
    Attributes:
      ActionName: send_welcome_email
      EntityName: user_account
```

### JavaScript Handler

```yaml
EventHandlers:
  - Event: before:create
    Handler: js
    Attributes:
      Script: |
        if (!input.email.includes('@')) {
          throw new Error('Invalid email');
        }
        return input;
```

### Validation Handler

```yaml
EventHandlers:
  - Event: before:update
    Handler: validation
    Attributes:
      Condition: "{{.amount}} > 0"
      Message: "Amount must be positive"
```

### Conformation Handler

Auto-set values:

```yaml
EventHandlers:
  - Event: before:create
    Handler: conformation
    Attributes:
      status: pending
      created_by: "{{.user.id}}"
```

## Template Variables

Available in handlers:

| Variable | Description |
|----------|-------------|
| `{{.}}` | Current record |
| `{{.field_name}}` | Specific field |
| `{{.reference_id}}` | Record UUID |
| `{{.user}}` | Current user |
| `{{.user.id}}` | User ID |
| `{{env.VAR}}` | Environment variable |
| `{{now}}` | Current timestamp |

## Conditional Events

Execute only when condition met:

```yaml
EventHandlers:
  - Event: after:update
    Condition: "{{.status}} == 'shipped'"
    Handler: http.post
    Attributes:
      Url: https://api.shipping.com/notify
```

## Multiple Handlers

Chain multiple handlers:

```yaml
EventHandlers:
  - Event: after:create
    Handler: action.execute
    Attributes:
      ActionName: send_notification

  - Event: after:create
    Handler: http.post
    Attributes:
      Url: https://analytics.example.com/track
```

## Error Handling

### Before Events

If handler fails, operation is cancelled:

```yaml
EventHandlers:
  - Event: before:create
    Handler: validation
    Attributes:
      Condition: "{{.inventory}} > 0"
      Message: "Out of stock"
```

### After Events

Failures logged but don't affect operation.

## Async Handlers

For long-running operations:

```yaml
EventHandlers:
  - Event: after:create
    Handler: async.http.post
    Attributes:
      Url: https://slow-api.example.com/process
```

## Event Payload

Handlers receive:

```json
{
  "event": "after:create",
  "table": "order",
  "record": {
    "reference_id": "abc-123",
    "status": "pending",
    "total": 99.99
  },
  "user": {
    "id": "user-456",
    "email": "user@example.com"
  },
  "timestamp": "2024-01-15T10:30:00Z"
}
```

## Debugging Events

Enable debug logging:

```bash
DAPTIN_LOG_LEVEL=debug ./daptin
```

Check logs for event execution.

## Built-in Events

Daptin has internal events for:
- User signup (sends confirmation)
- Password reset (sends email)
- Permission changes (cache invalidation)
