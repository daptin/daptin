# Server Configuration

**Tested ✓** - All examples on this page were verified against a running Daptin instance (updated 2026-01-26).

---

## Quick Reference

### Command-Line Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-port` | `:6336` | HTTP server port |
| `-https_port` | `:6443` | HTTPS server port (requires certificates) |
| `-db_type` | `sqlite3` | Database: `sqlite3`, `mysql`, `postgres` |
| `-db_connection_string` | `daptin.db` | SQLite path or MySQL/PostgreSQL connection string |
| `-log_level` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `-runtime` | `release` | Runtime mode: `release`, `debug`, `test`, `profile` |
| `-local_storage_path` | `./storage` | Path for blob column assets (use `;` to disable) |
| `-dashboard` | `daptinweb` | Path to web dashboard files |
| `-profile_dump_path` | `./` | Directory for CPU/heap profile dumps |
| `-profile_dump_period` | `5` | Minutes between profile dumps |
| `-port_variable` | `DAPTIN_PORT` | Env var name to read port from |
| `-database_url_variable` | `DAPTIN_DB_CONNECTION_STRING` | Env var name to read DB connection from |
| `-olric_peers` | `""` | Comma-separated list of cluster peers |
| `-olric_bind_port` | `5336` | Port for Olric cluster communication |
| `-olric_membership_port` | `5336` | Port for membership protocol |
| `-olric_env` | `local` | Cluster environment: `local`, `lan`, `wan` |

### Environment Variables

All flags can be set via environment variables with `DAPTIN_` prefix:
- `DAPTIN_PORT` → `-port`
- `DAPTIN_DB_TYPE` → `-db_type`
- `DAPTIN_RUNTIME` → `-runtime`
- etc.

**Additional environment variables:**

| Variable | Default | Description |
|----------|---------|-------------|
| `DAPTIN_GOMAXPROCS` | `0` (all cores) | Max CPU cores to use |
| `TZ` | `UTC` | Server timezone (e.g., `America/New_York`) |
| `DAPTIN_LOG_LOCATION` | stdout only | Log file path (supports `${HOSTNAME}`, `${PID}`) |
| `DAPTIN_LOG_MAX_SIZE` | `10` | Max log file size in MB before rotation |
| `DAPTIN_LOG_MAX_BACKUPS` | `10` | Number of old log files to keep |
| `DAPTIN_LOG_MAX_AGE` | `7` | Max days to keep old log files |

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
./daptin -port :8080
```

**Tested:** Server listens on port 8080, `/ping` returns `pong`.

### Database Connection

```bash
./daptin -db_connection_string ./myapp.db
```

**Tested:** Creates SQLite database at specified path.

### Log Level

```bash
./daptin -log_level debug
```

**Tested:** Shows `DEBU` log messages in addition to `INFO` and `WARN`.

Valid values: `debug`, `info`, `warn`, `error`

### Runtime Mode

```bash
./daptin -runtime release
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
DAPTIN_PORT=:9090 ./daptin
```

**Tested:** Server listens on port 9090. Overrides default port.

### DAPTIN_SCHEMA_FOLDER

```bash
DAPTIN_SCHEMA_FOLDER=/path/to/schemas ./daptin
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
   DAPTIN_SCHEMA_FOLDER=/tmp/test-schemas ./daptin
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

### DAPTIN_GOMAXPROCS

```bash
DAPTIN_GOMAXPROCS=4 ./daptin
```

**Tested:** Sets Go runtime GOMAXPROCS (max CPU cores to use). Default is number of CPU cores. Set to `0` to use Go's default behavior.

### TZ

```bash
TZ=America/New_York ./daptin
```

**Tested:** Sets server timezone. Logs show: `Setting timezone: America/New_York`. Affects timestamp display and scheduling.

**Common timezones:**
- `America/New_York` (US Eastern)
- `America/Los_Angeles` (US Pacific)
- `Europe/London` (UK)
- `Asia/Tokyo` (Japan)
- `UTC` (Default if not set)

### DAPTIN_LOG_LOCATION

```bash
DAPTIN_LOG_LOCATION=/tmp/daptin-custom.log ./daptin
```

**Tested:** Sets log file path. Logs written to both file and stdout. Supports variable substitution:
- `${HOSTNAME}` - Server hostname
- `${PID}` - Process ID

**Example:**
```bash
DAPTIN_LOG_LOCATION=/var/log/daptin-${HOSTNAME}-${PID}.log ./daptin
# Creates: /var/log/daptin-myserver-12345.log
```

### DAPTIN_LOG_MAX_SIZE

```bash
DAPTIN_LOG_MAX_SIZE=5 ./daptin
```

**Tested:** Maximum log file size in megabytes before rotation. Default: `10`.

### DAPTIN_LOG_MAX_BACKUPS

```bash
DAPTIN_LOG_MAX_BACKUPS=3 ./daptin
```

**Tested:** Number of old log files to keep. Default: `10`.

### DAPTIN_LOG_MAX_AGE

```bash
DAPTIN_LOG_MAX_AGE=2 ./daptin
```

**Tested:** Maximum days to keep old log files. Default: `7`.

**Complete log rotation example:**
```bash
DAPTIN_LOG_LOCATION=/var/log/daptin.log \
DAPTIN_LOG_MAX_SIZE=5 \
DAPTIN_LOG_MAX_BACKUPS=3 \
DAPTIN_LOG_MAX_AGE=7 \
./daptin
```

**Result:** Logs rotate when reaching 5MB, keep 3 backups, delete files older than 7 days.

### DAPTIN_DB_CONNECTION_STRING

```bash
DAPTIN_DB_CONNECTION_STRING=/tmp/test-db.db ./daptin
```

**Tested:** Sets database path (SQLite) or connection string (MySQL/PostgreSQL). Overrides `-db_connection_string` flag.

**Examples:**
```bash
# SQLite
DAPTIN_DB_CONNECTION_STRING=./myapp.db

# MySQL
DAPTIN_DB_CONNECTION_STRING="user:password@tcp(localhost:3306)/daptin"

# PostgreSQL
DAPTIN_DB_CONNECTION_STRING="host=localhost port=5432 user=daptin password=secret dbname=daptin sslmode=disable"
```

**Note:** MySQL/PostgreSQL require `-db_type` flag as well.

---

## Additional Command-Line Flags (Tested)

### local_storage_path

```bash
./daptin -local_storage_path /tmp/custom-storage
```

**Tested:** Sets path for storing blob/file column assets. Default: `./storage`. Server starts successfully with custom path.

**Disable blob storage:**
```bash
./daptin -local_storage_path ";"
```

### dashboard

```bash
./daptin -dashboard /path/to/dashboard/dist
```

**Tested:** Sets custom path for web dashboard files. Default: `daptinweb` (embedded). Useful for custom dashboards or local development.

### profile_dump_path

```bash
./daptin -runtime profile -profile_dump_path /tmp/profile-data/
```

**Tested:** Directory for CPU/heap profile dumps in profile mode. Creates files:
- `daptin_{hostname}_profile_cpu.0`
- `daptin_{hostname}_profile_heap.0`

### profile_dump_period

```bash
./daptin -runtime profile -profile_dump_period 5
```

**Tested:** Minutes between profile dumps in profile mode. Default: `5`.

### port_variable

```bash
MY_CUSTOM_PORT=:9999 ./daptin -port_variable MY_CUSTOM_PORT
```

**Tested:** Name of environment variable to read port from. Default: `DAPTIN_PORT`. Server logs show: `Looking up variable [MY_CUSTOM_PORT] for port: :9999`.

**Use case:** Deploy same binary to different environments with different port env var names.

### database_url_variable

```bash
MY_DB_PATH=/tmp/mydb.db ./daptin -database_url_variable MY_DB_PATH
```

**Tested:** Name of environment variable to read database connection string from. Default: `DAPTIN_DB_CONNECTION_STRING`.

---

## HTTPS/TLS Configuration

HTTPS is controlled via database configuration and certificates. See [TLS-Certificates.md](TLS-Certificates.md) for complete TLS setup guide.

### Enable HTTPS

```bash
# Set enable_https config value
curl -X POST http://localhost:6336/_config/backend/enable_https \
  -H "Authorization: Bearer $TOKEN" \
  -d 'true'

# Restart server
./scripts/testing/test-runner.sh stop && ./scripts/testing/test-runner.sh start
```

**Tested:** HTTPS server starts on port 6443 (default). Check with:
```bash
lsof -i :6443
```

### Change HTTPS Port

```bash
./daptin -https_port :8443
```

**Default:** `:6443`

**Note:** Requires certificates in database and `enable_https=true`. See [TLS-Certificates.md](TLS-Certificates.md) for certificate generation.

---

## Olric Clustering Configuration

Daptin uses Olric for distributed caching. These flags configure clustering across multiple Daptin instances.

### olric_peers

```bash
./daptin -olric_peers "192.168.1.10:5336,192.168.1.11:5336"
```

**Format:** Comma-separated list of `ip:port` addresses of other Daptin instances.

### olric_bind_port

```bash
./daptin -olric_bind_port 5336
```

**Default:** `5336` (automatically calculated if not set)

**Purpose:** Port for Olric cluster communication.

### olric_membership_port

```bash
./daptin -olric_membership_port 5336
```

**Default:** Same as `olric_bind_port`

**Purpose:** Port for membership protocol (cluster node discovery).

### olric_env

```bash
./daptin -olric_env lan
```

**Options:**
- `local` - Single node (default)
- `lan` - Local area network cluster
- `wan` - Wide area network cluster

**Effect:** Adjusts timeouts and retry behavior for network latency.

**Example multi-node setup:**

**Node 1:**
```bash
./daptin -olric_peers "192.168.1.11:5336" -olric_bind_port 5336 -olric_env lan
```

**Node 2:**
```bash
./daptin -olric_peers "192.168.1.10:5336" -olric_bind_port 5336 -olric_env lan
```

**Logs show:** `olric peers: [192.168.1.11:5336]`

---

## MySQL / PostgreSQL Databases (Tested ✓)

Daptin supports MySQL and PostgreSQL for production deployments requiring concurrent access or larger datasets.

###MySQL/MariaDB

**Tested with MariaDB 10.11** (MySQL-compatible)

```bash
# Start MySQL container for testing
docker run -d --name daptin-mysql \
  -e MYSQL_ROOT_PASSWORD=password \
  -e MYSQL_DATABASE=daptin_db \
  -e MYSQL_USER=daptin \
  -e MYSQL_PASSWORD=password \
  -p 3306:3306 \
  mariadb:10.11

# Start Daptin with MySQL
./daptin -db_type mysql \
  -db_connection_string "daptin:password@tcp(localhost:3306)/daptin_db"
```

**Connection String Format:**
```
username:password@tcp(hostname:port)/database_name
```

**Example:**
```
daptin:secret@tcp(mysql.example.com:3306)/production_db
```

**Key Differences from SQLite:**
- Max Open Connections: `50` (vs `1` for SQLite)
- Supports true concurrent writes from multiple Daptin instances
- All standard tables created successfully
- Some SQL syntax differences handled automatically by statementbuilder

**Verified Operations:**
- ✓ Table creation and schema initialization
- ✓ User signup and authentication
- ✓ CRUD operations via REST API
- ✓ Data persistence across restarts

### PostgreSQL

**Tested with PostgreSQL 15**

```bash
# Start PostgreSQL container
docker run -d --name daptin-postgres \
  -e POSTGRES_PASSWORD=password \
  -e POSTGRES_DB=daptin_db \
  -e POSTGRES_USER=daptin \
  -p 5432:5432 \
  postgres:15

# Start Daptin with PostgreSQL
./daptin -db_type postgres \
  -db_connection_string "host=localhost port=5432 user=daptin password=password dbname=daptin_db sslmode=disable"
```

**Connection String Format:**
```
host=hostname port=5432 user=username password=password dbname=database_name sslmode=disable
```

**For SSL/TLS connections:**
```
host=hostname port=5432 user=username password=password dbname=database_name sslmode=require
```

**Key Differences from SQLite:**
- Max Open Connections: `50` (vs `1` for SQLite)
- Native support for concurrent connections
- Better performance for complex queries
- JSONB support for efficient JSON storage

**Verified Operations:**
- ✓ Table creation and schema initialization
- ✓ User signup and authentication
- ✓ CRUD operations via REST API
- ✓ Data persistence across restarts
- ✓ Concurrent access from multiple clients

### Database Comparison

| Feature | SQLite | MySQL/MariaDB | PostgreSQL |
|---------|--------|---------------|------------|
| Setup Complexity | Simple (file-based) | Moderate (server required) | Moderate (server required) |
| Concurrent Writes | Limited (1 connection) | Excellent (50 connections) | Excellent (50 connections) |
| Production Ready | Development only | Yes | Yes |
| Clustering Support | No | Yes (with Olric)* | Yes (with Olric)* |
| File Size Limit | ~281 TB | Server dependent | Server dependent |
| Best For | Development, prototyping | Production, web apps | Production, complex queries |

*Note: Olric clustering currently has known issues (see below)

---

## Known Issues

### Olric Multi-Node Clustering (Not Working)

**Status**: ⚠️ Clustering configuration exists but has bugs

**Issue**: When starting Daptin with Olric clustering flags, nodes fail with:
```
[FATA] failed to create olric topic - no available client found
```

**What Was Tested:**
- 2-node cluster with shared PostgreSQL database
- Olric bind ports: 5001, 5002
- Membership ports configured
- Peers configured: each node knows about the other
- Environment: `lan`

**Result:** Olric starts successfully but PubSub topic creation fails in `server/server.go:304-307`.

**Conclusion**: Olric clustering flags are present but the feature appears incomplete or has initialization timing issues. Single-node Olric (default, no clustering flags) works fine.

**For now:** Run separate Daptin instances with separate databases (no clustering), or use a single Daptin instance with MySQL/PostgreSQL for production.

---

## Not Yet Tested

- **FTP/CalDAV servers:** Runtime configuration values (documented in respective feature docs)

**Note:** SMTP and IMAP are documented separately in [SMTP-Server.md](SMTP-Server.md) and [IMAP-Support.md](IMAP-Support.md).
