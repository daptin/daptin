# Enable data auditing

Set `IsAuditEnabled: true` on a table to keep its previous values when rows are updated, deleted, or changed by a state transition.

```yaml
Tables:
- TableName: project
  IsAuditEnabled: true
  Columns:
  - Name: title
    ColumnType: label
    DataType: varchar(200)
```

Daptin generates `project_audit`. Each audit row contains the eligible previous column values together with:

- `source_reference_id`: reference ID of the source row.
- `operation`: `update`, `delete`, or `state_transition`.
- `created_at`: time at which the audit row was written.
- `user_account_id`: account which initiated the operation, when the request has an authenticated account.

Create operations are not currently audited.

## Permissions

Generated audit tables do not grant guest create, peek, or read access. Their rows do not grant guest access. Administrators can inspect audit data through Daptin's normal administrator-group authorization bypass.

Audit writes are server-owned writes made inside the same database transaction as the source change. Non-administrator clients cannot manufacture audit rows through the generated table's API permissions.

To use different columns, permissions, `AccessGroups`, or `DefaultGroups`, declare the complete `<source>_audit` table in the schema. Daptin preserves an explicitly declared audit table instead of adding the source table's remaining columns to it.

## Sensitive columns

Automatically generated audit tables do not copy:

- password or bcrypt columns;
- encrypted columns;
- file columns;
- blob or binary columns;
- source `permission`, `reference_id`, or `user_account_id` values.

An explicitly declared audit table is operator-managed and is not subject to these generated-schema exclusions.

## Existing installations

During schema synchronization, Daptin updates the table permissions for generated audit tables. Existing audit rows are changed only when their permission exactly matches the former generated default. Custom row permissions and explicitly declared audit tables are preserved.

Audit history has no automatic retention period. Operators should include audit tables in storage, privacy, backup, and cleanup policies. Removing historical columns or rows is destructive and should be performed only after taking an appropriate backup.
