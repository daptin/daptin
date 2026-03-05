# Clustering

High availability and horizontal scaling with shared database and Olric distributed cache.

## Overview

Daptin supports clustering for:
- High availability
- Load balancing
- Horizontal scaling
- Distributed caching (Olric)
- Cross-node WebSocket PubSub
- Outbox deduplication via Olric NX claims

## Architecture

```
                   Load Balancer
                        |
         +--------------+--------------+
         |              |              |
      Node 1         Node 2         Node 3
     HTTP:6336       HTTP:6338       HTTP:6340
     Olric:5336      Olric:5338      Olric:5340
     Member:5337     Member:5339     Member:5341
         |              |              |
         +--------------+--------------+
                        |
                   PostgreSQL
                   (shared)
```

## Requirements

1. **Shared Database** — All nodes connect to the same PostgreSQL (or MySQL) instance
2. **Load Balancer** — Distribute HTTP traffic across nodes
3. **Shared Storage** — For file assets (use cloud storage)
4. **Network** — Olric ports (olric_port and olric_port+1 for membership) must be reachable between all nodes

## CLI Flags

| Flag | Description | Example |
|------|-------------|---------|
| `-port` | HTTP API listen port | `:6336` |
| `-db_type` | Database driver | `postgres` |
| `-db_connection_string` | Database DSN | `host=... port=5432 ...` |
| `-olric_peers` | Comma-separated peer list (ip:membership_port) | `10.0.0.1:5337,10.0.0.2:5339` |
| `-olric_port` | Olric port (membership is automatically olric_port+1) | `5336` |
| `-olric_seed` | DNS hostname for peer discovery | `daptin-headless.default.svc.cluster.local` |
| `-olric_env` | Discovery mode: `local`, `lan`, `wan` | `local` |

## Node Configuration

### Node 1 (creates schema on first start)

```bash
go run main.go \
  -port :6336 \
  -db_type postgres \
  -db_connection_string "host=db.example.com port=5432 user=daptin password=pass dbname=daptin sslmode=disable" \
  -olric_peers "10.0.0.1:5337,10.0.0.2:5339,10.0.0.3:5341" \
  -olric_port 5336 \
  -olric_env lan
```

### Node 2

```bash
go run main.go \
  -port :6338 \
  -db_type postgres \
  -db_connection_string "host=db.example.com port=5432 user=daptin password=pass dbname=daptin sslmode=disable" \
  -olric_peers "10.0.0.1:5337,10.0.0.2:5339,10.0.0.3:5341" \
  -olric_port 5338 \
  -olric_env lan
```

### Node 3

```bash
go run main.go \
  -port :6340 \
  -db_type postgres \
  -db_connection_string "host=db.example.com port=5432 user=daptin password=pass dbname=daptin sslmode=disable" \
  -olric_peers "10.0.0.1:5337,10.0.0.2:5339,10.0.0.3:5341" \
  -olric_port 5340 \
  -olric_env lan
```

**Startup order:** Start Node 1 first (it creates the database schema). Wait for it to be healthy before starting Nodes 2 and 3.

## Ports

Each node uses 3 ports:

| Port Type | Default | Purpose |
|-----------|---------|---------|
| HTTP | 6336 | API, WebSocket (`/live`), dashboard |
| Olric | 5336 | Distributed cache data transfer |
| Olric Membership | 5337 (olric_port+1) | Gossip protocol for cluster discovery |

The membership port is always `olric_port + 1` and is derived automatically. You only need to set `-olric_port`.

**All three ports must be reachable between nodes.**

## Distributed Features

### Olric DMap (Key-Value Cache)
- Admin reference IDs (60-minute TTL)
- Permission data
- Subsite cache
- Outbox NX claims for deduplication

### Olric PubSub
- Cross-node WebSocket event propagation
- System topic events (table create/update/delete)
- User-created topic messaging

### Outbox Deduplication
When multiple nodes run `process_outbox`, each mail is claimed via Olric NX (Not-if-eXists) with a 10-minute TTL. Only the node that successfully claims a mail ID processes it.

## DNS-Based Peer Discovery

Instead of listing peers explicitly with `-olric_peers`, you can use `-olric_seed` to discover peers via DNS. Daptin resolves the A records for the given hostname and uses the returned IPs (on the membership port, i.e., `olric_port + 1`) as cluster peers.

This is the preferred approach for container orchestrators where pod IPs are dynamic.

### Kubernetes (Headless Service)

Create a headless service (ClusterIP: None) so that DNS returns individual pod IPs:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: daptin-headless
spec:
  clusterIP: None
  ports:
    - name: olric
      port: 5336
    - name: olric-member
      port: 5337
  selector:
    app: daptin
```

Then configure each Daptin pod to use the headless service for discovery:

```bash
./daptin \
  -port :6336 \
  -db_type postgres \
  -db_connection_string "host=postgres port=5432 user=daptin password=pass dbname=daptin sslmode=disable" \
  -olric_seed "daptin-headless.default.svc.cluster.local" \
  -olric_port 5336 \
  -olric_env lan
```

Daptin resolves `daptin-headless.default.svc.cluster.local` to the set of pod IPs, filters out its own IP, and joins the cluster automatically.

### Docker Compose

Docker Compose DNS resolves a service name to the IPs of all its containers. Use the service name as the seed:

```yaml
version: '3.8'
services:
  daptin:
    image: daptin/daptin
    deploy:
      replicas: 3
    command: >
      -port :6336
      -db_type postgres
      -db_connection_string "host=postgres port=5432 user=daptin password=pass dbname=daptin sslmode=disable"
      -olric_seed "daptin"
      -olric_port 5336
      -olric_env lan
    ports:
      - "6336:6336"

  postgres:
    image: postgres:15
    environment:
      POSTGRES_USER: daptin
      POSTGRES_PASSWORD: pass
      POSTGRES_DB: daptin
```

Each Daptin container resolves `daptin` to the IPs of all replicas and joins the cluster.

### How It Works

1. Daptin resolves all A records for the `-olric_seed` hostname
2. Each resolved IP is paired with the membership port (`olric_port + 1`)
3. The node's own IP is filtered out from the peer list
4. The remaining IPs are used as peers for Olric cluster formation

This replaces the need to manually maintain `-olric_peers` lists. Both flags can coexist -- if both are set, the resolved seed IPs are merged with the explicit peers.

## Docker Swarm

```yaml
version: '3.8'
services:
  daptin:
    image: daptin/daptin
    deploy:
      replicas: 3
      update_config:
        parallelism: 1
        delay: 10s
    command: >
      -port :6336
      -db_type postgres
      -db_connection_string "host=postgres port=5432 user=daptin password=pass dbname=daptin sslmode=disable"
      -olric_seed "daptin"
      -olric_port 5336
      -olric_env lan
    ports:
      - "6336:6336"
    networks:
      - daptin-net

networks:
  daptin-net:
    driver: overlay
```

## Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: daptin
spec:
  replicas: 3
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
            - containerPort: 5336
            - containerPort: 5337
          args:
            - "-port=:6336"
            - "-db_type=postgres"
            - "-db_connection_string=host=postgres port=5432 user=daptin password=pass dbname=daptin sslmode=disable"
            - "-olric_seed=daptin-headless.default.svc.cluster.local"
            - "-olric_port=5336"
            - "-olric_env=lan"
---
apiVersion: v1
kind: Service
metadata:
  name: daptin
spec:
  type: LoadBalancer
  ports:
    - name: http
      port: 6336
      targetPort: 6336
  selector:
    app: daptin
---
# Headless service for DNS-based peer discovery
apiVersion: v1
kind: Service
metadata:
  name: daptin-headless
spec:
  clusterIP: None
  ports:
    - name: olric
      port: 5336
      targetPort: 5336
    - name: olric-member
      port: 5337
      targetPort: 5337
  selector:
    app: daptin
```

## Load Balancer Configuration

### Nginx

```nginx
upstream daptin_cluster {
    least_conn;
    server node1:6336;
    server node2:6336;
    server node3:6336;
}

server {
    listen 80;
    location / {
        proxy_pass http://daptin_cluster;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    # WebSocket support
    location /live {
        proxy_pass http://daptin_cluster;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

### HAProxy

```haproxy
frontend daptin_front
    bind *:80
    default_backend daptin_back

backend daptin_back
    balance roundrobin
    server node1 node1:6336 check
    server node2 node2:6336 check
    server node3 node3:6336 check
```

## Session Affinity

For WebSocket connections, enable sticky sessions:

```nginx
upstream daptin_cluster {
    ip_hash;
    server node1:6336;
    server node2:6336;
}
```

## Health Checks

```bash
curl http://node1:6336/api/world
```

## Cluster Test Suite

A test suite is available at `scripts/testing/` for verifying cluster behavior:

```bash
cd scripts/testing

# Start PostgreSQL + 3 local Daptin nodes + admin bootstrap
./cluster-test-runner.sh bootstrap

# Test 1: Outbox NX claim deduplication across nodes
./cluster-test-outbox-dedup.sh

# Test 2: Cross-node WebSocket PubSub propagation
./cluster-test-websocket-pubsub.sh

# Test 3: Cross-node mail delivery and sync
./cluster-test-mail.sh

# Tear down
./cluster-test-runner.sh stop
```

**Prerequisites:** `docker`, `jq`, `websocat`, `swaks`, `psql`, Go toolchain.

See [Cluster Testing](#cluster-testing) section below for details.

## Cluster Testing

### Port Layout (Local Test)

| Node | HTTP | Olric | Membership |
|------|------|-------|------------|
| 1 | 6336 | 5336 | 5337 |
| 2 | 6338 | 5338 | 5339 |
| 3 | 6340 | 5340 | 5341 |

PostgreSQL: Docker on port 5433 (avoids conflict with local Postgres.app on 5432).

### Test Scripts

| Script | Tests | Status |
|--------|-------|--------|
| `cluster-test-runner.sh` | Cluster lifecycle (PG + 3 nodes) | Working |
| `cluster-test-outbox-dedup.sh` | Olric NX outbox deduplication | 5/5 PASS |
| `cluster-test-websocket-pubsub.sh` | Cross-node WebSocket events | 6/6 PASS |
| `cluster-test-mail.sh` | SMTP delivery + cross-node sync | 0/5 (SMTP listener bug) |

### Verified Cross-Node Features

The following features have been verified working across a 3-node cluster:

- **Olric DMap NX claims** — Outbox deduplication: 10 mails split 5/5 across two nodes, zero duplicates
- **Olric PubSub (system topics)** — CRUD events on Node B reach WebSocket subscribers on Node A
- **Olric PubSub (user topics)** — Messages published on Node B reach user topic subscribers on Node A
- **Olric DMap (topic metadata)** — Topics created on Node A are visible on Node B

### Known Issues

1. **SMTP Listener** — The SMTP server logs "Started mail server" but the port does not actually open. The `guerrillad.Start()` call returns nil but the listener goroutine may fail silently.

2. **`emb.Start()` timeout warning** — Olric's `Start()` is a blocking server loop that never returns. The 10-second timeout in `main.go` always fires — this is expected, not a bug. The cluster forms correctly after the timeout.

## Troubleshooting

### Olric Cluster Not Forming

**Symptoms:**
- Each node logs `Forming a new Olric cluster` independently
- NX claims don't prevent duplicate processing across nodes
- WebSocket events don't propagate cross-node

**Note:** `Olric start timeout, proceeding anyway` is normal — `emb.Start()` is a blocking server loop. The cluster still forms correctly after the timeout.

**Diagnosis:**
```bash
# Check for isolated clusters — only 1 node should form
grep "Forming a new Olric cluster" /tmp/daptin-node*.log

# Check filtered peers (each node should list 2 peers, not itself)
grep "Olric peers (filtered)" /tmp/daptin-node*.log

# Check for peer joins via PubSub
grep "Joining from" /tmp/daptin-node*.log
```

**Resolution:**
- Verify `-olric_peers` uses **membership ports** (e.g., `10.0.0.1:5337`, which is `olric_port + 1`), NOT the olric_port itself
- Consider using `-olric_seed` for automatic DNS-based discovery instead of manual peer lists
- Verify peers use the **actual interface IP** (not `127.0.0.1`) — Olric resolves `0.0.0.0` to the primary interface
- Ensure all Olric ports (olric_port and olric_port+1) are reachable between nodes
- Start Node 1 first, wait for it to be fully ready, then start others
- Use `-olric_env lan` for local network, `-olric_env wan` for cross-datacenter

### Stale Olric Cache (403 / Unauthorized)

See the [Caching](Caching.md) page. Key point: kill **both** the HTTP port and Olric bind port when restarting.

```bash
lsof -ti:6336 | xargs kill -9  # HTTP
lsof -ti:5336 | xargs kill -9  # Olric cache (CRITICAL)
```

### Split Brain

If nodes can't communicate:
1. Check network connectivity between all Olric ports
2. Verify `-olric_peers` lists include all nodes
3. Check firewall rules for both olric_port and olric_port+1 (membership)
