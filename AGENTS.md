# Working in Daptin

This file defines the repository-wide engineering rules for coding agents.
Daptin is a connected application server, not a collection of independent
features. A change is correct only when it preserves that connected model.

The product idea is summarized at <https://daptin.github.io/> and
<https://daptin.github.io/product/architecture/>: define the product once, then
let data, identity, permissions, actions, files, integrations, protocols,
metering, and operations reuse the same context.

## The Daptin way

Prefer the smallest change that composes existing capabilities. New behavior
should normally be expressed as a new combination of resources, relationships,
permissions, actions, and existing services—not as a parallel subsystem.

The non-negotiable rules are:

1. One concern has one authority.
2. Every surface that offers the same operation uses the same core path.
3. Attached services adapt Daptin resources; they do not recreate Daptin.
4. Policy is explicit data or backend configuration, never inferred from
   incidental request state or provenance.
5. Add no compatibility branch, legacy fallback, fuzzy interpretation, mode,
   or second path unless the task explicitly requires it.
6. Do not broaden a shared interface to solve a local problem when a local
   adapter can reuse the existing interface.
7. Minimize production code and affected files. Comprehensive tests are not
   considered waste; duplicated production logic is.

## Authorities already present

Use these authorities instead of creating new ones:

| Concern | Daptin authority |
|---|---|
| Authenticated identity | `auth.SessionUser` and the `user_account` resource |
| Team or tenant membership | persisted `user_account` ↔ `usergroup` relationships |
| Access decisions | `permission.PermissionInstance` and its operation-specific checks |
| Record identity | Public `reference_id`; internal numeric `id` only inside persistence |
| Record access context | Every entity record has an owner and related usergroups |
| Data shape and relations | schema plus `world` metadata and `table_info` |
| CRUD lifecycle | `resource.DbResource` and its middleware chain |
| Product behavior | named actions and ordered outcomes |
| Background behavior | persisted tasks invoking the same action handler |
| Credentials | permissioned `credential` or OAuth records |
| Usage and quota | `MeteringService`, `api_plan`, `api_member`, `api_usage`, and `api_quota` |
| Live coordination | Olric cache, PubSub, counters, and leases |
| Durable state | the configured SQL database and resource rows |
| Runtime composition | `server/server.go` and the ordered runtime lifecycle |

These concerns are orthogonal. Permission to invoke an action does not grant
permission to every record, credential, model, or provider used by its outcomes.
Likewise, authorization answers *whether* work may run; metering answers *how
much* may run. Do not collapse these decisions into one flag or infer one from
the other.

## Start every change with repository investigation

Before designing code:

1. State the invariant being changed in one sentence.
2. Find the current entry point, authority, and side effect with `rg`.
3. Trace upstream callers and downstream dependencies.
4. Inspect sibling surfaces such as REST, GraphQL, actions, schedules, batch
   workers, protocol adapters, and administration flows.
5. Find the closest established implementation and its tests.
6. Identify which files truly need to change. If a local feature requires edits
   in unrelated mail, FTP, storage, OAuth, or resource code, stop and reconsider
   the abstraction.

Do not begin with a new type, interface, mode, or configuration field. Begin
with the existing product invariant and the path that already enforces it.

## Reuse the core paths

### Resources and persistence

- Feature and adapter packages use `DbResource` operations and normal relation
  resources. They do not issue SQL to bypass resource behavior.
- Preserve validation, conformation, ownership, permissions, audit, events,
  cache invalidation, and metering by entering through the established resource
  path.
- Use Daptin reference IDs at API and feature boundaries. Convert to internal
  numeric IDs only through existing resource helpers and only where persistence
  requires them.
- Represent relationships as Daptin relationships. Do not add shadow mapping
  tables, role strings, or private membership caches for a feature.
- A local feature must not change a widely used `DbResource` signature merely
  to obtain different error handling. Adapt locally unless the shared contract
  itself is the subject of the task.
- If the task genuinely changes a shared resource contract, enumerate and
  inspect every caller before editing it.

Direct SQL belongs only in the established database/resource implementation or
tests specifically exercising that layer. Feature code, feature-test setup,
examples, and user guides must not use raw SQL to sidestep Daptin APIs or schema
synchronization.

### Identity and permissions

- Use `auth.SessionUser`; do not introduce a second user/principal model unless
  an external library requires an adapter at its boundary.
- Use persisted `usergroup` relationships as membership authority.
- Use the existing `PermissionInstance` method matching the operation:
  `CanRead`, `CanCreate`, `CanUpdate`, `CanDelete`, `CanRefer`, `CanPeek`, or
  `CanExecute`.
- Keep table, row, relation, and action checks distinct. Passing one check does
  not bypass the others.
- Administrator behavior must come from Daptin's administrator-group rules.
  Transient privileges used while executing trusted action outcomes must not
  leak into an orthogonal permission decision.
- Authorization failure or unavailable permission data must never become
  permission granted. Follow the existing subsystem's failure convention
  rather than inventing a feature-specific error hierarchy.
- Never accept caller-supplied identity, group lists, permission modes, or
  ownership claims as authorization authority.

### Actions and trusted backend behavior

- Actions are the reusable backend home for product operations. REST, GraphQL,
  schedules, and integrations should invoke the same named action or performer
  rather than implement equivalent behavior separately.
- Action definitions are trusted backend definitions. Action input values are
  still caller input and must be validated and constrained by the action's
  permissions and schema.
- Outcomes run in order and share the active action context. `SWITCH_USER` is an
  explicit backend action outcome; later outcomes use the selected account.
- Wrapper-action permission only controls who may start the workflow. Each
  downstream resource, credential, provider, or model keeps its own authority.
- Do not infer privileged behavior from `sessionUser`, template provenance,
  ownership coincidences, fixed-looking evaluated values, or whether execution
  came from an action. If trusted behavior is required, express it explicitly
  in the backend-controlled action definition using established outcomes.

### Attached services and protocols

- Storage, sites, integrations, LLM routing, mail, WebSockets, Yjs, FTP, DAV,
  feeds, and other attached surfaces are adapters around shared Daptin state.
- When direct HTTP, an action outcome, a batch worker, and a scheduled task
  expose the same operation, they must converge before authorization and side
  effects. Do not maintain similar implementations for each transport.
- Provider credentials and operational routing remain trusted backend
  configuration. User entitlement remains a permission decision. Metering
  remains a metering decision.
- Keep live coordination and durable truth separate. Olric may coordinate,
  cache, publish, or count; it must not silently replace database authority.
- Register new runtime components through the existing composition and shutdown
  lifecycle. A component that starts goroutines or listeners must have bounded
  ownership, cancellation, readiness, and drain behavior.

### Transactions and external work

- Follow the transaction boundary of the core path being reused.
- Pass the active transaction through resource and action operations that are
  intended to be atomic. Do not create an independent write path around it.
- Do not keep a database transaction open across a long provider stream or
  other unbounded network operation. Use the established admission/reservation
  and terminalization pattern where applicable.
- Commit, rollback, retry, and idempotency belong at the owning boundary. Do not
  add nested retries or partial commits in adapters without an explicit design.

### Metering

- Use `resource.MeteringService`; do not add feature-specific quota tables or
  counters as the durable authority.
- Preserve the established admit → complete/cancel lifecycle.
- Meter the active Daptin account for the operation. If an action explicitly
  changes the active account, later metered work follows that account according
  to the shared action context.
- Durable plan limits live in the database. Olric protection counters are
  operational safeguards, not customer quota authority.
- Authorization happens before an external side effect. A denied request must
  not reach the provider merely so that it can be metered.

## Avoid architectural drift

Treat these as warning signs:

- a new boolean or enum that duplicates existing permission, ownership, action,
  or relation semantics;
- branches based on whether the caller used REST, GraphQL, an action, a task, or
  a batch;
- a second implementation of an operation for a new transport;
- caller input selecting an internal security mode;
- direct SQL in a feature package, test fixture, example, or user guide;
- changing many unrelated callers because one feature wants a different local
  contract;
- new caches for data already owned by Daptin resources or Olric;
- compatibility or fallback logic not required by the issue;
- duplicated documentation that explains the same workflow in several places;
- speculative abstractions with only one implementation or one caller;
- broad cleanup, renaming, formatting, or bug fixes mixed into a focused change.

When one of these appears, search again for the existing Daptin authority and
reduce the design.

## Security review checklist

For every changed path, verify:

- Which server-created identity is active?
- Which persisted owner/group/relation supplies access?
- Is the permission check appropriate to the operation?
- Can transient action privilege escape into another concern?
- Can caller input choose identity, privilege, credential policy, or an
  internal execution mode?
- Does denial happen before provider calls, writes, publication, or other side
  effects?
- Does `SWITCH_USER` affect only subsequent outcomes and the authorities that
  intentionally use the active account?
- Are provider secrets kept behind backend resource/configuration boundaries?
- Are durable limits enforced by the database-backed authority?
- Do cache failure, stale state, missing relations, and malformed references
  fail safely under existing Daptin semantics?

## Scalability review checklist

- Resolve invariant request context once, not once per listed model, row, or
  deployment.
- Avoid N+1 resource and permission queries; reuse paginated/bulk helpers where
  the repository already has them.
- Keep network calls outside long-lived SQL transactions.
- Use bounded workers, explicit timeouts, and existing coordination primitives.
- Do not turn PubSub into a durable queue or an in-memory map into durable state.
- Preserve SQLite, PostgreSQL, and MySQL behavior by using established database
  abstractions.

## Tests must prove composition

Tests should demonstrate the invariant, not merely exercise new functions.

- Unit-test the permission or policy boundary, including denial and revocation.
- Test through Daptin resource APIs and relationship resources. Avoid raw SQL
  setup except for database-layer tests.
- Add end-to-end coverage when behavior crosses authentication, actions,
  permissions, metering, providers, batches, or transports.
- For shared operations, cover each meaningful entry path and assert that all
  converge on the same result.
- Assert denied requests do not reach downstream side effects.
- Assert ownership and metering records belong to the intended active user.
- Prefer existing test fixtures and helpers over a new parallel harness.
- Architecture tests that prevent SQL or lifecycle bypasses are intentional;
  do not work around them.

Use the narrowest relevant checks while developing, then run the broader suite
appropriate to the change. Common commands include:

```bash
go test ./server/<affected-package> -count=1
go test ./...
go vet ./server/<affected-package>
git diff --check
```

Real E2E suites are opt-in; follow the existing environment variable and helper
patterns in the neighboring tests.

## Documentation

- Teach users how to achieve the outcome with Daptin schema, JSON:API resources,
  relationships, permissions, and actions.
- Do not document database manipulation as an application workflow.
- Keep one canonical detailed guide for a concept. Other pages should state the
  local implication briefly and link to that guide.
- Document the product invariant and observable behavior, not incidental
  implementation details or historical compatibility behavior.
- Update public docs when an interface, configuration, permission requirement,
  or operational boundary changes.

## Scope and completion

Before declaring a change complete:

1. Review `git diff --stat` and `git diff --name-only` against the task's base.
2. Explain why every changed production file is necessary.
3. Remove incidental refactors and unrelated fixes.
4. Confirm that direct, action, scheduled, batch, and protocol paths have not
   diverged where they share behavior.
5. Confirm that identity, authorization, credentials, metering, and routing
   remain orthogonal.
6. Run relevant unit, integration, architecture, and E2E tests.
7. Run `git diff --check` and leave unrelated user changes untouched.

The preferred result is a small production diff with broad effect because it
uses Daptin's existing connected model.

## Code landmarks

Start investigation at these established locations rather than inventing a new
framework:

- `server/auth/auth.go` — session identity and group loading
- `server/permission/permission.go` — permission decisions
- `server/resource/dbresource.go` and `server/resource/dbmethods.go` — resource
  authority and lookup helpers
- `server/resource/resource_create.go`, `resource_update.go`, and
  `resource_delete.go` — canonical resource lifecycle
- `server/resource/handle_action.go` — action authorization, context, outcomes,
  and `SWITCH_USER`
- `server/resource/metering.go` — generic metering authority
- `server/resource/columns.go` — built-in system resource definitions
- `server/server.go` — dependency composition and runtime lifecycle
- `server/llm/architecture_test.go` and `server/llm_architecture_test.go` —
  examples of architectural guard tests
- `wiki/Core-Concepts.md`, `wiki/Permissions.md`, `wiki/Actions-Overview.md`, and
  `wiki/API-Metering.md` — user-facing concepts and supported workflows
