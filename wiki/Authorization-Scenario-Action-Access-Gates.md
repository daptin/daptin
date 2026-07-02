# Authorization Scenario: Action Access Gates

Use this when selected schema-managed actions should be available to signed-in users without opening every action.

## Schema Shape

```yaml
Tables:
  - TableName: action_doc
    Permission: 524288
    AccessGroups:
      - Name: users
        Permission: 524288   # GroupExecute on the table/type gate
    Columns:
      - Name: title
        DataType: varchar(100)
        ColumnType: label

Actions:
  - Name: allowed_action
    OnType: action_doc
    InstanceOptional: true
    Permission: 0
    AccessGroups:
      - Name: users
        Permission: 524288   # GroupExecute on this action row
    OutFields:
      - Method: ACTIONRESPONSE
        Type: client.notify
        Attributes:
          type: success
          message: allowed

  - Name: denied_action
    OnType: action_doc
    InstanceOptional: true
    Permission: 0
    OutFields:
      - Method: ACTIONRESPONSE
        Type: client.notify
        Attributes:
          type: success
          message: denied
```

## Expected Behavior

| Caller | Action | Expected |
|--------|--------|----------|
| Signed-in user in `users` | `allowed_action` | Allowed |
| Signed-in user in `users` | `denied_action` | Forbidden |
| Guest | `allowed_action` | Forbidden |
| Admin | Any action | Allowed |

## Notes

Action execution checks both:

```text
world(action_doc).CanExecute
AND
action(allowed_action).CanExecute
```

Use `Actions[].AccessGroups` for selected actions. `TableName: action` plus `DefaultGroups` remains broad and applies to every schema-managed action.

## See Also

- [[Authorization-Scenarios]] - choose another tested pattern
- [[Actions-Overview#schema-managed-action-usergroups]] - action `AccessGroups` syntax
- [[Action-Permission-Schema-Sync-Technical-KT]] - maintainer-level sync details
- [[Permissions#schema-provisioning-table-and-action-accessgroups]] - table and action access group semantics
