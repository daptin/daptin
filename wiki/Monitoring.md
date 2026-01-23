# Monitoring

System monitoring and health check endpoints.

## Health Check

```bash
curl http://localhost:6336/health
```

**Response:**
```json
{
  "status": "ok"
}
```

## Statistics

```bash
curl http://localhost:6336/statistics
```

**Response:**
```json
{
  "cpu": {
    "percent": 15.2,
    "count": 8
  },
  "memory": {
    "used": 256000000,
    "total": 16000000000,
    "percent": 1.6
  },
  "disk": {
    "used": 50000000000,
    "total": 500000000000,
    "percent": 10.0
  },
  "runtime": {
    "goroutines": 45,
    "gc_pause_ns": 1234567,
    "num_gc": 100
  }
}
```

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
