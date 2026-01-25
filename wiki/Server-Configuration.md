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

## Command-Line Flags (Tested)

### Port

```bash
go run main.go -port :8080
```

**Tested:** Server listens on port 8080, `/ping` returns `pong`.

### Database Connection

```bash
go run main.go -db_connection_string ./myapp.db
```

**Tested:** Creates SQLite database at specified path.

### Log Level

```bash
go run main.go -log_level debug
```

**Tested:** Shows `DEBU` log messages in addition to `INFO` and `WARN`.

Valid values: `debug`, `info`, `warn`, `error`

### Runtime Mode

```bash
go run main.go -runtime release
```

**Tested values:**

| Mode | Effect |
|------|--------|
| `release` | Default. Standard logging, no debug features |
| `debug` | Verbose Gin framework logging |
| `test` | Test mode (minimal output) |
| `profile` | Creates CPU/heap profile dump files |

**Profile mode** creates files in current directory:
- `daptin_hostname_profile_cpu.0` - CPU profile
- `daptin_hostname_profile_heap.0` - Heap profile

---

## Environment Variables (Tested)

### DAPTIN_PORT

```bash
DAPTIN_PORT=:9090 go run main.go
```

**Tested:** Server listens on port 9090. Overrides default port.

### DAPTIN_SCHEMA_FOLDER

```bash
DAPTIN_SCHEMA_FOLDER=/path/to/schemas go run main.go
```

**Tested:**
1. Created `/tmp/test-schemas/schema_mytest.yaml`:
   ```yaml
   Tables:
     - TableName: test_products
       Columns:
         - Name: name
           ColumnType: label
           DataType: varchar(200)
         - Name: price
           ColumnType: measurement
           DataType: int
   ```

2. Started server:
   ```bash
   DAPTIN_SCHEMA_FOLDER=/tmp/test-schemas go run main.go
   ```

3. Server logs show:
   ```
   Found files to load: [/tmp/test-schemas/schema_mytest.yaml]
   Process file: /tmp/test-schemas/schema_mytest.yaml
   ```

4. API confirms table exists:
   ```bash
   curl http://localhost:6336/api/test_products
   # Returns: {"data":[],...}  (empty but table exists)
   ```

5. Creating records works:
   ```bash
   curl -X POST http://localhost:6336/api/test_products \
     -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/vnd.api+json" \
     -d '{"data":{"type":"test_products","attributes":{"name":"Widget","price":999}}}'
   # Returns: created record with id, reference_id, timestamps
   ```

**Schema file naming:** Must match pattern `schema_*.yaml`, `schema_*.json`, or `schema_*.toml`

---

## Not Yet Tested

The following are documented in code but not verified in this doc:

- MySQL/PostgreSQL connection strings (`-db_type mysql -db_connection_string "user:pass@tcp(host:port)/dbname"`)
- HTTPS port (`-https_port :443`)
- TLS certificate paths
- Olric clustering flags
- FTP/CalDAV config values

**Note:** SMTP and IMAP are documented separately in [SMTP-Server.md](SMTP-Server.md) and [IMAP-Support.md](IMAP-Support.md).
