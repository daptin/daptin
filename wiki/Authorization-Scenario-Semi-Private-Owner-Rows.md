# Authorization Scenario: Semi-Private Owner Rows

Use this when all signed-in users may reach a table, but each user should only see their own records unless records are explicitly shared.

## Schema Shape

```yaml
Tables:
  - TableName: owner_note
    Permission: 16384
    DefaultPermission: 256   # UserRead for the row owner
    AccessGroups:
      - Name: users
        Permission: 114688   # GroupPeek + GroupRead + GroupCreate
    Columns:
      - Name: title
        DataType: varchar(100)
        ColumnType: label
```

## Expected Behavior

| Caller | Operation | Expected |
|--------|-----------|----------|
| Guest | Table access | Forbidden |
| Owner | Read own row | Allowed |
| Other signed-in user | Read owner's row | Not returned |
| Signed-in user | Create own row | Allowed |
| Admin | Any CRUD | Allowed |

## Notes

This is not a group-shared model. `AccessGroups` allows signed-in users to reach the table, while `DefaultPermission` keeps rows owner-readable.

The real E2E signs in two users, creates a row as user A, verifies user A sees it, and verifies user B does not.

## See Also

- [[Authorization-Scenarios]] - choose another tested pattern
- [[Authorization-Scenario-Private-Site]] - all signed-in users share rows through `DefaultGroups`
- [[Authorization-Scenario-Mixed-Public-Private]] - public and private rows in one table
- [[Permissions]] - owner, guest, and group permission bits
