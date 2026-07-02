# Authorization Scenario: Public Site

Use this for public content where anonymous visitors can list/read records, but cannot write.

## Schema Shape

```yaml
Tables:
  - TableName: public_page
    Permission: 3          # GuestPeek + GuestRead on the table gate
    DefaultPermission: 2   # GuestRead on new rows
    Columns:
      - Name: title
        DataType: varchar(100)
        ColumnType: label
```

## Expected Behavior

| Caller | Operation | Expected |
|--------|-----------|----------|
| Guest | `GET /api/public_page` | Allowed |
| Guest | `POST /api/public_page` | Forbidden |
| Signed-in user | `GET /api/public_page` | Allowed through guest permission |
| Admin | Any CRUD | Allowed |

## Notes

No `AccessGroups` are required because the table and rows intentionally grant guest read access.

The real E2E creates a public row as admin, verifies guest list/read, and verifies guest create is forbidden.

## See Also

- [[Authorization-Scenarios]] - choose another tested pattern
- [[Authorization-Scenario-Mixed-Public-Private]] - public table with selected private rows
- [[Authorization-Scenario-Private-Site]] - authenticated-only table access
- [[Permissions]] - permission bits and two-level checks
