# Action Permission Schema Sync Technical KT

This page is for maintainers changing schema startup, action permissions, or usergroup relation defaults.

## Problem

Schema-managed actions need two permission values to stay in sync with schema files:

| Concern | Storage |
|---------|---------|
| Can the action be executed? | `action.permission` |
| Which usergroups can access the action row? | `action_action_id_has_usergroup_usergroup_id.permission` |

The usergroup relation is not action-specific. Every entity has the default `has_many usergroup` relation, so action usergroups are configured through `TableInfo.DefaultGroups` on `TableName: action`.

## Public Schema Contract

Action row permission:

```yaml
Actions:
  - Name: post_gig
    Label: Post gig
    OnType: gig
    InstanceOptional: true
    Permission: 32
```

Default usergroup relation permission:

```yaml
Tables:
  - TableName: action
    DefaultGroups:
      - Name: administrators
        Permission: 524288
```

The old string form remains valid:

```yaml
DefaultGroups:
  - administrators
```

## Implementation Files

| File | Responsibility |
|------|----------------|
| `server/actionresponse/action_pojo.go` | Adds optional `Action.Permission` for schema-managed actions |
| `server/table_info/tableinfo.go` | Adds `DefaultGroupList` and `DefaultGroupBinding`; preserves string-form compatibility |
| `server/resource/dbresource.go` | Resolves default group names into IDs plus optional relation permissions |
| `server/resource/resource_create.go` | Applies configured default-group relation permission when creating any entity row |
| `server/resource/dbfunctions_update.go` | Syncs `action.permission` and generic action/usergroup relation rows during `UpdateActionTable` |
| `server/resource/dbmethods.go` | Adds cache invalidation helpers for action and where-clause permission cache entries |

## Startup Flow

```text
LoadConfigFiles
  includes StandardTables and schema files
UpdateWorldTable
  stores merged TableInfo JSON in world.world_schema_json
UpdateActionTable
  reads TableInfo for action from world.world_schema_json
  inserts or updates action rows
  syncs action.permission when Action.Permission is present
  rebuilds action_action_id_has_usergroup_usergroup_id rows from action.DefaultGroups
  invalidates action and permission/group caches
```

## Important Rules

- Do not add `DefaultGroups` to `actionresponse.Action`; relation defaults belong to `TableInfo`.
- If `Action.Permission` is omitted, existing actions keep their stored permission and newly inserted actions use `auth.ALLOW_ALL_PERMISSIONS`.
- If a `DefaultGroups` item includes `Permission`, that permission belongs to the join-table relation row.
- If a `DefaultGroups` item omits `Permission`, Daptin uses the relation table's default permission.
- Missing default group names are strict errors during schema sync, but non-strict during resource construction so partially configured runtime resources do not crash from optional defaults.

## Tests

| Test | Coverage |
|------|----------|
| `server/table_info/tableinfo_test.go` | String and object `DefaultGroups` parsing/serialization |
| `server/resource/action_schema_sync_test.go` | `UpdateActionTable` syncs `action.permission` and `action_action_id_has_usergroup_usergroup_id.permission` |

## Runtime Verification

The issue was reproduced and verified with a fresh SQLite instance, trace logging, and this schema shape:

```yaml
Tables:
  - TableName: action
    DefaultGroups:
      - Name: administrators
        Permission: 524288
Actions:
  - Name: post_gig
    OnType: gig
    InstanceOptional: true
    Permission: 32
```

After manually drifting the database values, restart restored:

```text
action.permission = 32
action_action_id_has_usergroup_usergroup_id.permission = 524288
```
