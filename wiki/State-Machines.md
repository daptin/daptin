# State Machines

Finite State Machines (FSM) for workflow automation.

## Overview

State machines:
- Define valid states for records
- Control transitions between states
- Trigger actions on state changes
- Enforce business rules

## Defining State Machines

### Schema Definition

```yaml
StateMachines:
  - Name: order_status
    Label: Order Status
    InitialState: pending
    Events:
      - Name: confirm
        Label: Confirm Order
        Src:
          - pending
        Dst: confirmed

      - Name: ship
        Label: Ship Order
        Src:
          - confirmed
        Dst: shipped

      - Name: deliver
        Label: Mark Delivered
        Src:
          - shipped
        Dst: delivered

      - Name: cancel
        Label: Cancel Order
        Src:
          - pending
          - confirmed
        Dst: cancelled

      - Name: refund
        Label: Issue Refund
        Src:
          - delivered
          - shipped
        Dst: refunded
```

### Link to Table

```yaml
Tables:
  - TableName: order
    StateMachineColumn: status
    StateMachine: order_status
    Columns:
      - Name: status
        DataType: varchar(50)
        ColumnType: label
        DefaultValue: pending
```

## State Transitions

### List Available Transitions

```bash
curl http://localhost:6336/api/order/ORDER_ID \
  -H "Authorization: Bearer $TOKEN"
```

Response includes available transitions:

```json
{
  "data": {
    "type": "order",
    "attributes": {
      "status": "pending",
      "__available_transitions": ["confirm", "cancel"]
    }
  }
}
```

### Execute Transition

```bash
curl -X POST http://localhost:6336/action/order/confirm \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "reference_id": "ORDER_ID"
    }
  }'
```

## Transition Events

### On Entry Actions

Trigger action when entering a state:

```yaml
StateMachines:
  - Name: order_status
    Events:
      - Name: ship
        Src: [confirmed]
        Dst: shipped
        OnEntry:
          - Action: send_shipping_notification
```

### On Exit Actions

Trigger action when leaving a state:

```yaml
Events:
  - Name: cancel
    Src: [pending, confirmed]
    Dst: cancelled
    OnExit:
      - Action: release_inventory
```

## Conditional Transitions

### Guard Conditions

```yaml
Events:
  - Name: ship
    Src: [confirmed]
    Dst: shipped
    Guards:
      - Column: payment_status
        Value: paid
```

Only allows transition if `payment_status` equals `paid`.

## State History

The `timeline` table tracks state changes:

```bash
curl 'http://localhost:6336/api/timeline?query=[{"column":"event_type","operator":"is","value":"state_change"}]' \
  -H "Authorization: Bearer $TOKEN"
```

Response:

```json
{
  "data": [
    {
      "type": "timeline",
      "attributes": {
        "event_type": "state_change",
        "old_value": "pending",
        "new_value": "confirmed",
        "created_at": "2024-01-15T10:30:00Z"
      }
    }
  ]
}
```

## State Machine API

### Get State Machine Definition

```bash
curl 'http://localhost:6336/api/smd?query=[{"column":"name","operator":"is","value":"order_status"}]' \
  -H "Authorization: Bearer $TOKEN"
```

### Visualize State Machine

The API returns state machine definition that can be visualized using tools like:
- [Mermaid](https://mermaid.js.org/)
- [State Machine Cat](https://state-machine-cat.js.org/)

```
stateDiagram-v2
    [*] --> pending
    pending --> confirmed: confirm
    pending --> cancelled: cancel
    confirmed --> shipped: ship
    confirmed --> cancelled: cancel
    shipped --> delivered: deliver
    shipped --> refunded: refund
    delivered --> refunded: refund
```

## Complex Example

### Ticket Workflow

```yaml
StateMachines:
  - Name: ticket_workflow
    Label: Support Ticket Workflow
    InitialState: open
    Events:
      - Name: assign
        Label: Assign to Agent
        Src: [open]
        Dst: assigned
        RequiredFields:
          - assignee_id

      - Name: start_work
        Label: Start Working
        Src: [assigned]
        Dst: in_progress

      - Name: request_info
        Label: Request Information
        Src: [in_progress]
        Dst: waiting_on_customer

      - Name: customer_replied
        Label: Customer Replied
        Src: [waiting_on_customer]
        Dst: in_progress

      - Name: resolve
        Label: Resolve Ticket
        Src: [in_progress]
        Dst: resolved
        OnEntry:
          - Action: send_resolution_survey

      - Name: reopen
        Label: Reopen Ticket
        Src: [resolved]
        Dst: open

      - Name: close
        Label: Close Ticket
        Src: [resolved]
        Dst: closed

      - Name: escalate
        Label: Escalate
        Src: [open, assigned, in_progress]
        Dst: escalated
```

## Permissions

Transitions can require permissions:

```yaml
Events:
  - Name: approve
    Src: [pending]
    Dst: approved
    RequiredPermission: Execute
```

## Error Handling

Invalid transitions return errors:

```json
{
  "errors": [
    {
      "status": "400",
      "title": "Invalid Transition",
      "detail": "Cannot transition from 'shipped' to 'confirmed'"
    }
  ]
}
```

## Bulk Transitions

Transition multiple records:

```bash
curl -X POST http://localhost:6336/action/order/ship \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "attributes": {
      "reference_ids": ["ORDER1", "ORDER2", "ORDER3"]
    }
  }'
```
