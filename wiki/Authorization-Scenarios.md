# Authorization and SaaS Access Scenarios

Start here when you know the product behavior you want but are not yet sure
which Daptin features configure it.

Daptin keeps four decisions independent:

1. **Identity** says who is making the request: guest, signed-in user, group
   member, or the active user selected by a trusted action.
2. **Permissions** decide whether that identity may reach a table, row, or
   action.
3. **Metering** records and limits how much an authenticated account may use.
4. **Credential ownership** decides which account may use an external-provider
   secret.

Passing one decision never bypasses another. This lets the same resources and
actions support public products, subscriptions, team applications, and shared
backend workflows without separate execution paths.

## Choose the Product Behavior

| Product behavior | Identity | Access gate | Usage control | Start with |
|---|---|---|---|---|
| Public content | Guest | Guest table and row permissions | None | [[Authorization-Scenario-Public-Site]] |
| Public content with abuse protection | Guest | Guest table and row permissions | Global or ingress IP/path rate limiting | [[Authorization-Scenario-Public-Site]] and [[Rate-Limiting]] |
| Free signed-in tier | Daptin user | User or group permissions | Active free `api_plan` membership | [[Authorization-Scenario-Private-Site]] and [[API-Metering]] |
| Paid individual tier | Daptin user | User or group permissions | Active paid plan with hard or soft limits | [[API-Metering]] |
| Team workspace | User plus persisted usergroup membership | World, row, and action group permissions | Meter each active user through their plan | [[Authorization-Scenario-Shared-Group-Workspace]] |
| Public catalogue with private customer data | Guest or signed-in user, depending on the row | World and row permissions | Meter authenticated operations when required | [[Authorization-Scenario-Mixed-Public-Private]] |
| User-connected external provider | Signed-in user | Generated integration action permission | Meter the generated action if required | [[Integrations#execute-integration-operations]] |
| Shared provider/service account | Caller starts a trusted action; `SWITCH_USER` selects the service user for later outcomes | Wrapper action permission plus credential ownership | Meter the caller-facing action separately from provider identity | [[Integrations#run-an-integration-as-a-service-account]] |
| Administrator provisioning workflow | Administrator group | Administrator-only action | Usually not customer-metered | [[Authorization-Scenario-Action-Access-Gates]] |

## Public Does Not Mean Individually Metered

A guest has no durable Daptin `user_account` identity. Daptin can grant that
guest access with guest permission bits, but API metering does not create
per-user usage or quota records for anonymous requests.

Choose one of these supported designs:

- For a genuinely anonymous site, use guest permissions and protect traffic
  with Daptin's global limiter or an ingress/API gateway. Anonymous limiting is
  based on network/request properties, not a customer subscription.
- For a free allowance, credit balance, or paid subscription that must follow
  one customer, authenticate the consumer as a Daptin user and assign an
  `api_member` and `api_plan`.
- A frontend may look public while obtaining a user session before a metered
  operation. The operation is then authorized and metered as that account.

Do not enable metering on a guest endpoint and assume anonymous requests will
become plan members. See [[Rate-Limiting]] for anonymous protection and
[[API-Metering]] for durable account quotas.

## Common SaaS Compositions

### Public documentation or catalogue

Grant guest read access at both required CRUD gates:

```text
world(table).CanRead
AND
row.CanRead
```

Do not grant guest create, update, delete, or execute unless the product
explicitly requires those operations. Follow the complete tested configuration
in [[Authorization-Scenario-Public-Site]]. Use
[[Authorization-Scenario-Mixed-Public-Private]] when only selected rows are
public.

### Free and paid application tiers

Keep entitlement and quantity separate:

```text
authenticated user
  -> table/row/action permission
  -> active api_member
  -> api_plan limits
  -> metered operation
```

The free and paid tiers can use the same endpoint and permission path. Give
their memberships different plans instead of creating separate routes or
actions. A hard limit rejects exhausted usage; a soft limit records overage
without denying the request. See [[API-Metering#create-a-plan-and-membership]]
and [[API-Metering#enable-metering-on-resources-and-actions]].

If an authenticated user has no active membership, Daptin may record metered
usage but has no plan limits to enforce. Every tier that requires a quota must
therefore have an active membership.

### Team or tenant application

Use persisted `user_account` to `usergroup` relationships for team membership.
Use `AccessGroups` for the world/action gate and row-group relationships for
the records shared with that team.

```text
group membership
  -> world permission
  -> row or action permission
```

Metering remains account-based: each request is charged to the authenticated
user's active membership. Group membership does not silently become a shared
quota pool. Follow [[Authorization-Scenario-Shared-Group-Workspace]] and
[[Users-and-Groups]].

### User-connected integration

Install the integration and allow the generated `{provider}/{operation}` action
for the intended users or groups. The caller supplies their own
`credential_id` or `oauth_token_id`; Daptin verifies ownership before contacting
the provider.

Provider-scoped REST, generated-action REST, and provider GraphQL use the same
installed action permission and execution path. See [[Integrations]] for the
complete setup.

### Shared backend or payment integration

Use a trusted wrapper action when many callers should initiate work using one
private service credential:

```text
caller
  -> wrapper action permission
  -> SWITCH_USER to fixed service user
  -> integration outcome with fixed credential reference
  -> provider
```

Keep the service-user and credential references in the backend-controlled
action definition; do not expose them as action input fields. The wrapper
permission controls who may start the workflow, while credential ownership
still controls which active user may use the secret. Follow
[[Integrations#run-an-integration-as-a-service-account]] and
[[Credentials#shared-backend-or-service-credential]].

If the caller-facing wrapper itself is metered, configure that action's
metering deliberately. Provider identity and customer billing identity are
different concerns and should not be inferred from each other.

### Restricted administrator operation

Set the action's broad permission to a restrictive value and relate the action
to the administrators group with `GroupExecute`. The action still requires the
execute gate on its `OnType`. Follow
[[Authorization-Scenario-Action-Access-Gates]].

## Tested Permission Guides

These focused guides contain complete schema shapes and expected behavior:

| Pattern | Use when | Guide |
|---|---|---|
| Public site | Anonymous visitors can read public records but cannot write | [[Authorization-Scenario-Public-Site]] |
| Private site | Signed-in users can access shared records; guests cannot access the table | [[Authorization-Scenario-Private-Site]] |
| Semi-private owner rows | Signed-in users can reach the table but only owners see their rows | [[Authorization-Scenario-Semi-Private-Owner-Rows]] |
| Mixed public/private rows | One table contains both guest-readable and private records | [[Authorization-Scenario-Mixed-Public-Private]] |
| Shared group workspace | Editors and members need different row permissions | [[Authorization-Scenario-Shared-Group-Workspace]] |
| Action access gates | Only selected groups should execute selected actions | [[Authorization-Scenario-Action-Access-Gates]] |

## The Permission Gates

CRUD access has two gates:

```text
world(table) gate
AND
row(table record) gate
```

Action access has two schema gates, plus an optional subject-row check for an
instance action:

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

Important: `Permission: 0` and `DefaultPermission: 0` are treated as unset
during schema synchronization. Use a non-zero restrictive value, then update an
existing row through the normal resource API if an exact zero permission is
required.

## Verification

The permission scenario guides are exercised against an independent Daptin
process by `TestAccessGroupsRealAuthorizationScenariosE2E`:

```bash
DAPTIN_REAL_E2E=1 go test . -run TestAccessGroupsRealAuthorizationScenariosE2E -count=1 -v
```

The equivalent shell/curl suite is:

```bash
scripts/testing/test-access-groups-e2e.sh
```

Integration action authorization and service-user credential composition are
covered by the integration real-E2E tests.

## Reference

- [[Permissions]] — permission bits and two-level access checks
- [[Users-and-Groups]] — users, groups, and membership
- [[Actions-Overview]] — trusted workflows and action permissions
- [[API-Metering]] — plans, quotas, reservations, and usage
- [[Rate-Limiting]] — anonymous/IP protection versus account metering
- [[Integrations]] — personal and shared provider workflows
- [[Credentials]] — secret ownership and provisioning
