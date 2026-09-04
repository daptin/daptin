# API Metering

API metering lets Daptin record usage, reserve capacity before work begins,
enforce durable named-metric limits, and optionally invoke a billing action
after a request completes. The current model is available from Daptin `v0.13.0`.

For maintainer internals, see [[API-Metering-Technical-KT]].

## Resources

| Resource | Purpose |
|---|---|
| `api_plan` | A named plan and its JSON `limits` array |
| `api_member` | A user's active membership and optional billing period |
| `api_usage` | One held, completed, cancelled, or expired request reservation |
| `api_quota` | Durable reserved and consumed totals for one metric/window bucket |

Every limit has this shape:

```json
{"metric":"total_tokens","window":"month","maximum":1000000,"mode":"hard"}
```

- `metric` is a named measure such as `requests`, `bytes`, `total_tokens`,
  `input_tokens`, `output_tokens`, or `cost_micros`.
- `window` is `minute`, `hour`, `day`, `month`, or `member_period`.
- `maximum` is a non-negative integer.
- `mode` is `hard` or `soft`. Hard limits deny admission; soft limits record
  usage without denying it.

Metric/window pairs must be unique within a plan. A `member_period` limit
requires valid `api_member.period_start` and `period_end` values.

## Create a plan and membership

This plan permits 60 requests per minute and one million tokens per member
billing period:

```bash
curl -X POST http://localhost:6336/api/api_plan \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  --data-binary '{
    "data": {
      "type": "api_plan",
      "attributes": {
        "name": "Developer",
        "price_monthly_cents": 0,
        "limits": "[{\"metric\":\"requests\",\"window\":\"minute\",\"maximum\":60,\"mode\":\"hard\"},{\"metric\":\"total_tokens\",\"window\":\"member_period\",\"maximum\":1000000,\"mode\":\"hard\"}]",
        "metadata": "{}"
      }
    }
  }'
```

Create the membership as the user who owns it, or use an administrator workflow
that explicitly assigns the correct owner:

```bash
curl -X POST http://localhost:6336/api/api_member \
  -H "Authorization: Bearer $USER_TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  --data-binary '{
    "data": {
      "type": "api_member",
      "attributes": {
        "status": "active",
        "period_start": "2026-09-01T00:00:00Z",
        "period_end": "2026-10-01T00:00:00Z",
        "metadata": "{}"
      },
      "relationships": {
        "api_plan": {
          "data": {"type": "api_plan", "id": "API_PLAN_REFERENCE_ID"}
        }
      }
    }
  }'
```

Only active memberships are considered. If more than one is active, the newest
row is selected.

## Enable metering on resources and actions

Metering is part of `TableInfo`, not a backend `metering.llm.*` configuration.
For a normal resource:

```json
{
  "TableName": "orders",
  "metering": {
    "enabled": true,
    "cost_expr": "1",
    "meter_type": "requests"
  }
}
```

Action-specific settings inherit omitted fields from the resource:

```json
{
  "metering": {
    "enabled": true,
    "cost_expr": "1",
    "meter_type": "requests",
    "on_actions": {
      "generate_report": {
        "enabled": true,
        "cost_expr": "50",
        "meter_type": "compute_units"
      }
    }
  }
}
```

`cost_expr` is evaluated only when the completed request did not already supply
the configured metric. It can read `request`, `response`, `metadata`, and
`user`. The defaults are `cost_expr: "1"` and `meter_type: "requests"`.

## LLM metering

`llm_model` has metering enabled for its `invoke` action. HTTP and declarative
LLM actions use the same admit/complete/cancel lifecycle. Request types are
operation-derived, for example `llm_chat`, `llm_embeddings`, and
`llm_text_completion`.

Provider usage is normalized into named measures including:

- `input_tokens`, `output_tokens`, and `total_tokens`;
- `cache_read_tokens`, `cache_write_tokens`, and `reasoning_tokens`;
- `cost_micros` calculated from `llm_deployment.pricing`;
- supplemental measures such as `search_units` or `ocr_pages` when reported.

A provider that omits usage can still produce a completed request record, but
token-based limits can only consume measures that are reported or safely
estimated by the gateway.

## Reservations and quota state

Admission creates an `api_usage` row in `held` state and reserves applicable
hard-limit measures in `api_quota`. Completion atomically releases reservations,
increments consumed totals, stores final measures, and marks the usage row
`completed`. Cancellation and expiry release reservations without counting them
as completed consumption.

Hard-limit admission failure returns `402 Payment Required`. Deployment RPM,
TPM, and concurrency protection are separate gateway controls and may return
`429 Too Many Requests` or a provider-availability error.

Inspect the durable records with:

```bash
curl http://localhost:6336/api/api_usage \
  -H "Authorization: Bearer $ADMIN_TOKEN"

curl http://localhost:6336/api/api_quota \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

## Post-metering billing actions

Set `post_metering_action` to `entity:action` in the metering configuration.
The action receives `api_usage_id`, `api_plan_id`, `api_member_id`, `measures`,
endpoint metadata, and the user reference. It runs with the internal-metering
marker so it does not recursively meter itself.

```json
{
  "metering": {
    "enabled": true,
    "meter_type": "requests",
    "post_metering_action": "credit_wallet:deduct_credits"
  }
}
```

## Operational checklist

1. Create a plan with unique named metric/window limits.
2. Create an active membership owned by the intended user.
3. Enable metering on the resource or action.
4. For LLM limits, configure deployment pricing and verify provider usage.
5. Exercise both successful and denied calls.
6. Inspect held/completed usage rows and reserved/consumed quota values.
7. Monitor reservation expiry recovery and database availability.

## Troubleshooting

| Problem | Check |
|---|---|
| No usage row | Metering configuration, authenticated user, and request logs |
| No quota bucket | Active membership, plan relationship, and matching metric |
| Limit never denies | Metric spelling, window, `maximum`, and `mode: hard` |
| LLM token count is absent | Provider response and normalized gateway usage |
| Cost is zero | `llm_deployment.pricing` keys and fixed-point values |
| Reservations remain held | Runtime recovery task and database errors |
| Billing action does not run | `post_metering_action` name and action logs |
