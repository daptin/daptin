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

An administrator can request a graceful restart through
`POST /action/world/restart_daptin`. See [[Admin-Actions#restarting-daptin]] for
the command and readiness checks. Normal resource permission edits invalidate
relevant caches and do not require this step.

## TTL (Time-To-Live)

Default cache TTLs:

| Cache Type | TTL |
|------------|-----|
| Session | 24 hours |
| API Response | 5 minutes |
| WebSocket State | Session lifetime |

## Monitoring Cache

Check Olric membership, partition ownership, and cache counters via statistics:

```bash
curl -H "Authorization: Bearer $ADMIN_TOKEN" -s http://localhost:6336/statistics | jq '.olric'
```

An administrator bearer token is required. See [[Monitoring#statistics]] for
access requirements, fields, and their limits.

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
