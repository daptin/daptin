# Installation

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

```bash
# Using SQLite (default)
docker run -p 6336:6336 -p 6443:6443 daptin/daptin

# With persistent storage
docker run -p 6336:6336 -p 6443:6443 \
  -v /path/to/data:/opt/daptin \
  daptin/daptin

# With MySQL
docker run -p 6336:6336 -p 6443:6443 \
  -e DAPTIN_DB_TYPE=mysql \
  -e DAPTIN_DB_CONNECTION_STRING="user:password@tcp(host:3306)/dbname" \
  daptin/daptin
```

## Docker Compose

```yaml
version: '3'
services:
  daptin:
    image: daptin/daptin
    ports:
      - "6336:6336"
      - "6443:6443"
    volumes:
      - ./data:/opt/daptin
    environment:
      - DAPTIN_PORT=6336
```

## Kubernetes

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
        image: daptin/daptin
        ports:
        - containerPort: 6336
        - containerPort: 6443
        env:
        - name: DAPTIN_PORT
          value: "6336"
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
    targetPort: 6336
  - name: https
    port: 6443
    targetPort: 6443
```

## Build from Source

```bash
git clone https://github.com/daptin/daptin.git
cd daptin
go build -o daptin main.go
./daptin
```

## Command Line Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-port` | 6336 | HTTP server port |
| `-https_port` | 6443 | HTTPS server port |
| `-db_type` | sqlite3 | Database type: sqlite3, mysql, postgres |
| `-db_connection_string` | daptin.db | Database connection string |
| `-local_storage_path` | ./storage | Local file storage path |
| `-dashboard` | - | Dashboard source path |
| `-runtime` | release | Mode: debug, release, test, profile |
| `-log_level` | info | Logging: debug, info, warn, error, fatal |
| `-profile_dump_path` | - | CPU/heap profile location |
| `-olric_peers` | - | Cluster peers (IP:port list) |
| `-olric_bind_port` | 5336 | Olric cache port |
| `-olric_membership_port` | 5350 | Cluster membership port |

## Database Configuration

### SQLite (Default)

```bash
./daptin -db_type=sqlite3 -db_connection_string=./daptin.db
```

### MySQL

```bash
./daptin -db_type=mysql \
  -db_connection_string="user:password@tcp(localhost:3306)/daptin?charset=utf8mb4&parseTime=True"
```

### PostgreSQL

```bash
./daptin -db_type=postgres \
  -db_connection_string="host=localhost user=daptin password=secret dbname=daptin port=5432 sslmode=disable"
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `DAPTIN_PORT` | HTTP port |
| `DAPTIN_DB_TYPE` | Database type |
| `DAPTIN_DB_CONNECTION_STRING` | Database connection |
| `DAPTIN_GOMAXPROCS` | Go runtime parallelism |
| `DAPTIN_LOG_LOCATION` | Log file path |
| `DAPTIN_SCHEMA_FOLDER` | Custom schema location |
| `DAPTIN_SKIP_CONFIG_FROM_DATABASE` | Force default config |
| `DAPTIN_DISABLE_SMTP` | Disable email server |
| `TZ` | Timezone setting |

## Verify Installation

```bash
# Check health
curl http://localhost:6336/health

# View statistics
curl http://localhost:6336/statistics

# View API documentation
curl http://localhost:6336/openapi.yaml
```
