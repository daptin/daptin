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
credentials and quota remain governed by the [LLM gateway](https://github.com/daptin/daptin/wiki/LLM-Providers).
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
credentials or arbitrary records. See [Actions](https://github.com/daptin/daptin/wiki/Actions-Overview) and
[Permissions](https://github.com/daptin/daptin/wiki/Permissions).

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

An administrator agent can create schema through the `world` resource, invoke
`world.restart_daptin`, and continue the same persisted run after the process
restarts. The restart action currently affects only the executing node. Shared
SQL schema definitions do not refresh another node's in-memory resources: that
node can still return `404` for a newly added entity, and a run handed to it can
receive an unknown-entity tool result. Restart each node before treating a
schema change as applied throughout the cluster. Schema uploads also write a
local file; restarting peers does not distribute that file.

Arguments map directly to action inputs. The locally validated schema dialect is
the structural OpenAPI 3 subset: type, description, properties, required,
additionalProperties, items, enum, minimum, maximum, minLength, maxLength,
minItems, maxItems, pattern and nullable. Unsupported keywords and remote
references are rejected. Both local schema validation and normal action
validations apply.

The [example application schema](https://github.com/daptin/daptin/blob/master/examples/agent/schema.yaml) defines an
`artifact` resource and its verification action. All application behavior stays
in that schema; no application-specific Go code is required.

## Run the example from start to finish

Start with an authenticated administrator token and a working `assistant` model
from the [LLM router walkthrough](https://github.com/daptin/daptin/wiki/Walkthrough-LLM-Router).
Enable `tools` in that model's `capabilities`, preserving any other capabilities.
Install the example schema in the server's schema folder and restart each node,
as described in [Schema definition](https://github.com/daptin/daptin/wiki/Schema-Definition).
Use the administrator for this first example; choose the intended execution
account and sharing grants before running an application workflow.

Save the configuration at the start of this guide as `agent-config.json`. With
`curl` and `jq` installed, create the definition:

```bash
export BASE=http://localhost:6336
# TOKEN is the administrator JWT obtained during sign-in.
jq -n --slurpfile config agent-config.json \
  '{data:{type:"agent",attributes:{name:"artifact-helper",config:$config[0]}}}' \
  > agent-create.json

AGENT_REF=$(curl --fail-with-body -sS "$BASE/api/agent" \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  --data-binary @agent-create.json | jq -er '.data.id')

jq -n --arg agent "$AGENT_REF" \
  '{attributes:{agent_id:$agent,messages:[{role:"user",content:"Create an artifact titled Agent demo with non-empty content explaining this workflow. Call artifact.verify on the saved record and report the verification result."}]}}' \
  > agent-start.json

RUN_REF=$(curl --fail-with-body -sS "$BASE/action/agent/start" \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  --data-binary @agent-start.json | jq -er '.[0].Attributes.reference_id')

curl --fail-with-body -sS "$BASE/api/agentrun/$RUN_REF" \
  -H "Authorization: Bearer $TOKEN" \
  | jq '.data.attributes | {status,state}'
```

Repeat the last request until the run reaches a terminal state or waits for a
decision. A completed run points to its final model step; that step identifies
the accepted attempt containing the answer:

```bash
STEP_REF=$(curl --fail-with-body -sS "$BASE/api/agentrun/$RUN_REF" \
  -H "Authorization: Bearer $TOKEN" | jq -er '.data.attributes.current_step_id')
TRY_REF=$(curl --fail-with-body -sS "$BASE/api/agentstp/$STEP_REF" \
  -H "Authorization: Bearer $TOKEN" | jq -er '.data.attributes.accepted_try_id')
curl --fail-with-body -sS "$BASE/api/agenttry/$TRY_REF" \
  -H "Authorization: Bearer $TOKEN" | jq '.data.attributes.result'
```

Verify the saved application data separately:

```bash
curl --fail-with-body -sS "$BASE/api/artifact?page[size]=25" \
  -H "Authorization: Bearer $TOKEN" | jq '.data'
```

The agent should create a record through ordinary CRUD, pass its public reference
as `artifact_id` to the named action, receive the verifier's result, and finish.
Inspect ordered `agentstp` records and their selected `agenttry.result` values
for the model's tool calls and returned results. The
control and recovery examples below cover decisions, cancellation and uncertain
effects. To manage the whole Daptin instance, retain the `*` selectors and run
as an administrator; selectors expose operations while the account's existing
permissions decide what each operation may do.

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
[Go tests](https://github.com/daptin/daptin/blob/master/scripts/testing/GO_TESTS.md#durable-agent-qualification).

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
`/api/agentrun/RUN_REFERENCE`. `config` and `input` hold the configuration snapshot and initial user messages,
written once at start. `state` is a small execution checkpoint containing counters,
accepted sequence, the next tool-call position, timing, budget progress and errors.
`current_step_id`, `active_try_id` and `source_step_id` identify execution records.

`agentstp` owns operation order and its stable logical-operation identity.
`accepted_try_id` selects exactly one attempt as that step's accepted result.
For a tool step, `source_step_id` and `call_id` identify the original call in an
accepted model response; arguments are not copied into each attempt.
`agenttry` owns attempt number, status and the complete result. Results and
conversation history are not copied into run checkpoints or step payloads.
All records use normal entity and row permissions. General caller writes to execution rows are denied by default;
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

## Execution limits and budgets

Limits live in the agent definition's `config.limits`. Effective values are copied
into each run at start; later edits to the definition affect subsequent runs.
Defaults are 20 model turns, 50 tool calls, 900 wall-clock seconds (including waits),
2048 output tokens per inference, and three recovery attempts per logical operation.
Maximums remain 100 turns, 1000 calls, 32768 output tokens and five attempts.
`seconds` accepts finite lifetimes beyond 24 hours, within the supported duration
and database timestamp range.

Optional limits add independent active-time and cumulative inference allowances:

```json
{
  "seconds": 2592000,
  "active_seconds": 3600,
  "tokens": 500000,
  "cost_micros": 2000000
}
```

These are fields inside `config.limits`: a 30-day lifetime, one hour of active
execution, 500,000 cumulative tokens and $2 of configured inference cost. Omit
an optional limit, or set it to zero, to leave that allowance disabled. Negative
values are rejected. There is no separate policy resource to configure.

To apply these limits to the tutorial, update the saved configuration before
creating the agent, then use the same create/start commands above:

```bash
jq '.limits += {seconds:2592000, active_seconds:3600, tokens:500000, cost_micros:2000000}' \
  agent-config.json > agent-config-bounded.json
mv agent-config-bounded.json agent-config.json
```

Configure USD-micro prices on each eligible deployment first, following
[LLM deployment pricing](https://github.com/daptin/daptin/wiki/LLM-Providers).
For example, `{"input_tokens":1000000,"output_tokens":2000000}` means $1 per
million input tokens and $2 per million output tokens. Omit `cost_micros` to use
only time/token bounds when a deployment has no configured prices.

After starting a run, inspect the limits, remaining budget and result through the
normal resource API:

```bash
curl --fail-with-body -sS "$BASE/api/agentrun/$RUN_REF" \
  -H "Authorization: Bearer $TOKEN" \
  | jq '.data.attributes | {status,deadline_at,limits:.config.limits,current_step_id,active_try_id,state}'
```

Absolute lifetime includes queueing and all waits. Active time covers inference
(including gateway retries/backoff) and tool execution. Persisted timers, approval
waits, worker queueing and scheduled agent retry delays do not consume active time.
For example, a five-day approval wait consumes five days of lifetime and no active
allowance. Timed waits must precede the absolute deadline, independently of the
remaining active allowance. Execution stops when either time allowance is exhausted.
Application performers must honor cancellation for the execution deadline to stop
an operation already in progress.

Token and cost ceilings apply cumulatively to inference performed by the run,
including synchronous LLM action outcomes invoked by its tools. They are separate
from account quotas. `SWITCH_USER` changes the active account for authorization and
account metering while preserving the enclosing run budget. Work separately queued
for later execution is not automatically part of the originating run's budget.
Total tokens include input plus output; cached input and reasoning subcategories
are not added again.

Daptin automatically uses existing `api_quota` buckets for enabled run ceilings and
links inference `api_usage` to the executing `agenttry`. No additional budget rows
are needed for runs without ceilings, and users do not create these rows manually.
Before inference, shared metering reserves exposure for the gateway's complete
retry/fallback plan. Settlement releases unused reservation and records aggregate
usage across attempts. Admission can reject a request even when some allowance
remains, if that allowance cannot cover the proposed inference.

Budgeted inference currently supports text chat and function tools. Reservations
use UTF-8 payload bytes plus message/tool framing allowance as a conservative token
estimate, and the normalized completion-token cap. Media and native external tools
are rejected because their exposure cannot be bounded this way. These are bounds
against configured gateway accounting, not a guarantee about undisclosed provider
billing or hidden tokenization overhead.

All cost budgets and deployment prices use USD micros: 1,000,000 micros is $1.
A cost ceiling requires explicit `input_tokens` and `output_tokens` prices, or a
single `total_tokens` price, on every possible route. Explicit zero prices mean
free; missing prices are rejected. Mixed or category-specific price schedules are
not accepted for cost-bounded runs. Each admission retains its pricing revision and
route quotes; changing prices does not reprice earlier usage. Missing provider
usage is conservatively settled using the reservation estimate and marked
`usage_estimated` in the usage metadata.

Normal `agentrun` reads expose effective limits in `config.limits`, accumulated
`active_ms`, the current interval's `active_since`, `active_remaining_ms` when
configured, and `progress_at`. Budget snapshots under `state.budget` contain
`maximum`, `consumed`, `reserved` and `remaining` for enabled metrics. Snapshots
refresh at execution checkpoints; `api_quota` exposes current reservations during
inference without copying the conversation on every heartbeat. The top-level
`deadline_at` is the absolute deadline. Failures include `state.error_code`, such as
`lifetime_exceeded`, `active_time_exceeded`, `turns_exceeded`,
`tool_calls_exceeded`, `attempts_exceeded`, `budget_unpriced`, `budget_unbounded`,
or `quota_exceeded`; recorded attempt results remain available. Account limits and usage continue through
[API metering](https://github.com/daptin/daptin/wiki/API-Metering).

## Stored history and inference context

Accepted steps and attempts are the authoritative conversation history. Before
inference, the worker reconstructs system instructions, initial user input, and
ordered accepted model/tool results. Tool-call identifiers and provider continuation
metadata remain attached to their original messages. Other attempts remain
inspectable evidence and are not replayed into the conversation.

Claims, heartbeats and tool dispatch read projected control/operation fields;
they do not load the whole conversation. Configuration is loaded once per worker
execution slice, and history is read in ordered batches only for model inference.
Step acceptance and checkpoint advancement commit atomically.

There is no aggregate 4 MiB history limit or 1 MiB stored-result limit. The 1 MiB
tool-argument admission bound remains separate from result retention. Context sent
to a provider remains subject to that provider/gateway's constraints and the run's
inference budgets. A rejected inference does not delete or truncate stored history.
Gateway usage capture retains its existing independent policy; it is not the agent
conversation authority.

This schema uses separate run input/configuration, step selections and attempt
results. It does not include readers or migration for the older embedded-history
format.

Lists and relationships return at most 100 rows per page. Relationship tools accept
`page`; resource discovery returns `resources` and an optional `next_page`. Action
discovery identifies `instance_optional` and the required subject input.

Arbitrary existing performers may not honor cancellation or have their own
network/transaction boundaries. Configure bounded application actions for long
external operations. Runtime shutdown stops dispatch, signals cancellation, and
drains agent workers before draining their gateway and Olric dependencies.
