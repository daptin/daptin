# Permissions

Daptin uses a permission system to control who can access what data.

**Related**: [[Authorization-Scenarios|SaaS Access Scenarios]] | [[Getting-Started-Guide|Getting Started]] | [[Users-and-Groups|Users and Groups]]

---

## Quick Start

### Fresh Install

On a fresh install, the system is **wide open** - anyone can do anything. This lets you:

1. Sign up your first user
2. Sign in
3. Become administrator
4. Explicitly lock and test public signup

### After Admin Setup

Once an admin exists:
- Bootstrap permissions are transitioned; verify that guest signup is rejected
- Guests can only view public data
- Only admins can create new users

Onboarding restricts actions using bootstrap permissions and preserves explicit
action permissions; signin stays publicly executable and
signup is closed. With the built-in policy, anonymous and ordinary users cannot
invoke `world/download_system_schema` after onboarding.

Upgrading does not rerun onboarding or repair existing action grants. On an
already configured instance, review administrative actions through the JSON:API
`action` resource and revoke unintended `GuestExecute` grants. Preserve explicitly
public application actions; the permission value alone does not distinguish an
intentional public grant from an unintended one.

See [[First-Admin-Setup]] for the required deny-and-test procedure. A successful
`become_an_administrator` response is not sufficient evidence that anonymous
signup is closed.

---

## Permission Basics

Every table and every record has permissions controlling:

| Permission | What it allows |
|------------|----------------|
| **Peek** | See that something exists (appears in lists) |
| **Read** | View the full data |
| **Create** | Make new records |
| **Update** | Change writable fields of existing records, including `permission` |
| **Delete** | Remove records |
| **Execute** | Run actions |
| **Refer** | Use in relationships |

### Update Access Includes Permission Changes

Direct resource Update permission allows the caller to change the record's
writable fields, including its `permission` bitmask. This applies whether access
comes from owner, group, or guest permissions, after the normal table and row
checks pass. An authorized editor can therefore change who may access the row,
including making it readable by guests. Update is not a field-level grant or an
owner-only permission-management capability; server-managed fields and normal
validation and relationship checks still apply.

Creation assigns server-controlled initial permissions. The `permission` field
is writable afterward through PATCH, as shown in [[Permissions#make-a-record-public-readable-by-guests|Make a Record Public]].
Changing a table's `DefaultPermission` affects new records, not the permissions
of existing records.

Grant direct Update access only to callers trusted to edit the row's writable
fields and access policy. If users should edit selected fields while sharing
policy remains administrator-controlled, withhold direct resource Update access
and expose a named action with validated inputs and explicit update-field
mappings. Configure its table, action, and subject-row execution permissions for
the intended callers. Hiding fields in a client UI does not restrict the API.
See [[Actions-Overview#constrained-record-edits|Constrained Record Edits]] for an
example.

### Who Gets Checked

The main bit ranges distinguish these callers:

| Group | Who |
|-------|-----|
| **Guest** | Anyone not logged in |
| **Owner** | The account that owns the record |
| **Group** | Members of a persisted group related to the record, using that relation’s Group bits |

Execution also has an `AuthenticatedExecute` bit for any signed-in account.
Administrator access comes from membership in the administrator group.

---

## Default Behavior

### Before First Admin

| Who | Can do |
|-----|--------|
| Guest | Everything (create, read, update, delete, execute) |
| Users | Everything |

### After First Admin

| Who | Can do |
|-----|--------|
| Guest | Peek at public data, execute signin |
| Owner | Read their own data, execute actions |
| Group members | Read shared data, execute actions |
| Administrators | Everything |

These describe the built-in policy, not a guarantee about every existing row.
Inspect persisted row and relation grants; onboarding and default changes do not
replace all previously stored permissions.

---

## Common Tasks

### Account Visibility

New accounts join the `users` group without granting that group read access to
their account records. After administrator onboarding, owners can read their
own accounts and administrators can read all accounts. Additional access requires
an explicit sharing grant.

Existing account-group relationships retain their persisted permissions. On an
existing installation, administrators must revoke `GroupRead` on the account's
`users` relationship to remove previously granted directory access; changing
the account row's permission alone does not revoke a relationship grant.

### Private Mail Tables for Signed-In Users

Mail applications often need all signed-in users to reach the mail tables while
keeping individual mail rows private to their owner. Keep those two concerns
separate:

- `AccessGroups` opens the table/type gate for a usergroup.
- Row `permission` and `DefaultPermission` decide which rows a caller can see.
- Existing rows keep their current `permission`; changing `DefaultPermission`
  only affects rows created after the schema change.

Example schema shape:

```yaml
Tables:
  - TableName: mail
    Permission: 561408
    DefaultPermission: 12672
    AccessGroups:
      - Name: users
        Permission: 638976

  - TableName: mail_account
    Permission: 577792
    DefaultPermission: 12672
    AccessGroups:
      - Name: users
        Permission: 573440

  - TableName: mail_box
    Permission: 577792
    DefaultPermission: 12672
    AccessGroups:
      - Name: users
        Permission: 573440
```

Do not add broad group row-read permission just because browser users need to
open `/api/mail`, `/api/mail_account`, or `/api/mail_box`. The table gate lets
the request reach the table; the row permission should still keep each user's
mail rows owner-scoped unless a mailbox is intentionally shared.

If old mail rows were created with overly broad permissions, repair those rows
explicitly after deploying the schema. For example, run an operator-controlled
SQL or API migration that sets the row `permission` on the affected rows to the
owner-only value you want. Verify with a normal user token, not only with an
administrator token.

### Check Who Can Access a Table

```bash
# Get table permission
curl --get \
  --data-urlencode 'query=[{"column":"table_name","operator":"is","value":"todo"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/world" | jq '.data[0].attributes.permission'
```

### Check Record Permission

```bash
curl "http://localhost:6336/api/todo/RECORD_ID" \
  -H "Authorization: Bearer $TOKEN" | jq '.data.attributes.permission'
```

### Make a Record Public (Readable by Guests)

```bash
curl -X PATCH "http://localhost:6336/api/todo/RECORD_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "todo",
      "id": "RECORD_ID",
      "attributes": {
        "permission": 33026
      }
    }
  }'
```

### Share with a Group

A group grant has two separate relationships:

- **Account → usergroup** records membership: which accounts belong to the group.
- **Resource → usergroup** records sharing: which operations that group may perform on that resource.

For example, adding an account to `assistant-users` does not by itself grant
access to an LLM model. The model must also be related to `assistant-users`,
with Execute enabled on that model–group relation. Editing the account–group
relation's permissions instead controls access to the account record; it does
not configure the group's access to the model. Remove membership to remove an
account from a group; clearing permission checkboxes is not membership removal.

#### Dashboard workflow

As an administrator:

1. Create a group under **Users & Access → Groups**.
2. Open the account's details, select **Groups → Add Group**, and attach that group.
3. Open the resource to share, select **Groups → Add Group**, and attach the same group.
4. In that resource's **Groups** tab, set the group's **Relation Permissions**.
   Changes save automatically. For an execute-only group grant, enable Execute
   and disable the other six group operations on that relation.
5. Hard-refresh the page and confirm the checkboxes retain the saved values.
6. Test the operation as an ordinary member, a nonmember, and a guest. Test
   revocation too: disable Execute and repeat the member request.

This was verified on a fresh Docker `v0.13.49` instance for account–group and
LLM-model–group relations: disabling Execute persisted after a hard refresh;
re-enabling it on the model relation also persisted. Normal resource permission
edits do not require a server restart.

The **Permissions** tab edits the resource row's own bitmask. The **Groups** tab
edits each resource–group relation's bitmask. They are separate grants. Setting
row `permission` to `0` does not revoke group grants, and disabling a group's
Execute grant does not revoke owner, guest, authenticated, administrator, or
other group grants.

The seven relation checkboxes edit the **Group** bits of the stored bitmask;
other bits can remain set. Only Execute checked therefore describes the group
grant, not necessarily a complete stored value of `524288`. For example, starting
from `2097151`, turning off GroupExecute saves `1572863`. Leaving only Execute
checked saves `540671` (`524288` plus the preserved lower `16383` bits), as
verified after refresh on `v0.13.49`. Inspect the saved value before claiming an
exact numeric mask. PATCHing a raw `permission` value
replaces that record's entire bitmask.

| Value | Meaning for execution |
|-------|-----------------------|
| Resource row `0` | No grant from the row bitmask; administrators and separately granted groups may still execute |
| Resource row `32` | GuestExecute; permits guest execution |
| Resource row `4096` | UserExecute; permits the record's owner, not every signed-in account |
| Resource row `2097152` | AuthenticatedExecute; permits any authenticated account |
| Resource–group relation `524288` | GroupExecute; permits members of that related group |

Group bits on the resource row are not a substitute for a persisted sharing
relation. The access check uses the matching resource–group relation's Group
bits. For generic CRUD, table/type and record checks must both pass; see
[Two-Level Permission Check](#two-level-permission-check-critical).

#### API boundaries and verification

Create and inspect links through parent resource relationships; see
[[Users-and-Groups#add-user-to-group|Add User to Group]] and
[[Relationships#relationship-endpoints|Relationship Endpoints]]. Use public
reference IDs, not internal numeric IDs or generated join-table names.

Relationship storage exists internally, but its generated table names are not
the application-facing CRUD contract. In `v0.13.49`, the dashboard internally
PATCHes those relation records to save permission changes. That working
implementation does **not** mean applications should list or manage generated
join resources directly; their collection routes can fail. Use the dashboard
workflow above for runtime group permission editing.

Do not treat a nested `attributes.permission` in a parent relationship request
as equivalent to the dashboard save. In `v0.13.46`, a resource-to-group nested
update returned success while discarding that field; the inverse group-to-resource
update persisted it. This is a direction-specific nested-update inconsistency,
not evidence that dashboard permission editing is broken. A success status
alone is insufficient: read back the relation and test effective access.

Creation uses configured defaults; supplying `permission` during resource
creation does not select the initial bitmask. Defaults are not universally
`2097151`, and changing defaults does not rewrite existing rows or relations.
For schema-managed group grants, use explicit `DefaultGroups` or `AccessGroups`
permissions as described below. Review initial sharing defaults before attaching
a group if even temporary broader access would be unacceptable.

Group operation values are bits 14–20: Read `32768`, Update `131072`, Execute
`524288`, and Read + Update + Execute `688128`.

---

## Permission Values

Permissions are stored as numbers. Here are common values:

| Value | Meaning |
|-------|---------|
| 0 | No grant from this row mask; independent group grants and administrator access remain |
| 2 | Guest can read |
| 256 | Owner can read |
| 32768 | GroupRead on a matching resource–group relation |
| 33026 | Everyone can read |
| 16256 | Owner has full access |
| 561441 | Generic public-action profile |
| 2097151 | Full access for everyone |

### Calculate Your Own

```javascript
// Permission calculator
function permission(guest, owner, group) {
  return guest | (owner << 7) | (group << 14);
}

// Bit values: Peek=1, Read=2, Create=4, Update=8, Delete=16, Execute=32, Refer=64
// Full = 127 (all bits)

permission(2, 127, 0);    // 16258 - Guest read, owner full
permission(0, 127, 127);  // 2097024 - Packed owner/group bits, no guest
permission(2, 2, 2);      // 33026 - Everyone can read
```

---

## Administrator Group

Members of the `administrators` group bypass all permission checks.

### Become First Admin

```bash
# After signing up and signing in
curl -X POST http://localhost:6336/action/world/become_an_administrator \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{}'
```

Server restarts. Sign in again to get admin token.

### Add User to Admin Group

```bash
curl -X PATCH "http://localhost:6336/api/user_account/USER_ID" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "user_account",
      "id": "USER_ID",
      "relationships": {
        "usergroup_id": {
          "data": [{"type": "usergroup", "id": "ADMIN_GROUP_ID"}]
        }
      }
    }
  }'
```

---

## Enable Public Signup (After Admin Exists)

By default, signup is disabled after admin setup. To re-enable:

1. Find the signup action ID
2. Add guest execute permission (value: current + 32)

On current code, `become_an_administrator` locks `signup` to `2085120` and re-opens only `signin` to `561441`. So the minimal re-enable value for `signup` is `2085152`:

- `2085120` = locked post-admin `signup`
- `2085152` = `2085120 + GuestExecute(32)`
- `561441` = generic public-action profile, not the required reopen value for `signup`

```bash
# Find signup action
curl --get \
  --data-urlencode 'query=[{"column":"action_name","operator":"is","value":"signup"}]' \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  "http://localhost:6336/api/action" | jq '.data[0].id'

# Update permission to add guest execute (32)
# Current permission + (32) = 2085120 + 32 = 2085152
curl -X PATCH "http://localhost:6336/api/action/ACTION_ID" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "action",
      "id": "ACTION_ID",
      "attributes": {
        "permission": 2085152
      }
    }
  }'
```

No restart is required. If authorization for `signup` was checked immediately
before the update, the previous decision can remain cached for up to 10
seconds; retry after that cache window.

---

## State Machine Permissions

State machines have special permission requirements for the **Refer** permission on the `smd` table.

### The Refer Permission

When creating a state machine instance with `/track/start/:smdId`, the system creates a relationship from `{table}_state` to the `smd` table. This requires **Refer** permission on the `smd` record.

**Without Refer permission, you'll see:**
```
[ERROR] User cannot refer this object [smd][<uuid>]
Failed to execute state insert query: refer object not allowed [smd][<uuid>]
HTTP 500 Internal Server Error
```

### Grant Refer Permission to SMD

**Option 1: Via API (Recommended)**

```bash
# Get your SMD ID
SMD_ID=$(curl -s "http://localhost:6336/api/smd" \
  -H "Authorization: Bearer $TOKEN" | \
  jq -r '.data[] | select(.attributes.name == "ticket_workflow") | .id')

# Update SMD to allow users to reference it
curl -X PATCH "http://localhost:6336/api/smd/$SMD_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "smd",
      "id": "'$SMD_ID'",
      "attributes": {
        "permission": 1621954
      }
    }
  }'
# Permission 1621954 = Guest:Read|Refer (66) + Owner:Full (127) + Group:Read|Execute|Refer (98)
# Calculation: 66 | (127 << 7) | (98 << 14) = 66 + 16256 + 1605632 = 1621954
```

Use the resource PATCH above so normal authorization, validation, audit, and cache invalidation run.

### State Transition Permissions

The `/track/event/:typename/:stateId/:eventName` endpoint requires **Execute** permission on the `smd` table:

```bash
# Verify SMD has execute permission
curl "http://localhost:6336/api/smd/$SMD_ID" \
  -H "Authorization: Bearer $TOKEN" | \
  jq '.data.attributes.permission'

# Permission bits needed:
# - Guest Execute: 32 (bit 5)
# - Owner Execute: 4096 (32 << 7)
# - Group Execute: 524288 (32 << 14)
```

**Full SMD Permission Example:**
```javascript
// Permission bits: Peek=1, Read=2, Create=4, Update=8, Delete=16, Execute=32, Refer=64
const guestPerms = 2 | 64;        // Read + Refer = 66
const ownerPerms = 127;           // Full access = 127
const groupPerms = 2 | 32 | 64;   // Read + Execute + Refer = 98

const smdPermission = guestPerms | (ownerPerms << 7) | (groupPerms << 14);
// = 66 | 16256 | 1605632 = 1621954
```

### Why Two Permissions?

1. **Refer (64)** - Required for `/track/start/:smdId`
   - Allows creating relationships to the SMD
   - Checked when creating state instances

2. **Execute (32)** - Required for `/track/event/:typename/:stateId/:eventName`
   - Allows applying state transitions
   - Checked before executing FSM events

**Related**: [[State-Machines|State Machines]] - Full state machine documentation

---

## Two-Level Permission Check (CRITICAL)

**Important**: Daptin checks permissions at TWO levels for group-based access:

1. **Table-level** (world record): Can this group access the table at all?
2. **Record-level**: Can this group access this specific record?

**Both levels must be configured**, or you'll get 403 Forbidden errors even when the record permissions look correct.

### Example: Share Product Table with Marketing Group

Find the `world` record whose `table_name` is `product`. In the dashboard, open
that world record's **Groups** tab and grant the marketing group the operations
needed at the table gate. Then open each product's **Groups** tab and configure
its row grant. Follow [Share with a Group](#share-with-a-group) for the workflow
and verification. A group relation at one level does not grant access at the other.

---

## Schema Provisioning: Table and Action AccessGroups

For complete application patterns, start with [[Authorization-Scenarios]]. It
shows how permissions compose with anonymous rate limiting, authenticated
plans, team membership, integrations, and trusted service-account actions.

**Reproduced on a fresh SQLite Daptin instance: 2026-07-02**

Some applications need schema-managed actions and table access to be deployed without a manual post-deploy repair step. Use `AccessGroups` to declare the usergroup relation for the schema-managed object itself.

For a table, `AccessGroups` relates the table's `world` row to a group:

```yaml
Tables:
  - TableName: document
    Permission: 1003811
    AccessGroups:
      - Name: users
        Permission: 999424
```

This creates or updates:

```text
world(document) -> users
```

`AccessGroups` accepts the same string and object forms as `DefaultGroups`. Use object form when the relation row needs an explicit `Permission`; if omitted, Daptin uses the relation table's default permission.

For an action, `AccessGroups` relates that specific `action` row to a group:

```yaml
Actions:
  - Name: set_document_private
    OnType: document
    Permission: 32
    AccessGroups:
      - Name: users
        Permission: 524288
  - Name: set_document_public
    OnType: document
    Permission: 32
    AccessGroups:
      - Name: users
        Permission: 524288
```

This creates or updates:

```text
action(set_document_private on document) -> users
action(set_document_public on document) -> users
```

### Why AccessGroups Is Different From DefaultGroups

`DefaultGroups` applies to data rows created inside a table:

```yaml
Tables:
  - TableName: document
    DefaultGroups:
      - Name: users
        Permission: 999424
```

This creates default relations for `document` records. It does not relate `world(document)` to `users`.

`AccessGroups` applies to the schema-managed object itself:

```text
Tables[].AccessGroups   = access to the table/type gate: world(table)
Tables[].DefaultGroups  = default groups for data rows created in that table
Actions[].AccessGroups  = access to one specific action row
```

### Broad Action Defaults

The older table-level form is still supported:

```yaml
Tables:
  - TableName: action
    DefaultGroups:
      - Name: users
        Permission: 524288
```

This applies to every schema-managed action. Use `Actions[].AccessGroups` when only selected actions should be related to a group.

If both broad action `DefaultGroups` and an action's `AccessGroups` contain the same group, Daptin applies the broad default first and the action-level `AccessGroups` second. The action-level relation permission wins.

### Canaster-Style Example

Action execution checks both the table/type gate and the action row gate:

```text
world(document) permission check
AND
action(set_canaster_document_private) permission check
```

Declare both gates in schema:

```yaml
Tables:
  - TableName: document
    Permission: 1003811
    AccessGroups:
      - Name: users
        Permission: 999424

  - TableName: asset
    Permission: 1003811
    AccessGroups:
      - Name: users
        Permission: 999424

Actions:
  - Name: set_canaster_document_private
    OnType: document
    Permission: 32
    AccessGroups:
      - Name: users
        Permission: 524288
  - Name: set_canaster_document_public
    OnType: document
    Permission: 32
    AccessGroups:
      - Name: users
        Permission: 524288
  - Name: set_canaster_asset_private
    OnType: asset
    Permission: 32
    AccessGroups:
      - Name: users
        Permission: 524288
  - Name: set_canaster_asset_public
    OnType: asset
    Permission: 32
    AccessGroups:
      - Name: users
        Permission: 524288
```

Expected startup result:

- `world(document) -> users`
- `world(asset) -> users`
- only the declared Canaster action rows are related to `users`
- unrelated actions are unchanged unless broad `TableName: action DefaultGroups` is also configured

---

## Troubleshooting

### 403 Forbidden (Most Common Causes)

**1. Table not shared with group** (MOST COMMON)

The user's group has permission on the record, but not on the table itself.

**Solution**: Check the world (table) grant and the record grant separately; follow [Share with a Group](#share-with-a-group).

**2. Stale Olric permission cache**

Normal resource updates invalidate the relevant permission caches. A restart is
not a required step for dashboard group-permission editing. First hard-refresh
the Groups tab, verify the saved relation permissions and membership, and test
with the intended account. Distinguish a value that was never saved from a saved
grant whose effective access is denied by another gate.

**3. Other checks**:
- Are you logged in?
- Do you own the record?
- Are you in a group that has access to the record?
- Is the operation allowed for your role?

### Can't Sign Up

Signup is disabled after admin setup. Ask admin to create your account or enable public signup.

### Can't See Records

The records exist but you don't have Peek permission. Contact the owner or admin.
Direct `GET /api/{table}/{reference_id}` also requires Read on the row: a
Peek-only grant does not expose its attributes, and the request returns 403.

---

## See Also

- [[Getting-Started-Guide|Getting Started Guide]] - First-time setup
- [[Users-and-Groups|Users and Groups]] - Managing users
- [[Authorization-Scenarios|SaaS Access Scenarios]] - Choose and compose an application access model
- [[Actions-Overview|Actions Overview]] - Running actions
