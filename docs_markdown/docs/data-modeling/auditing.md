# Data Audits

All changes in daptin can be recorded by enabling **auditing**. History is maintained in separate audit tables which maintain a copy of all columns at each change. Audit table are entities just like regular entities. All Patch/Put/Delete calls to daptin will create an entry in the audit table if the entity is changed.

## Audit tables

For any entity named ```<X>```, another tables ```<X>_audit``` is added by daptin. The audit table will contain all the columns which are present in the original table, plus an extra column ```is_audit_of``` is added, which contains the ID of the original row. The ```is_audit_of``` is a foreign key column to the parent tables ```id``` column.

## Audit row

Each row in the audit table is the copy of the original row just before it is being modified. The audit rows can be accessed just like any other relation.

## Audit table permissions

By default, everyone has the access to create audit row, and no one has the access to update or delete them. These permissions can be changed, but it is not recommended at present.

Type | Permission
--- | ---
Audit table permission | 007007007
Audit object permission | 003003003