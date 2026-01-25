# Server Configuration

**Tested ✓** - All examples on this page were verified against a running Daptin instance on 2026-01-25.

---

## Monitoring Endpoints

### Health Check

```bash
curl http://localhost:6336/ping
```

**Response:** `pong`

### Statistics

```bash
curl http://localhost:6336/statistics
```

**Response (tested):**
```json
{
  "cpu": {"counts": 10, "percent": [...]},
  "db": {
    "MaxOpenConnections": 1,
    "OpenConnections": 1,
    "InUse": 0,
    "Idle": 1,
    "WaitCount": 0
  },
  "disk": {...},
  "host": {"hostname": "...", "uptime": 3790344, "os": "darwin"},
  "load": {"avg": {"load1": 5.56, "load5": 6.30, "load15": 5.96}},
  "memory": {"virtual": {...}, "swap": {...}},
  "web": {
    "pid": 65018,
    "uptime": "1h29m1s",
    "total_count": 204,
    "average_response_time": "26.02ms",
    "total_status_code_count": {"200": 204}
  }
}
```

### OpenAPI Specification

```bash
curl http://localhost:6336/openapi.yaml
```

Returns full OpenAPI 3.0 specification.

### JS Model

```bash
curl http://localhost:6336/jsmodel/world
```

**Response:**
```json
{
  "Actions": [...],
  "ColumnModel": {...},
  "IsStateMachineEnabled": false,
  "StateMachines": [...]
}
```

### Aggregate

```bash
TOKEN="your-jwt-token"
curl http://localhost:6336/aggregate/user_account \
  -H "Authorization: Bearer $TOKEN"
```

**Response:**
```json
{
  "data": [{
    "type": "aggregate_user_account",
    "id": "...",
    "attributes": {
      "__type": "aggregate_user_account",
      "count": 3
    }
  }]
}
```

---

## Runtime Configuration API

Config values are stored in database. Most take effect immediately, some require restart.

### Get All Config

```bash
curl http://localhost:6336/_config \
  -H "Authorization: Bearer $TOKEN"
```

**Response (tested):**
```json
{
  "caldav.enable": "false",
  "enable_https": "false",
  "ftp.enable": "false",
  "graphql.enable": "false",
  "gzip.enable": "true",
  "hostname": "Parths-MacBook-Pro.local",
  "imap.enabled": "false",
  "jwt.token.issuer": "daptin-019bee",
  "jwt.token.life.hours": "72",
  "language.default": "en",
  "limit.max_connections": "100",
  "limit.rate": "{\"version\":\"default\"}",
  "rclone.retries": "5",
  "yjs.enabled": "true",
  "yjs.storage.path": "./storage/yjs-documents"
}
```

### Get Specific Value

```bash
curl http://localhost:6336/_config/backend/graphql.enable \
  -H "Authorization: Bearer $TOKEN"
```

**Response:** `false`

### Set Value (POST)

```bash
curl -X POST http://localhost:6336/_config/backend/my.setting \
  -H "Authorization: Bearer $TOKEN" \
  -d 'my_value'
```

**Response:** `my_value`

### Update Value (PUT)

```bash
curl -X PUT http://localhost:6336/_config/backend/my.setting \
  -H "Authorization: Bearer $TOKEN" \
  -d 'updated_value'
```

**Response:** `updated_value`

### Delete Value

```bash
curl -X DELETE http://localhost:6336/_config/backend/my.setting \
  -H "Authorization: Bearer $TOKEN"
```

After deletion, GET returns HTTP 404.

---

## What Requires Restart

**Tested finding:** The `restart_daptin` action does NOT actually restart the server. It returns a success response but `trigger.Fire("restart")` is never called in the codebase.

### Requires HARD Restart (stop and start process)

- Enabling/disabling GraphQL (`graphql.enable`)
- Any route-level changes
- New schema files (schema_*.yaml/json/toml)
- Port changes
- Database connection changes

### Takes Effect Immediately

- `jwt.token.life.hours` - New tokens use new value
- `limit.max_connections` - Connection limit
- `gzip.enable` - Compression
- Custom config values you create

---

## Known Issues

### Hot Restart is Broken

The `restart_daptin` action:

```bash
curl -X POST http://localhost:6336/action/world/restart_daptin \
  -H "Authorization: Bearer $TOKEN" \
  -d '{}'
```

Returns:
```json
[
  {"ResponseType": "client.notify", "Attributes": {"message": "Initiating system update."}},
  {"ResponseType": "client.redirect", "Attributes": {"delay": 5000, "location": "/"}}
]
```

But the server does NOT actually restart. The `trigger.On("restart")` handler is registered in main.go but `trigger.Fire("restart")` is never called.

**Workaround:** Stop and restart the process for any changes that require restart.

---

## Not Yet Tested

The following are documented in code but not verified:

- MySQL/PostgreSQL connection strings
- Environment variables (DAPTIN_PORT, DAPTIN_DB_TYPE, etc.)
- Command-line flags
- TLS/HTTPS configuration
- Olric clustering
- SMTP/IMAP/FTP/CalDAV configuration

These will be documented after testing.
