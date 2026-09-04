# Rate Limiting

Daptin v0.13.0 has two independent request-limiting mechanisms:

1. A global in-process token bucket keyed by client IP and URL path.
2. Plan limits enforced by the API metering service for authenticated users.

The checks are cumulative. A request must pass the global limiter before it can reach authentication, routing, or metering.

## At a Glance

| Property | Global limiter | Metering plan limit |
|---|---|---|
| Scope | Every route on one Daptin process | Metering-enabled CRUD, actions, and LLM invokes |
| Identity | Client IP plus path | Authenticated user, plan, metric, and window |
| Configuration | Internal `limit.rate` configuration | `api_plan.limits` JSON array |
| Window | Token bucket | UTC minute, hour, day, month, or membership period |
| Storage | Process memory | Transactional `api_quota` rows |
| Rejection | HTTP 429 | HTTP 402 for a hard limit |

## Global Request Limiter

The global limiter runs before authentication. Its key is the resolved client IP plus request path; query parameters and the HTTP method are not part of the key. Different item paths therefore normally have different buckets, while `GET` and `POST` to the same path share one.

The default is 500 requests per second with a burst of 500. Buckets are local to one Daptin process and expire from memory after one minute. This expiration is not a one-minute quota window.

An exhausted bucket returns an empty HTTP 429 response. It does not include `Retry-After` or `X-RateLimit-*` headers.

Custom path limits are not currently a supported deployment interface. Use an ingress, reverse proxy, or API gateway when a deployment needs configurable anonymous or IP-based throttling.

## Metering Plan Limits

Use metering when a limit must follow an authenticated member and API plan. This is the mechanism used for request quotas, compute credits, and LLM token quotas.

Metering applies when:

- the table or action has `Metering.enabled` set to `true`;
- the request has an authenticated user; and
- that user has an active `api_member` linked to an `api_plan`.

The plan's `limits` value is a JSON array. Each item has:

| Field | Meaning |
|---|---|
| `metric` | A named measure such as `requests`, `compute_units`, or `total_tokens` |
| `window` | `minute`, `hour`, `day`, `month`, or `member_period` |
| `maximum` | Non-negative amount allowed in the window |
| `mode` | `hard` to reject or `soft` to record usage beyond the maximum |

For example, this plan allows two metered requests per minute and 100,000 LLM tokens per membership period:

```json
[
  {"metric":"requests","window":"minute","maximum":2,"mode":"hard"},
  {"metric":"total_tokens","window":"member_period","maximum":100000,"mode":"hard"}
]
```

Limits with the same metric and window are invalid. A negative maximum is invalid. The default mode is `hard` when it is omitted.

Metering admission creates a held `api_usage` reservation and reserves matching quota before endpoint work. Completion replaces the estimate with final measures; cancellation or expiry releases the reservation. These operations use the request transaction and durable `api_quota` rows rather than an Olric counter.

A hard plan-limit failure returns HTTP 402 before the protected operation runs. A soft limit permits the operation and preserves its usage record.

## End-to-End Configuration

### 1. Create a plan

```bash
curl -X POST http://localhost:6336/api/api_plan \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "api_plan",
      "attributes": {
        "name": "Two requests per minute",
        "limits": "[{\"metric\":\"requests\",\"window\":\"minute\",\"maximum\":2,\"mode\":\"hard\"}]",
        "metadata": "{}"
      }
    }
  }'
```

Save the returned `data.id` as `API_PLAN_REFERENCE_ID`.

### 2. Create the membership

Create the membership as the consuming user, or use the normal administrator ownership workflow for another user:

```bash
curl -X POST http://localhost:6336/api/api_member \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "api_member",
      "attributes": {
        "status": "active",
        "period_start": "2026-09-01T00:00:00Z",
        "period_end": "2026-10-01T00:00:00Z",
        "metadata": "{}"
      },
      "relationships": {
        "api_plan_id": {
          "data": {"type":"api_plan","id":"API_PLAN_REFERENCE_ID"}
        }
      }
    }
  }'
```

Use the relationship name `api_plan_id`.

### 3. Enable metering on a table

Add this block to the table in the complete world schema:

```json
{
  "Metering": {
    "enabled": true,
    "cost_expr": "1",
    "meter_type": "requests"
  }
}
```

Preserve the table's columns, relations, permissions, validations, conformations, and other schema properties. Restart Daptin after updating the stored world schema so the in-memory `TableInfo` loads the configuration.

Action overrides belong under `Metering.on_actions`. LLM metering belongs on the `llm_model` `invoke` action. See [[API-Metering]] for complete examples.

### 4. Verify

Call the protected endpoint three times in the same UTC minute. With an unused counter and the plan above, the first two calls succeed and the third returns HTTP 402.

```bash
for request in 1 2 3; do
  curl -sS -o response.json -w "request=$request status=%{http_code}\n" \
    http://localhost:6336/api/orders \
    -H "Authorization: Bearer $TOKEN"
done
```

Inspect `api_usage` for reservation state and final measures, and `api_quota` for `maximum`, `reserved`, and `consumed` values.

## Client Handling

The global 429 response and metering 402 response do not advertise a reset time. If a client knows the governing plan window, it can wait for that boundary. Otherwise use bounded backoff and surface exhausted credits to the user. Do not automatically retry non-idempotent writes.

## Authentication and OTP Protection

These general limiters are not substitutes for authentication-specific brute-force controls. OTP verification has separate distributed attempt limits and replay protection; see [[Two-Factor-Auth]].

## Troubleshooting

- Confirm the protected table/action loaded `Metering.enabled=true` after restart.
- Confirm the user has an active `api_member` with a populated `api_plan_id`.
- Validate that `api_plan.limits` is JSON and has no duplicate metric/window pair.
- Match the limit metric to the table's `meter_type` or the final measures returned by the operation.
- For `member_period`, confirm the membership has valid `period_start` and `period_end` values.
- Use the response status to distinguish the global limiter (429) from a hard metering limit (402).
