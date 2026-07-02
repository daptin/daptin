# Authorization Scenario: Mixed Public And Private Rows

Use this when one table contains both public records and private records.

## Schema Shape

```yaml
Tables:
  - TableName: mixed_article
    Permission: 3          # GuestPeek + GuestRead on table gate
    DefaultPermission: 2   # New rows are public by default
    Columns:
      - Name: title
        DataType: varchar(100)
        ColumnType: label
```

Private records must have their row `permission` tightened after creation, for example by admin/API workflow:

```json
{
  "data": {
    "type": "mixed_article",
    "id": "ARTICLE_ID",
    "attributes": {
      "permission": 0
    }
  }
}
```

## Expected Behavior

| Caller | Operation | Expected |
|--------|-----------|----------|
| Guest | List table | Allowed |
| Guest | See public rows | Allowed |
| Guest | See private rows | Not returned |
| Admin | List table | Sees public and private rows |

## Notes

The table gate is public. Privacy is controlled at the row gate.

The real E2E creates two records, patches one private, then verifies guests see only the public record.

## See Also

- [[Authorization-Scenarios]] - choose another tested pattern
- [[Authorization-Scenario-Public-Site]] - fully guest-readable records
- [[Authorization-Scenario-Semi-Private-Owner-Rows]] - owner-only private rows
- [[Common-Errors#403-forbidden-after-setting-permissions]] - debugging unexpected access
