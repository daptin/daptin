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

> **Important:** The site cache is built at server startup. New sites won't be served until you restart Daptin.

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

Upload files to a site via the cloud_store upload_file action:

```bash
# First get your cloud_store_id
CLOUD_STORE_ID=$(curl -s http://localhost:6336/api/cloud_store \
  -H "Authorization: Bearer $TOKEN" | jq -r '.data[0].id')

# Upload a file to the site path
curl -X POST http://localhost:6336/action/cloud_store/upload_file \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"attributes\": {
      \"cloud_store_id\": \"$CLOUD_STORE_ID\",
      \"path\": \"mysite\",
      \"file\": [{
        \"name\": \"index.html\",
        \"file\": \"data:text/html;base64,$(echo -n '<h1>Hello</h1>' | base64)\"
      }]
    }
  }"
```

### List Files

```bash
curl -X POST http://localhost:6336/action/site/list_files \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "site_id": "SITE_REFERENCE_ID",
      "path": "/"
    }
  }'
```

> **Note:** list_files requires the site to be in the site cache. Restart Daptin after creating a new site.

### Get File

```bash
curl -X POST http://localhost:6336/action/site/get_file \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "site_id": "SITE_REFERENCE_ID",
      "path": "/index.html"
    }
  }'
```

Returns file content as base64.

## Cloud Storage Backend

Create a site linked to cloud storage:

```bash
curl -X POST http://localhost:6336/action/cloud_store/create_site \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "cloud_store_id": "CLOUD_STORE_REFERENCE_ID",
      "hostname": "static.example.com",
      "path": "static-site",
      "site_type": "static"
    }
  }'
```

This creates both a site record in the database AND syncs the initial folder structure.

### Sync Site Storage

Sync site files from cloud storage to local cache:

```bash
curl -X POST http://localhost:6336/action/site/sync_storage \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "site_id": "SITE_REFERENCE_ID",
      "cloud_store_id": "CLOUD_STORE_REFERENCE_ID",
      "path": ""
    }
  }'
```

> **Note:** After creating a new site, restart Daptin to load it into the site cache.

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
