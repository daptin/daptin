# API Metering Technical KT

This page describes the metering implementation introduced with Daptin
`v0.13.0`. For operator setup, see [[API-Metering]].

## Ownership and files

Metering remains a Daptin resource concern:

| File | Responsibility |
|---|---|
| `server/table_info/tableinfo.go` | `MeteringConfig` and `TableInfo.Metering` |
| `server/resource/columns.go` | Canonical metering resource definitions and relations |
| `server/resource/metering.go` | Admission, reservations, completion, cancellation, expiry, and post-metering actions |
| `server/resource/metering_expr.go` | Bounded expression evaluation |
| `server/resource/metering_middleware.go` | CRUD lifecycle integration |
| `server/resource/handle_action.go` | Action lifecycle integration |
| `server/llm/ports.go` | LLM authorization and metering adapter |

There is one durable ledger (`api_usage`) and one quota authority
(`api_quota`). Olric protects LLM deployments but is not the authority for
customer quota enforcement.

## Entity model

| Resource | Important fields |
|---|---|
| `api_plan` | `name`, `limits`, `price_monthly_cents`, `metadata` |
| `api_member` | `status`, `period_start`, `period_end`, `metadata`, `api_plan_id` |
| `api_usage` | request identity, lifecycle state, reservation data, final `measures`, metadata, terminal status |
| `api_quota` | `bucket_key`, metric/window bounds, `maximum`, `reserved`, `consumed` |

`api_plan.limits` is a JSON array of `{metric, window, maximum, mode}` values.
Supported windows are minute, hour, day, month, and member period. Supported
modes are hard and soft.

## Configuration resolution

```go
type MeteringConfig struct {
    Enabled            bool
    CostExpr           string
    MeterType          string
    PostMeteringAction string
    OnActions          map[string]MeteringConfig
}
```

`MeteringConfigForAction` resolves an action override and inherits its missing
cost expression, meter type, and post-metering action from the resource-level
configuration. There is no separate LLM metering configuration store.

## Lifecycle

All entry points use the same state machine:

```text
Admit -> held -> Complete  -> completed
               Cancel    -> cancelled
               recovery  -> expired
```

Admission:

1. Normalize estimated named measures and always include `requests`.
2. Lock metering state for the user.
3. Select the newest active membership and its plan.
4. Resolve each plan limit into a deterministic metric/window bucket.
5. Reserve hard-limit amounts transactionally.
6. Create an `api_usage` row with a unique request ID and reservation token.

Completion:

1. Lock and reload the held usage row.
2. Normalize final measures and evaluate `cost_expr` only when needed.
3. Release reserved amounts and increment consumed amounts.
4. Persist final measures, status, metadata, and terminal time.
5. Invoke the optional post-metering action inside the owning transaction.

Cancellation and expiry release reservations exactly once. Repeated terminal
operations are rejected or resolve idempotently through the request identity.

## LLM path

`server/llm/ports.go` adapts gateway admission and terminalization to
`MeteringService`. It uses the `llm_model` resource's `invoke` configuration and
records `entity_type=llm_model` with an operation-derived request type.

The gateway supplies estimated measures at admission and canonical final
measures at completion or cancellation. This includes token categories,
fixed-point `cost_micros`, attempts, cache status, provider status, and
operation-specific supplemental measures. Streaming uses the same reservation;
it does not open a database transaction for the lifetime of the upstream stream.

## Concurrency and failure behavior

- Database row locking serializes quota changes for the same user and bucket.
- Hard-limit authority fails closed when the database operation fails.
- Request IDs and reservation tokens are unique.
- Quota arithmetic is checked for overflow and negative measures are rejected.
- Recovery expires abandoned held reservations after their deadline.
- Internal post-metering actions are marked with `WithMeteringInternal` to
  prevent recursive charging.
- LLM max concurrency, RPM, and TPM counters use Olric and are intentionally
  separate from durable plan limits.

## Verification

The primary suites are:

```bash
go test ./server/resource -run Metering
go test ./server/llm
go test . -run 'LLM.*E2E|Metering' -count=1
```

Database-matrix tests exercise concurrent hard admission and terminalization on
SQLite, PostgreSQL, and MySQL when their opt-in environments are available.
Live provider tests additionally verify that normalized LLM usage reaches the
same generic ledger without exposing credentials.
