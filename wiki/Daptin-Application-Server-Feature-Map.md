# Daptin Application Server Feature Map

Daptin is a batteries-included application server for your next software
project.

Most apps need the same backend foundation: data modeling, APIs, identity,
usergroups, permissions, relations, actions, workflows, state machines, files,
sites, integrations, LLM routing, metering, events, protocols, caching,
auditing, and operations. Daptin brings those pieces together in one runtime so
you can build the application while Daptin runs and enforces the backend.

## How Daptin Fits Together

Daptin is organized around a small set of recurring concepts:

1. Schema and config are loaded into `CmsConfig`.
2. The `world` table stores table metadata and schema JSON.
3. `AddResourcesToApi2Go` turns every configured table into JSON:API resources
   under `/api/{entity}`.
4. A shared middleware chain enforces permissions, validation, metering, events,
   data exchange, and YJS behavior around CRUD operations.
5. The `action` table and action performers provide backend logic beyond CRUD.
6. System tables such as `cloud_store`, `site`, `oauth_connect`,
   `integration`, `llm_provider`, `api_plan`, `mail_server`, and `task` turn
   built-in infrastructure into data-managed runtime capabilities.
7. Side services attach protocols and product surfaces to the same resource
   graph: GraphQL, WebSocket, OAuth/OIDC, LLM `/v1`, SMTP, IMAP, FTP,
   CalDAV/CardDAV, feeds, subsites, assets, config, OpenAPI, and statistics.

Together, these pieces make Daptin an application server rather than only a CMS,
API generator, or backend-as-a-service.

## 1. Data And Schema Runtime

Core capabilities:

- Schema-defined entities stored through the `world` table.
- Standard columns on every table:
  - `id` - internal primary key, excluded from API.
  - `version` - modification counter for optimistic/change tracking.
  - `created_at` and `updated_at` - automatic timestamps.
  - `reference_id` - public external UUID-style identifier.
  - `permission` - row access-control bitmask.
- Column types for identity, security, text, files, JSON, dates, location,
  media, colors, ratings, and measurements.
- Relations through schema definitions plus built-in standard relations.
- REST JSON:API CRUD under `/api/{entity}`.
- Optional GraphQL under `/graphql`.
- API discovery through `/meta`, `/jsmodel/{typename}`, `/openapi.yaml`, and
  generated OpenAPI paths.
- Import/export and schema/data actions such as random data generation,
  CSV/XLS import, and data export.

Important system tables:

- `world` - schema and table metadata.
- `action` - executable backend actions.
- entity join tables - relationship and usergroup membership links.
- `{entity}_audit` - optional audit tables when auditing is enabled.

Why it matters:

Apps should not reinvent persistence, identifiers, relations, validation, API
shape, or metadata discovery. Daptin gives projects a durable backend contract.

## 2. Identity, Usergroups, Permissions, And Multi-Tenancy Patterns

Core capabilities:

- `user_account` for users.
- `usergroup` for groups, roles, teams, tenant-style boundaries, and
  permission scoping.
- Generic relation/join tables connect users, groups, actions, integrations,
  storage, and app data.
- Entity-level access checks happen before and after CRUD operations.
- Row/object-level access checks filter or reject individual records.
- Permission bits support guest, owner/user, and group scopes for operations
  such as peek, read, create, update, delete, refer, and execute.
- Action execution is permission-checked both at action level and, when an
  action targets a specific record, at object level.
- Default groups and default relations can be part of table schema.
- Multi-tenancy should be described as a supported pattern built from
  ownership, usergroups, default groups, relations, and row-level permissions.

Related auth capabilities:

- JWT/session authentication.
- Password signup/signin flows.
- OTP/2FA flows.
- OAuth as a client through `oauth_connect` and `oauth_token`.
- OAuth as a provider through `oauth_app`, `oauth_code`, `oauth_access`,
  `oauth_refresh`, `oauth_grant`, and `oauth_key`.
- OAuth/OIDC-style endpoints:
  - `/.well-known/oauth-authorization-server`
  - `/.well-known/openid-configuration`
  - `/oauth/authorize`
  - `/oauth/token`
  - `/oauth/revoke`
  - `/oauth/introspect`
  - `/oauth/userinfo`
  - `/oauth/jwks`
- Browser-facing OAuth login consumer routes:
  - `/oauth/login/:authenticator`
  - `/oauth/response`

Why it matters:

Most apps eventually need users, teams, roles, tenant boundaries, login, and
access policy. Daptin has those backend invariants in the runtime instead of
leaving every project to recreate them.

## 3. Actions, Backend Logic, Workflows, And State Machines

Core capabilities:

- `action` rows describe executable operations on entities.
- Actions define input fields, validations, conformations, conditions, outcomes,
  and whether an instance reference is required.
- Action outcomes can perform CRUD, execute Go performers, render content,
  download files, set client headers/cookies/storage, switch session users, and
  chain multiple backend steps.
- Built-in actions include:
  - signup, signin, password reset, OTP registration/verification.
  - OAuth login begin/response and OAuth app registration.
  - integration install and execution.
  - cloud-store file upload/delete/folder/path operations.
  - site creation and site storage sync.
  - mail send, mail-server sync, and outbox processing.
  - schema/data import, export, random data generation, and restart.
  - TLS certificate generation.
  - publish to topic.
- `task` rows schedule action execution through the task scheduler.
- `smd` rows define state machine descriptions.
- State machine endpoints:
  - `/track/start/:stateMachineId`
  - `/track/event/:typename/:objectStateId/:eventName`
- State transition audit tables record workflow history.

Why it matters:

Apps should call stable backend actions. They should not hide business logic,
secrets, workflow transitions, or scheduled jobs inside frontend code or
one-off service glue.

## 4. Events, Realtime, WebSocket, Streams, And Collaboration

Core capabilities:

- CRUD middleware publishes create/update/delete events.
- Events are transported through Olric PubSub.
- `/live` WebSocket server exposes table/topic updates to clients.
- WebSocket messages use a structured response/event/session shape.
- Event worker pool controls publish concurrency and queueing.
- Optional YJS support adds collaborative document behavior and YJS endpoints.
- `stream` and `feed` tables support data streams and feeds.
- `/feed/:feedname` exposes RSS/Atom/JSON-style feed output.

Why it matters:

Daptin turns backend data changes into live app events. Apps can be realtime
without inventing their own event bus, change feed, or websocket server.

## 5. Storage, Assets, Sites, And Templates

Core capabilities:

- `credential` stores encrypted secrets.
- `cloud_store` represents local or cloud storage connections.
- Storage uses rclone-backed configuration and providers.
- Asset columns connect entity fields to stored files, images, videos, blobs,
  and compressed data.
- Asset routes:
  - `/asset/:typename/:resource_id/:columnname`
  - `/asset/:typename/:resource_id/:columnname/upload`
- Upload handling supports specialized asset operations such as multipart and
  presigned flows where configured.
- File route handlers include memory cache, ETag, gzip, client cache headers,
  negative cache, and optimized static file serving.
- `site` rows define static/subsite hosting with host/path routing.
- Site actions create sites and sync storage.
- Subsite/template hooks allow sites and templates to run from the same Daptin
  runtime.

Why it matters:

Most apps need file uploads, media, static frontends, blogs, docs, and site
hosting. Daptin includes storage and site hosting instead of requiring a
separate storage provider, file server, and static host for every project.

## 6. Protocols And Built-In Servers

Built-in or config-gated surfaces:

- HTTP and HTTPS application server.
- TLS certificate management through certificate tables and ACME/self-signed
  certificate actions.
- SMTP server backed by `mail_server`, `mail_account`, `mail_box`, `mail`, and
  `outbox`.
- IMAP server when enabled.
- FTP server when enabled.
- CalDAV/CardDAV endpoints when enabled.
- WebSocket endpoint at `/live`.
- RSS/Atom/JSON feeds through feed routes.
- GraphQL endpoint when enabled.
- OpenAI-compatible `/v1` LLM endpoints.
- Gzip, CORS, rate limiting, language middleware, `/ping`, `/statistics`, and
  config endpoints.

Why it matters:

Daptin is not just CRUD over HTTP. It is a multi-protocol application server
that can run app data, app APIs, app files, app sites, app mail, app auth, and
app realtime behavior from one runtime.

## 7. Integrations, LLM Routing, Metering, And Monetization

Integration capabilities:

- `integration` stores OpenAPI specifications and auth metadata.
- Provider-scoped routes:
  - `GET /integration/{provider}/operations`
  - `GET /integration/{provider}/operations/{operation}`
  - `GET /integration/{provider}/openapi.yaml`
  - `POST /integration/{provider}/{operation}`
- Integrations support OAuth tokens and custom credentials.
- Runtime fields such as selected token/credential are separated from user
  operation input.
- Installed provider operations are exported into OpenAPI docs.

LLM capabilities:

- `llm_provider` stores provider routing config.
- Credentials are linked through the same encrypted credential system.
- Daptin exposes OpenAI-compatible endpoints:
  - `/v1/chat/completions`
  - `/v1/completions`
  - `/v1/embeddings`
  - `/v1/models`
- Model names resolve to configured providers.
- Streaming chat uses SSE response format.

Metering capabilities:

- `api_plan` defines request, compute, rate, price, overage, and quota settings.
- `api_member` assigns users to active plans.
- `api_usage` records metered usage.
- `api_quota` stores period counters.
- CRUD, actions, and LLM endpoints can be metered.
- Olric backs clustered rate limit counters.
- `post_metering_action` enables credit/billing hooks after usage is recorded.
- Hard quota failures can deny requests before usage is recorded.

Why it matters:

Modern apps are often productized as APIs, SaaS products, internal tools, or AI
features. Daptin gives them provider routing, external API execution, credential
boundaries, usage tracking, quotas, credits, and rate limits from the backend
runtime.

## 8. Operations, Caching, And Reliability

Operational surfaces:

- Olric embedded client for distributed cache, PubSub, and clustered rate-limit
  behavior.
- LRU/file caches for asset and static file serving.
- Gzip middleware and file compression decisions.
- Health and stats endpoints: `/ping`, `/statistics`.
- Runtime config through `/_config`.
- CORS and connection/rate-limit middleware.
- Background tasks for mail sync and outbox processing.
- Certificate manager for TLS material.
- Optional audit tables and audit row creation on update/delete.
- OpenAPI and metadata endpoints for tool/client discovery.

Why it matters:

Apps need production controls that are easy to postpone or underbuild: cache,
limits, health, audit, runtime config, background work, metadata, and TLS.

## Ecosystem Repos

Related repositories show how Daptin is used from clients, demos, schemas, and
larger applications:

- `daptin/daptin` - core application server.
- `daptin/daptin-cli` - CLI for contexts, CRUD, actions, OAuth, integrations,
  storage, assets, and discovery.
- `daptin/daptin-js-client` - JS/TS client for JSON:API, actions, auth, and
  file uploads.
- `daptin/daptin-go-client` - Go client.
- `daptin/daptin-schema-samples` - reusable schemas for blog, store, task list,
  FAQ, payments, and other app patterns.
- `daptin/dadadash` - larger application example with file browser, document
  editor, spreadsheet editor, calendar, and CRUD data tables.
- `daptin/daptin-llm-demo` - OpenAI-compatible LLM endpoint contract demo.
- `daptin/daptin-metering-credit-demo` - credits, quotas, usage audit, LLM
  usage, and denial-path demo.
- `daptin/daptin-integration-auth-demo` - OAuth/custom-credential integration
  execution and wrong-user denial demo.
- OAuth demo repos - Daptin as OAuth provider, client, and two-instance
  provider/consumer flow.

## What Daptin Can Replace

- Ad-hoc database glue.
- Custom auth/session logic.
- Handwritten permission checks.
- Hand-wired file storage and static hosting.
- One-off background job runners.
- Hardcoded third-party API secrets.
- Custom WebSocket/event infrastructure.
- Separate LLM gateway/proxy.
- Separate API metering/rate-limit layer.
- Separate operational metadata and health endpoints.

## Where To Go Next

- Start with [Getting Started Guide](Getting-Started-Guide) and
  [Core Concepts](Core-Concepts).
- Define entities with [Schema Definition](Schema-Definition),
  [Column Types](Column-Types), and [Relationships](Relationships).
- Add users and access control with [Authentication](Authentication),
  [Users and Groups](Users-and-Groups), and [Permissions](Permissions).
- Build backend behavior with [Actions Overview](Actions-Overview),
  [Custom Actions](Custom-Actions), [Task Scheduling](Task-Scheduling), and
  [State Machines](State-Machines).
- Manage files and sites with [Asset Columns](Asset-Columns),
  [Cloud Storage](Cloud-Storage), [Subsites](Subsites), and
  [Template Rendering](Template-Rendering).
- Connect external services with [Integrations](Integrations),
  [Credentials](Credentials), and [OAuth Authentication](OAuth-Authentication).
- Run AI and productized API workloads with [LLM Providers](LLM-Providers),
  [API Metering](API-Metering), and [Rate Limiting](Rate-Limiting).
- Operate Daptin with [Configuration](Configuration),
  [Production Deployment](Production-Deployment), [Monitoring](Monitoring),
  [Caching](Caching), [Clustering](Clustering), and
  [TLS Certificates](TLS-Certificates).
