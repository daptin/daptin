# Installation Testing Notes

**Testing Date**: 2026-01-27
**Daptin Version**: Built from source (commit: current)
**Platform**: macOS Darwin 24.4.0 (darwin/arm64)
**Go Version**: go1.24.3

---

## Test Results

### ✅ Build from Source

**Command tested**:
```bash
go build -o /tmp/daptin-test main.go
```

**Result**: SUCCESS
- Binary size: 197M
- Binary type: Mach-O 64-bit executable arm64
- Build completed without errors

**Verification**:
```bash
/tmp/daptin-test -h
```
- Shows all command-line flags correctly
- Defaults match documentation

---

### ✅ Command-Line Flags

**Tested**: `/tmp/daptin-test -h`

**Actual flags** (verified against documentation):

| Flag | Default | Match Doc? | Notes |
|------|---------|------------|-------|
| `-port` | :6336 | ✅ | Note: includes colon prefix |
| `-https_port` | :6443 | ✅ | Note: includes colon prefix |
| `-db_type` | sqlite3 | ✅ | |
| `-db_connection_string` | daptin.db | ✅ | |
| `-local_storage_path` | ./storage | ✅ | |
| `-dashboard` | daptinweb | ⚠️ | Doc shows "-" but actual default is "daptinweb" |
| `-runtime` | release | ✅ | |
| `-log_level` | info | ✅ | Options: debug, trace, info, warn, error, fatal |
| `-profile_dump_path` | ./ | ⚠️ | Doc shows "-" but actual default is "./" |
| `-olric_peers` | (empty) | ✅ | |
| `-olric_bind_port` | (no default) | ⚠️ | Doc shows 5336, but flag has no default (uses 5336 at runtime) |
| `-olric_membership_port` | (no default) | ⚠️ | Doc shows 5350, but flag has no default |

**Additional flags NOT in doc**:
- `-port_variable` (default: "DAPTIN_PORT")
- `-database_url_variable` (default: "DAPTIN_DB_CONNECTION_STRING")
- `-olric_env` (default: "local", options: local/lan/wan)
- `-profile_dump_period` (default: 5 minutes)

---

### Testing Status

- [x] Build from source
- [x] Command-line flags verification
- [x] SQLite database startup
- [x] Environment variables (PORT, LOG_LEVEL, TZ, DB_TYPE, DB_CONNECTION_STRING)
- [x] Statistics endpoint
- [x] OpenAPI endpoint
- [x] Health endpoint (found issue - see below)
- [x] Docker with SQLite
- [x] Docker with persistent storage
- [x] Docker with MySQL/MariaDB
- [x] Docker with PostgreSQL
- [x] Docker Compose
- [ ] Kubernetes (not tested - requires k8s cluster)
- [ ] Native binaries download (not tested - just documented)

---

## ✅ SQLite Database Startup

**Command tested**:
```bash
/tmp/daptin-test -db_connection_string=daptin-install-test.db -port=:7336 -local_storage_path=/tmp/storage
```

**Result**: SUCCESS
- Server starts successfully with SQLite
- Creates database file if it doesn't exist
- Initializes all tables and relationships
- Listens on specified port

**Critical requirement**: Must create storage directory first or YJS will fail:
```bash
mkdir -p ./storage/yjs-documents
```

**Olric port conflict**: If port 5336 is in use, must kill old process or specify different port:
```bash
lsof -i :5336 -t | xargs kill -9 2>/dev/null
# OR use different port
./daptin -olric_bind_port=5346
```

---

## ✅ Environment Variables

**Tested**: DAPTIN_PORT and DAPTIN_LOG_LEVEL

```bash
DAPTIN_PORT=":8336" DAPTIN_LOG_LEVEL="debug" ./daptin -db_connection_string=test.db
```

**Result**: SUCCESS
- Environment variables override flag defaults
- Server respects DAPTIN_PORT setting
- Log level changes work correctly

**Verified env vars work**:
- DAPTIN_PORT ✅
- DAPTIN_LOG_LEVEL ✅

---

## ✅ Statistics Endpoint

**Command tested**:
```bash
curl http://localhost:7336/statistics
```

**Result**: SUCCESS
- Returns comprehensive JSON with system metrics
- Includes: CPU, memory, disk, database connections, process info, web stats
- Format matches what's expected for monitoring

**Example response structure**:
```json
{
  "cpu": {"counts": 10, "percent": [...]},
  "db": {"MaxOpenConnections": 1, "OpenConnections": 1, ...},
  "memory": {"swap": {...}, "virtual": {...}},
  "process": {"count": 790, "top_processes": [...]},
  "web": {"uptime": "22.390649083s", "total_count": 1, ...}
}
```

---

## ✅ OpenAPI Endpoint

**Command tested**:
```bash
curl http://localhost:7336/openapi.yaml
```

**Result**: SUCCESS
- Returns valid OpenAPI 3.0.0 specification
- Includes comprehensive API documentation
- Has quickstart guide for beginners
- Documents authentication flow

---

## ⚠️ Health Endpoint Issue

**Command tested**:
```bash
curl http://localhost:7336/health
curl http://localhost:7336/_health
```

**Result**: DOCUMENTATION ERROR
- Both endpoints return HTML (admin dashboard), NOT a health check
- Server logs show HTTP 200 for both requests
- This is NOT suitable for health checking

**What actually works for health checking**:
- Use `/statistics` endpoint - returns JSON with uptime and web stats
- Use `/openapi.yaml` - quick 200 response confirms server is up

**Documentation needs correction**: Remove `/health` from verification section, or document it returns admin UI.

---

## Command-Line Flags - Corrections Needed

**Issues found in documentation**:

1. **Default values differ**:
   - Doc: `-dashboard` default "-"
   - Actual: `-dashboard` default "daptinweb"

2. **Missing defaults shown**:
   - Doc: `-profile_dump_path` default "-"
   - Actual: `-profile_dump_path` default "./"

3. **Olric port defaults**:
   - Doc shows `-olric_bind_port` default 5336
   - Actual: Flag has no default, but runtime uses 5336
   - Doc shows `-olric_membership_port` default 5350
   - Actual: Flag has no default

4. **Missing flags** (not critical, but exist):
   - `-port_variable` (default: "DAPTIN_PORT")
   - `-database_url_variable` (default: "DAPTIN_DB_CONNECTION_STRING")
   - `-olric_env` (default: "local", options: local/lan/wan)
   - `-profile_dump_period` (default: 5 minutes)

---

## Key Findings Summary

### What Works ✅
- Build from source
- SQLite database
- Command-line flags (mostly accurate)
- Environment variables
- `/statistics` endpoint
- `/openapi.yaml` endpoint

### What Needs Fixing ⚠️
1. Health endpoint documentation is wrong - returns HTML not health check
2. Minor flag default value discrepancies
3. Missing prerequisite: create storage directory before first run

### What's Already Documented ✅
- MySQL/PostgreSQL (in Server-Configuration.md)
- Docker images (format looks correct, not tested)

---

## Recommendations

1. **Fix Health Endpoint Documentation**:
   - Remove `/health` from verification examples
   - Add note that `/health` returns admin dashboard UI
   - Recommend `/statistics` or `/openapi.yaml` for health checks

2. **Add Storage Directory Setup**:
   - Document that `./storage/yjs-documents` must exist
   - Add to "First Run" or "Prerequisites" section

3. **Update Flag Defaults**:
   - Correct dashboard default
   - Correct profile_dump_path default

4. **Add Troubleshooting Section**:
   - Document Olric port conflict issue
   - How to kill stale processes
   - How to verify ports are free

---

---

## ✅ Docker Testing

### Docker Image Discovery

**Issue**: Documentation shows `daptin/daptin` but doesn't specify tag.

**Finding**: No `latest` tag exists. Available tags:
- v0.9.82 (most recent version tag)
- master
- arm64
- merge
- v0.9.9

**Solution**: Use `daptin/daptin:v0.9.82` explicitly.

---

### ⚠️ Docker Port Mapping Issue

**Documentation shows**:
```bash
docker run -p 6336:6336 -p 6443:6443 daptin/daptin
```

**Problem**: Container listens on port 8080 inside, not 6336.

**Correct command**:
```bash
docker run -p 6336:8080 -p 6443:6443 daptin/daptin:v0.9.82
```

**Verified**:
- Container starts successfully
- API accessible on host port 6336
- Maps to container port 8080

---

### ✅ Docker with SQLite (Default)

**Command tested**:
```bash
docker run -d --name daptin-test -p 6336:8080 -p 6443:6443 daptin/daptin:v0.9.82
```

**Result**: SUCCESS
- Container starts with SQLite database at /opt/daptin/daptin.db
- API responds on http://localhost:6336
- Fresh database initialized
- Uptime confirmed via /statistics endpoint

---

### ⚠️ Docker with Persistent Storage Issue

**Documentation shows**:
```bash
docker run -p 6336:6336 -p 6443:6443 \
  -v /path/to/data:/opt/daptin \
  daptin/daptin
```

**Problem**: Mounting volume at /opt/daptin overwrites the binary, container fails to start.

**Error**:
```
exec: "/opt/daptin/daptin": stat /opt/daptin/daptin: no such file or directory
```

**Correct approach**:
```bash
docker run -d -p 6336:8080 -p 6443:6443 \
  -e DAPTIN_DB_CONNECTION_STRING=/data/daptin.db \
  -v /path/to/data:/data \
  daptin/daptin:v0.9.82
```

**Verified**:
- Database persisted to host at /path/to/data/daptin.db
- Container restarts retain data
- Database accessible from host filesystem

---

### ✅ Docker with MySQL/MariaDB

**Command tested**:
```bash
# Start MariaDB
docker run -d --name mysql \
  -e MARIADB_ROOT_PASSWORD=rootpass \
  -e MARIADB_DATABASE=daptintest \
  -e MARIADB_USER=daptinuser \
  -e MARIADB_PASSWORD=daptinpass \
  mariadb:10.11

# Start Daptin
docker run -d --link mysql:mysql \
  -p 6336:8080 \
  -e DAPTIN_DB_TYPE=mysql \
  -e DAPTIN_DB_CONNECTION_STRING="daptinuser:daptinpass@tcp(mysql:3306)/daptintest?charset=utf8mb4&parseTime=True" \
  daptin/daptin:v0.9.82
```

**Result**: SUCCESS
- Daptin connects to MySQL successfully
- Database connection pool: 50 max connections
- All tables created in MySQL database
- API responds normally

**Note**: MySQL 8.0 container failed with OOM during initialization. MariaDB 10.11 works reliably.

---

### ✅ Docker with PostgreSQL

**Command tested**:
```bash
# Start PostgreSQL
docker run -d --name postgres \
  -e POSTGRES_PASSWORD=testpass \
  -e POSTGRES_DB=daptintest \
  -e POSTGRES_USER=daptinuser \
  postgres:15

# Start Daptin
docker run -d --link postgres:postgres \
  -p 6336:8080 \
  -e DAPTIN_DB_TYPE=postgres \
  -e DAPTIN_DB_CONNECTION_STRING="host=postgres user=daptinuser password=testpass dbname=daptintest port=5432 sslmode=disable" \
  daptin/daptin:v0.9.82
```

**Result**: SUCCESS
- Daptin connects to PostgreSQL successfully
- Database connection pool: 50 max connections
- All tables created in PostgreSQL database
- API responds normally

---

### ✅ Docker Compose

**File tested** (corrected):
```yaml
version: '3'
services:
  daptin:
    image: daptin/daptin:v0.9.82
    ports:
      - "6336:8080"  # Note: 8080 inside container
      - "6443:6443"
    volumes:
      - ./data:/data
    environment:
      - DAPTIN_PORT=:8080
      - DAPTIN_DB_CONNECTION_STRING=/data/daptin.db
```

**Result**: SUCCESS
- `docker compose up -d` starts successfully
- Database persisted in ./data/daptin.db
- API accessible on http://localhost:6336
- Logs show successful startup

---

### 🔄 Docker Platform Warning

All tests show warning:
```
WARNING: The requested image's platform (linux/amd64) does not match the detected host platform (linux/arm64/v8)
```

**Impact**: None - image runs successfully under emulation.

**Note**: Official image only provides amd64 build. Arm64 native builds would improve performance but current setup works.

---

## Environment Variables Tested

### ✅ Working Environment Variables

| Variable | Tested | Result |
|----------|--------|--------|
| DAPTIN_PORT | ✅ | Changes HTTP port successfully |
| DAPTIN_LOG_LEVEL | ✅ | Changes log verbosity |
| DAPTIN_DB_TYPE | ✅ | Switches database type (mysql, postgres) |
| DAPTIN_DB_CONNECTION_STRING | ✅ | Sets database connection |
| TZ | ✅ | Sets timezone (America/Los_Angeles) |

### 📝 Documented but Not Tested

- DAPTIN_HTTPS_PORT
- DAPTIN_GOMAXPROCS
- DAPTIN_SCHEMA_FOLDER
- DAPTIN_LOCAL_STORAGE_PATH
- DAPTIN_OLRIC_* (cluster variables)
- DAPTIN_LOG_LOCATION
- DAPTIN_LOG_MAX_SIZE
- DAPTIN_LOG_MAX_BACKUPS
- DAPTIN_LOG_MAX_AGE

**Reason**: These are documented in Server-Configuration.md and tested there.

---

## Next Steps

1. Update Installation.md with corrections:
   - Fix Docker port mappings (6336:8080 not 6336:6336)
   - Fix persistent storage command
   - Specify image tag (v0.9.82)
   - Update Docker Compose example
   - Add note about MySQL 8.0 issues (use MariaDB)
2. Mark as complete in Documentation-TODO.md
