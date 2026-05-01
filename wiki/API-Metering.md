# API Metering

API metering lets Daptin track usage, enforce quotas, rate-limit users across a cluster, and optionally trigger credit or billing actions after requests complete.

Use it when you want plans such as Free, Pro, or Internal, where each user gets a request allowance, token/compute allowance, and optional per-minute traffic limit.

For maintainer internals, see [[API-Metering-Technical-KT]].

## What It Does

| Capability | Description |
|------------|-------------|
| Plans | Define request, compute, rate, and price settings |
| Memberships | Assign a Daptin user to one active plan |
| Usage logs | Store a row for each metered request |
| Quotas | Track usage for a plan period |
| Rate limiting | Enforce per-user, per-plan, per-minute limits through Olric |
| Actions | Meter custom Daptin actions |
| LLM endpoints | Meter chat, completion, and embedding token usage |
| Credit hooks | Call a configured action such as `credit:deduct_credits` after usage is recorded |

## System Tables

Daptin creates these metering tables automatically.

| Table | What you use it for |
|-------|---------------------|
| `api_plan` | Create plans and limits |
| `api_member` | Assign users to plans |
| `api_usage` | Review individual usage events |
| `api_quota` | Review counters for the current period |

`api_member` is the shorter name for an API subscription. It represents one user's membership in a metering plan.

## Important Concepts

### Plans

An `api_plan` controls the limits.

| Field | Meaning |
|-------|---------|
| `name` | Unique plan name |
| `requests_per_period` | Total request count allowed for the member period. Use `-1` for unlimited |
| `compute_units_per_period` | Total compute units allowed for the member period. Use `-1` for unlimited |
| `rate_limit_per_minute` | Maximum metered requests per minute for one user on this plan. Use `-1` for unlimited |
| `price_monthly_cents` | Optional monthly price metadata |
| `overage_price_micros` | Optional price per usage unit in micros |
| `meter_type` | Default plan meter type, usually `requests` or `compute_units` |
| `quota_enforce_mode` | Usually `hard` |
| `metadata` | Free-form JSON |

### Memberships

An `api_member` assigns a user to a plan.

| Field | Meaning |
|-------|---------|
| `status` | Use `active` for the active membership |
| `period_start` | Start of the quota period |
| `period_end` | Optional end of the quota period |
| `metadata` | Free-form JSON |

Every normal Daptin table already belongs to the creating user by default. You do not need to add a separate user relation to `api_member`.

### Usage

`api_usage` is the audit-style event log. It records the endpoint, method, request type, status, cost, and metadata for each completed metered request.

### Quota

`api_quota` stores aggregate counters for a user, plan, member, and period:

| Counter | Meaning |
|---------|---------|
| `request_count` | Number of recorded metered requests |
| `compute_units` | Sum of compute-unit cost |
| `bytes_used` | Sum of response bytes recorded by the metering layer |

## What Gets Metered

### CRUD APIs

CRUD APIs are metered when the entity schema has `metering.enabled=true`.

Examples:

```text
GET    /api/orders
POST   /api/orders
PATCH  /api/orders/{id}
DELETE /api/orders/{id}
```

### Actions

Actions are metered when the entity's metering config is enabled for that action. You can use one default config for all actions or override individual action costs.

Example:

```text
POST /action/report/generate_monthly_report
```

### LLM APIs

LLM endpoints are metered by default against token usage:

| Endpoint family | Metered as |
|-----------------|------------|
| Chat completions | `llm_chat` |
| Streaming chat completions | `llm_chat` |
| Completions | `llm_completion` |
| Embeddings | `llm_embedding` |

Default LLM cost:

```text
response.usage.total_tokens
```

## Setup Flow

### 1. Create a Plan

```bash
curl -X POST http://localhost:6336/api/api_plan \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "api_plan",
      "attributes": {
        "name": "Free",
        "requests_per_period": 1000,
        "compute_units_per_period": 100000,
        "rate_limit_per_minute": 60,
        "price_monthly_cents": 0,
        "overage_price_micros": 0,
        "meter_type": "requests",
        "quota_enforce_mode": "hard",
        "metadata": "{}"
      }
    }
  }'
```

Save the returned `api_plan` reference id.

### 2. Assign a User to the Plan

Create an `api_member` while authenticated as the user who should consume the plan, or create it as an administrator with the correct user ownership fields according to your admin workflow.

```bash
curl -X POST http://localhost:6336/api/api_member \
  -H "Authorization: Bearer $USER_TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "api_member",
      "attributes": {
        "status": "active",
        "period_start": "2026-04-01T00:00:00Z",
        "period_end": "2026-05-01T00:00:00Z",
        "metadata": "{}"
      },
      "relationships": {
        "api_plan": {
          "data": {
            "type": "api_plan",
            "id": "API_PLAN_REFERENCE_ID"
          }
        }
      }
    }
  }'
```

Only active memberships are used. If a user has multiple active memberships, Daptin uses the newest one.

### 3. Enable Metering on an Entity

Set the entity's `metering` block in its schema.

Example schema fragment:

```json
{
  "TableName": "orders",
  "Columns": [],
  "metering": {
    "enabled": true,
    "cost_expr": "1",
    "meter_type": "requests",
    "enforce_mode": "hard"
  }
}
```

In a full world schema update, preserve the entity's existing columns, permissions, relations, validations, conformations, and other settings. Only add or update the `metering` block.

### 4. Make a Metered Request

```bash
curl http://localhost:6336/api/orders \
  -H "Authorization: Bearer $USER_TOKEN"
```

After the request completes, check:

```bash
curl http://localhost:6336/api/api_usage \
  -H "Authorization: Bearer $ADMIN_TOKEN"

curl http://localhost:6336/api/api_quota \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

## Cost Expressions

`cost_expr` is a JavaScript expression that returns a number. It can use request, response, metadata, user, and plan values supplied by Daptin.

Common examples:

| Use case | Expression |
|----------|------------|
| One unit per request | `1` |
| One unit per returned row | `metadata.row_count` |
| Charge by LLM tokens | `response.usage.total_tokens` |
| Charge by response size bucket | `Math.ceil(request.response_bytes / 1024)` |
| Charge a fixed action cost | `25` |

If the expression returns a fraction, Daptin rounds up. If it returns a negative value, Daptin records zero.

## Action-Specific Costs

Use `on_actions` when different actions on the same entity should have different costs.

```json
{
  "metering": {
    "enabled": true,
    "cost_expr": "1",
    "meter_type": "requests",
    "enforce_mode": "hard",
    "on_actions": {
      "generate_report": {
        "enabled": true,
        "cost_expr": "50",
        "meter_type": "compute_units"
      },
      "preview_report": {
        "enabled": true,
        "cost_expr": "5",
        "meter_type": "compute_units"
      }
    }
  }
}
```

## Credit Charging

Daptin's metering layer records usage and can call a normal Daptin action after recording. This keeps billing rules customizable.

Example:

```json
{
  "metering": {
    "enabled": true,
    "cost_expr": "metadata.row_count || 1",
    "meter_type": "compute_units",
    "post_metering_action": "credit:deduct_credits",
    "enforce_mode": "hard"
  }
}
```

The `credit:deduct_credits` action is your own Daptin action. It receives usage details such as `api_usage_id`, `cost_units`, `cost_micros`, `api_plan_id`, and `api_member_id`.

## Quota and Limit Responses

When a hard quota is exceeded:

```http
HTTP/1.1 402 Payment Required
```

The response is a JSON:API-style error with `insufficient_quota`.

When the one-minute plan rate limit is exceeded:

```http
HTTP/1.1 429 Too Many Requests
```

The response uses `rate_limit_exceeded`.

## Cluster Behavior

`rate_limit_per_minute` uses Olric, Daptin's distributed cache. In a cluster, all nodes share the same one-minute counter for the same user and plan.

Durable period counters are stored in `api_quota`. Usage events are stored in `api_usage`.

If Olric is not available, the short-window rate limit is skipped and Daptin logs a warning. Period quota checks still use the database.

## Operational Checklist

1. Create at least one `api_plan`.
2. Assign users with active `api_member` rows.
3. Enable `metering` on entities or actions that should be charged.
4. Confirm LLM providers return usage if you need LLM metering.
5. Check `api_usage` after test calls.
6. Check `api_quota` for period counters.
7. Configure Olric correctly before relying on `rate_limit_per_minute` in a cluster.
8. Implement a post-metering action if credits must be deducted.

## Troubleshooting

| Problem | Check |
|---------|-------|
| No usage rows | Entity may not have `metering.enabled=true`, user may not be authenticated, or user may not have an active `api_member` |
| User is not charged | Check `post_metering_action` and the target action logs |
| Quota never blocks | Check plan limits. `-1` means unlimited |
| Rate limit does not block | Check `rate_limit_per_minute` and Olric initialization |
| LLM usage missing | Confirm the LLM provider response includes usage totals |
| Cost is zero | Check `api_usage.error_message` for expression errors |
| User gets blocked too early | Check for multiple clients using the same user and same plan in the same minute |

## Best Practices

- Start with request-based metering before adding compute-unit expressions.
- Use `api_usage` as the audit source for invoices and credit ledgers.
- Keep credit deduction in an action so business rules can evolve independently.
- Use `-1` for unlimited limits instead of very large numbers.
- Keep one active membership per user unless you intentionally want newest-active-member behavior.
- Run Daptin with Olric configured before relying on cluster-wide short-window limits.
