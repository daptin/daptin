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

## Action with HTTP Call

Integrate external APIs:

```yaml
Actions:
  - Name: send_notification
    Label: Send Notification
    OnType: user_account
    InFields:
      - Name: message
        ColumnType: content
    OutcomeAttributes:
      - Type: http.post
        Attributes:
          Url: https://api.notifications.com/send
          Headers:
            Authorization: "Bearer ${env.NOTIFICATION_API_KEY}"
          Body:
            user_id: "{{.user_account.reference_id}}"
            message: "{{.message}}"
```

## Action Chains

Execute multiple steps:

```yaml
Actions:
  - Name: complete_order
    Label: Complete Order
    OnType: order
    OutcomeAttributes:
      - Type: action.execute
        Attributes:
          ActionName: send_confirmation_email
          EntityName: order

      - Type: action.execute
        Attributes:
          ActionName: update_inventory
          EntityName: product

      - Type: conformation
        Attributes:
          status: completed
          completed_at: "~now"
```

## Conditional Actions

Execute based on conditions:

```yaml
OutcomeAttributes:
  - Type: condition
    Condition: "{{.order.total}} > 1000"
    Attributes:
      - Type: action.execute
        Attributes:
          ActionName: flag_for_review
```

## Action Permissions

Control who can execute:

```yaml
Actions:
  - Name: admin_only_action
    Label: Admin Only
    OnType: entity
    ReferenceId: action-uuid
    Permission: 262142  # Admin only
```

## JavaScript Actions

Execute JavaScript:

```yaml
OutcomeAttributes:
  - Type: js
    Attributes:
      Script: |
        var result = {};
        result.calculated = input.price * input.quantity;
        result.tax = result.calculated * 0.1;
        return result;
```

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
