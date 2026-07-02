# Authorization Scenario: Shared Group Workspace

Use this for collaborative workspaces where different groups have different row-level permissions.

## Schema Shape

```yaml
Tables:
  - TableName: workspace_item
    Permission: 16384
    DefaultPermission: 1
    AccessGroups:
      - Name: users
        Permission: 245760   # GroupPeek + GroupRead + GroupCreate + GroupUpdate
    Columns:
      - Name: title
        DataType: varchar(100)
        ColumnType: label
```

Create runtime groups over the API:

```text
editors
members
```

Then share records:

| Group | Row Relation Permission | Meaning |
|-------|-------------------------|---------|
| editors | `180224` | GroupPeek + GroupRead + GroupUpdate |
| members | `49152` | GroupPeek + GroupRead |

## Expected Behavior

| Caller | Operation | Expected |
|--------|-----------|----------|
| Editor | Read shared row | Allowed |
| Editor | Update shared row | Allowed |
| Member | Read shared row | Allowed |
| Member | Update shared row | Forbidden |
| Guest | Table access | Forbidden |
| Admin | Any CRUD | Allowed |

## Notes

The table gate allows signed-in users to reach the table. The row relation decides who can update each item.

Runtime groups should be created via `/api/usergroup`, and users should be added through `user_account_user_account_id_has_usergroup_usergroup_id`.

## See Also

- [[Authorization-Scenarios]] - choose another tested pattern
- [[Authorization-Scenario-Private-Site]] - simpler authenticated-only sharing
- [[Users-and-Groups]] - group and membership API
- [[Permissions]] - join-table relation permission values
