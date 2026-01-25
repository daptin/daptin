# Daptin Wiki

**Daptin** is a Backend-as-a-Service (BaaS) platform that provides database-driven REST/GraphQL APIs with automatic CRUD generation, user authentication, real-time communication, and enterprise features.

## What is Daptin?

Daptin turns YAML table definitions into a full-featured backend API in seconds:

```
schema.yaml → Daptin → REST/GraphQL API + Auth + Storage + Real-time
```

**You get automatically**:
- ✅ REST API (JSON:API compliant)
- ✅ GraphQL API
- ✅ User authentication (JWT)
- ✅ Permission system
- ✅ File storage (S3, GCS, local)
- ✅ Real-time updates (WebSocket, YJS)
- ✅ Email server (SMTP/IMAP)
- ✅ Custom actions and workflows

## First Time Here?

**Start with one of these**:

1. **Complete Walkthrough** → [Building a Product Catalog](../docs_source/walkthrough-product-catalog-with-permissions.md)
   *Build a real product catalog with cloud storage and permissions (30-45 min, beginner-friendly)*

2. **Quick Start** → [Getting Started Guide](Getting-Started-Guide.md)
   *Get Daptin running in 5 minutes*

3. **Core Concepts** → [Understanding Daptin](Core-Concepts.md)
   *Learn how Daptin works*

## Quick Start (5 Minutes)

```bash
# 1. Download and run
./daptin -port=6336

# 2. Create admin account
curl -X POST http://localhost:6336/action/user_account/signup \
  -H "Content-Type: application/json" \
  -d '{"attributes":{"email":"admin@admin.com","password":"adminadmin","passwordConfirm":"adminadmin","name":"Admin"}}'

# 3. Sign in to get JWT token
TOKEN=$(curl -s -X POST http://localhost:6336/action/user_account/signin \
  -H "Content-Type: application/json" \
  -d '{"attributes":{"email":"admin@admin.com","password":"adminadmin"}}' | \
  jq -r '.[] | select(.ResponseType == "client.store.set") | .Attributes.value')

echo "$TOKEN" > /tmp/daptin-token.txt

# 4. Become administrator (first user only)
curl -X POST http://localhost:6336/action/world/become_an_administrator \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{}'

# Wait 5 seconds, then sign in again (server may restart)
sleep 5
TOKEN=$(curl -s -X POST http://localhost:6336/action/user_account/signin \
  -H "Content-Type: application/json" \
  -d '{"attributes":{"email":"admin@admin.com","password":"adminadmin"}}' | \
  jq -r '.[] | select(.ResponseType == "client.store.set") | .Attributes.value')

echo "$TOKEN" > /tmp/daptin-token.txt
echo "✓ Admin setup complete! Token saved to /tmp/daptin-token.txt"
```

**Next**: Create your first table → [Schema Definition](Schema-Definition.md)

## Common Workflows

**Choose your path based on what you want to do**:

### I want to...

**Build a complete app from scratch**
→ Follow the [Product Catalog Walkthrough](../docs_source/walkthrough-product-catalog-with-permissions.md) (comprehensive tutorial)

**Set up user authentication**
→ [Users and Groups](Users-and-Groups.md) + [Authentication](Authentication.md)

**Upload files to S3/cloud storage**
→ [Cloud Storage](Cloud-Storage.md) + [Asset Columns](Asset-Columns.md)

**Create custom business logic**
→ [Custom Actions](Custom-Actions.md) + [Actions Overview](Actions-Overview.md)

**Control who can access what data**
→ [Permissions](Permissions.md) + [Users and Groups](Users-and-Groups.md)

**Filter and search data**
→ [Filtering and Pagination](Filtering-and-Pagination.md) + [Aggregation API](Aggregation-API.md)

**Send emails from my app**
→ [SMTP Server](SMTP-Server.md) + [Email Actions](Email-Actions.md)

**Build real-time features**
→ [WebSocket API](WebSocket-API.md) + [YJS Collaboration](YJS-Collaboration.md)

---

## Documentation Sections

### Getting Started
- [[Installation]] - Binary, Docker, Kubernetes deployment
- [[Configuration]] - Environment variables, flags, runtime config
- [[Database Setup]] - SQLite, MySQL, PostgreSQL support

### Core Concepts
- [[Schema Definition]] - Getting started with tables (beginner-friendly)
- [[Schema Reference Complete]] - All 18 TableInfo properties (complete reference)
- [[Schema Examples]] - 5 complete working use cases
- [[Column Types]] - All 41 types with decision tree
- [[Column Type Reference]] - Detailed per-type documentation
- [[Permissions]] - Linux FS-like permission model
- [[Users and Groups]] - Authentication and authorization

### Advanced Features
- [[State Machines]] - Workflow automation (⚠️ API bugs: #170, #171)
- [[Audit Logging]] - Automatic change history ✅ Working
- [[Relationships]] - Foreign keys and cascade behavior
- [[Asset Columns]] - File storage (inline and cloud)

### REST API
- [[API Overview]] - JSON:API compliant endpoints
- [[CRUD Operations]] - Create, Read, Update, Delete
- [[Filtering and Pagination]] - Query parameters
- [[Relationships]] - Loading related data
- [[Aggregation API]] - SQL-like aggregations via REST

### Actions
- [[Actions Overview]] - Business logic layer
- [[User Actions]] - Signup, signin, password reset
- [[Admin Actions]] - System management
- [[Data Actions]] - Import, export, schema operations
- [[Cloud Actions]] - Storage operations
- [[Email Actions]] - SMTP and SES integration
- [[Custom Actions]] - Creating your own actions

### Real-time Features
- [[WebSocket API]] - Pub/sub messaging
- [[YJS Collaboration]] - Real-time document editing
- [[Event System]] - Database change events

### Communication Protocols
- [[SMTP Server]] - Email sending and receiving
- [[IMAP Support]] - Email retrieval
- [[CalDAV CardDAV]] - Calendar and contact sync
- [[FTP Server]] - File transfer protocol
- [[RSS Atom Feeds]] - Feed generation

### Storage
- [[Cloud Storage]] - S3, GCS, Dropbox integration
- [[Asset Columns]] - File storage in columns
- [[Subsites]] - Multi-site hosting

### Advanced Features
- [[GraphQL API]] - Auto-generated GraphQL
- [[State Machines]] - FSM for workflow automation
- [[Task Scheduling]] - Cron-like job scheduling
- [[Data Exchange]] - External API integration
- [[Integrations]] - Third-party services

### Security
- [[Authentication]] - JWT tokens, OAuth
- [[TLS Certificates]] - HTTPS, Let's Encrypt
- [[Two-Factor Auth]] - TOTP/OTP support
- [[Encryption]] - Data encryption at rest

### Operations
- [[Monitoring]] - Statistics and health endpoints
- [[Caching]] - Olric distributed cache
- [[Rate Limiting]] - API throttling
- [[Clustering]] - Multi-node deployment

## System Tables

Daptin creates these tables automatically:

| Table | Purpose |
|-------|---------|
| `user_account` | User records |
| `usergroup` | Group definitions |
| `world` | Entity metadata |
| `action` | Available actions |
| `smd` | State machine definitions |
| `timeline` | Audit trail |
| `document` | File storage |
| `site` | Subsite configuration |
| `cloud_store` | Storage backends |
| `mail_server` | Email servers |
| `_config` | Runtime configuration |

## Default Ports

| Port | Protocol | Purpose |
|------|----------|---------|
| 6336 | HTTP | Main API server |
| 6443 | HTTPS | TLS-encrypted API |
| 465/587 | SMTP | Email server |
| 993 | IMAP | Email retrieval |
| 8008 | CalDAV | Calendar sync |
| 21 | FTP | File transfer |

## API Endpoints Summary

| Endpoint | Description |
|----------|-------------|
| `/api/{entity}` | CRUD operations |
| `/api/{entity}/{id}` | Single record operations |
| `/action/{entity}/{action}` | Execute actions |
| `/aggregate/{entity}` | Aggregation queries |
| `/graphql` | GraphQL endpoint |
| `/live` | WebSocket endpoint |
| `/meta` | API metadata |
| `/statistics` | System stats |
| `/health` | Health check |
| `/_config` | Configuration API |
| `/openapi.yaml` | OpenAPI spec |

## Common Issues Quick Fix

| Problem | Quick Solution |
|---------|----------------|
| 403 Forbidden on fresh DB | Kill old processes: `pkill -9 -f daptin`, remove `daptin.db`, restart |
| 403 after permission change | Restart server to clear Olric cache |
| Group members get 403 | Share the **table** (world record) with group, not just records |
| Filter/query doesn't work | Use `curl --get --data-urlencode 'query=[...]'` syntax |
| Action not found | Restart server after creating actions |
| Can't sign up new users | After first admin, use API to create users (signup is locked) |
| File upload fails | Format: `[{name, file, type}]` array, not string |
| Permission ignored on POST | Use POST then PATCH to set permission on join tables |

## Help and Resources

- **Documentation Issues?** Submit at https://github.com/daptin/daptin/issues
- **Questions?** Check [Troubleshooting](Troubleshooting.md) or ask on GitHub Discussions
- **Examples?** See complete walkthrough in `docs_source/` directory

## License

Daptin is licensed under LGPL v3.
