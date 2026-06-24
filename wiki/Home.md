# Daptin Wiki

**Daptin is the batteries-included application server for your next software project.**

Most apps need the same backend foundation: data models, REST and GraphQL APIs,
auth, usergroups, permissions, files, sites, actions, state machines, events,
integrations, LLM routing, metering, caching, auditing, protocols, and
operations.

Use Daptin as the primary backend for a new app, or run it beside an existing
stack as a sidecar for the backend features you are missing.

**Build your app. Let Daptin run and enforce the backend.**

## Start Here

### First Time Users

- **[[Installation]]** (2 min)
- **[[First-Admin-Setup]]** (5 min)
- **[[Getting-Started-Guide]]**

### Building Apps And Filling Backend Gaps

- **[[Daptin-Application-Server-Feature-Map]]** - source-grounded architecture and capability map
- **[[LLM-Providers]]** - OpenAI-compatible provider routing
- **[[API-Metering]]** - plans, quotas, credits, and usage logs
- **[[Integrations]]** - external APIs without hardcoded app secrets

### Understanding The Backend

- **[[Core-Concepts]]**
- **[[Schema-Definition]]**
- **[[Permissions]]**
- **[[Users-and-Groups]]**
- **[[Actions-Overview]]**
- **[[State-Machines]]**

### Complete Tutorial

- **[[Walkthrough-Product-Catalog]]** (30-45 min, tested end-to-end)

### Troubleshooting

- **[[Common-Errors]]**
- **[[Key-Behaviors]]**
- **[[Testing-Onboarding-Journey]]**

## What Daptin Provides

### Data And APIs

- Schema-defined entities with standard columns, relations, validations, and metadata.
- JSON:API CRUD under `/api/{entity}`.
- Optional GraphQL under `/graphql`.
- OpenAPI and metadata through `/openapi.yaml`, `/meta`, and `/jsmodel/{typename}`.
- Filtering, pagination, aggregation, import, export, and generated data.

### Identity And Permissions

- Users, usergroups, group membership, and ownership-aware rows.
- Entity-level and row-level permission checks.
- Guest, owner, and group permission scopes.
- JWT/session auth, signup/signin, password reset, OTP/2FA.
- OAuth as a client and OAuth/OIDC-style provider endpoints.
- Multi-tenant patterns through usergroups, relations, ownership, and row permissions.

### Logic And Workflows

- Buildable actions with input fields, validation, conditions, and outcomes.
- Backend-side action chains for CRUD, rendering, file downloads, and performer execution.
- State machines and state transition tracking.
- Scheduled tasks and data exchange.

### Files, Sites, And Protocols

- rclone-backed local/cloud storage through `cloud_store`.
- Encrypted credentials.
- Asset columns, uploads, file serving, cache headers, ETags, and gzip.
- Static site/subsite hosting for app frontends, blogs, docs, and sites.
- WebSocket events, optional YJS collaboration, streams, RSS/Atom feeds.
- Config-gated SMTP, IMAP, FTP, CalDAV/CardDAV, HTTPS/TLS, CORS, and rate limiting.

### LLM, Integrations, And Product Runtime

- OpenAI-compatible `/v1` endpoints for chat, completions, embeddings, and models.
- LLM provider routing through `llm_provider`.
- OpenAPI-backed third-party integrations under `/integration/{provider}/{operation}`.
- OAuth-token and custom-credential integration execution.
- API plans, members, usage logs, quotas, rate limits, and credit hooks.

### Operations

- Runtime config through `/_config`.
- Health and statistics through `/ping` and `/statistics`.
- Optional audit tables and audit rows.
- Olric-backed cache, PubSub, and clustered rate-limit counters.
- TLS certificate management and background tasks.

## Production Readiness

Before deploying production apps:

- Use PostgreSQL or MySQL/MariaDB instead of development SQLite.
- Set stable JWT and encryption secrets.
- Enable HTTPS/TLS.
- Configure backups and restore tests.
- Use durable storage for files and media.
- Enable only the protocols your app needs.
- Configure monitoring, metering, rate limits, and audit behavior.

See **[[Production-Deployment]]**, **[[Database-Setup]]**, and
**[[TLS-Certificates]]**. For outbound email, see
**[[Production-Mail-Delivery]]**.

## Documentation Sections

### Setup

- [[Installation]]
- [[Configuration]]
- [[Database-Setup]]
- [[Server-Configuration]]
- [[Production-Deployment]]

### Data Modeling

- [[Core-Concepts]]
- [[Schema-Definition]]
- [[Schema-Reference-Complete]]
- [[Schema-Examples]]
- [[Column-Types]]
- [[Column-Type-Reference]]
- [[Relationships]]
- [[Validation-Reference]]

### APIs

- [[API-Overview]]
- [[API-Reference]]
- [[CRUD-Operations]]
- [[Filtering-and-Pagination]]
- [[Aggregation-API]]
- [[GraphQL-API]]
- [[WebSocket-API]]
- [[GET-API-Complete-Reference]]

### Identity And Access

- [[Authentication]]
- [[OAuth-Authentication]]
- [[OAuth-Provider]]
- [[Two-Factor-Auth]]
- [[Users-and-Groups]]
- [[Permissions]]

### Logic And Automation

- [[Actions-Overview]]
- [[Action-Reference]]
- [[Custom-Actions]]
- [[Data-Actions]]
- [[Cloud-Actions]]
- [[Email-Actions]]
- [[Production-Mail-Delivery]]
- [[Admin-Actions]]
- [[User-Actions]]
- [[State-Machines]]
- [[Task-Scheduling]]
- [[Data-Exchange]]

### Storage, Sites, And Content

- [[Cloud-Storage]]
- [[Cloud-Storage-Complete-Guide]]
- [[Asset-Columns]]
- [[Subsites]]
- [[Template-Rendering]]
- [[RSS-Atom-Feeds]]

### LLM, Integrations, And Metering

- [[LLM-Providers]]
- [[Integrations]]
- [[Credentials]]
- [[API-Metering]]
- [[Rate-Limiting]]

### Protocols And Operations

- [[SMTP-Server]]
- [[IMAP-Support]]
- [[FTP-Server]]
- [[CalDAV-CardDAV]]
- [[TLS-Certificates]]
- [[Monitoring]]
- [[Caching]]
- [[Clustering]]
- [[Audit-Logging]]

## System Tables

Daptin creates and manages system tables for users, usergroups, schema metadata,
actions, state machines, audit/timeline data, documents, sites, cloud storage,
credentials, OAuth, integrations, LLM providers, metering, mail, tasks, and
runtime configuration. See **[[Core-Concepts]]** and
**[[Daptin-Application-Server-Feature-Map]]** for the connected model.

## Help

- Documentation issues: https://github.com/daptin/daptin/issues
- Releases: https://github.com/daptin/daptin/releases
- Community: https://discord.gg/t564q8SQVk

## License

Daptin is licensed under LGPL v3.
