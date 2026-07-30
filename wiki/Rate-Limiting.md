# Rate Limiting

Daptin has two independent request-limiting mechanisms:

1. A global, in-process token-bucket limiter keyed by client IP and URL path.
2. An opt-in API metering limiter keyed by authenticated user and API plan.

Both can return HTTP `429`, but their scope, configuration, counters, and responses are different.

## At a Glance

| Property | Global request limiter | API metering limiter |
|---|---|---|
| Enabled | Always on for the main router | Only for metering-enabled CRUD entities, actions, and enabled LLM metering |
| Identity | Client IP plus request path | Authenticated user plus API plan |
| Algorithm | Token bucket | Fixed UTC calendar-minute counter |
| Default | 500 requests/second with burst 500 | Unlimited (`api_plan.rate_limit_per_minute=-1`) |
| Storage | In-process memory | Olric distributed cache |
| Cluster-wide | No | Yes, when Olric is shared correctly |
| Rejection | Empty HTTP 429 | JSON:API-style HTTP 429 with `rate_limit_exceeded` |
| Limit headers | None | None |

The two checks are cumulative. A request must pass the global limiter before it can reach authentication, routing, or metering.

## Global Request Limiter

### Behavior

The global limiter runs on the main Gin router before authentication. Its bucket key is:

```text
client IP + request path
```

The query string is removed before the key is calculated. For example, these requests use the same bucket:

```text
GET /api/orders?page[number]=1
GET /api/orders?page[number]=2
```

Different literal paths use different buckets. In particular, item URLs normally have independent buckets:

```text
/api/orders
/api/orders/ORDER_REFERENCE_ID
```

The HTTP method, authenticated user, and user role are not part of the key. Consequently:

- `GET` and `POST` to the same path share a bucket.
- Anonymous, authenticated, and administrator requests from the same resolved IP share a bucket.
- Authentication does not provide a higher global limit.
- There is no administrator exemption or IP whitelist.

`ClientIP()` determines the address. Its behavior depends on Gin's trusted-proxy configuration, so deployments behind a reverse proxy must ensure the application sees the intended client IP.

### Rate and Burst

For paths without a usable custom entry, the limiter is configured as:

```text
rate:  500 requests/second
burst: 500 requests
```

This is a token bucket, not a limit of 500 requests per minute. A new or fully replenished bucket can admit a burst of 500 requests, after which tokens replenish at approximately 500 per second.

Each bucket is retained for one minute from creation. Expiration removes the in-memory limiter; the next request creates a new full bucket. This duration is bucket lifetime, not the rate-limit window.

The global counters are local to one Daptin process. Requests distributed over multiple Daptin nodes do not share this limiter's state.

### Rejection Response

When the global bucket has no token, Daptin aborts the request with:

```http
HTTP/1.1 429 Too Many Requests
Content-Length: 0
```

The response has no application error document and no `Retry-After` or `X-RateLimit-*` headers.

### Current Configuration Limitation

Daptin stores a backend configuration value named `limit.rate`, but the current server cannot successfully load custom path limits from it:

- the JSON decode is invoked with a non-pointer value; and
- the Go configuration fields are unexported.

At startup, Daptin therefore falls back to the empty default configuration and writes:

```json
{"version":"default"}
```

to `backend/limit.rate`. All paths then use the 500 requests/second, burst-500 fallback.

Do not rely on `POST /_config/backend/limit.rate` to customize this limiter in the current release. Configuration changes would require a restart even after the loader is fixed because the middleware is constructed only during startup.

There are no supported settings named:

```text
rate_limit.api.read
rate_limit.api.write
rate_limit.ip.enabled
rate_limit.admin_exempt
rate_limit.whitelist
```

There are also no supported per-table fields named `rate_limit_read` or `rate_limit_write`.

## API Metering Rate Limiter

Use API metering when limits must follow an authenticated user and plan, or must be shared across a cluster.

### When It Applies

The metering rate check runs only when all of the following are true:

1. The request reaches a metering-enabled CRUD entity, action, or LLM endpoint.
2. The request has an authenticated user with a non-zero internal user ID.
3. That user has an active `api_member` with an associated `api_plan`.
4. The plan's `rate_limit_per_minute` is zero or greater.

The check is skipped when any prerequisite is absent. In particular, anonymous requests and authenticated users without an active plan membership are not rejected by this limiter.

Metering system tables (`api_plan`, `api_member`, `api_usage`, and `api_quota`) are excluded from CRUD metering to avoid recursion.

If a user has multiple active memberships, the membership with the greatest internal numeric ID is selected.

### Counter Semantics

The Olric key contains the authenticated user's internal ID, the plan's internal ID, and the current UTC minute:

```text
api-rate-limit:<user-id>:<plan-id>:<YYYYMMDDHHMM>
```

The counter is therefore:

- shared by all metered endpoints and request types using the same user and plan;
- shared by all Daptin nodes connected to the same Olric cluster; and
- reset by using a new key at each UTC calendar-minute boundary.

This is not a rolling 60-second window. A request near the end of one minute and another just after the next minute begins belong to different counters.

The counter is incremented during preflight, before the endpoint performs its work. When the new count is greater than the configured limit, hard enforcement rejects the request. The rejected attempt consumes a count in the current minute but does not produce an `api_usage` row or increment the durable `api_quota.request_count` counter because the request did not complete.

Special values:

| `rate_limit_per_minute` | Behavior |
|---:|---|
| `-1` or any negative value | Unlimited; the Olric counter is not touched |
| `0` | Every applicable request is over the limit |
| Positive integer | That many applicable preflights are admitted per UTC minute |

### Failure Behavior

The metering rate limiter fails open. If Olric is unavailable, key initialization fails, or counter increment fails, Daptin logs a warning and allows the request to continue.

Correct Olric configuration is therefore required before treating this limiter as a security or billing boundary.

### Enforcement Mode

Hard versus soft enforcement comes from the metering configuration attached to the entity, action, or LLM subsystem:

```json
"enforce_mode": "hard"
```

An omitted value defaults to `hard`. With `hard`, an exceeded per-minute limit returns HTTP 429. With any other value, including `soft`, Daptin records the failed decision internally but allows the request to continue.

The current implementation does not read `api_plan.quota_enforce_mode` when making this decision. Set `enforce_mode` in the metering configuration that protects the endpoint.

### Rejection Response

Hard metering enforcement returns:

```http
HTTP/1.1 429 Too Many Requests
Content-Type: application/vnd.api+json

{"errors":[{"status":"429","title":"rate_limit_exceeded"}]}
```

No `Retry-After`, `X-RateLimit-Limit`, `X-RateLimit-Remaining`, or `X-RateLimit-Reset` headers are emitted.

## End-to-End Configuration Example

This example creates a two-request-per-minute plan, assigns the authenticated user to it, and enables hard metering on an entity.

### 1. Create a Plan

```bash
curl -X POST http://localhost:6336/api/api_plan \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "api_plan",
      "attributes": {
        "name": "Two requests per minute",
        "requests_per_period": -1,
        "compute_units_per_period": -1,
        "rate_limit_per_minute": 2,
        "meter_type": "requests",
        "quota_enforce_mode": "hard",
        "metadata": "{}"
      }
    }
  }'
```

Save the returned `data.id` as `API_PLAN_REFERENCE_ID`.

`quota_enforce_mode` is included because it is part of the plan schema, but current per-minute enforcement uses the endpoint's metering `enforce_mode`, as described above.

### 2. Create the Membership

Create the membership while authenticated as the user who will consume the plan. Administrators can instead use their normal ownership workflow to create it for another user.

```bash
curl -X POST http://localhost:6336/api/api_member \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "api_member",
      "attributes": {
        "status": "active",
        "period_start": "2026-07-01T00:00:00Z",
        "period_end": "2026-08-01T00:00:00Z",
        "metadata": "{}"
      },
      "relationships": {
        "api_plan_id": {
          "data": {
            "type": "api_plan",
            "id": "API_PLAN_REFERENCE_ID"
          }
        }
      }
    }
  }'
```

Use the relationship name `api_plan_id`. A relationship named `api_plan` is ignored and creates a membership without a plan.

The rate limiter currently checks only `status = "active"`; it does not compare the current time with `period_start` or `period_end`.

### 3. Enable Metering on an Entity

Add a metering block to the entity's complete world schema:

```json
{
  "metering": {
    "enabled": true,
    "cost_expr": "1",
    "meter_type": "requests",
    "enforce_mode": "hard"
  }
}
```

Preserve the entity's existing columns, relations, permissions, validations, conformations, and other schema properties. Updating `world_schema_json` replaces the stored schema; it is not a merge patch.

Restart Daptin after changing the stored world schema so that the entity's in-memory `TableInfo` contains the new metering configuration.

For action-specific configuration and LLM metering, see [[API-Metering]].

### 4. Verify the Limit

Within one UTC minute, call the metered endpoint four times:

```bash
for request in 1 2 3 4; do
  curl -sS -o response.json -w "request=$request status=%{http_code}\n" \
    http://localhost:6336/api/orders \
    -H "Authorization: Bearer $TOKEN"
  cat response.json
done
```

For a newly unused user-plan counter and a limit of 2, the expected status sequence is:

```text
200
200
429
429
```

At the next UTC minute boundary, the next request is admitted again.

Successful metered requests create `api_usage` rows and update `api_quota`. Rate-rejected requests do neither.

## Client Handling

Because neither limiter supplies `Retry-After` or a reset timestamp, a client cannot determine the exact safe retry time from response headers. Use bounded exponential backoff with jitter, or—when the client is known to be governed by a metering plan—wait until the next UTC minute boundary.

Do not retry writes automatically unless the operation is idempotent or uses an idempotency mechanism. A 429 from metering occurs before endpoint work; a 429 from the global middleware also occurs before routing, but intermediaries and client retries can still make write behavior difficult to reason about.

## Authentication and OTP Protection

The generic global and API-metering limiters are not substitutes for authentication-specific brute-force controls. The global fallback is intentionally high, while API metering skips anonymous users.

OTP verification therefore has a separate application-level distributed control:

- five atomic attempts per account per 15 minutes;
- 50 atomic attempts per source per 15 minutes;
- counters shared through Olric across Daptin nodes;
- fail-closed behavior when the shared protection state is unavailable; and
- successful-code replay prevention per account and TOTP period.

These controls apply inside the OTP performer, including the guest-reachable password-recovery flow. See [[Two-Factor-Auth]]. Authentication endpoints should retain their application-specific controls even if a reverse proxy or WAF also limits traffic.

Example bounded backoff:

```javascript
async function apiCall(url, options, retries = 3) {
  for (let attempt = 0; ; attempt++) {
    const response = await fetch(url, options);
    if (response.status !== 429 || attempt >= retries) return response;

    const base = Math.min(1000 * 2 ** attempt, 30000);
    const jitter = Math.floor(Math.random() * 250);
    await new Promise(resolve => setTimeout(resolve, base + jitter));
  }
}
```

## Monitoring and Troubleshooting

There is no endpoint or response header that reports the current global bucket state or remaining metering count.

For API metering:

- inspect `api_usage` for completed metered requests;
- inspect `api_quota` for durable period counters;
- inspect Daptin logs for Olric warnings;
- confirm the entity schema loaded at startup contains `metering.enabled=true`;
- confirm the user has an active `api_member` whose `api_plan_id` is populated; and
- confirm `rate_limit_per_minute` is not negative.

Remember that `api_quota.request_count` is a period usage counter, not the current minute's Olric counter. It will not include rejected requests, and it cannot be used to calculate the remaining per-minute allowance.

## Verified Behavior

The behavior documented here was checked against the current implementation and a clean SQLite-backed Daptin instance with a local Olric node. The end-to-end verification covered:

- global response headers;
- query-string normalization;
- burst exhaustion and empty global 429 responses;
- the persisted `backend/limit.rate` fallback value;
- plan and membership creation using `api_plan_id`;
- entity metering loaded after restart;
- two admitted requests followed by metering 429 responses;
- no usage/quota increment for rejected calls; and
- admission after the next UTC minute boundary.
