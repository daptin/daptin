# Caching

In-memory caching with Olric distributed cache.

## Overview

Daptin uses Olric for:
- Session caching
- API response caching
- WebSocket state
- Distributed locking

## Olric Configuration

### Enable Olric

```bash
DAPTIN_OLRIC_ENABLED=true ./daptin
```

### Configuration Options

| Environment Variable | Description | Default |
|---------------------|-------------|---------|
| `DAPTIN_OLRIC_ENABLED` | Enable Olric | false |
| `DAPTIN_OLRIC_PEERS` | Cluster peers | (none) |
| `DAPTIN_OLRIC_PORT` | Olric port | 3320 |

## Single Node Mode

Default mode, no configuration needed:

```bash
./daptin
```

In-memory caching works automatically.

## Cluster Mode

For distributed caching across multiple nodes:

### Node 1

```bash
DAPTIN_OLRIC_ENABLED=true \
DAPTIN_OLRIC_PORT=3320 \
DAPTIN_OLRIC_PEERS="node2:3320,node3:3320" \
./daptin
```

### Node 2

```bash
DAPTIN_OLRIC_ENABLED=true \
DAPTIN_OLRIC_PORT=3320 \
DAPTIN_OLRIC_PEERS="node1:3320,node3:3320" \
./daptin
```

## Cache Usage

### WebSocket State

WebSocket subscriptions are cached for:
- Message delivery
- Presence tracking
- Room membership

### Session Data

User sessions cached for fast authentication.

### API Cache

Response caching for read-heavy endpoints.

## Cache Invalidation

Cache automatically invalidates on:
- Data updates (CREATE, UPDATE, DELETE)
- Schema changes
- Server restart

## Manual Cache Clear

Restart server to clear all caches:

```bash
curl -X POST http://localhost:6336/action/world/restart \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

## TTL (Time-To-Live)

Default cache TTLs:

| Cache Type | TTL |
|------------|-----|
| Session | 24 hours |
| API Response | 5 minutes |
| WebSocket State | Session lifetime |

## Monitoring Cache

Check Olric status via statistics:

```bash
curl http://localhost:6336/statistics \
  -H "Authorization: Bearer $TOKEN"
```

## Performance Benefits

| Operation | Without Cache | With Cache |
|-----------|---------------|------------|
| Auth check | ~10ms | ~1ms |
| Repeated query | ~50ms | ~5ms |
| WebSocket delivery | ~20ms | ~2ms |

## Best Practices

1. **Enable in production** - Significant performance boost
2. **Monitor memory** - Cache uses RAM
3. **Cluster for HA** - Distributed cache for reliability
4. **Tune TTLs** - Balance freshness vs performance
