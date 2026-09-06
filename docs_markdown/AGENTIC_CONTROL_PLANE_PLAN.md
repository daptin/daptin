# Agentic Daptin Control Plane Plan

Status: proposal

This document captures the proposed shape for a native agentic control plane in
Daptin. The goal is to let Daptin inspect, configure, and maintain itself when
an LLM provider is configured, without adding a second orchestration runtime or
bypassing Daptin's existing permission and action model.

## Summary

The feature should be a Daptin-native action subsystem:

```text
HTTP action or task scheduler
  -> agent.run action performer
  -> bounded LLM/tool loop
  -> Daptin actions, CRUD, config, schema, and metadata APIs
  -> persisted audit trace
```

It should not start as a parallel agent framework, worker pool, external daemon,
or generic LangChain-style runtime. Existing Daptin primitives already cover the
important platform concerns:

- `llm_provider` and `$llm.chat` provide model access.
- `action` rows and action performers provide command execution.
- action and row permissions decide who can do what.
- `task` already schedules recurring action runs.
- `credential`, `config`, `world`, `integration`, `smd`, `api_usage`,
  `/statistics`, and related system entities expose live runtime state.
- existing audit and metering patterns can be extended for traceability.

The missing piece is a bounded loop that can reason over those primitives and
invoke them safely.

## Design Principles

1. Use Daptin as the control boundary.

   The model must not mutate the database directly. It should request typed tool
   calls, and those tool calls should execute through Daptin's permission-aware
   CRUD, action, config, relation, and metadata surfaces.

2. Keep the MVP serial.

   A serial loop is enough for diagnostics, planning, and controlled apply. A
   parallel execution model would require leases, locks, child runs, step DAGs,
   conflict handling, cancellation, mutation ordering, and idempotency rules.
   That is a different feature.

3. Reuse the existing scheduler.

   Scheduled maintenance should be a normal `task` row that invokes `agent.run`.
   No new cron or background scheduler should be introduced for the MVP.

4. Persist traces, not "agent state" for its own sake.

   `agent_run` and `agent_step` exist to make every LLM/tool decision auditable,
   replayable enough for debugging, and visible to admins. They do not imply
   parallel agents.

5. Gate high-risk operations.

   Shell execution, raw SQL, table deletion, column deletion, credential reads,
   restart, and schema-destructive changes must be denied by default or require
   explicit admin approval.

## Current Framework Fit

The feature can fit cleanly into existing Daptin ownership boundaries:

- LLM routing is already exposed through OpenAI-compatible endpoints in
  `server/endpoint_llm.go`.
- LLM use inside actions already exists through `$llm.chat` and
  `$llm.embedding` performers registered from `server/action_provider`.
- action execution already loads the session user, validates action inputs,
  checks action permissions, and dispatches to action performers in
  `server/resource/handle_action.go`.
- task scheduling already runs actions as a selected user in
  `server/resource/task_scheduler.go`.

The agent should therefore be implemented as one or more action performers,
plus a small set of system entities for profile, run, and step records.

## Proposed System Entities

### `agent_profile`

Stores reusable agent configuration.

Suggested fields:

- `name`
- `model`
- `system_prompt`
- `mode`: `readonly`, `propose`, `apply`
- `max_steps`
- `max_tokens`
- `temperature`
- `tool_allowlist`
- `approval_policy`
- `enable`

Notes:

- Profiles should be admin-owned by default.
- A profile defines what the loop is allowed to do, not what a user is allowed
  to bypass. User permissions still apply at execution time.

### `agent_run`

Represents one invocation of the loop.

Suggested fields:

- `agent_profile_id`
- `resumed_from_run_id`
- `prompt`
- `resume_message`
- `mode`
- `status`: `queued`, `running`, `waiting_for_approval`,
  `waiting_for_input`, `cancel_requested`, `succeeded`, `failed`,
  `cancelled`
- `requested_by`
- `cancel_requested_by`
- `cancel_requested_at`
- `plan_json`
- `result_json`
- `error_json`
- `started_at`
- `finished_at`
- token and cost metadata where available

Notes:

- A run can be created by an HTTP action call or by an existing scheduled task.
- The run should contain the final outcome and enough metadata to diagnose
  failures without reading server logs.
- Cancellation is persisted on the run. It is not an in-memory signal.
- Resumption should preserve the previous run by creating a linked continuation
  run through `resumed_from_run_id`.

### `agent_step`

Ordered trace inside one run.

Suggested fields:

- `agent_run_id`
- `sequence`
- `kind`: `llm`, `tool_call`, `tool_result`, `approval`,
  `input_request`, `cancel`, `resume`, `final`, `error`
- `tool_name`
- `input_json`
- `output_json`
- `error_json`
- token and latency metadata where available

Notes:

- `sequence` should be a strict integer order for the MVP.
- Future parallel read-only discovery can add `parent_step_id` or
  `child_run_id`, but the MVP should not depend on those fields.

## Proposed Actions

### `agent.run`

Runs a bounded serial LLM/tool loop.

Inputs:

- `profile_id` or `profile_name`
- `prompt`
- optional `mode` override, restricted by the profile
- optional `max_steps` override, capped by the profile
- optional `approved_plan_json` for apply mode

Behavior:

1. Create an `agent_run`.
2. Load the selected `agent_profile`.
3. Build a live Daptin tool manifest from the current runtime.
4. Call the configured LLM.
5. Execute allowed tool calls through Daptin primitives.
6. Persist each LLM call, tool call, result, and error as `agent_step`.
7. Stop on final answer, max steps, error, cancellation, or approval required.
8. Persist final result into `agent_run`.

### `agent.plan`

Optional wrapper around `agent.run` in `propose` mode.

It should return a structured plan with:

- observed state
- intended operations
- risk level
- required approvals
- validation checks
- rollback notes where possible

No mutation should happen in this mode.

### `agent.apply`

Applies an approved plan through allowlisted Daptin operations.

This should be separate from `agent.plan` so an admin can review the exact plan
before mutation. `agent.apply` should be serial and should stop on the first
failed required operation unless the plan explicitly marks an operation as
optional.

### `agent.cancel`

Required action.

Cancels a run by updating the corresponding `agent_run` state.

Inputs:

- `run_id`
- optional `reason`

Behavior:

1. Load the target `agent_run`.
2. Verify the caller can control the run.
3. Update `agent_run.status` to `cancel_requested`.
4. Set `cancel_requested_by` and `cancel_requested_at`.
5. Append an `agent_step` with kind `cancel`.

The active loop must reload the run state before each LLM call, before each
tool call, and after each tool call. If it observes `cancel_requested` or
`cancelled`, it must stop, append a final cancellation step, and persist
`agent_run.status = cancelled`.

This makes cancellation a normal Daptin state transition that works across HTTP
requests, scheduled runs, and future longer-running implementations.

### `agent.resume`

Required action.

Resumes from a prior run with or without a new user message.

Inputs:

- `run_id`
- optional `message`
- optional `mode` override, restricted by the profile and prior run
- optional `approved_plan_json` when resuming from approval

Behavior:

1. Load the previous `agent_run` and its ordered `agent_step` trace.
2. Verify the caller can control or continue the run.
3. Reject resume for completed successful runs unless an explicit continuation
   flag is provided.
4. Create a new `agent_run` with `resumed_from_run_id` pointing to the prior run.
5. Carry forward the profile, mode, prompt, plan, and relevant prior context.
6. Add `resume_message` when the caller provides a new message.
7. Continue the same bounded LLM/tool loop from the reconstructed context.

Resume should not mutate the old run back to `running`. The old run remains an
auditable fact; the resumed run is a linked continuation.

## Tool Model

Tools should be typed Daptin operations, not arbitrary code execution.

### Read-only tools for MVP

- inspect entity definitions from `world`
- inspect actions and action schemas
- inspect relations using explicit relation metadata
- inspect permissions for a table, action, row, user, or group
- inspect tasks
- inspect integrations and installed provider operations
- inspect LLM providers without exposing secrets
- inspect state-machine definitions
- inspect metering, usage, and health/statistics surfaces
- read public or non-secret config values

### Controlled mutation tools for later phases

- create or update rows through normal CRUD paths
- call existing Daptin actions through the action handler
- create or update `task` rows
- create or update `llm_provider` rows without exposing credential contents
- update config through existing config mechanisms
- create or update state-machine definitions
- install or configure integrations through existing integration actions

### High-risk tools

These should be denied by default or require explicit admin approval:

- `command.execute`
- `$transaction.query`
- table deletion
- column deletion
- restart
- raw credential read
- raw secret output
- destructive schema changes
- broad permission changes

## Modes

### `readonly`

The agent can inspect and explain but cannot mutate anything.

Primary use cases:

- explain why a user cannot access a table/action
- inspect current LLM provider setup
- summarize tasks, integrations, metering, or runtime state
- identify stale or risky configuration

### `propose`

The agent can inspect and return a structured plan, but cannot apply it.

Primary use cases:

- propose schema/action changes for an app idea
- propose permission changes with expected impact
- propose task setup
- propose integration setup

### `apply`

The agent can execute an approved plan through allowlisted tools.

Primary use cases:

- create safe configuration records
- create scheduled tasks
- update non-secret config
- call existing maintenance actions

Apply mode should always be serial in the MVP.

## Scheduling

Scheduled agents should use the existing `task` table.

Example shape:

```text
task.entity_name = agent_profile
task.action_name = agent.run
task.schedule = @hourly
task.attributes = {
  "profile_name": "hourly-health-check",
  "prompt": "Inspect health, failed tasks, metering anomalies, and integration failures.",
  "mode": "readonly"
}
```

The task scheduler already executes actions as a selected user. That user
context should be preserved and passed into the agent loop. The agent does not
need its own scheduler.

## Parallelism Decision

The MVP should not support parallel running agents.

Parallelism changes the paradigm from a simple tool/API loop to an orchestration
runtime. It introduces unresolved questions:

- How are locks or leases acquired?
- Can two runs mutate the same table, action, config, or permission at once?
- How are child-run results merged?
- Which operations are idempotent?
- How are partial failures represented?
- How are transaction boundaries handled across branches?

If parallelism is added later, it should start with read-only discovery only.
Mutation should remain serial unless Daptin adds explicit conflict and locking
semantics for agent operations.

## Execution Contract

Each loop iteration should produce one of these outcomes:

- final answer
- tool call
- approval request
- input request
- cancellation
- error

Tool calls should use structured JSON:

```json
{
  "tool": "daptin.permissions.explain",
  "input": {
    "entity": "project",
    "user_reference_id": "..."
  }
}
```

Tool results should also be structured:

```json
{
  "ok": true,
  "data": {},
  "warnings": [],
  "redactions": []
}
```

The loop must enforce:

- maximum step count
- maximum token/cost budget where metering data is available
- profile tool allowlist
- user permissions
- mode restrictions
- approval requirements for high-risk operations
- persisted cancellation checks before each LLM/tool boundary
- resumable checkpoints after each persisted step
- redaction before persisting secrets or credential-like data

## Phase Plan

### Phase 1: Read-only diagnostics

Implement:

- `agent_profile`
- `agent_run`
- `agent_step`
- `agent.run` in `readonly` mode
- `agent.cancel`
- `agent.resume`
- read-only tool registry
- step persistence
- persisted run-state checks inside the loop
- max-step guard
- redaction helpers

Acceptance:

- An admin can ask why a user/action/table access path fails.
- The run persists all LLM and tool steps.
- Updating a running run to `cancel_requested` causes the loop to stop and mark
  the run `cancelled` at the next loop boundary.
- A cancelled, failed, waiting-for-input, or waiting-for-approval run can be
  resumed into a linked continuation run, with or without a new user message.
- No mutation tools exist in the registry.
- Scheduled readonly execution works through `task`.

### Phase 2: Propose mode

Implement:

- structured plan output
- risk classification
- required approvals
- validation checks
- plan persistence in `agent_run.plan_json`

Acceptance:

- The agent can propose Daptin-native changes without applying them.
- Plans identify exact Daptin entities/actions/config surfaces involved.
- The plan distinguishes proven current state from proposed changes.

### Phase 3: Apply approved plans

Implement:

- `agent.apply`
- allowlisted mutation tools
- approval validation
- serial execution
- applied result persistence
- stop-on-failure semantics

Acceptance:

- The agent can apply a reviewed plan through Daptin primitives.
- Existing action and row permissions still govern execution.
- High-risk tools remain denied or approval-gated.
- Failed apply runs leave a clear trace.

### Phase 4: Scheduled maintenance profiles

Implement:

- documented examples for scheduled readonly/propose runs using `task`
- health/config/integration inspection profiles

Acceptance:

- No new scheduler exists.
- Admins can create recurring agent checks through the existing task model.

### Phase 5: Limited read-only parallel discovery

Only consider this if serial discovery becomes too slow.

Scope:

- read-only tools only
- child steps or child runs for independent inspections
- no mutation in parallel branches

Acceptance:

- Parallel branches cannot mutate Daptin state.
- Results merge into a single ordered run trace.

## Non-goals

- No generic external agent framework in the MVP.
- No model access to raw SQL by default.
- No model access to shell execution by default.
- No separate scheduler.
- No parallel mutation.
- No permission bypass for admin convenience.
- No secret exfiltration through tool outputs or persisted traces.

## Open Questions

- Should `agent_run` be an entity users can create directly, or should all runs
  be created through `agent.run`?
- Should apply plans be immutable once approved?
- What is the smallest useful approval representation in Daptin's existing
  action model?
- Which existing config values are safe to expose to read-only tools?
- Should token/cost limits use existing LLM metering records directly or a
  separate per-run budget field?
- How should long-running HTTP-triggered runs behave if the client disconnects?
- Which run statuses are resumable by default?
- Should direct writes to `agent_run.status = cancel_requested` be allowed, or
  should cancellation always go through `agent.cancel` so permission checks and
  audit steps are guaranteed?
