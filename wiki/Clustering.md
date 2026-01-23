# Clustering

High availability and horizontal scaling.

## Overview

Daptin supports clustering for:
- High availability
- Load balancing
- Horizontal scaling

## Architecture

```
                   Load Balancer
                        |
         +--------------+--------------+
         |              |              |
      Node 1         Node 2         Node 3
         |              |              |
         +--------------+--------------+
                        |
                   Database
                   (shared)
```

## Requirements

1. **Shared Database** - All nodes connect to same database (MySQL/PostgreSQL)
2. **Load Balancer** - Distribute traffic across nodes
3. **Shared Storage** - For file assets (use cloud storage)
4. **Olric Clustering** - For distributed caching

## Node Configuration

### Node 1

```bash
DAPTIN_DB_TYPE=postgres \
DAPTIN_DB_CONNECTION_STRING="host=db.example.com port=5432 user=daptin password=pass dbname=daptin" \
DAPTIN_OLRIC_ENABLED=true \
DAPTIN_OLRIC_PORT=3320 \
DAPTIN_OLRIC_PEERS="node2:3320,node3:3320" \
./daptin
```

### Node 2

```bash
DAPTIN_DB_TYPE=postgres \
DAPTIN_DB_CONNECTION_STRING="host=db.example.com port=5432 user=daptin password=pass dbname=daptin" \
DAPTIN_OLRIC_ENABLED=true \
DAPTIN_OLRIC_PORT=3320 \
DAPTIN_OLRIC_PEERS="node1:3320,node3:3320" \
./daptin
```

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
    environment:
      DAPTIN_DB_TYPE: postgres
      DAPTIN_DB_CONNECTION_STRING: "host=postgres port=5432 user=daptin password=pass dbname=daptin"
      DAPTIN_OLRIC_ENABLED: "true"
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
            - containerPort: 3320
          env:
            - name: DAPTIN_DB_TYPE
              value: "postgres"
            - name: DAPTIN_DB_CONNECTION_STRING
              valueFrom:
                secretKeyRef:
                  name: daptin-secrets
                  key: db-connection
            - name: DAPTIN_OLRIC_ENABLED
              value: "true"
---
apiVersion: v1
kind: Service
metadata:
  name: daptin
spec:
  type: LoadBalancer
  ports:
    - port: 6336
      targetPort: 6336
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

### Nginx

```nginx
upstream daptin_cluster {
    ip_hash;
    server node1:6336;
    server node2:6336;
}
```

## Shared Storage

Use cloud storage for files:

```bash
curl -X POST http://localhost:6336/api/cloud_store \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "cloud_store",
      "attributes": {
        "name": "shared-storage",
        "store_type": "s3",
        "store_provider": "AWS",
        "root_path": "daptin-files"
      }
    }
  }'
```

## Health Checks

Load balancer health check endpoint:

```bash
curl http://node1:6336/health
```

Expected response:
```json
{"status": "ok"}
```

## Scaling

### Scale Up

Add more nodes with same configuration.

### Scale Down

Remove nodes gracefully:
1. Remove from load balancer
2. Wait for connections to drain
3. Shutdown node

## Monitoring

Monitor all nodes:

```bash
for node in node1 node2 node3; do
  echo "=== $node ==="
  curl http://$node:6336/statistics
done
```

## Troubleshooting

### Split Brain

If nodes can't communicate:
1. Check network connectivity
2. Verify Olric peer configuration
3. Check firewall rules for port 3320

### Inconsistent Data

1. Ensure all nodes use same database
2. Check database connection strings
3. Verify Olric cache sync
