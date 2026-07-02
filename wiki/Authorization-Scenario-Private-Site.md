# Authorization Scenario: Private Site

Use this for authenticated-only applications where signed-in users can access shared records, and guests cannot even pass the table gate.

## Schema Shape

```yaml
Tables:
  - TableName: private_note
    Permission: 16384        # non-zero restrictive table permission
    DefaultPermission: 1     # non-zero restrictive row permission
    AccessGroups:
      - Name: users
        Permission: 114688   # GroupPeek + GroupRead + GroupCreate
    DefaultGroups:
      - Name: users
        Permission: 49152    # GroupPeek + GroupRead on new rows
    Columns:
      - Name: title
        DataType: varchar(100)
        ColumnType: label
```

## Expected Behavior

| Caller | Operation | Expected |
|--------|-----------|----------|
| Guest | `GET /api/private_note` | Forbidden at table gate |
| Signed-in user in `users` | `GET /api/private_note` | Allowed |
| Signed-in user in `users` | `POST /api/private_note` | Allowed |
| Signed-in user outside `users` | CRUD | Forbidden unless another group grants access |
| Admin | Any CRUD | Allowed |

## Notes

`AccessGroups` opens the table/type gate. `DefaultGroups` shares new rows with `users`.

Do not use `Permission: 0` as the schema-level restrictive value; schema sync treats zero as unset.

## See Also

- [[Authorization-Scenarios]] - choose another tested pattern
- [[Authorization-Scenario-Semi-Private-Owner-Rows]] - signed-in table access with owner-only rows
- [[Authorization-Scenario-Shared-Group-Workspace]] - group-based row sharing
- [[Users-and-Groups]] - creating groups and assigning users
