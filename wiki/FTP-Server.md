# FTP Server

**Tested ✓ 2026-01-26** - All features verified end-to-end including FTPS/TLS.

Host site files via FTP/FTPS with site-based access control and automatic TLS encryption.

## Overview

Daptin includes an FTP/FTPS server that provides file access to subsites. Features:

- **Site-based access**: Each FTP-enabled site appears as a directory
- **Daptin authentication**: Login with your Daptin username (email) and password
- **Automatic FTPS/TLS**: Encryption enabled automatically using site certificates
- **Full file operations**: Upload, download, delete, create directories
- **Default port**: 2121

## Prerequisites

Before using FTP, you must have:

1. **Cloud storage configured** - See [Cloud Storage](Cloud-Storage.md)
2. **At least one site with FTP enabled** - See [Subsites](Subsites.md)
3. **FTP enabled in configuration** - Set `ftp.enable` to `true`

## Quick Start

### Step 1: Enable FTP Server

FTP is disabled by default. Enable it via the configuration:

```bash
TOKEN=$(cat /tmp/daptin-token.txt)

# Enable FTP (stored as value in _config table)
sqlite3 daptin.db "INSERT OR REPLACE INTO _config (name, value, configtype, configstate, configenv, created_at) VALUES ('ftp.enable', 'true', 'backend', 'enabled', 'release', datetime('now'));"
```

**⚠️ Note**: Config API currently returns HTML instead of JSON. Use direct database update as shown above.

### Step 2: Create Cloud Store

```bash
# Create storage directory
mkdir -p /tmp/ftp-storage

# Create cloud_store via API
curl -X POST http://localhost:6336/api/cloud_store \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "cloud_store",
      "attributes": {
        "name": "ftp-storage",
        "store_type": "local",
        "store_provider": "local",
        "root_path": "/tmp/ftp-storage",
        "store_parameters": "{}"
      }
    }
  }'
```

### Step 3: Create Site with FTP Enabled

```bash
# Create site directory
mkdir -p /tmp/ftp-storage/mysite

# Create site with ftp_enabled=true
STORE_ID="YOUR_CLOUD_STORE_ID"
curl -X POST http://localhost:6336/api/site \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "site",
      "attributes": {
        "name": "my-ftp-site",
        "hostname": "ftp.example.com",
        "path": "mysite",
        "enable": true,
        "ftp_enabled": true,
        "site_type": "static"
      },
      "relationships": {
        "cloud_store_id": {
          "data": {"type": "cloud_store", "id": "'$STORE_ID'"}
        }
      }
    }
  }'
```

### Step 4: Restart Server

**CRITICAL**: FTP server only starts if sites with `ftp_enabled=true` exist.

```bash
./scripts/testing/test-runner.sh stop
./scripts/testing/test-runner.sh start

# Verify FTP is listening
lsof -i :2121
```

### Step 5: Connect via FTP

**Using Command Line FTP** (if available):
```bash
ftp localhost 2121
# Username: admin@admin.com
# Password: adminadmin
```

**Using Python**:
```python
from ftplib import FTP

ftp = FTP()
ftp.connect('localhost', 2121)
ftp.login('admin@admin.com', 'adminadmin')

# Navigate to your site
ftp.cwd('/ftp.example.com')

# Upload file
with open('local.txt', 'rb') as f:
    ftp.storbinary('STOR remote.txt', f)

# Download file
with open('downloaded.txt', 'wb') as f:
    ftp.retrbinary('RETR remote.txt', f.write)

ftp.quit()
```

**Using FTPS (TLS)**:
```python
from ftplib import FTP_TLS

ftps = FTP_TLS()
ftps.connect('localhost', 2121)
ftps.auth()  # Enable TLS
ftps.login('admin@admin.com', 'adminadmin')
ftps.prot_p()  # Secure data connection

ftps.cwd('/ftp.example.com')
# ... file operations ...

ftps.quit()
```

---

## Configuration

### FTP Settings

| Setting | Description | Default |
|---------|-------------|---------|
| `ftp.enable` | Enable/disable FTP server | `false` |
| `ftp.listen_interface` | Interface and port to listen on | `0.0.0.0:2121` |

**Set via database**:
```bash
sqlite3 daptin.db "INSERT OR REPLACE INTO _config (name, value, configtype, configstate, configenv, created_at) VALUES ('ftp.listen_interface', '0.0.0.0:2121', 'backend', 'enabled', 'release', datetime('now'));"
```

### Site Configuration

To enable FTP for a site, set `ftp_enabled: true` when creating or updating the site:

```bash
curl -X PATCH "http://localhost:6336/api/site/$SITE_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "site",
      "id": "'$SITE_ID'",
      "attributes": {
        "ftp_enabled": true
      }
    }
  }'
```

**After changing FTP settings**: Restart the server for changes to take effect.

---

## Directory Structure

### Root Directory

When you connect to FTP, the root directory (`/`) lists all FTP-enabled sites as subdirectories:

```
/
├── site1.example.com/
├── site2.example.com/
└── site3.example.com/
```

### Site Directories

Each site directory maps to the site's cloud storage path:

```
{cloud_store.root_path}/{site.path}/
```

**Example**:
- `cloud_store.root_path`: `/tmp/ftp-storage`
- `site.path`: `mysite`
- FTP path: `/site.hostname/`
- Actual files: `/tmp/ftp-storage/mysite/`

---

## File Operations

### Upload File

```python
from ftplib import FTP

ftp = FTP()
ftp.connect('localhost', 2121)
ftp.login('admin@admin.com', 'adminadmin')
ftp.cwd('/ftp.example.com')

# Binary mode upload
with open('image.jpg', 'rb') as f:
    ftp.storbinary('STOR image.jpg', f)

# Text mode upload
with open('index.html', 'r') as f:
    ftp.storlines('STOR index.html', f)

ftp.quit()
```

### Download File

```python
from ftplib import FTP

ftp = FTP()
ftp.connect('localhost', 2121)
ftp.login('admin@admin.com', 'adminadmin')
ftp.cwd('/ftp.example.com')

# Binary mode download
with open('downloaded.jpg', 'wb') as f:
    ftp.retrbinary('RETR image.jpg', f.write)

# Text mode download
lines = []
ftp.retrlines('RETR index.html', lines.append)
content = '\n'.join(lines)

ftp.quit()
```

### Delete File

```python
ftp.delete('oldfile.txt')
```

### Create Directory

```python
# Create directory in site
ftp.cwd('/ftp.example.com')
ftp.mkd('uploads')

# Upload to subdirectory
with open('file.txt', 'rb') as f:
    ftp.storbinary('STOR uploads/file.txt', f)
```

**Note**: Cannot create directories in root (`/`). This is by design - sites must be created via API.

---

## Authentication

### User Authentication

FTP uses Daptin user accounts for authentication:

- **Username**: User's email address (e.g., `admin@admin.com`)
- **Password**: User's Daptin password

### Permission Model

Users can access FTP if:
1. They have valid Daptin credentials
2. At least one FTP-enabled site exists
3. They have permission to access the site (same as HTTP/API permissions)

**All authenticated users can see all FTP-enabled sites**. Use separate Daptin instances or cloud stores for tenant isolation.

---

## TLS/FTPS Support

FTPS (FTP with TLS) is automatically enabled using the site's certificate.

### Connect with FTPS

```python
from ftplib import FTP_TLS
import ssl

ftps = FTP_TLS()

# Connect and enable TLS
ftps.connect('localhost', 2121)
ftps.auth()  # Negotiate TLS
ftps.login('admin@admin.com', 'adminadmin')

# Secure data connection
ftps.prot_p()

# ... operations ...

ftps.quit()
```

### Certificate Management

Daptin automatically retrieves or generates certificates for FTP-enabled sites:

1. On first FTP connection, looks up certificate for site's hostname
2. If no certificate exists, generates a self-signed certificate
3. For production, configure ACME/Let's Encrypt certificates via [TLS Certificates](TLS-Certificates.md)

**Self-signed certificates**: FTP clients may show warnings. Either:
- Accept the certificate (for testing)
- Configure proper TLS certificates (for production)
- Disable TLS verification in client (not recommended)

---

## Use Cases

### 1. Legacy Application Integration

Connect legacy applications that only support FTP:

```bash
# Configure FTP client in legacy app
Host: your-daptin-server.com
Port: 2121
Username: app-user@example.com
Password: <app-password>
Directory: /app-files.example.com
```

### 2. File Management GUI

Use FTP clients like FileZilla for visual file management:

1. Open FileZilla
2. Host: `sftp://your-server.com`, Port: `2121`
3. Username: your Daptin email
4. Password: your Daptin password
5. Browse and manage site files visually

### 3. Automated Deployments

Deploy static sites via FTP:

```python
import ftplib
import os

def deploy_site(local_dir, ftp_host, ftp_site, user, password):
    ftp = ftplib.FTP()
    ftp.connect(ftp_host, 2121)
    ftp.login(user, password)
    ftp.cwd(f'/{ftp_site}')

    for root, dirs, files in os.walk(local_dir):
        # Create directories
        for dir in dirs:
            rel_path = os.path.relpath(os.path.join(root, dir), local_dir)
            try:
                ftp.mkd(rel_path)
            except:
                pass  # Directory might exist

        # Upload files
        for file in files:
            local_path = os.path.join(root, file)
            remote_path = os.path.relpath(local_path, local_dir)
            with open(local_path, 'rb') as f:
                ftp.storbinary(f'STOR {remote_path}', f)

    ftp.quit()

# Deploy
deploy_site('./build', 'localhost', 'my-site.com', 'admin@admin.com', 'adminadmin')
```

---

## Troubleshooting

### FTP Server Not Starting

**Symptom**: Port 2121 not listening, no "FTP server started" log message.

**Causes**:
1. `ftp.enable` is not set to `"true"`
2. No sites with `ftp_enabled=true` exist
3. Site directory doesn't exist in cloud storage

**Solutions**:

```bash
# Check ftp.enable
sqlite3 daptin.db "SELECT name, value FROM _config WHERE name='ftp.enable';"
# Should show: ftp.enable|true

# Check for FTP-enabled sites
sqlite3 daptin.db "SELECT name, hostname, ftp_enabled FROM site WHERE ftp_enabled=1;"
# Should show at least one site

# Create site directory
mkdir -p /path/to/cloud_store/site_path/

# Restart server
./scripts/testing/test-runner.sh stop && ./scripts/testing/test-runner.sh start

# Verify FTP port
lsof -i :2121
```

### Cannot Connect to FTP

**Symptom**: Connection refused or timeout.

**Check firewall**:
```bash
# Verify port is listening
lsof -i :2121

# Check if accessible externally (replace with your IP)
telnet your-server-ip 2121
```

**Check FTP interface**:
```bash
sqlite3 daptin.db "SELECT value FROM _config WHERE name='ftp.listen_interface';"
# Default: 0.0.0.0:2121 (all interfaces)
# Change to specific IP if needed
```

### Authentication Failed

**Symptom**: "Login incorrect" or "530 Authentication failed".

**Verify credentials**:
```bash
# Check user exists
sqlite3 daptin.db "SELECT email FROM user_account WHERE email='admin@admin.com';"

# Test via HTTP first
curl -X POST http://localhost:6336/action/user_account/signin \
  -H "Content-Type: application/json" \
  -d '{"attributes":{"email":"admin@admin.com","password":"adminadmin"}}'
```

### Empty Directory Listing

**Symptom**: FTP LIST command shows empty directory, but files exist.

**This is a known quirk**. Files ARE accessible, just not shown in LIST:

```python
# LIST may return empty
ftp.retrlines('LIST')  # Shows nothing

# But RETR works fine
ftp.retrbinary('RETR index.html', print)  # Works!
```

**Workaround**: Access files directly by name. Directory listings work for directories you create via FTP.

### Cannot Create Directory in Root

**Symptom**: Error when trying `mkd` in `/`.

**This is by design**. Root directory shows sites only. Create directories inside sites:

```python
# Wrong - fails
ftp.cwd('/')
ftp.mkd('newsite')  # Error

# Right - works
ftp.cwd('/existing-site.com')
ftp.mkd('uploads')  # Success
```

### FTPS Certificate Warnings

**Symptom**: FTP client shows "Certificate not trusted" warning.

**For testing**: Accept the certificate or disable verification:

```python
import ssl
ftps.ssl_version = ssl.PROTOCOL_TLSv1_2
# In production: configure proper certificates
```

**For production**: Configure ACME certificates:
```bash
curl -X POST http://localhost:6336/action/world/acme.tls.generate \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"attributes":{"domain":"ftp.example.com","email":"admin@example.com"}}'
```

---

## Security Considerations

### Access Control

- **All authenticated users can access all FTP-enabled sites**
- For tenant isolation:
  - Use separate Daptin instances, OR
  - Use separate cloud stores with different credentials, OR
  - Implement custom authentication (requires code changes)

### TLS Encryption

- **Always use FTPS in production** to encrypt credentials and file transfers
- Configure proper TLS certificates via ACME for trusted connections
- Self-signed certificates provide encryption but not identity verification

### Port Exposure

- Default port 2121 is non-standard (FTP normally uses 21)
- Configure firewall to restrict FTP access:
  - Allow only from known IPs, OR
  - Use VPN for FTP access, OR
  - Use SSH tunneling: `ssh -L 2121:localhost:2121 user@server`

### Password Security

- FTP sends credentials during authentication
- Use strong passwords for Daptin accounts
- Consider using dedicated service accounts with limited permissions
- Rotate passwords periodically

---

## Performance

### File Upload Speed

FTP performance depends on:
- Network bandwidth between client and server
- Disk I/O speed of cloud storage
- File size (larger files = better throughput)

### Concurrent Connections

Default maximum: 100 concurrent FTP connections.

**To change**:
```go
// In server/ftp_server.go:DaptinFtpServerSettings
MaxConnections: 100,  // Change this value
```

Requires code change and recompilation.

### Passive Mode Ports

FTP uses dynamic passive ports for data transfer. Ensure firewall allows these connections.

**Configure passive port range** (requires code change in `ftp_server.go`):
```go
PassiveTransferPortRange: &server.PortRange{Start: 50000, End: 51000},
```

---

## Limitations

| Feature | Supported | Notes |
|---------|-----------|-------|
| Upload files | ✅ | All file types |
| Download files | ✅ | All file types |
| Delete files | ✅ | |
| Create directories | ✅ | Within sites only |
| Directory listings | ⚠️ | May show empty, but files accessible |
| Rename files | ✅ | |
| File permissions (chmod) | ✅ | |
| Symbolic links | ❌ | Not supported |
| Resume transfers | ❌ | Not supported |
| Site creation via FTP | ❌ | Must use API |
| ASCII mode | ⚠️ | Not fully supported (use binary) |

---

## See Also

- [Subsites](Subsites.md) - Static site hosting (required for FTP)
- [Cloud Storage](Cloud-Storage.md) - Storage configuration (required for FTP)
- [TLS Certificates](TLS-Certificates.md) - ACME/Let's Encrypt setup for FTPS
- [Authentication](Authentication.md) - User authentication details
