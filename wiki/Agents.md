# Durable agents

An agent run calls the configured LLM gateway, validates tool proposals, performs
Daptin resource operations or named actions, saves their results, and continues.
The initiating HTTP request returns a run reference. Closing the browser does
not stop the run. Any node can serve progress and controls.

## Configure an agent

Create an `agent` record as a maintainer through `/api/agent`. Its `config` is a
JSON object (or JSON text in a JSON:API client):

```json
{
  "instructions": "Find relevant artifacts, create an artifact, and verify it before finishing.",
  "model": "assistant",
  "toolset": {
    "resources": ["*"],
    "operations": ["list", "read", "create", "update", "delete", "relations"],
    "actions": ["*"],
    "tools": []
  },
  "limits": {
    "turns": 20,
    "tool_calls": 50,
    "seconds": 900,
    "output_tokens": 2048,
    "attempts": 3
  }
}
```

The model must have its `tools` capability enabled. Model execution, routing,
credentials and quota remain governed by the [LLM gateway](LLM-Providers.md).
`resources` accepts exact entity names or `*`; `actions` accepts exact
`entity.action` names or `*`. These selectors expose capabilities without granting
permissions. Use narrower selectors when an application needs a smaller toolset.

Resource discovery exposes columns, relationships and named-action inputs.
Tools use normal resource paths for paginated queries, record reads, creation,
versioned updates, deletion and relationship reads. Relationship writes use
resource update attributes or ordinary relationship-resource CRUD. Each update
requires `attributes.version` from a recent read. Creation assigns a persisted
logical-operation reference; caller-supplied create references are rejected.

Named actions use the existing action dispatcher, including its distinct action
and subject checks and trusted outcome behavior. Validate the targets and scope
of application action inputs. An action grant is not a blanket grant to models,
credentials or arbitrary records. See [Actions](Actions-Overview.md) and
[Permissions](Permissions.md).

For a stable application tool name, add a binding to `tools`:

```json
{
  "name": "verify_artifact",
  "description": "Verify an artifact created by this workflow",
  "entity": "artifact",
  "action": "verify",
  "parameters": {
    "type": "object",
    "properties": {"artifact_id": {"type": "string"}},
    "required": ["artifact_id"],
    "additionalProperties": false
  },
  "repeatable": true
}
```

Each run stores its agent configuration and invokes actions through the normal
Daptin action handler. Normal action, subject, and resource permission checks
apply. Restart Daptin processes after changing action definitions or schemas;
in a cluster, restart every node.

Arguments map directly to action inputs. The locally validated schema dialect is
the structural OpenAPI 3 subset: type, description, properties, required,
additionalProperties, items, enum, minimum, maximum, minLength, maxLength,
minItems, maxItems, pattern and nullable. Unsupported keywords and remote
references are rejected. Both local schema validation and normal action
validations apply.

The [example application schema](../examples/agent/schema.yaml) defines an
`artifact` resource and its verification action. All application behavior stays
in that schema; no application-specific Go code is required.

## Design an application workflow

The generic CRUD and action tools let an agent discover an unfamiliar application
and work with its existing records. Named bindings give frequently used actions
explicit input schemas, reducing mistakes such as confusing `reference_id` with
an action's `artifact_id` input. Both interfaces use the same permission checks.

Useful combinations include support triage that updates cases and creates a
report, data cleanup that checks relationships before deleting temporary rows,
and a report workflow that pauses for a decision or a future date. A paused run
can continue after a node replacement because its conversation and pending tool
result are persisted.

Put business requirements in application actions: approval flags, valid state
transitions, record ownership, and verification of the actual persisted result.
An `agent_decision` response supplies information to the model; it does not
enforce a purchase approval or grant update access. A `completed` run means the
model ended its conversation, not that every requested business condition holds.
Have a verification action inspect those conditions before a release action can
change business state.

A verification action can return structured findings such as `valid: false` and
missing requirements. The agent can then revise a draft through normal CRUD and
call the same verifier again. Keep an expected validation finding in the action
result; execution errors stop the run or require effect reconciliation.

Treat retrieved ticket text, documents, and tool results as untrusted data.
Tool selectors limit exposure; persisted permissions remain the access authority
even when both selectors are `*`. Test with the intended execution account,
including records belonging to another account.

Qualify the selected provider with a multi-turn tool workflow and process
replacement. Supporting a first tool call does not establish continuation
support. Opaque provider continuation state, including Gemini thought signatures,
is retained on each tool call in the persisted conversation and replayed through
the shared gateway. See the live qualification command in
[Go tests](../scripts/testing/GO_TESTS.md).

## Start and inspect

Give the execution account read, refer and execute access to the `agent` record,
plus access to the selected model and resource operations. Use normal persisted
usergroup relationships to grant team access. Agent definitions are maintained
by administrators by default.

```bash
curl -X POST "$BASE/action/agent/start" \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"attributes":{"agent_id":"AGENT_REFERENCE","messages":[{"role":"user","content":"Create and verify an artifact about the current project."}]}}'
```

The action response contains `reference_id` and `status`. Poll
`/api/agentrun/RUN_REFERENCE`. `state` contains the submitted configuration,
conversation, current operation, counters, final result and error. `agentstp`
and `agenttry` provide step/attempt evidence governed by normal entity and row
permissions. General caller writes to execution rows are denied by default;
use controls.

Run states are `queued`, `running`, `waiting_decision`, `unknown`, `completed`,
`cancelled` and `failed`. `queued` with `wake_at` is a persisted timed wait.
Rejected tool inputs and version conflicts are returned as tool errors so the
agent can correct them. Uncertain mutation effects still require reconciliation.
Models request a decision with `agent_decision` or a time with `agent_wait`.
Submit a decision through `/action/agentrun/decide`:

```json
{"attributes":{"agentrun_id":"RUN_REFERENCE","decision":{"approved":true}}}
```

Cancellation uses `/action/agentrun/cancel` with `agentrun_id`. Controls require
run read/execute access and their normal action/table permissions. A decision
becomes a tool result; it never selects an execution identity or grants access.
The run's original account remains active, even when another authorized account
supplies the decision. Current account groups and authentication version are
reloaded before dispatch. Authentication-version changes invalidate delegation.
Action-local transient privileges do not carry into the next independent tool.

## Recovery and effects

Daptin saves an operation intent before dispatch. Read-only tools may retry after
an interrupted attempt. Explicitly retryable gateway failures schedule a persisted
retry with bounded exponential delay and the same attempt limit. Mutations and generic named actions are not blindly
repeated. If the effect is uncertain, the run enters `unknown` and preserves its
attempt reference. Reconcile with `/action/agentrun/reconcile`:

```json
{"attributes":{"agentrun_id":"RUN_REFERENCE","attempt":"ATTEMPT_REFERENCE","result":{"verified_outcome":"application-specific result"}}}
```

Only reconcile after checking the actual resource or provider outcome. The result
is supplied to the model as the completed tool result. A late saved result is
retained rather than overwritten by reconciliation.

A maintainer may declare a named binding `repeatable: true` for an action that is
safe to repeat. For a mutating action that implements durable idempotency, set
`idempotency_input` to its dedicated input name. Daptin supplies the same logical
operation reference across recovery attempts; the model cannot choose that input.
The target action/provider must implement deduplication. This configuration alone
does not make an action idempotent. An action can partially commit around external
work, so the contract must cover its entire outcome chain.

Cancellation prevents further dispatch and signals active work where the invoked
performer supports cancellation. It does not undo effects. Late results remain
attempt evidence. A lost model response can cause another charged inference on
recovery. Daptin does not promise exactly-once external effects.

## Cluster and execution bounds

Every stateless node runs bounded workers. Renewable Olric locks coordinate live
ownership; version-checked resource writes and persisted claim tokens fence
checkpoints. A heartbeat renews ownership independently of model/tool execution.
PubSub messages are wake hints; periodic database scans recover lost messages.
Progress, deadlines, and waiting state live in the shared SQL database.
Decision waits and unresolved attempts expire at the run deadline. An Olric failure stops
new work requiring a lock and cancels work whose ownership cannot be renewed.

The default worker bound is four per node. Ownership leases last 30 seconds and
renew every five seconds. A replacement worker waits for persisted ownership to
expire and reconciles an unfinished attempt before proceeding. SQL transactions
are not held across controller inference calls; tool actions retain the transaction
boundaries of their existing performers.

Limits default to 20 model turns, 50 tool calls, 900 wall-clock seconds (including
waits), 2048 output tokens per inference, and three recovery attempts per logical
operation. Maximums are 100 turns, 1000 calls, 86400 seconds, 32768 output tokens
and five attempts. Encoded results/arguments are bounded to 1 MiB and run history to
4 MiB. Lists and relationships return at most 100 rows per page. Relationship tools
accept `page`; resource discovery returns `resources` and an optional `next_page`.
Action discovery identifies `instance_optional` and the required subject input. Timed waits must precede the run
deadline. Run-wide hard token/cost ceilings are not provided; customer quota and
usage continue through [API metering](API-Metering.md).

Arbitrary existing performers may not honor cancellation or have their own
network/transaction boundaries. Configure bounded application actions for long
external operations. Runtime shutdown stops dispatch, signals cancellation, and
drains agent workers before draining their gateway and Olric dependencies.
