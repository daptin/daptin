# Production Deployment Checklist

**Before deploying Daptin to production, complete ALL items in this checklist.**

⚠️ **The Quick Start uses SQLite + HTTP - NOT PRODUCTION READY!**

---

## Pre-Deployment Checklist

### 1. Database ✅ REQUIRED

**Switch from SQLite to production database:**

#### Option A: PostgreSQL (Recommended)

```bash
# 1. Create PostgreSQL database
createdb daptin
createuser daptin -P  # Set strong password

# 2. Configure Daptin
export DAPTIN_DB_TYPE=postgres
export DAPTIN_DB_CONNECTION_STRING="host=localhost port=5432 user=daptin password=STRONG_PASSWORD dbname=daptin sslmode=require"

# 3. Start Daptin
./daptin
```

#### Option B: MySQL

```bash
# 1. Create MySQL database
mysql -u root -p
CREATE DATABASE daptin CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'daptin'@'localhost' IDENTIFIED BY 'STRONG_PASSWORD';
GRANT ALL PRIVILEGES ON daptin.* TO 'daptin'@'localhost';

# 2. Configure Daptin
export DAPTIN_DB_TYPE=mysql
export DAPTIN_DB_CONNECTION_STRING="daptin:STRONG_PASSWORD@tcp(localhost:3306)/daptin?charset=utf8mb4&parseTime=True"

# 3. Start Daptin
./daptin
```

**Why?** SQLite is single-file, not suitable for high-traffic or multi-server deployments.

See: [[Database-Setup]] for Docker Compose examples

---

### 2. HTTPS/TLS ✅ REQUIRED

**Enable HTTPS for secure communication:**

#### Option A: Let's Encrypt (Free, Automated)

```bash
# Prerequisites:
# - Domain pointing to your server
# - Port 80 accessible for ACME challenge

TOKEN=$(cat /tmp/daptin-token.txt)

curl -X POST http://localhost:6336/action/world/generate_acme_tls_certificate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "hostname": "api.example.com",
      "email": "admin@example.com"
    }
  }'

# Restart Daptin to use certificate
curl -X POST http://localhost:6336/action/world/restart_daptin \
  -H "Authorization: Bearer $TOKEN"
```

#### Option B: Your Own Certificate

```bash
export DAPTIN_TLS_CERT_PATH=/path/to/cert.pem
export DAPTIN_TLS_KEY_PATH=/path/to/key.pem
export DAPTIN_PORT=443
./daptin
```

#### Option C: Nginx Reverse Proxy

```nginx
server {
    listen 443 ssl http2;
    server_name api.example.com;

    ssl_certificate /etc/letsencrypt/live/api.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/api.example.com/privkey.pem;

    # Security headers
    add_header Strict-Transport-Security "max-age=31536000" always;
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;

    location / {
        proxy_pass http://localhost:6336;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # WebSocket support
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}

# Redirect HTTP to HTTPS
server {
    listen 80;
    server_name api.example.com;
    return 301 https://$server_name$request_uri;
}
```

**Why?** HTTP transmits passwords and tokens in plain text - unacceptable in production.

See: [[TLS-Certificates]] for details

---

### 3. Security Hardening ✅ REQUIRED

**Secure your deployment:**

#### Set JWT Secret

```bash
TOKEN=$(cat /tmp/daptin-token.txt)

# Generate strong secret (min 32 characters)
JWT_SECRET=$(openssl rand -base64 32)

curl -X POST http://localhost:6336/_config/backend/jwt.secret \
  -H "Authorization: Bearer $TOKEN" \
  -d "\"$JWT_SECRET\""
```

#### Set Encryption Secret

```bash
# For data encryption at rest
ENCRYPTION_SECRET=$(openssl rand -base64 32)

curl -X POST http://localhost:6336/_config/backend/encryption.secret \
  -H "Authorization: Bearer $TOKEN" \
  -d "\"$ENCRYPTION_SECRET\""
```

#### Enable Rate Limiting

```bash
# Limit to 100 requests/second per IP
curl -X POST http://localhost:6336/_config/backend/limit.rate \
  -H "Authorization: Bearer $TOKEN" \
  -d '100'
```

#### Configure Firewall

```bash
# UFW (Ubuntu)
sudo ufw allow 80/tcp      # HTTP (for Let's Encrypt)
sudo ufw allow 443/tcp     # HTTPS
sudo ufw allow 6336/tcp    # Daptin HTTP (if not using nginx)
sudo ufw deny 5336/tcp     # Block Olric (internal only)
sudo ufw deny 5350/tcp     # Block Olric membership
sudo ufw enable

# iptables
iptables -A INPUT -p tcp --dport 80 -j ACCEPT
iptables -A INPUT -p tcp --dport 443 -j ACCEPT
iptables -A INPUT -p tcp --dport 5336 -j DROP   # Block external Olric
iptables -A INPUT -p tcp --dport 5350 -j DROP
```

#### Review Default Permissions

```bash
# Check default permission for new tables
# Default: 2097151 (full access)
# Production: Consider restrictive defaults

# In schema files:
Tables:
  - TableName: sensitive_data
    DefaultPermission: 16256  # Owner-only by default
```

#### Enable 2FA for Admins

```bash
# Admin accounts should use TOTP
curl -X POST http://localhost:6336/action/user_account/generate_totp_secret \
  -H "Authorization: Bearer $TOKEN"
```

**Why?** Default settings optimize for development speed, not security.

See: [[Authentication]], [[Permissions]]

---

### 4. Monitoring ✅ REQUIRED

**Set up health monitoring:**

#### Health Check

```bash
# Add to monitoring tool (Uptime Robot, Pingdom, etc.)
curl http://api.example.com/ping
# Expected: "pong"
# Frequency: Every 30 seconds
# Alert on: Connection refused, timeout, non-"pong" response
```

#### Statistics Monitoring

```bash
# Monitor system metrics
curl http://api.example.com/statistics | jq '{
  db: .db,
  memory: .memory.virtual.usedPercent,
  cpu: .cpu.percent
}'

# Set up alerts:
# - Database connections > 90% of max
# - Memory usage > 90%
# - CPU usage > 90% for 5 minutes
```

#### Application Logs

```bash
# Configure log location
export DAPTIN_LOG_LOCATION=/var/log/daptin/daptin.log
export DAPTIN_LOG_LEVEL=info  # Don't use 'debug' in production

# Log rotation (logrotate)
cat > /etc/logrotate.d/daptin << EOF
/var/log/daptin/*.log {
    daily
    rotate 30
    compress
    delaycompress
    missingok
    notifempty
    create 0640 daptin daptin
}
EOF
```

**Why?** You need to know when things break before users do.

See: [[Monitoring]] for complete setup

---

### 5. Backups 🔄 REQUIRED

**Implement backup strategy:**

#### Database Backups

```bash
# PostgreSQL daily backup (cron)
0 2 * * * pg_dump -U daptin daptin | gzip > /backups/daptin-$(date +\%Y\%m\%d).sql.gz

# Keep 30 days
0 3 * * * find /backups -name "daptin-*.sql.gz" -mtime +30 -delete

# MySQL daily backup
0 2 * * * mysqldump -u daptin -p${MYSQL_PASSWORD} daptin | gzip > /backups/daptin-$(date +\%Y\%m\%d).sql.gz
```

#### Schema Files (Version Control)

```bash
# Track schema changes in git
git add schema_*.yaml
git commit -m "Add customer table"
git push origin main

# Tag releases
git tag v1.0.0
git push --tags
```

#### File Storage

```bash
# If using local storage
rsync -av /opt/daptin/storage /backups/storage-$(date +%Y%m%d)/

# If using S3/GCS
# Backups handled by cloud provider
# Enable versioning: aws s3api put-bucket-versioning --bucket daptin-files --versioning-configuration Status=Enabled
```

#### Test Restore (Monthly)

```bash
# Create test database
createdb daptin_test

# Restore latest backup
gunzip < /backups/daptin-20260126.sql.gz | psql daptin_test

# Verify data
psql daptin_test -c "SELECT COUNT(*) FROM customer;"

# Document RTO (Recovery Time Objective)
# Target: < 15 minutes
```

**Why?** Data loss = business loss. Test restores regularly.

See: [[Database-Setup]] for backup commands

---

### 6. Performance Tuning ⚙️ RECOMMENDED

**Optimize for production load:**

#### Database Connection Pool

```bash
# Set based on expected concurrency
export DAPTIN_DB_MAX_OPEN_CONNECTIONS=100
export DAPTIN_DB_MAX_IDLE_CONNECTIONS=10
export DAPTIN_DB_CONNECTION_MAX_LIFETIME=3600  # seconds
```

#### Enable Caching

```bash
# Olric distributed cache (built-in)
# Default: enabled
# For multi-server: configure peers

export DAPTIN_OLRIC_PEERS="server1:5336,server2:5336"
```

#### Enable Compression

```bash
TOKEN=$(cat /tmp/daptin-token.txt)

curl -X POST http://localhost:6336/_config/backend/gzip.enable \
  -H "Authorization: Bearer $TOKEN" \
  -d 'true'
```

See: [[Caching]], [[Clustering]]

---

### 7. High Availability (Optional)

**For critical production systems:**

#### Load Balancer (HAProxy)

```
frontend http_front
    bind *:80
    bind *:443 ssl crt /etc/ssl/certs/api.example.com.pem
    default_backend daptin_backend

backend daptin_backend
    balance roundrobin
    option httpchk GET /ping
    http-check expect string pong
    server daptin1 10.0.1.10:6336 check
    server daptin2 10.0.1.11:6336 check
    server daptin3 10.0.1.12:6336 check
```

#### Olric Cluster

```bash
# Server 1
./daptin -olric_peers="server2:5336,server3:5336"

# Server 2
./daptin -olric_peers="server1:5336,server3:5336"

# Server 3
./daptin -olric_peers="server1:5336,server2:5336"
```

#### Database Replication

PostgreSQL streaming replication or MySQL master-slave setup.

See: [[Clustering]]

---

## Production Deployment Examples

### Docker Compose (Production)

```yaml
version: '3.8'

services:
  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_DB: daptin
      POSTGRES_USER: daptin
      POSTGRES_PASSWORD_FILE: /run/secrets/db_password
    secrets:
      - db_password
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./backups:/backups
    restart: unless-stopped
    networks:
      - internal

  daptin:
    image: daptin/daptin:latest
    depends_on:
      - postgres
    environment:
      DAPTIN_DB_TYPE: postgres
      DAPTIN_DB_CONNECTION_STRING: "host=postgres port=5432 user=daptin password_file=/run/secrets/db_password dbname=daptin sslmode=require"
      DAPTIN_DB_MAX_OPEN_CONNECTIONS: 100
      DAPTIN_LOG_LOCATION: /var/log/daptin/daptin.log
      DAPTIN_LOG_LEVEL: info
      TZ: UTC
    secrets:
      - db_password
    volumes:
      - ./schema:/schema:ro
      - ./storage:/opt/daptin/storage
      - ./logs:/var/log/daptin
    restart: unless-stopped
    networks:
      - internal
      - web
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:6336/ping"]
      interval: 30s
      timeout: 5s
      retries: 3
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.daptin.rule=Host(`api.example.com`)"
      - "traefik.http.routers.daptin.tls=true"
      - "traefik.http.routers.daptin.tls.certresolver=letsencrypt"

  traefik:
    image: traefik:v2.10
    command:
      - "--api.dashboard=true"
      - "--providers.docker=true"
      - "--entrypoints.web.address=:80"
      - "--entrypoints.websecure.address=:443"
      - "--certificatesresolvers.letsencrypt.acme.email=admin@example.com"
      - "--certificatesresolvers.letsencrypt.acme.storage=/letsencrypt/acme.json"
      - "--certificatesresolvers.letsencrypt.acme.httpchallenge.entrypoint=web"
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./letsencrypt:/letsencrypt
    networks:
      - web
    restart: unless-stopped

networks:
  internal:
    internal: true
  web:
    external: true

volumes:
  postgres_data:

secrets:
  db_password:
    file: ./secrets/db_password.txt
```

### Kubernetes (Production)

See: [[Installation]] for Kubernetes manifests

---

## Post-Deployment Checklist

After deployment, verify:

- [ ] Database is PostgreSQL or MySQL (not SQLite)
- [ ] HTTPS is enabled and working
- [ ] Health check (`/ping`) returns "pong"
- [ ] Monitoring is configured
- [ ] Backups are running
- [ ] Firewall blocks Olric ports (5336, 5350)
- [ ] JWT secret is set (not default)
- [ ] Encryption secret is set
- [ ] Rate limiting is enabled
- [ ] Admin users have 2FA
- [ ] Logs are being written and rotated
- [ ] Test restore from backup works

---

## Troubleshooting Production Issues

### Database Connection Errors

```bash
# Check connection string
echo $DAPTIN_DB_CONNECTION_STRING

# Test connection manually
psql "$DAPTIN_DB_CONNECTION_STRING"

# Check connection pool
curl http://localhost:6336/statistics | jq '.db'
```

### HTTPS Certificate Issues

```bash
# Verify certificate
openssl s_client -connect api.example.com:443 -servername api.example.com

# Check certificate expiry
curl -vI https://api.example.com 2>&1 | grep "expire"

# Renew Let's Encrypt
curl -X POST http://localhost:6336/action/world/generate_acme_tls_certificate \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"attributes":{"hostname":"api.example.com","email":"admin@example.com"}}'
```

### Performance Issues

```bash
# Check statistics
curl http://localhost:6336/statistics | jq '.'

# Database slow queries (PostgreSQL)
psql daptin -c "SELECT query, calls, total_time, mean_time FROM pg_stat_statements ORDER BY mean_time DESC LIMIT 10;"

# Connection pool exhausted
# Increase DAPTIN_DB_MAX_OPEN_CONNECTIONS
```

---

## Security Incident Response

If compromised:

1. **Immediate Actions:**
   - Rotate JWT secret
   - Rotate all API tokens
   - Change database password
   - Review access logs

2. **Investigation:**
   - Check `/statistics` for unusual activity
   - Review database audit logs
   - Check file system for unauthorized changes

3. **Recovery:**
   - Restore from known-good backup
   - Update security configurations
   - Document incident for future prevention

---

## Maintenance Schedule

### Daily
- [ ] Check health endpoints
- [ ] Review error logs
- [ ] Verify backups completed

### Weekly
- [ ] Review monitoring metrics
- [ ] Check disk space
- [ ] Review security logs

### Monthly
- [ ] Test backup restore
- [ ] Update dependencies
- [ ] Review and rotate old backups
- [ ] Performance review

### Quarterly
- [ ] Security audit
- [ ] Review and update firewall rules
- [ ] Disaster recovery drill

---

## Getting Help

- **Documentation Issues:** https://github.com/daptin/daptin/issues
- **Production Support:** GitHub Discussions
- **Security Issues:** security@daptin.org (if available)

---

**Last Updated:** 2026-01-26
**Tested With:** Daptin v0.9.7
