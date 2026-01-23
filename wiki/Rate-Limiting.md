# Rate Limiting

API rate limiting and throttling.

## Overview

Daptin implements rate limiting to:
- Prevent abuse
- Ensure fair usage
- Protect resources

## Rate Limit Headers

Responses include rate limit headers:

```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1642345678
```

## Default Limits

| Endpoint Type | Limit | Window |
|--------------|-------|--------|
| API Read | 1000/min | 1 minute |
| API Write | 100/min | 1 minute |
| Auth | 10/min | 1 minute |
| Actions | 50/min | 1 minute |

## Configure Rate Limits

### Via Config API

```bash
curl -X POST http://localhost:6336/_config/backend/rate_limit.api.read \
  -H "Authorization: Bearer $TOKEN" \
  -d '"500"'

curl -X POST http://localhost:6336/_config/backend/rate_limit.api.write \
  -H "Authorization: Bearer $TOKEN" \
  -d '"50"'
```

### Per-Table Limits

Set limits on specific tables:

```bash
curl -X PATCH http://localhost:6336/api/world/TABLE_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "world",
      "id": "TABLE_ID",
      "attributes": {
        "rate_limit_read": 100,
        "rate_limit_write": 10
      }
    }
  }'
```

## Rate Limit Response

When limit exceeded:

```http
HTTP/1.1 429 Too Many Requests
Content-Type: application/json
Retry-After: 60

{
  "errors": [{
    "status": "429",
    "title": "Rate limit exceeded",
    "detail": "Too many requests. Please wait before retrying."
  }]
}
```

## Rate Limit by User

Limits applied per authenticated user:

- Anonymous: Stricter limits
- Authenticated: Standard limits
- Admin: Higher limits

## IP-Based Limiting

For unauthenticated requests, limits by IP:

```bash
curl -X POST http://localhost:6336/_config/backend/rate_limit.ip.enabled \
  -H "Authorization: Bearer $TOKEN" \
  -d 'true'
```

## Bypass Rate Limits

### Admin Override

Admins can be exempt:

```bash
curl -X POST http://localhost:6336/_config/backend/rate_limit.admin_exempt \
  -H "Authorization: Bearer $TOKEN" \
  -d 'true'
```

### Whitelist IPs

```bash
curl -X POST http://localhost:6336/_config/backend/rate_limit.whitelist \
  -H "Authorization: Bearer $TOKEN" \
  -d '["10.0.0.0/8", "192.168.0.0/16"]'
```

## Client Handling

### Retry Logic

```javascript
async function apiCall(url, options, retries = 3) {
  const response = await fetch(url, options);

  if (response.status === 429 && retries > 0) {
    const retryAfter = response.headers.get('Retry-After') || 60;
    await sleep(retryAfter * 1000);
    return apiCall(url, options, retries - 1);
  }

  return response;
}
```

### Exponential Backoff

```javascript
function backoff(attempt) {
  return Math.min(1000 * Math.pow(2, attempt), 30000);
}
```

## Monitoring

Check current rate limit status:

```bash
curl -I http://localhost:6336/api/entity \
  -H "Authorization: Bearer $TOKEN"
```

## Best Practices

1. **Handle 429 gracefully** - Implement retry logic
2. **Use authentication** - Get higher limits
3. **Batch requests** - Reduce request count
4. **Cache responses** - Avoid redundant calls
