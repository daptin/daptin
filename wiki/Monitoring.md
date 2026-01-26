# Monitoring

**Tested ✓ 2026-01-26**

System monitoring and health check endpoints for production operations.

## Health Check

### Quick Health (Ping)

```bash
curl http://localhost:6336/ping
```

**Response:**
```
pong
```

### /health - Admin Dashboard

**Purpose:** Browser-accessible admin dashboard for visual monitoring

```bash
curl http://localhost:6336/health
```

**Response:**
- HTTP 200 OK
- Content-Type: text/html
- Full admin UI interface (HTML/JavaScript application)

**Note:** This is the full admin web application, not a simple health check. For programmatic health checks (monitoring scripts, uptime tools), use `/ping` instead.

## Statistics

```bash
curl http://localhost:6336/statistics
```

**Response (Full Structure):**
```json
{
  "cpu": {
    "counts": 10,
    "info": [{
      "cores": 10,
      "modelName": "Apple M1 Max",
      "mhz": 3228
    }],
    "percent": [50.88, 50.29, 38.40, ...]
  },
  "db": {
    "MaxOpenConnections": 1,
    "OpenConnections": 1,
    "InUse": 0,
    "Idle": 1,
    "WaitCount": 0,
    "WaitDuration": 0,
    "MaxIdleClosed": 0,
    "MaxIdleTimeClosed": 0,
    "MaxLifetimeClosed": 0
  },
  "disk": {
    "io": {
      "disk0": {
        "readBytes": 9204122632192,
        "writeBytes": 10038080880640,
        "readCount": 426742310,
        "writeCount": 308656138
      }
    }
  },
  "host": {
    "info": {
      "hostname": "server",
      "os": "darwin",
      "platform": "darwin",
      "kernelVersion": "24.4.0",
      "uptime": 3895867,
      "procs": 538
    },
    "temperatures": [
      {"sensorKey": "PMU tdie1", "temperature": 55.06}
    ],
    "users": null
  },
  "load": {
    "avg": {"load1": 5.07, "load5": 5.95, "load15": 6.64},
    "misc": {"procsRunning": 5, "procsBlocked": 1, "procsTotal": 538}
  },
  "memory": {
    "swap": {"total": 3221225472, "used": 2442330112, "usedPercent": 75.82},
    "virtual": {"total": 34359738368, "used": 25720160256, "usedPercent": 74.86}
  },
  "process": {
    "count": 538,
    "top_processes": [{
      "pid": 1,
      "name": "launchd",
      "cpu_percent": 0,
      "mem_percent": 0
    }]
  },
  "web": {
    "pid": 21477,
    "uptime": "24.352022625s",
    "uptime_sec": 24.352022625,
    "total_status_code_count": {"200": 4},
    "total_count": 4,
    "average_response_time": "35.30001ms",
    "average_response_time_sec": 0.03530001
  }
}
```

### Key Statistics Fields

| Section | Fields | Description |
|---------|--------|-------------|
| cpu | counts, info, percent | CPU cores count and per-core utilization array |
| db | OpenConnections, InUse, Idle, WaitCount | Database pool stats (SQLite: MaxOpenConnections=1) |
| disk | io | Storage I/O metrics (structure varies by OS) |
| host | info, temperatures, users | System info, sensors (macOS has 29+ sensors!) |
| load | avg (1/5/15 min), misc | System load averages and process counts |
| memory | virtual, swap | Memory and swap utilization |
| process | count, top_processes | Total process count (integer) and top processes |
| web | uptime, total_status_code_count, average_response_time | HTTP server metrics and response times |

## Meta Endpoint

**Status:** ⚠️ Currently returns empty response

```bash
curl http://localhost:6336/meta
```

**Response:**
- HTTP 200 OK
- Empty body

**Purpose:** Intended to return API metadata and schema information for all entities.

**Current Status:** May require specific schema setup or is partially implemented. For entity information, use `/api/world` instead.

## OpenAPI Documentation

```bash
curl http://localhost:6336/openapi.yaml
```

Full OpenAPI 3.0 specification.

## Entity Listing

List all entities:

```bash
curl http://localhost:6336/api/world \
  -H "Authorization: Bearer $TOKEN"
```

## Action Listing

List all available actions:

```bash
curl http://localhost:6336/api/action \
  -H "Authorization: Bearer $TOKEN"
```

## Database Status

Check database connection:

```bash
curl http://localhost:6336/api/world?page[size]=1 \
  -H "Authorization: Bearer $TOKEN"
```

Success indicates database is accessible.

## Configuration Access

**Note:** The `/api/_config` endpoint returns the admin dashboard UI (HTML), not configuration data.

Configuration is stored in the `_config` database table. To view configuration:

**Option 1: Direct database query**
```bash
sqlite3 daptin.db "SELECT name, value FROM _config LIMIT 10;"
```

**Option 2: Via standard API (if configured)**
```bash
TOKEN=$(cat /tmp/daptin-token.txt)
# Configuration access requires custom actions or direct database queries
```

**Note:** Configuration typically contains sensitive data (encryption keys, credentials) and is not exposed via simple REST endpoints.

## Logging

### Log Levels

| Level | Description |
|-------|-------------|
| debug | Detailed debugging |
| info | General information |
| warn | Warnings |
| error | Errors |
| fatal | Fatal errors |

### Set Log Level

```bash
./daptin -log_level=debug
```

### Log Location

```bash
DAPTIN_LOG_LOCATION=/var/log/daptin.log ./daptin
```

## Profiling (File-Based)

**Important:** Daptin uses file-based profiling, not HTTP pprof endpoints.

### Enable Profiling

Start Daptin in profile mode with periodic dumps:

```bash
./daptin \
  -runtime=profile \
  -profile_dump_path=/tmp/profiles \
  -profile_dump_period=5
```

**Parameters:**
- `-runtime=profile`: Enable profiling mode
- `-profile_dump_path`: Directory for profile files (default: current directory)
- `-profile_dump_period`: Minutes between dumps (default: 5)

**Generated Files:**
```bash
/tmp/profiles/daptin_<hostname>_profile_cpu.0
/tmp/profiles/daptin_<hostname>_profile_heap.0
/tmp/profiles/daptin_<hostname>_profile_cpu.1
/tmp/profiles/daptin_<hostname>_profile_heap.1
# ... increments with each dump
```

### Analyze CPU Profile

```bash
go tool pprof /tmp/profiles/daptin_*_profile_cpu.0
```

**Interactive Commands:**
```
(pprof) top10               # Show top 10 functions by CPU time
(pprof) list <function>     # Show source code for function
(pprof) web                 # Generate call graph (requires graphviz)
(pprof) pdf                 # Generate PDF call graph
```

### Analyze Memory Profile

```bash
go tool pprof /tmp/profiles/daptin_*_profile_heap.0
```

**Interactive Commands:**
```
(pprof) top10               # Show top 10 memory allocators
(pprof) list <function>     # Show source code
(pprof) inuse_space         # Sort by currently in-use memory
(pprof) alloc_space         # Sort by total allocated memory
```

### Compare Profiles Over Time

```bash
# Compare two CPU profiles to see changes
go tool pprof -base=/tmp/profiles/daptin_*_cpu.0 \
  /tmp/profiles/daptin_*_cpu.1
```

### Production Profiling Tips

**CPU Profiling:**
- Enable only during performance investigation
- 5-10 minute intervals recommended
- Low overhead (~5% CPU)

**Memory Profiling:**
- Useful for detecting memory leaks
- Compare profiles over time to see growth
- Check for goroutine leaks with `alloc_space` view

**Disk Space:**
- Profile files can be large (10-100MB each)
- Set up log rotation or periodic cleanup
- Keep last 24 hours of profiles for analysis

## Metrics to Monitor

### Application Metrics

| Metric | Description |
|--------|-------------|
| Goroutine count | Active goroutines |
| GC pause time | Garbage collection latency |
| Memory usage | Heap allocation |
| Request rate | API requests/second |

### System Metrics

| Metric | Description |
|--------|-------------|
| CPU usage | Processor utilization |
| Memory usage | RAM utilization |
| Disk usage | Storage utilization |
| Network I/O | Traffic in/out |

### Database Metrics

| Metric | Description |
|--------|-------------|
| Connection pool | Active connections |
| Query time | Average query latency |
| Error rate | Failed queries |

## Alerting Recommendations

### Critical Alerts

- Health check fails
- CPU > 90% for 5 minutes
- Memory > 90%
- Disk > 90%
- Error rate spike

### Warning Alerts

- CPU > 70%
- Memory > 70%
- Slow query time
- High goroutine count

## Integration with Monitoring Tools

### Prometheus

Export metrics endpoint or use external exporter.

### Grafana

Create dashboards from:
- `/statistics` endpoint
- Application logs
- Database metrics

### Uptime Monitoring

Monitor `/health` endpoint:
- Expected: 200 OK
- Alert on: 5xx errors, timeouts
