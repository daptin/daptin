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
          Condition: "{{.old.status}} != 'cancelled'"
          Message: "Cannot modify cancelled orders"

      - Event: after:delete
        Handler: action.execute
        Attributes:
          ActionName: cleanup_order_files
          EntityName: order
```

The `after:create` webhook uses the bounded delivery and administrator retry
process described in [After Events](#after-events).

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

See [After Events](#after-events) for the synchronous wait, attempt limit, and
recovery procedure for this webhook.

### Execute Action

`action.execute` invokes the named Daptin action as the account that made the
write. The action and every resource used by its outcomes retain their own
permission checks. An action triggered by `after:delete` must have
`InstanceOptional: true`, because the deleted record is no longer available
for instance lookup. Its deleted record is passed in the action attributes as
`subject`.

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

Before handlers read proposed values through `{{.field}}`. On updates and
deletes, `{{.old.field}}` reads the stored record. This lets a rule reject
changes to an already cancelled record while allowing the update that first
marks it cancelled.

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
| `{{.old.field_name}}` | Stored field before update or delete |
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

List multiple handlers in execution order:

```yaml
EventHandlers:
  - Event: after:create
    Handler: action.execute
    Attributes:
      ActionName: send_notification
      EntityName: order

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

After handlers are queued in the write transaction and run after commit. Failed
delivery does not undo the write. A delete handler receives the deleted record
as it was at the time of deletion. The writer must be able to read the source
record when the handler is queued and delivered.

`http.post` waits up to 35 seconds after commit for delivery before its HTTP
request completes. The write still succeeds if delivery fails or the wait
expires. `async.http.post` returns as soon as delivery has been durably queued.
Both use an `exchange_run` with five attempts by default. Failures are retried
with backoff until the attempt limit is reached; then the run becomes
`terminal_failed` and will not retry automatically, even if the receiver
recovers. A wait timeout does not cancel a run that is still eligible for retry.

Administrators should monitor `exchange_run` for `terminal_failed` and alert on
those rows. After resolving the delivery failure, retry a terminal run with
its reference ID:

```bash
curl -X POST http://localhost:6336/action/exchange_run/retry_data_exchange_execution \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"attributes":{"exchange_run_id":"EXECUTION_REFERENCE_ID"}}'
```

The action grants five more attempts through the scheduled processor; it does
not deliver inline. See [[Data-Exchange|Data Exchange]] for the durable delivery
contract and [[Action-Reference|Action Reference]] for the action definition.

## Async Handlers

For long-running operations:

```yaml
EventHandlers:
  - Event: after:create
    Handler: async.http.post
    Attributes:
      Url: https://slow-api.example.com/process
```

This handler uses the same bounded `exchange_run` delivery and recovery process
described in [After Events](#after-events), without waiting in the write request.

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

This is the default webhook body when `Body` is omitted. A configured `Body`
replaces it. The timestamp is captured when the write queues the event.

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
