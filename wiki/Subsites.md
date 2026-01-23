# Subsites

Host multiple websites on a single Daptin instance.

## Overview

Subsites allow:
- Multiple websites per instance
- Host-based routing (subdomains)
- Path-based routing (subpaths)
- Static file serving
- Cloud storage backend
- Template support

## Creating a Subsite

```bash
curl -X POST http://localhost:6336/api/site \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "site",
      "attributes": {
        "name": "blog",
        "hostname": "blog.example.com",
        "path": "/blog",
        "site_type": "static",
        "enable": true
      }
    }
  }'
```

## Site Properties

| Property | Description |
|----------|-------------|
| name | Unique site identifier |
| hostname | Domain/subdomain for routing |
| path | Path prefix for routing |
| site_type | static, hugo, etc. |
| enable | Enable/disable site |
| ftp_enabled | Allow FTP access |

## Routing Methods

### Host-Based Routing

Access site via subdomain:

```yaml
hostname: "blog.example.com"
path: "/"
```

Access: `http://blog.example.com/`

### Path-Based Routing

Access site via path:

```yaml
hostname: "example.com"
path: "/blog"
```

Access: `http://example.com/blog/`

### Combined

```yaml
hostname: "docs.example.com"
path: "/api"
```

Access: `http://docs.example.com/api/`

## Static Site Hosting

### Upload Files

Via action:

```bash
curl -X POST http://localhost:6336/action/site/cloudstore_file_upload \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "attributes": {
      "site_id": "SITE_ID",
      "path": "/index.html",
      "file": [{
        "name": "index.html",
        "file": "data:text/html;base64,..."
      }]
    }
  }'
```

### List Files

```bash
curl -X POST http://localhost:6336/action/site/site_file_list \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "attributes": {
      "site_id": "SITE_ID",
      "path": "/"
    }
  }'
```

### Get File

```bash
curl -X POST http://localhost:6336/action/site/site_file_get \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "attributes": {
      "site_id": "SITE_ID",
      "path": "/index.html"
    }
  }'
```

## Cloud Storage Backend

Link site to cloud storage:

```bash
curl -X POST http://localhost:6336/action/cloud_store/cloudstore_site_create \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "attributes": {
      "cloud_store_id": "STORE_ID",
      "site_name": "static-site",
      "hostname": "static.example.com"
    }
  }'
```

### Sync Site Storage

```bash
curl -X POST http://localhost:6336/action/site/site_sync_storage \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "attributes": {
      "site_id": "SITE_ID"
    }
  }'
```

## FTP Access

Enable FTP for site management:

### Enable FTP Server

```bash
curl -X POST http://localhost:6336/_config/backend/ftp.enable \
  -H "Authorization: Bearer $TOKEN" \
  -d 'true'
```

### Enable FTP on Site

```bash
curl -X PATCH http://localhost:6336/api/site/SITE_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "site",
      "id": "SITE_ID",
      "attributes": {
        "ftp_enabled": true
      }
    }
  }'
```

### FTP Connection

```
Host: localhost
Port: 21
Username: site_name
Password: user_password (Daptin user)
```

## Template Support

Sites support Go templates for dynamic content:

### Template Variables

```html
<!DOCTYPE html>
<html>
<head>
    <title>{{.Site.Name}}</title>
</head>
<body>
    <h1>Welcome to {{.Site.Name}}</h1>
    <p>Hostname: {{.Site.Hostname}}</p>
</body>
</html>
```

## Asset Caching

Static assets are cached:

| Type | Cache Duration |
|------|----------------|
| Images | 7 days |
| Videos | 14 days |
| CSS/JS | 1 day |
| HTML | 1 hour |
| Other | 24 hours |

## Multiple Sites Example

```yaml
# Documentation site
- name: docs
  hostname: docs.example.com
  path: /
  site_type: static

# Blog
- name: blog
  hostname: example.com
  path: /blog
  site_type: static

# Marketing site
- name: marketing
  hostname: www.example.com
  path: /
  site_type: static

# API docs
- name: api-docs
  hostname: api.example.com
  path: /docs
  site_type: static
```

## Site Table Schema

| Column | Type | Description |
|--------|------|-------------|
| name | varchar | Site identifier |
| hostname | varchar | Domain/subdomain |
| path | varchar | URL path prefix |
| site_type | varchar | Site type |
| enable | bool | Is active |
| ftp_enabled | bool | FTP access |
| cloud_store_id | reference | Storage backend |

## Permissions

Sites respect Daptin permissions:
- Admin can manage all sites
- Users can manage owned sites
- Guest access configurable per site
