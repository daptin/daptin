# User groups

Users and objects (rows/tables/actions) can be added to usergroups to allow restricted access to a set of accounts.

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

For actions, configure this on `TableName: action`. Daptin stores the relation in `action_action_id_has_usergroup_usergroup_id`, the same generic usergroup relation used by all entities.
