# Daptin Wiki

**Daptin** is a Backend-as-a-Service (BaaS) platform that provides database-driven REST/GraphQL APIs with automatic CRUD generation, user authentication, real-time communication, and enterprise features.

## Quick Start

```bash
# Download and run
./daptin -port=6336

# Create user account
curl -X POST http://localhost:6336/action/user_account/signup \
  -H "Content-Type: application/json" \
  -d '{"attributes":{"email":"admin@example.com","password":"password123","name":"Admin"}}'

# Sign in to get JWT token
curl -X POST http://localhost:6336/action/user_account/signin \
  -H "Content-Type: application/json" \
  -d '{"attributes":{"email":"admin@example.com","password":"password123"}}'

# Become administrator (first user only)
curl -X POST http://localhost:6336/action/world/become_an_administrator \
  -H "Authorization: Bearer $TOKEN"
```

## Documentation Sections

### Getting Started
- [[Installation]] - Binary, Docker, Kubernetes deployment
- [[Configuration]] - Environment variables, flags, runtime config
- [[Database Setup]] - SQLite, MySQL, PostgreSQL support

### Core Concepts
- [[Schema Definition]] - Defining tables, columns, relationships
- [[Column Types]] - 100+ supported data types
- [[Permissions]] - Linux FS-like permission model
- [[Users and Groups]] - Authentication and authorization

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

## License

Daptin is licensed under LGPL v3.
