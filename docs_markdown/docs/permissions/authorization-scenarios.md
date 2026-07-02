# Authorization scenarios

Use these tested patterns to choose how a Daptin app should expose data and actions.

The full wiki pages include schema examples, expected behavior, and real E2E coverage:

- [Public site](https://github.com/daptin/daptin/wiki/Authorization-Scenario-Public-Site)
- [Private site](https://github.com/daptin/daptin/wiki/Authorization-Scenario-Private-Site)
- [Semi-private owner rows](https://github.com/daptin/daptin/wiki/Authorization-Scenario-Semi-Private-Owner-Rows)
- [Mixed public/private rows](https://github.com/daptin/daptin/wiki/Authorization-Scenario-Mixed-Public-Private)
- [Shared group workspace](https://github.com/daptin/daptin/wiki/Authorization-Scenario-Shared-Group-Workspace)
- [Action access gates](https://github.com/daptin/daptin/wiki/Authorization-Scenario-Action-Access-Gates)

## Quick map

| Pattern | Main idea |
|---------|-----------|
| Public site | Table and rows grant guest read; guest writes remain denied. |
| Private site | `AccessGroups` opens the table/type gate for signed-in users, and `DefaultGroups` shares new rows. |
| Semi-private owner rows | Users can reach the table, but `DefaultPermission` keeps records owner-readable. |
| Mixed public/private rows | The table is public, while selected rows are tightened after creation. |
| Shared group workspace | Runtime groups such as editors/members get different row relation permissions. |
| Action access gates | `Tables[].AccessGroups` opens the type execute gate and `Actions[].AccessGroups` opens selected action rows. |

## Core distinction

```text
Tables[].AccessGroups   = access to the table/type gate: world(table)
Tables[].DefaultGroups  = default groups for rows created in that table
Actions[].AccessGroups  = access to one specific action row
```

The real E2E test is:

```bash
DAPTIN_REAL_E2E=1 go test . -run TestAccessGroupsRealAuthorizationScenariosE2E -count=1 -v
```

The standalone shell/curl E2E is:

```bash
scripts/testing/test-access-groups-e2e.sh
```
