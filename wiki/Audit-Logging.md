# Audit Logging

Set `IsAuditEnabled: true` on a resource to preserve snapshots of its previous
values when a record changes. Daptin stores these snapshots in an internal
`{tablename}_audit` table. Audit tables are not JSON:API or GraphQL
resources and have no CRUD routes. They do not have the source resource's
usergroup relationships.

```yaml
Tables:
  - TableName: account
    IsAuditEnabled: true
    Columns:
      - Name: account_name
        DataType: varchar(200)
        ColumnType: label
      - Name: balance
        DataType: float
        ColumnType: float
```

Creating an account does not write an audit snapshot. Each update stores the
old values before the change; deleting a record stores its final values. Audit
snapshots include the operation, a creation timestamp, and
`source_reference_id` containing the source record's public reference ID.
Sensitive columns such as passwords, encrypted values, files, and blobs are
excluded from generated audit tables.

Audit storage is internal. Audit rows have no direct resource API for
listing, filtering, editing, or deleting them. Snapshots do not currently record
the authenticated actor. Do not treat this feature alone as a complete
compliance trail or rollback mechanism.

Administrator backup exports may include audit tables; those exports do not
create per-record audit CRUD endpoints. See [[Data-Actions|Data Actions]].

Audit history grows with updates and deletes. Enable it only for resources
whose previous values need to be retained.

See [[Schema-Reference-Complete|Schema Reference]] for the schema option and
[[Permissions|Permissions]] for access to the source resource.
