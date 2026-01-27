# Installation

**Tested ✓** 2026-01-27 on macOS Darwin 24.4.0 (darwin/arm64) with Go 1.24.3

## Prerequisites

Before installing Daptin:
- **Go 1.19+** (for building from source)
- **SQLite3** (built-in, no external setup needed)
- **MySQL/PostgreSQL** (optional, for production databases)
- **Docker** (optional, for containerized deployment)

## Native Binary

Download the latest release from [GitHub Releases](https://github.com/daptin/daptin/releases).

```bash
# Linux
wget https://github.com/daptin/daptin/releases/latest/download/daptin-linux-amd64
chmod +x daptin-linux-amd64
./daptin-linux-amd64

# macOS
wget https://github.com/daptin/daptin/releases/latest/download/daptin-darwin-amd64
chmod +x daptin-darwin-amd64
./daptin-darwin-amd64

# Windows
# Download daptin-windows-amd64.exe and run
```

## Docker

**Note**: The container listens on port 8080 internally, so map host port 6336 to container port 8080.

```bash
# Using SQLite (default) - TESTED ✓
docker run -p 6336:8080 -p 6443:6443 daptin/daptin:v0.9.82

# With persistent storage - TESTED ✓
docker run -p 6336:8080 -p 6443:6443 \
  -e DAPTIN_DB_CONNECTION_STRING=/data/daptin.db \
  -v /path/to/data:/data \
  daptin/daptin:v0.9.82

# With MySQL/MariaDB - TESTED ✓
# First start MariaDB
docker run -d --name mysql \
  -e MARIADB_ROOT_PASSWORD=rootpass \
  -e MARIADB_DATABASE=daptindb \
  -e MARIADB_USER=daptinuser \
  -e MARIADB_PASSWORD=daptinpass \
  mariadb:10.11

# Then start Daptin
docker run -p 6336:8080 -p 6443:6443 \
  --link mysql:mysql \
  -e DAPTIN_DB_TYPE=mysql \
  -e DAPTIN_DB_CONNECTION_STRING="daptinuser:daptinpass@tcp(mysql:3306)/daptindb?charset=utf8mb4&parseTime=True" \
  daptin/daptin:v0.9.82

# With PostgreSQL - TESTED ✓
# First start PostgreSQL
docker run -d --name postgres \
  -e POSTGRES_PASSWORD=pgpass \
  -e POSTGRES_DB=daptindb \
  -e POSTGRES_USER=daptinuser \
  postgres:15

# Then start Daptin
docker run -p 6336:8080 -p 6443:6443 \
  --link postgres:postgres \
  -e DAPTIN_DB_TYPE=postgres \
  -e DAPTIN_DB_CONNECTION_STRING="host=postgres user=daptinuser password=pgpass dbname=daptindb port=5432 sslmode=disable" \
  daptin/daptin:v0.9.82
```

**Important Notes**:
- Always specify the image tag (e.g., `v0.9.82`) - there is no `latest` tag
- Port mapping is `6336:8080` (host:container), not `6336:6336`
- For persistent storage, mount to `/data` and set `DAPTIN_DB_CONNECTION_STRING=/data/daptin.db`
- MySQL 8.0 may fail with OOM errors; use MariaDB 10.11 instead

## Docker Compose

**TESTED ✓**

```yaml
version: '3'
services:
  daptin:
    image: daptin/daptin:v0.9.82
    ports:
      - "6336:8080"  # Map host 6336 to container 8080
      - "6443:6443"
    volumes:
      - ./data:/data  # Persist database
    environment:
      - DAPTIN_PORT=:8080  # Container listens on 8080
      - DAPTIN_DB_CONNECTION_STRING=/data/daptin.db
```

**Usage**:
```bash
# Start
docker compose up -d

# View logs
docker compose logs -f

# Stop
docker compose down
```

## Kubernetes

**Not tested** - Provided as reference based on Docker configuration.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: daptin
spec:
  replicas: 1
  selector:
    matchLabels:
      app: daptin
  template:
    metadata:
      labels:
        app: daptin
    spec:
      containers:
      - name: daptin
        image: daptin/daptin:v0.9.82
        ports:
        - containerPort: 8080  # Daptin listens on 8080
        - containerPort: 6443
        env:
        - name: DAPTIN_PORT
          value: ":8080"
        - name: DAPTIN_DB_TYPE
          value: "sqlite3"
        - name: DAPTIN_DB_CONNECTION_STRING
          value: "/data/daptin.db"
        volumeMounts:
        - name: data
          mountPath: /data
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: daptin-pvc
---
apiVersion: v1
kind: Service
metadata:
  name: daptin
spec:
  selector:
    app: daptin
  ports:
  - name: http
    port: 6336
    targetPort: 8080  # Map to container's 8080
  - name: https
    port: 6443
    targetPort: 6443
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: daptin-pvc
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
```

**Note**: You'll need to create the PersistentVolumeClaim and configure it according to your cluster's storage class.

## Build from Source

```bash
git clone https://github.com/daptin/daptin.git
cd daptin
go build -o daptin main.go

# Create storage directories (required for YJS and file uploads)
mkdir -p ./storage/yjs-documents

# Start Daptin
./daptin
```

**Note**: The binary will be approximately 200MB in size. Build time depends on your system (~1-2 minutes on modern hardware).

## Command Line Flags

Run `./daptin -h` to see all available flags.

| Flag | Default | Description |
|------|---------|-------------|
| `-port` | :6336 | HTTP server port (includes colon) |
| `-https_port` | :6443 | HTTPS server port (includes colon) |
| `-db_type` | sqlite3 | Database type: sqlite3, mysql, postgres |
| `-db_connection_string` | daptin.db | Database connection string |
| `-local_storage_path` | ./storage | Local file storage path |
| `-dashboard` | daptinweb | Dashboard source path |
| `-runtime` | release | Mode: debug, release, test, profile |
| `-log_level` | info | Logging: debug, trace, info, warn, error, fatal |
| `-profile_dump_path` | ./ | CPU/heap profile location |
| `-profile_dump_period` | 5 | Profile dump interval (minutes) |
| `-olric_peers` | (empty) | Cluster peers (IP:port list) |
| `-olric_bind_port` | 5336* | Olric cache port |
| `-olric_membership_port` | 5350* | Cluster membership port |
| `-olric_env` | local | Environment: local, lan, wan |

\* Runtime defaults to these values when flag is not set

## Database Configuration

### SQLite (Default) - TESTED ✅

```bash
./daptin -db_type=sqlite3 -db_connection_string=./daptin.db
```

**Note**: Database file is created automatically if it doesn't exist.

### MySQL/MariaDB - TESTED ✅

```bash
./daptin -db_type=mysql \
  -db_connection_string="user:password@tcp(localhost:3306)/daptin?charset=utf8mb4&parseTime=True"
```

**Important**:
- MySQL 8.0 may fail with OOM errors during container initialization
- **Recommended**: Use MariaDB 10.11 instead (fully compatible, more stable)
- See [Server-Configuration.md](Server-Configuration.md) for Docker setup

### PostgreSQL - TESTED ✅

```bash
./daptin -db_type=postgres \
  -db_connection_string="host=localhost user=daptin password=secret dbname=daptin port=5432 sslmode=disable"
```

**Note**: Works with PostgreSQL 15. See [Server-Configuration.md](Server-Configuration.md) for Docker setup.

## Environment Variables

All command-line flags can be set via environment variables (see `-h` for full list).

| Variable | Tested | Description | Example |
|----------|--------|-------------|---------|
| `DAPTIN_PORT` | ✅ | HTTP port | `:8080` or `:6336` |
| `DAPTIN_HTTPS_PORT` | | HTTPS port | `:6443` |
| `DAPTIN_DB_TYPE` | ✅ | Database type | `sqlite3`, `mysql`, `postgres` |
| `DAPTIN_DB_CONNECTION_STRING` | ✅ | Database connection | See Database Configuration below |
| `DAPTIN_LOG_LEVEL` | ✅ | Log verbosity | `debug`, `info`, `warn`, `error` |
| `DAPTIN_RUNTIME` | | Runtime mode | `release`, `debug`, `test`, `profile` |
| `DAPTIN_LOCAL_STORAGE_PATH` | | File storage path | `./storage` |
| `DAPTIN_OLRIC_BIND_PORT` | | Cache server port | `5336` |
| `DAPTIN_OLRIC_PEERS` | | Cluster peers | `ip1:port1,ip2:port2` |
| `TZ` | ✅ | Timezone | `America/Los_Angeles`, `UTC` |

**Additional environment variables** (documented in [Server-Configuration.md](Server-Configuration.md)):
- `DAPTIN_GOMAXPROCS` - Go runtime parallelism
- `DAPTIN_LOG_LOCATION` - Log file path
- `DAPTIN_LOG_MAX_SIZE` - Max log file size (MB)
- `DAPTIN_LOG_MAX_BACKUPS` - Number of log backups to keep
- `DAPTIN_LOG_MAX_AGE` - Max age of log files (days)
- `DAPTIN_SCHEMA_FOLDER` - Custom schema location
- `DAPTIN_DISABLE_SMTP` - Disable email server

## Verify Installation

Once Daptin is running, verify with these endpoints:

```bash
# View system statistics (includes uptime, DB connections, web stats)
curl http://localhost:6336/statistics

# View API documentation (confirms server is responding)
curl http://localhost:6336/openapi.yaml

# Access admin dashboard (web UI)
curl http://localhost:6336/health
# Note: /health returns HTML dashboard, not a health check response
```

**For monitoring/health checks**: Use `/statistics` endpoint, which returns JSON with server metrics. The `/health` endpoint returns the admin dashboard HTML and is not suitable for automated health checks.

**Verify the server is ready**:
```bash
# Server is ready when this returns JSON
curl -s http://localhost:6336/statistics | jq '.web.uptime'
```

---

## Troubleshooting

### Port Already in Use

**Symptom**: `failed to start cache server: listen tcp :5336: bind: address already in use`

**Cause**: Old Daptin process still running.

**Solution**: Kill the old process and restart:
```bash
# Kill both HTTP API (6336) and Olric cache (5336) ports
lsof -i :6336 -t | xargs kill -9 2>/dev/null || true
lsof -i :5336 -t | xargs kill -9 2>/dev/null || true

# Or use different Olric port
./daptin -olric_bind_port=5346
```

### Failed to Create YJS Storage Directory

**Symptom**: `Failed to create yjs storage directory: mkdir ./storage/yjs-documents: no such file or directory`

**Cause**: Storage directory doesn't exist.

**Solution**: Create storage directories before starting:
```bash
mkdir -p ./storage/yjs-documents
```

### Failed to Create Olric Topic

**Symptom**: `FATAL: failed to create olric topic - no available client found`

**Cause**: Olric cache port conflict from old process.

**Solution**: Kill processes holding port 5336:
```bash
lsof -i :5336 -t | xargs kill -9 2>/dev/null || true
pkill -9 -f daptin 2>/dev/null || true
sleep 2
./daptin
```

### Database Connection Errors

**For MySQL/PostgreSQL setup**, see [Server-Configuration.md](Server-Configuration.md) for:
- Docker-based database setup
- Connection string formats
- Database initialization steps

---

## Next Steps

After installation:
1. [Getting Started Guide](Getting-Started-Guide.md) - Create your first admin user
2. [Schema Definition](Schema-Definition.md) - Define your data models
3. [Server Configuration](Server-Configuration.md) - Configure HTTPS, databases, clustering
