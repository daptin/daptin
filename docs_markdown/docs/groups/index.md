# User groups

Users and objects (rows/tables/actions) can be added to usergroups to allow restricted access to a set of accounts.

For complete app patterns using groups, start with [Authorization scenarios](../permissions/authorization-scenarios.md).

## Default groups from schema

Tables can declare `DefaultGroups` so newly created rows are automatically related to usergroups.

The historical string form is still supported:

```yaml
Tables:
- TableName: project
  DefaultGroups:
  - administrators
```

The object form also sets the permission on the generated relation row:

```yaml
Tables:
- TableName: project
  DefaultGroups:
  - Name: administrators
    Permission: 524288
```

Use `AccessGroups` when the group should access the table/type itself:

```yaml
Tables:
- TableName: project
  AccessGroups:
  - Name: administrators
    Permission: 999424
```

This creates or updates the `world(project) -> administrators` relation. It does not apply to project records.
`AccessGroups` accepts the same string and object forms as `DefaultGroups`; use the object form when the relation row needs an explicit permission.

Selected actions can also declare `AccessGroups`:

```yaml
Actions:
- Name: publish_project
  OnType: project
  AccessGroups:
  - Name: administrators
    Permission: 524288
```

This creates or updates the relation for that action row only. The older `TableName: action` plus `DefaultGroups` form is still supported, but it applies to every schema-managed action.
