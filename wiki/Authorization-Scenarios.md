# Authorization Scenarios

This page indexes practical Daptin authorization patterns tested against a real independent Daptin server.

The E2E coverage lives in `TestAccessGroupsRealAuthorizationScenariosE2E` and starts a fresh Daptin process with an isolated SQLite database and schema folder.

Run it with:

```bash
DAPTIN_REAL_E2E=1 go test . -run TestAccessGroupsRealAuthorizationScenariosE2E -count=1 -v
```

There is also a standalone shell/curl E2E that does not use `go test`:

```bash
scripts/testing/test-access-groups-e2e.sh
```

The shell test builds a temporary Daptin binary, starts an isolated server, and verifies the same public/private/mixed/shared/action scenarios through HTTP.

## Scenario Pages

| Pattern | Use When | Page |
|---------|----------|------|
| Public site | Anonymous visitors can read public records, but cannot write | [[Authorization-Scenario-Public-Site]] |
| Private site | Signed-in users can access shared records, guests cannot access the table | [[Authorization-Scenario-Private-Site]] |
| Semi-private owner rows | Signed-in users can reach the table, but only owners see their own rows | [[Authorization-Scenario-Semi-Private-Owner-Rows]] |
| Mixed public/private rows | One table contains both guest-readable and private records | [[Authorization-Scenario-Mixed-Public-Private]] |
| Shared group workspace | Editors and members need different row permissions | [[Authorization-Scenario-Shared-Group-Workspace]] |
| Action access gates | Selected schema actions should be callable by a group | [[Authorization-Scenario-Action-Access-Gates]] |

## Permission Gates

CRUD access has two gates:

```text
world(table) gate
AND
row(table record) gate
```

Action access has two schema gates, plus optional subject-row checks for instance actions:

```text
world(action.OnType) execute gate
AND
action(action_name, on_type) execute gate
```

Use:

```text
Tables[].AccessGroups   = table/type gate: world(table)
Tables[].DefaultGroups  = default row-group relation for new rows
Actions[].AccessGroups  = selected action-row gate
```

Important: `Permission: 0` and `DefaultPermission: 0` are treated as unset during schema sync. Use a non-zero restrictive value, then patch existing rows if an exact zero permission is required.

## Where To Go Next

- [[Permissions]] - permission bits, two-level CRUD checks, and `AccessGroups`
- [[Users-and-Groups]] - creating groups and assigning users
- [[Actions-Overview]] - action permission gates and action usergroup membership
- [[Schema-Reference-Complete#accessgroups]] - schema field reference
- [[Common-Errors#403-forbidden-after-setting-permissions]] - debugging forbidden responses
