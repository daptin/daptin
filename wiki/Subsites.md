# Subsites

**Tested ✓ 2026-01-26** (with limitations noted)

Host static websites from Daptin with domain-based routing and cloud storage integration.

## Overview

Daptin subsites allow you to:
- Host multiple static websites from a single instance
- Route traffic by domain name (Host header)
- Store site files in any cloud storage provider
- Serve HTML, CSS, JavaScript, and assets
- Enable FTP access for file management (optional)

## Quick Start

### 1. Create a Site

```bash
TOKEN=$(cat /tmp/daptin-token.txt)

curl -X POST http://localhost:6336/api/site \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "site",
      "attributes": {
        "name": "my-website",
        "hostname": "www.example.com",
        "path": "my-website",
        "enable": true,
        "site_type": "static"
      }
    }
  }'
```

### 2. Link to Cloud Storage

```bash
# Get your cloud_store ID
STORE_ID=$(curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/cloud_store" | jq -r '.data[0].id')

# Get your site ID from step 1 response
SITE_ID="YOUR_SITE_ID_HERE"

# Link them
curl -X PATCH "http://localhost:6336/api/site/$SITE_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "site",
      "id": "'$SITE_ID'",
      "relationships": {
        "cloud_store_id": {
          "data": {
            "type": "cloud_store",
            "id": "'$STORE_ID'"
          }
        }
      }
    }
  }'
```

### 3. Upload Website Files

For local storage (default):
```bash
# Files go in: {cloud_store.root_path}/{site.path}/
mkdir -p ./storage/my-website

cat > ./storage/my-website/index.html << 'EOF'
<!DOCTYPE html>
<html>
<head>
    <title>My Website</title>
    <link rel="stylesheet" href="/style.css">
</head>
<body>
    <h1>Welcome to My Website</h1>
    <p>Hosted on Daptin!</p>
</body>
</html>
EOF
```

### 4. Restart Server and Wait for Sync

⚠️ **CRITICAL**: Site routes register on startup, then files sync to cache

```bash
./scripts/testing/test-runner.sh stop
./scripts/testing/test-runner.sh start

# ⚠️ WAIT 15 seconds for initial file sync to complete
sleep 15
```

**Why wait?** Files are copied from `./storage/` to a temp cache directory. This takes ~10-15 seconds after server start.

### 5. Access Your Site

```bash
# Via Host header (for testing)
curl -H "Host: www.example.com" http://localhost:6336/

# Test non-index files
curl -H "Host: www.example.com" http://localhost:6336/style.css

# In production: Configure DNS to point www.example.com to your Daptin server
```

## Site Configuration

### site Table Fields

| Field | Type | Description |
|-------|------|-------------|
| `name` | varchar(100) | Site identifier for admin use |
| `hostname` | varchar(100) | Domain name for routing (e.g., www.example.com) |
| `path` | varchar(100) | Subdirectory in cloud_store root_path |
| `enable` | bool | Enable/disable site serving |
| `ftp_enabled` | bool | Allow FTP access to site files |
| `site_type` | varchar(20) | Site category (default: 'static') |
| `cloud_store_id` | belongs_to | Storage provider for site files |

### Storage Path Resolution

Files are served from:
```
{cloud_store.root_path}/{site.path}/{requested_file}
```

**Example**:
- `cloud_store.root_path`: `./storage`
- `site.path`: `my-website`
- Request: `/index.html`
- File served: `./storage/my-website/index.html`

## Multi-Site Hosting

Host multiple sites on one Daptin instance:

```bash
TOKEN=$(cat /tmp/daptin-token.txt)

# Site 1
curl -X POST http://localhost:6336/api/site \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "site",
      "attributes": {
        "name": "marketing-site",
        "hostname": "marketing.company.com",
        "path": "marketing",
        "enable": true,
        "site_type": "static"
      }
    }
  }'

# Site 2
curl -X POST http://localhost:6336/api/site \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type": application/vnd.api+json" \
  -d '{
    "data": {
      "type": "site",
      "attributes": {
        "name": "docs-site",
        "hostname": "docs.company.com",
        "path": "docs",
        "enable": true,
        "site_type": "static"
      }
    }
  }'
```

**Routing**: Daptin matches incoming `Host` header to `site.hostname` and serves from the corresponding path.

## Production Deployment

### 1. DNS Configuration

Point your domain to Daptin server:
```
A Record: www.example.com → YOUR_SERVER_IP
```

### 2. TLS Certificates

Use Let's Encrypt for HTTPS:
```bash
# Generate certificate
curl -X POST http://localhost:6336/action/world/acme.tls.generate \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "attributes": {
      "domain": "www.example.com",
      "email": "admin@example.com"
    }
  }'
```

See [TLS Certificates](TLS-Certificates.md) for details.

### 3. Cloud Storage for Scale (Recommended)

Use S3/GCS for production sites, especially large ones:

```bash
# Create S3 cloud_store
curl -X POST http://localhost:6336/api/cloud_store \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "cloud_store",
      "attributes": {
        "name": "sites-s3",
        "store_type": "s3",
        "store_provider": "s3",
        "root_path": "sites-s3:my-bucket/sites"
      }
    }
  }'

# Upload files directly to S3 (bypasses Daptin for large uploads)
aws s3 sync ./site-files/ s3://my-bucket/sites/my-site/
```

**Benefits for Large Sites (GB+)**:
- **No upload bottleneck**: Deploy directly to cloud storage
- **Faster sync**: Daptin downloads only on-demand from cloud
- **Bandwidth efficiency**: Files served from edge locations (with CDN)
- **Zero downtime updates**: Update S3, wait for hourly sync (or restart)

See [Cloud Storage](Cloud-Storage.md) for provider setup.

### 4. Performance Optimization for Large Sites

#### Storage Configuration

```bash
# Set temp cache directory with enough space
export DAPTIN_CACHE_FOLDER=/mnt/large-disk/cache

# Start server
./daptin
```

**Disk Space Requirements**:
- Temp cache holds recently accessed files
- Size: ~10-20% of total site size (most-accessed files)
- Example: 15GB site → ~2-3GB cache directory
- Old files auto-evicted as cache fills

#### CDN Integration

For multi-GB sites, use CloudFront/CloudFlare in front of Daptin:

```
Client → CDN (static assets) → S3/GCS
       → Daptin (dynamic content, APIs)
```

**Configuration**:
1. Serve static assets directly from S3 via CDN
2. Route API requests to Daptin
3. Daptin serves SPA index.html only
4. Result: Minimal bandwidth through Daptin server

#### Memory Tuning

For servers hosting multiple large sites:

```bash
# Limit memory per site (example: 500MB max)
# Edit server config or set ulimits
ulimit -v 524288  # 512MB virtual memory limit
```

#### Monitoring Large Site Sync

```bash
# Create systemd service that waits for sync
[Unit]
Description=Daptin Subsite Server
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/daptin
ExecStartPost=/usr/local/bin/wait-for-sync.sh
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

## How File Serving Works

⚠️ **IMPORTANT**: Understanding Daptin's scalable subsite architecture

**Files are NOT served directly from your storage directory**. Daptin uses a smart cache-based architecture designed to handle sites of any size (from KB to multi-GB):

1. **Temp Cache Directory**: Each site gets a UUID-based temp directory in `/tmp` (or `DAPTIN_CACHE_FOLDER`)
2. **Progressive Sync**: Files are copied from `./storage/{site.path}/` to temp cache as needed
3. **Sync Schedule**:
   - First sync: Starts 5 seconds after server start
   - Sync duration: Depends on site size (seconds for small sites, minutes for GB+ sites)
   - Ongoing: Hourly automatic sync to pick up changes
4. **On-Demand Loading**: HTTP requests serve from temp cache; missing files trigger download from cloud storage

**Why This Architecture?**
- ✅ **Scalability**: Handles sites from 1MB to 15GB+ without memory issues
- ✅ **Cloud Storage**: Works seamlessly with S3/GCS without downloading entire site
- ✅ **Progressive Availability**: Site routes active immediately, assets load as needed
- ✅ **Bandwidth Optimization**: Only syncs changed files on hourly schedule

**For Testing**: Small sites (<100MB) sync in ~15 seconds. Large sites may take minutes. Monitor logs for sync completion.

## Operational Characteristics

**Important behaviors to understand** (as of 2026-01-26):

1. **Initial Sync Time**: Site assets sync progressively after server start
   - Small sites (<100MB): ~15-30 seconds
   - Medium sites (100MB-1GB): ~1-5 minutes
   - Large sites (1GB-15GB+): ~5-30 minutes depending on bandwidth
   - **Monitoring**: Check logs for "Temp dir for site sync" messages
   - **Best Practice**: Use health checks or startup scripts that wait for sync completion

2. **Server Restart Required**: New sites/hostname changes need full restart
   - Routes register only at startup
   - No hot-reload for site configuration
   - **Reason**: Ensures all sites have proper route registration and cache initialization

3. **File Update Propagation**: Changes to files in ./storage/ require sync
   - **Automatic**: Hourly sync picks up changes (no action needed)
   - **Immediate**: Restart server to trigger fresh sync
   - **Production**: Design CI/CD to sync directly to cloud storage (S3/GCS) for instant updates

4. **SPA Fallback Behavior**: Missing files serve index.html instead of 404
   - **By Design**: Enables Single Page Applications with client-side routing
   - **Benefit**: React/Vue/Angular apps work out-of-the-box
   - **Traditional Sites**: Ensure all linked files exist to avoid fallback

## Troubleshooting

### Site Returns 404

**Check 1: cloud_store linked?**
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/site/$SITE_ID" | jq .data.attributes.cloud_store_id
# Should NOT be null
```

**Check 2: Site enabled?**
```bash
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/site/$SITE_ID" | jq .data.attributes.enable
# Should be 1 (true)
```

**Check 3: Files exist?**
```bash
# For local storage
ls -la ./storage/{site.path}/
# Should show index.html
```

**Check 4: Server restarted after site creation?**
```bash
./scripts/testing/test-runner.sh stop
./scripts/testing/test-runner.sh start
```

### Wrong Content Served

**Check hostname match**:
```bash
# View all sites
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/site" | jq '.data[].attributes | {name, hostname, path}'
```

Daptin matches `Host` header EXACTLY to `site.hostname`. Ensure DNS or test header matches.

### Permission Denied

**Check file permissions**:
```bash
# Storage directory must be readable by Daptin process
chmod -R 755 ./storage
```

**Check user ownership** in site record matches authenticated user or admin group.

## Use Cases

### 1. Static Marketing Website

```bash
# Upload HTML, CSS, JS, images to ./storage/marketing/
# Access via https://marketing.company.com
```

### 2. Documentation Portal

```bash
# Generate static docs with MkDocs/Docusaurus
# Upload build output to site path
# Access via https://docs.company.com
```

### 3. Landing Pages per Customer

```bash
# Create site per customer: customer1.saas.com, customer2.saas.com
# Customize content per customer
# Single Daptin instance serves all
```

### 4. Blog with Static Site Generator

```bash
# Generate with Hugo/Jekyll/Gatsby
# Upload to Daptin storage
# Serve via custom domain
```

## Monitoring Sync Progress

For production deployments with large sites, monitor sync progress:

### Check Sync Status

```bash
# Watch sync start
./scripts/testing/test-runner.sh logs | grep "Temp dir for site sync"

# Monitor sync progress (rclone output)
./scripts/testing/test-runner.sh logs | grep -A 20 "Starting to copy drive"

# Watch for completion
tail -f /tmp/daptin.log | grep -i "sync\|rclone"
```

### Verify Sync Completion

```bash
# Get temp directory path from logs
TEMP_DIR=$(./scripts/testing/test-runner.sh logs | grep "Temp dir for site sync" | tail -1 | sed 's/.*==>//' | tr -d ' ')

# Check file count (compare to source)
echo "Source files: $(find ./storage/test-site -type f | wc -l)"
echo "Cached files: $(find $TEMP_DIR -type f | wc -l)"

# Check total size
du -sh $TEMP_DIR
```

### Production Deployment: Health Check

For large sites, implement a startup health check:

```bash
#!/bin/bash
# health-check.sh - Wait for site sync before routing traffic

MAX_WAIT=1800  # 30 minutes max for very large sites
ELAPSED=0

echo "Waiting for site sync to complete..."

while [ $ELAPSED -lt $MAX_WAIT ]; do
  # Check if sync has written files to temp directory
  TEMP_DIR=$(grep "Temp dir for site sync" /tmp/daptin.log | tail -1 | sed 's/.*==>//' | tr -d ' ')

  if [ -n "$TEMP_DIR" ] && [ -d "$TEMP_DIR" ]; then
    FILE_COUNT=$(find $TEMP_DIR -type f | wc -l)

    if [ $FILE_COUNT -gt 10 ]; then  # Adjust threshold for your site
      echo "✓ Sync complete - $FILE_COUNT files in cache"
      exit 0
    fi
  fi

  sleep 10
  ELAPSED=$((ELAPSED + 10))
  echo "Still syncing... (${ELAPSED}s elapsed)"
done

echo "⚠ Sync timeout after ${MAX_WAIT}s"
exit 1
```

## Advanced: Manual File Sync

### Option 1: Server Restart (Recommended)

```bash
./scripts/testing/test-runner.sh stop
./scripts/testing/test-runner.sh start
# Wait time depends on site size (see Monitoring section)
```

### Option 2: Direct Cloud Upload (Production)

For immediate updates without restart, upload directly to cloud storage:

```bash
# AWS S3 example
aws s3 sync ./local-site-files/ s3://your-bucket/site-path/

# Then wait for next hourly sync, or restart server
```

### Finding the Temp Directory

```bash
# Check logs for temp directory path
./scripts/testing/test-runner.sh logs | grep "Temp dir for site sync"

# Example output:
# Temp dir for site sync [localstore]/./storage ==> /tmp/019bf9c8-c6c8-.../

# List files in temp directory
TEMP_DIR=$(./scripts/testing/test-runner.sh logs | grep "Temp dir" | tail -1 | sed 's/.*==>//' | tr -d ' ')
ls -la $TEMP_DIR
```

## Troubleshooting File Serving

### Files Return Index.html Instead of Expected Content

This happens when the sync hasn't completed yet:

**Solution**:
1. Check server uptime - if less than 15 seconds, wait longer
2. Verify files exist in source: `ls -la ./storage/{site.path}/`
3. Check temp directory has files (see "Finding the Temp Directory" above)
4. Restart server to trigger fresh sync

### Files Updated But Changes Don't Appear

**Cause**: Files are cached in temp directory

**Solutions**:
1. **Quick**: Restart server (15-second wait after)
2. **Wait**: Next hourly sync will pick up changes
3. **Verify**: Check temp directory to confirm old files are there

### Sync Appears to Fail (exitcode 1)

You may see `rclone session exitcode - 1` in logs. This is often non-critical:
- Files may still sync correctly
- Check temp directory to verify files exist
- If files are present, sync succeeded despite exit code

## Related

- [Cloud Storage](Cloud-Storage.md) - Storage provider setup
- [Asset Columns](Asset-Columns.md) - File uploads via API
- [TLS Certificates](TLS-Certificates.md) - HTTPS configuration
- [Template Rendering](Template-Rendering.md) - Dynamic content generation (advanced)
