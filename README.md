# Daptin

[![Release](https://img.shields.io/github/v/release/daptin/daptin?style=flat-square)](https://github.com/daptin/daptin/releases/latest)
[![License](https://img.shields.io/badge/license-LGPL%20v3-brightgreen?style=flat-square)](LICENSE)
[![Go Report](https://goreportcard.com/badge/github.com/daptin/daptin?style=flat-square)](https://goreportcard.com/report/github.com/daptin/daptin)
[![Discord](https://img.shields.io/badge/chat-Discord-5865F2?style=flat-square&logo=discord&logoColor=white)](https://discord.gg/t564q8SQVk)

**Daptin is the batteries-included application server for your next software project.**

Most apps need the same backend capabilities: data modeling, APIs, auth,
permissions, files, background jobs, integrations, metering, realtime events,
and production operations. Daptin gives you those pieces in one server.

Use Daptin as the primary backend for a new app, or run it beside an existing
stack as a sidecar for the features you are missing.

```text
schema.yaml -> Daptin -> APIs + Auth + Permissions + Files + Sites + Actions + Integrations + Ops
```

Build your app. Let Daptin run and enforce the backend.

## Why Daptin

Most software projects eventually need the same backend foundation. Daptin keeps
that foundation stable:

- Your data model becomes REST, GraphQL, OpenAPI, metadata, validation, and
  standard columns.
- Your users, teams, tenants, and roles become `user_account`, `usergroup`,
  relations, OAuth, and row-level permissions.
- Your backend logic becomes actions, state machines, scheduled tasks, data
  exchange, and integration calls.
- Your product runtime gets files, sites, realtime events, LLM routing,
  metering, quotas, rate limits, audit trails, caching, TLS, and monitoring.

Use Daptin behind web apps, mobile apps, internal tools, stores, blogs, SaaS
products, dashboards, portals, API products, content sites, and AI products.

## What You Get

### Data Model And APIs

- Schema-defined entities backed by SQLite, PostgreSQL, or MySQL/MariaDB.
- Standard columns: internal `id`, public `reference_id`, `version`,
  `created_at`, `updated_at`, and row `permission`.
- Relations, joins, default groups, default relations, validations,
  conformations, composite keys, and column types.
- JSON:API CRUD endpoints under `/api/{entity}`.
- Optional GraphQL endpoint under `/graphql`.
- OpenAPI and metadata through `/openapi.yaml`, `/meta`, and
  `/jsmodel/{typename}`.
- Import, export, aggregation, filtering, pagination, and generated test data.

### Identity, Policy, And Multi-Tenancy

- Users, usergroups, group membership, and ownership-aware rows.
- Entity-level and row-level permission checks.
- Guest, owner, and group permission scopes for peek, read, create, update,
  delete, refer, and execute.
- JWT/session authentication, signup/signin, password reset, and OTP/2FA flows.
- OAuth as a client through `oauth_connect` and `oauth_token`.
- OAuth/OIDC-style provider endpoints through `oauth_app`, authorization codes,
  access tokens, refresh tokens, grants, JWKS, UserInfo, introspection, and
  revocation.
- Multi-tenant app patterns through usergroups, relations, ownership, default
  groups, and row permissions.

### Backend Logic And Workflows

- Buildable actions with input fields, validation, conditions, outcomes, and
  action-level permissions.
- Action chains for CRUD, performer execution, rendering, file download,
  response shaping, user switching, and backend-side orchestration.
- State machines and state transition tracking.
- Scheduled tasks that execute actions on cron-like schedules.
- Data exchange and custom business logic hooks.

### Files, Storage, Sites, And Templates

- Encrypted credentials and `cloud_store` records.
- rclone-backed local and cloud storage providers.
- Asset columns for files, images, videos, blobs, markdown, HTML, JSON, and
  compressed content.
- Upload routes, direct asset serving, cache headers, ETags, gzip, and
  optimized static file serving.
- Static site and subsite hosting from the same runtime.
- Site creation and site storage sync actions.
- Template hooks for static sites and app frontends.

### Realtime, Protocols, And Collaboration

- WebSocket endpoint at `/live`.
- Create/update/delete events published through Olric PubSub.
- Optional YJS collaboration support.
- Streams and RSS/Atom/JSON feeds.
- Built-in or config-gated SMTP, IMAP, FTP, CalDAV/CardDAV, HTTPS/TLS, CORS,
  gzip, rate limiting, and language middleware.

### LLM, Integrations, Metering, And Product Runtime

- OpenAI-compatible endpoints:
  - `/v1/chat/completions`
  - `/v1/completions`
  - `/v1/embeddings`
  - `/v1/models`
- LLM provider routing through `llm_provider` and encrypted credentials.
- OpenAPI-backed third-party integrations under
  `/integration/{provider}/{operation}`.
- OAuth-token and custom-credential execution boundaries for integrations.
- API plans, members, usage logs, quotas, rate limits, and credit hooks.
- CRUD, action, and LLM metering for paid AI/API products.

### Operations

- Runtime configuration through `/_config`.
- Health and statistics through `/ping` and `/statistics`.
- Optional audit tables and audit rows.
- Olric-backed cache, PubSub, and clustered rate-limit counters.
- TLS certificate management, including ACME/self-signed certificate actions.
- Background mail sync and outbox processing.
- OpenAPI and metadata endpoints for tools, agents, and client generation.

## Quick Start

Download the latest release binary:

```bash
# Linux amd64
curl -L -o daptin https://github.com/daptin/daptin/releases/latest/download/daptin-linux-amd64
chmod +x daptin
./daptin -port=6336
```

Or run the Docker image with the current release tag:

```bash
docker run --rm -p 6336:8080 -p 6443:6443 daptin/daptin:v0.12.15
```

Then open:

```text
http://localhost:6336
```

Create the first admin user and finish setup using the wiki:

- [Installation](https://github.com/daptin/daptin/wiki/Installation)
- [First Admin Setup](https://github.com/daptin/daptin/wiki/First-Admin-Setup)
- [Getting Started Guide](https://github.com/daptin/daptin/wiki/Getting-Started-Guide)

## Build Your First Backend

Create a schema file such as `schema_product.yaml`:

```yaml
Tables:
  - TableName: product
    Columns:
      - Name: name
        DataType: varchar(200)
        ColumnType: label
        IsIndexed: true
      - Name: price
        DataType: float
        ColumnType: measurement
      - Name: published
        DataType: bool
        ColumnType: truefalse
        DefaultValue: "false"
```

Start Daptin in the same directory:

```bash
./daptin -port=6336
```

Daptin creates the table and exposes the backend surfaces around it:

```text
GET    /api/product
POST   /api/product
PATCH  /api/product/{reference_id}
DELETE /api/product/{reference_id}
GET    /openapi.yaml
GET    /meta
POST   /graphql              # when GraphQL is enabled
```

From there, add users, usergroups, relations, permissions, actions, asset
columns, state machines, integrations, metering, or site hosting as your app
needs them.

## For New Projects And Existing Stacks

Daptin works well as the backend foundation for new software and as a sidecar
for existing systems:

- Point frontends, tools, and services at JSON:API, GraphQL, OpenAPI, or
  metadata endpoints instead of building bespoke backend glue.
- Use usergroups, relations, and row permissions for access boundaries.
- Keep secrets in Daptin credentials, OAuth tokens, and integration config
  instead of application client code.
- Put backend logic in Daptin actions and state machines.
- Store uploads through asset columns and cloud stores.
- Host static frontends, docs, blogs, and sites as Daptin subsites.
- Route LLM calls through Daptin's OpenAI-compatible `/v1` endpoints.
- Add plans, quotas, usage logs, credits, and rate limits through API metering.
- Subscribe to data changes through WebSocket events when apps need realtime
  behavior.

## Ecosystem

- [Daptin CLI](https://github.com/daptin/daptin-cli) - manage contexts, CRUD,
  actions, OAuth, integrations, storage, assets, and discovery from the command
  line.
- [Daptin JS client](https://github.com/daptin/daptin-js-client) - JavaScript
  and TypeScript client for auth, JSON:API, actions, and uploads.
- [Daptin Go client](https://github.com/daptin/daptin-go-client) - Go client
  for Daptin APIs.
- [Schema samples](https://github.com/daptin/daptin-schema-samples) - reusable
  schemas for blogs, stores, task lists, FAQs, payments, and more.
- [Daptin LLM demo](https://github.com/daptin/daptin-llm-demo) - contract demo
  for OpenAI-compatible LLM endpoints.
- [Metering credit demo](https://github.com/daptin/daptin-metering-credit-demo)
  - quotas, usage, credits, LLM metering, and denial paths.
- [Integration auth demo](https://github.com/daptin/daptin-integration-auth-demo)
  - OAuth/custom-credential integration execution and wrong-user denial.
- [OAuth provider demo](https://github.com/daptin/daptin-oauth-provider-demo)
  - Daptin as OAuth 2.0 / OpenID Connect provider.
- [Dadadash](https://github.com/daptin/dadadash) - larger app proof with file
  browser, document editor, spreadsheet editor, calendar, and CRUD data tables.

## Production Notes

For production deployments:

- Use PostgreSQL or MySQL/MariaDB instead of development SQLite.
- Set stable `jwt.secret` and `encryption.secret` values.
- Enable HTTPS/TLS and configure hostnames.
- Configure backups and restore testing for your database and file storage.
- Use cloud storage for durable files and media when appropriate.
- Enable only the protocols your app needs.
- Configure rate limits, metering, monitoring, and audit behavior for your
  product.

Useful docs:

- [Production Deployment](https://github.com/daptin/daptin/wiki/Production-Deployment)
- [Database Setup](https://github.com/daptin/daptin/wiki/Database-Setup)
- [TLS Certificates](https://github.com/daptin/daptin/wiki/TLS-Certificates)
- [Permissions](https://github.com/daptin/daptin/wiki/Permissions)
- [Integrations](https://github.com/daptin/daptin/wiki/Integrations)
- [LLM Providers](https://github.com/daptin/daptin/wiki/LLM-Providers)
- [API Metering](https://github.com/daptin/daptin/wiki/API-Metering)

## Documentation

- [Wiki](https://github.com/daptin/daptin/wiki)
- [Application Server Feature Map](https://github.com/daptin/daptin/wiki/Daptin-Application-Server-Feature-Map)
- [API Reference](https://github.com/daptin/daptin/wiki/API-Reference)
- [Core Concepts](https://github.com/daptin/daptin/wiki/Core-Concepts)

## Community

- [Discord](https://discord.gg/t564q8SQVk)
- [GitHub Issues](https://github.com/daptin/daptin/issues)
- [Releases](https://github.com/daptin/daptin/releases)

## License

Daptin is licensed under [LGPL v3](LICENSE).
