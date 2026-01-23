# Monitoring

System monitoring and health check endpoints.

## Health Check

### Quick Health (Ping)

```bash
curl http://localhost:6336/ping
```

**Response:**
```
pong
```

### Dashboard

The `/health` endpoint redirects to the web dashboard (HTML response).

```bash
curl http://localhost:6336/health
# Returns: HTML dashboard
```

## Statistics

```bash
curl http://localhost:6336/statistics
```

**Response (Full Structure):**
```json
{
  "cpu": {
    "count": 8,
    "info": [{"cpu": 0, "vendorId": "...", "cores": 8}],
    "percent": [15.2]
  },
  "db": {
    "MaxOpenConnections": 50,
    "OpenConnections": 5,
    "InUse": 1,
    "Idle": 4,
    "WaitCount": 0,
    "WaitDuration": 0,
    "MaxIdleClosed": 0,
    "MaxIdleTimeClosed": 0,
    "MaxLifetimeClosed": 0
  },
  "disk": {
    "ioCounters": {"disk0": {...}},
    "partitions": [{"device": "/dev/disk1", "mountpoint": "/"}],
    "usage": {"path": "/", "total": 500000000000, "used": 50000000000, "usedPercent": 10.0}
  },
  "host": {
    "info": {"hostname": "server", "os": "darwin", "platform": "darwin"},
    "temperatures": [],
    "users": []
  },
  "load": {
    "avg": {"load1": 2.5, "load5": 2.1, "load15": 1.8},
    "misc": {"procsRunning": 250, "procsTotal": 600}
  },
  "memory": {
    "swap": {"total": 8000000000, "used": 1000000000},
    "virtual": {"total": 16000000000, "used": 8000000000, "usedPercent": 50.0}
  },
  "process": {
    "count": {"total": 350},
    "top_processes": [{"pid": 1234, "name": "daptin", "cpu": 5.2, "memory": 128000000}]
  },
  "web": {
    "pid": 12345,
    "uptime": "5d 12h 30m",
    "status_codes": {"200": 50000, "404": 100, "500": 5},
    "response_times": {"avg": 25.5, "p95": 100.0}
  }
}
```

### Key Statistics Fields

| Section | Fields | Description |
|---------|--------|-------------|
| cpu | count, percent | CPU cores and utilization |
| db | OpenConnections, InUse, Idle, WaitCount | Database pool stats |
| disk | usage, partitions, ioCounters | Storage metrics |
| host | info, temperatures | System information |
| load | avg (1/5/15 min) | System load averages |
| memory | virtual, swap | Memory utilization |
| process | count, top_processes | Process information |
| web | uptime, status_codes, response_times | HTTP server metrics |

## Meta Endpoint

Get API metadata:

```bash
curl http://localhost:6336/meta
```

Returns schema information for all entities.

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

## Configuration Status

View current configuration:

```bash
curl http://localhost:6336/api/_config \
  -H "Authorization: Bearer $TOKEN"
```

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

## Profiling

### Enable Profiling

```bash
./daptin -runtime=profile -profile_dump_path=/tmp/profiles
```

### CPU Profile

```bash
go tool pprof http://localhost:6336/debug/pprof/profile
```

### Memory Profile

```bash
go tool pprof http://localhost:6336/debug/pprof/heap
```

### Goroutine Profile

```bash
go tool pprof http://localhost:6336/debug/pprof/goroutine
```

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
