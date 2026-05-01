# API Metering Technical KT

This page explains the API metering, quota, and credit-charging layer internally. It is for maintainers who need to extend, debug, or review the implementation.

For operator usage, see [[API-Metering]].

## Design Boundary

API metering is a Daptin-native capability layered on top of existing entities, middleware, action handling, LLM endpoints, and Olric.

It does not introduce a separate billing service, a second user model, or an in-memory counter path. Durable usage data is stored in Daptin tables. Short-window rate limiting uses the existing Olric distributed cache so limits work across a cluster.

The system has three responsibilities:

| Responsibility | Storage or path | Notes |
|----------------|-----------------|-------|
| Plan and membership model | `api_plan`, `api_member` | Defines quota limits and assigns a user to a plan |
| Durable usage accounting | `api_usage`, `api_quota` | Stores per-call usage rows and per-period counters |
| Cluster-wide short-window throttling | `resource.OlricCache` | Uses atomic Olric counters with one-minute TTL |

## Files

| File | Responsibility |
|------|----------------|
| `server/table_info/tableinfo.go` | Adds `MeteringConfig` and `TableInfo.Metering` schema support |
| `server/resource/columns.go` | Declares `api_plan`, `api_member`, `api_usage`, `api_quota` and their non-user relations |
| `server/resource/metering.go` | Metering service, quota checks, Olric rate-limit checks, usage recording, post-metering action dispatch |
| `server/resource/metering_expr.go` | Safe cost expression evaluation through Goja |
| `server/resource/metering_middleware.go` | CRUD middleware integration |
| `server/resource/handle_action.go` | Action preflight and usage recording |
| `server/endpoint_llm.go` | LLM chat, completion, embedding metering |
| `server/utils.go` | Middleware registration |
| `server/resource/metering_expr_test.go` | Focused tests for cost expression and quota helper behavior |

## Entity Model

The system tables are declared in `server/resource/columns.go`.

| Table | Purpose | Main columns |
|-------|---------|--------------|
| `api_plan` | A metering plan | `name`, `requests_per_period`, `compute_units_per_period`, `rate_limit_per_minute`, `price_monthly_cents`, `overage_price_micros`, `meter_type`, `quota_enforce_mode`, `metadata` |
| `api_member` | A user's active plan membership | `status`, `period_start`, `period_end`, `metadata` |
| `api_usage` | Immutable-ish per-request usage log | `endpoint`, `method`, `entity_type`, `action_name`, `request_type`, `status_code`, `latency_ms`, `request_bytes`, `response_bytes`, `cost_units`, `cost_micros`, `meter_type`, `metadata`, `error_message` |
| `api_quota` | Per-member period aggregate | `period_start`, `period_end`, `request_count`, `compute_units`, `bytes_used` |

Relations explicitly declared by this feature:

```text
api_member has_one api_plan
api_usage  has_one api_plan
api_usage  has_one api_member
api_quota  has_one api_plan
api_quota  has_one api_member
```

Do not add explicit `belongs_to user_account` or `has_one user_account` relations for these tables. Daptin already adds the default user-account ownership relation for normal non-join, non-audit tables during table preparation. The metering service uses the resulting `user_account_id` column, but the schema must not duplicate that relation.

## TableInfo Metering Configuration

`TableInfo` now has:

```go
type MeteringConfig struct {
    Enabled            bool                      `json:"enabled,omitempty"`
    CostExpr           string                    `json:"cost_expr,omitempty"`
    MeterType          string                    `json:"meter_type,omitempty"`
    PostMeteringAction string                    `json:"post_metering_action,omitempty"`
    EnforceMode        string                    `json:"enforce_mode,omitempty"`
    OnActions          map[string]MeteringConfig `json:"on_actions,omitempty"`
}
```

`TableInfo.Metering` is optional. If absent or disabled, CRUD/action metering is skipped.

Defaults are applied in `normalizeMeteringConfig`:

| Field | Default |
|-------|---------|
| `cost_expr` | `1` |
| `meter_type` | `requests` |
| `enforce_mode` | `hard` |

Action-specific overrides are resolved by `meteringConfigForAction`. An action override inherits missing fields from the table-level config.

## Request Lifecycle

### CRUD Path

CRUD metering is registered in `server/utils.go` through `NewMeteringMiddleware`.

Before phase:

```text
MeteringMiddleware.InterceptBefore
  shouldMeterResource
  sessionUserFromAPIRequest
  MeteringService.Preflight
    normalize config
    find active api_member for user
    load api_plan
    ensure api_quota row
    check period quota
    check Olric one-minute rate limit
```

After phase:

```text
MeteringMiddleware.InterceptAfter
  MeteringService.Record
    preflight without consuming rate-limit again when needed
    evaluate cost expression
    insert api_usage
    increment api_quota
    invoke optional post_metering_action
```

The after path intentionally does not decrement or compensate the Olric rate-limit counter. The short-window counter represents accepted attempts, while `api_usage` records completed attempts that reach the after middleware.

### Action Path

`DbResource.HandleActionRequest` checks metering after subject and request metadata are prepared and before performers execute.

```text
HandleActionRequest
  meteringConfigForAction
  MeteringService.Preflight
  execute performer/outcomes
  MeteringService.Record
```

Action metadata recorded in `api_usage.metadata` includes:

| Key | Meaning |
|-----|---------|
| `action_type` | Entity type the action ran on |
| `action_name` | Action name |
| `outcome_count` | Number of action responses |
| `response_types` | Action response type list |

Internal post-metering actions are marked with `WithMeteringInternal`. Middleware and action metering skip those requests to avoid recursive usage rows.

### LLM Path

LLM metering is wired directly in `server/endpoint_llm.go` for:

| Endpoint behavior | Request type |
|-------------------|--------------|
| Chat completion | `llm_chat` |
| Streaming chat completion | `llm_chat` |
| Text completion compatibility handler | `llm_completion` |
| Embeddings | `llm_embedding` |

The default LLM config is:

```go
MeteringConfig{
    Enabled:   true,
    CostExpr:  "response.usage.total_tokens",
    MeterType: "compute_units",
}
```

Streaming records usage only when a final usage block is seen. If the provider does not return usage for a stream, no `api_usage` row is written for that stream.

## Metering Service Details

### Membership Lookup

`findActiveMember` reads:

```text
api_member where user_account_id = session user id and status = active order by id desc limit 1
```

If there is no active membership, metering is a no-op. This keeps old deployments and users without a plan working unless a membership is assigned.

### Plan Lookup

`findPlan` loads `api_plan` by the foreign key from `api_member.api_plan_id`.

### Quota Row Creation

`ensureQuota` uses the member's `period_start` as the quota period key. If no matching `api_quota` row exists, it inserts one with zero counters.

### Quota Enforcement

`checkMeteringQuota` currently enforces:

| Meter type | Enforced columns |
|------------|------------------|
| `requests` | `requests_per_period` through `request_count` |
| `compute_units` | `requests_per_period` and `compute_units_per_period` |

`-1` means unlimited.

Hard quota denial returns a JSON:API HTTP error with status `402` and code/message `insufficient_quota`.

### Rate Limit Enforcement

`checkMeteringRateLimit` enforces `api_plan.rate_limit_per_minute`.

Important implementation rules:

- `-1` means unlimited.
- The key is plan-wide per user per minute, not per endpoint.
- It uses `resource.OlricCache`.
- It uses `OlricCache.Put(..., olric.NX(), olric.EX(time.Minute))` to initialize the key.
- It uses `OlricCache.Incr(..., 1)` for atomic cluster-wide increments.
- If Olric is unavailable, it logs a warning and allows the request. Durable quota still applies.

Key shape:

```text
api-rate-limit:{user_account_id}:{api_plan_id}:{yyyyMMddHHmm}
```

Hard rate-limit denial returns status `429` and code/message `rate_limit_exceeded`.

### Cost Evaluation

`EvaluateMeteringCost` evaluates a JavaScript expression with Goja. The expression receives:

| Name | Contents |
|------|----------|
| `request` | Endpoint and method context |
| `response` | Response metadata supplied by the caller |
| `metadata` | Caller-specific metadata |
| `user` | User id and reference id |
| `plan` | Loaded `api_plan` row |

The return value is converted to `int64`.

Rules:

- Empty expression defaults to `1`.
- Fractional values round up.
- Negative values clamp to `0`.
- Unsupported values return an error; the usage row is still written with `cost_units=0` and `error_message` set.

### Usage Insert

`Record` inserts directly into `api_usage` with `statementbuilder.Squirrel`. It sets baseline Daptin columns:

```text
reference_id
permission
created_at
updated_at
user_account_id
```

It then resolves the inserted usage id through `GetReferenceIdToIdWithTransaction` and increments the quota row.

### Post-Metering Action

If `post_metering_action` is configured, it must use:

```text
entity:action
```

Example:

```text
credit:deduct_credits
```

The service calls `HandleActionRequest` for that entity/action with an internal request context. The action receives attributes such as:

| Attribute | Meaning |
|-----------|---------|
| `user_account_id` | User reference id |
| `api_usage_id` | Internal id of the usage row |
| `api_plan_id` | Internal plan id |
| `api_member_id` | Internal membership id |
| `cost_units` | Evaluated cost |
| `cost_micros` | `cost_units * overage_price_micros` |
| `meter_type` | `requests` or `compute_units` |
| `endpoint` | Request path |
| `entity_type` | CRUD/action entity |
| `action_name` | Action name, when applicable |
| `metadata` | Metering metadata |
| `metering_internal` | Always true |

The post-metering action is best-effort. Errors are logged and do not roll back the primary request after usage has been recorded.

## System Table Recursion Guard

`api_plan`, `api_member`, `api_usage`, and `api_quota` are metering system tables. CRUD middleware skips them to avoid recording metering operations as metered usage.

Internal post-metering action calls also set a request context flag so action metering skips recursive billing.

## Failure Behavior

| Failure | Behavior |
|---------|----------|
| No metering config | Allow request, no usage |
| No authenticated user | Allow request, no usage |
| No active `api_member` | Allow request, no usage |
| Plan not found | Return error |
| Quota exceeded in hard mode | Deny with 402 |
| Rate limit exceeded in hard mode | Deny with 429 |
| Olric unavailable for rate limit | Log warning, allow |
| Cost expression fails | Record usage with zero cost and error message |
| Usage insert fails | Return/log error from caller path |
| Post-metering action fails | Log error, do not fail completed request |

## Extension Points

### Add a New Meter Type

1. Add the plan column if the meter needs a new limit.
2. Add the quota column if it needs durable aggregation.
3. Extend `checkMeteringQuota`.
4. Extend `incrementQuota`.
5. Add focused tests.

### Meter a New Endpoint Family

1. Build a `MeteringContext`.
2. Call `MeteringService.Preflight` before expensive work.
3. Call `MeteringService.Record` after work completes.
4. Include enough `metadata` and `response` data for the configured `cost_expr`.
5. Avoid calling `Preflight` twice with rate-limit consumption for the same request.

### Add Credit Charging

Credit charging belongs in a Daptin action, not inside the metering service. Configure `post_metering_action` to call that action after usage is recorded. This keeps the metering layer generic and lets each deployment define credit semantics in normal Daptin action logic.

## Debugging Checklist

1. Confirm the target entity has `metering.enabled=true` in `world_schema_json`.
2. Confirm the user is authenticated.
3. Confirm the user has one active `api_member`.
4. Confirm the active member points to an `api_plan`.
5. Check `api_quota` for period counters.
6. Check `api_usage` for usage rows and `error_message`.
7. For per-minute denials, check Olric availability and the plan's `rate_limit_per_minute`.
8. For missing LLM usage rows, confirm the provider returned `usage.total_tokens`.
9. For duplicate or recursive usage rows, check `IsMeteringSystemTable` and the internal context flag.

## Verification

Focused verification commands used during implementation:

```bash
go test ./server/resource ./server -run 'TestEvaluateMeteringCost|TestCheckMeteringQuota|TestNonExistent'
go test ./server/resource -run 'TestEvaluateMeteringCost|TestCheckMeteringQuota'
go test . -run '^TestServerApis$' -count=1 -v
go test ./...
```

The root integration test uses fixed local ports and a long-lived embedded server. If rerunning broad tests repeatedly in the same shell, clear any old test server process or run the root test in a fresh process before interpreting failures.
