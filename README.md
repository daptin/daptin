<div align="center">
  <img src="https://daptin.github.io/images/theme-logo.png" width="96" alt="Daptin logo">
  <h1>Daptin</h1>
  <p><strong>A self-hosted application server built around your data model.</strong></p>
  <p>Define resources once. Daptin runs their APIs, identity, permissions, files, automation, integrations, realtime delivery, and operational controls.</p>

  <p>
    <a href="https://github.com/daptin/daptin/releases/latest"><img src="https://img.shields.io/github/v/release/daptin/daptin?style=flat-square" alt="Latest release"></a>
    <a href="LICENSE"><img src="https://img.shields.io/badge/license-LGPL%20v3-brightgreen?style=flat-square" alt="License: LGPL v3"></a>
    <a href="https://goreportcard.com/report/github.com/daptin/daptin"><img src="https://goreportcard.com/badge/github.com/daptin/daptin?style=flat-square" alt="Go Report"></a>
    <a href="https://discord.gg/t564q8SQVk"><img src="https://img.shields.io/badge/chat-Discord-5865F2?style=flat-square&amp;logo=discord&amp;logoColor=white" alt="Discord"></a>
  </p>
  <p>
    <a href="https://daptin.github.io/">Website</a> · <a href="#start-in-a-minute">Quickstart</a> · <a href="#the-complete-feature-map">Feature map</a> · <a href="#actions-scheduled-tasks-and-state-tracking">Automation explained</a> · <a href="https://github.com/daptin/daptin/wiki">Documentation</a>
  </p>
</div>

<br>

## One model, many backend surfaces

Daptin loads schema files into its runtime configuration. Each configured table is recorded as `world` metadata and becomes a permission-aware JSON:API resource at `/api/{entity}`. The running server attaches its other API, automation, storage, integration, and protocol surfaces around that resource runtime.

Many capabilities are configured as ordinary data. Rows in tables such as `action`, `task`, `smd`, `cloud_store`, `site`, `integration`, `llm_provider`, and `api_plan` tell the running server what to execute or expose.

| Part | Plain-English meaning |
|---|---|
| **Schema** | Describes the records, fields, relationships, validation, and optional behavior flags your application needs. |
| **Resource runtime** | Turns every configured table into stored data plus CRUD and relationship APIs. |
| **Permission gates** | Check access to the table, individual row, relation, or action before work is performed. |
| **System tables** | Configure Daptin itself—actions, schedules, state definitions, storage, sites, integrations, mail, plans, and more. |
| **Attached surfaces** | Expose the same model through JSON:API, optional GraphQL, WebSockets, feeds, files, OAuth/OIDC, LLM routes, and built-in protocols. |

Read the source-oriented [application server feature map](https://github.com/daptin/daptin/wiki/Daptin-Application-Server-Feature-Map) for the runtime components behind this diagram.

## Start in a minute

### Docker

```bash
docker run --pull=always --rm -p 127.0.0.1:6336:8080 daptin/daptin:latest
```

For a durable local stack with PostgreSQL:

```bash
cp .env.example .env
docker compose up --wait
```

The Compose stack binds Daptin only to `127.0.0.1:6336` and keeps PostgreSQL
inside its private network. Database and asset data are stored in named
volumes. Change the development password in `.env` before using the stack
beyond local evaluation. Kubernetes users should start with the
[Kustomize deployment guide](kubernetes/README.md).

### Linux binary

```bash
curl -L -o daptin https://github.com/daptin/daptin/releases/latest/download/daptin-linux-amd64
chmod +x daptin
./daptin -port=6336
```

Open **http://localhost:6336**, create the first administrator, and continue with the [Getting Started Guide](https://github.com/daptin/daptin/wiki/Getting-Started-Guide).

## The complete feature map

The table below groups the implemented surface by responsibility.

| Capability | What Daptin provides | Learn more |
|---|---|---|
| **Schema and data runtime** | YAML or JSON schema; 41 column types; relations and joins; composite keys; validation and conformation; translations; audit, state, and metering flags; import, export, and generated test data. | [Core concepts](https://github.com/daptin/daptin/wiki/Core-Concepts) · [Relationships](https://github.com/daptin/daptin/wiki/Relationships) · [Validation](https://github.com/daptin/daptin/wiki/Validation-Reference) |
| **APIs and discovery** | JSON:API CRUD and relationships with filtering, sorting, and pagination; a separate aggregation API; optional GraphQL queries, mutations, and actions; OpenAPI, metadata, and JavaScript model discovery routes. | [API overview](https://github.com/daptin/daptin/wiki/API-Overview) · [GET reference](https://github.com/daptin/daptin/wiki/GET-API-Complete-Reference) · [GraphQL](https://github.com/daptin/daptin/wiki/GraphQL-API) |
| **Identity and permissions** | Users, groups, ownership, default and access groups; table, row, relation, and action permissions; JWT login flows and OTP; OAuth client connections; OAuth 2/OIDC provider endpoints and JWKS. | [Permissions](https://github.com/daptin/daptin/wiki/Permissions) · [OAuth authentication](https://github.com/daptin/daptin/wiki/OAuth-Authentication) · [OAuth provider](https://github.com/daptin/daptin/wiki/OAuth-Provider) |
| **Actions and orchestration** | Actions with inputs, validation, conformations, conditions, ordered outcomes, CRUD steps, Go performers, client responses, and chained work; scheduled action invocation; state tracking; data exchange. | [Actions overview](https://github.com/daptin/daptin/wiki/Actions-Overview) · [Action reference](https://github.com/daptin/daptin/wiki/Action-Reference) · [Task scheduling](https://github.com/daptin/daptin/wiki/Task-Scheduling) |
| **Files, sites, and templates** | Encrypted credentials; rclone-backed local or cloud stores; asset columns and uploads; direct and presigned file flows; caching, ETags, gzip, and storage sync; sites, subsites, host/path routing, and Go templates. | [Cloud storage](https://github.com/daptin/daptin/wiki/Cloud-Storage-Complete-Guide) · [Asset columns](https://github.com/daptin/daptin/wiki/Asset-Columns) · [Subsites](https://github.com/daptin/daptin/wiki/Subsites) · [Templates](https://github.com/daptin/daptin/wiki/Template-Rendering) |
| **Realtime and protocols** | CRUD events over Olric PubSub and `/live` WebSockets; topic permissions and optional YJS collaboration; streams and RSS/Atom/JSON feeds; HTTP(S), SMTP/IMAP, FTP/FTPS, and WebDAV-style CalDAV/CardDAV routes. | [WebSocket API](https://github.com/daptin/daptin/wiki/WebSocket-API) · [Feeds](https://github.com/daptin/daptin/wiki/RSS-Atom-Feeds) · [FTP server](https://github.com/daptin/daptin/wiki/FTP-Server) |
| **Integrations and LLM routing** | OpenAPI v2/v3 operations installed as Daptin actions; REST, GraphQL, short-lived WebSocket, and unary gRPC transports; OAuth or custom credentials; multi-provider LLM routing with OpenAI-compatible chat, streaming, completion, embedding, model, and tool-call surfaces. | [Integrations](https://github.com/daptin/daptin/wiki/Integrations) · [LLM providers](https://github.com/daptin/daptin/wiki/LLM-Providers) |
| **Metering, clustering, and operations** | Plans, memberships, usage, quotas, credit hooks, and rate limits; metering for CRUD, actions, and LLM tokens; Olric cache, PubSub, counters, clustering, and outbox deduplication; audit logs, TLS, ping, statistics, config, logs, and profiles. | [API metering](https://github.com/daptin/daptin/wiki/API-Metering) · [Clustering](https://github.com/daptin/daptin/wiki/Clustering) · [Audit logging](https://github.com/daptin/daptin/wiki/Audit-Logging) · [Production](https://github.com/daptin/daptin/wiki/Production-Deployment) |

> **Protocol scope:** Daptin’s CalDAV/CardDAV routes provide basic WebDAV-style file storage. They are not documented as complete calendar/contact protocol implementations.

## A small schema becomes a permission-aware API

Place `schema_product.yaml` beside the Daptin binary:

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

When Daptin starts, it creates the table and exposes the resource through JSON:API:

```http
GET    /api/product
POST   /api/product
PATCH  /api/product/{reference_id}
DELETE /api/product/{reference_id}
```

The shared middleware checks table and row access, validates and conforms data, applies metering when configured, and publishes successful CRUD events. Optional GraphQL, `/openapi.yaml`, `/meta`, and `/jsmodel/{typename}` expose additional ways to query or discover the same model.

Add [relations](https://github.com/daptin/daptin/wiki/Relationships), [permissions](https://github.com/daptin/daptin/wiki/Permissions), [assets](https://github.com/daptin/daptin/wiki/Asset-Columns), or [audit history](https://github.com/daptin/daptin/wiki/Audit-Logging) as the resource needs them.

## Actions, scheduled tasks, and state tracking

These are related building blocks, not one workflow engine:

| Term | What it actually stores | What happens at runtime |
|---|---|---|
| **Action** | Inputs, validation/conformation rules, conditions, permissions, and ordered output fields attached to a `world` entity. | A call to `/action/{type}/{name}` or a GraphQL action can run CRUD outcomes, registered Go performers, and client-response outcomes. |
| **Task** | A schedule, entity name, action name, attributes, active flag, job type, and an `as_user` relation. | At server startup it is loaded into the cron scheduler. When due, Daptin loads that user’s identity and groups, then calls the same action handler inside a database transaction. |
| **State machine definition (`smd`)** | An initial state and named events with allowed source and destination states. | `/track/start` creates an `{entity}_state` record. `/track/event` verifies the current source state, records the transition, increments its version, writes audit data, and publishes an event. |
| **Data exchange** | A mapping for synchronizing resource data with another source or destination. | Exchange middleware runs before and after configured resource operations. |

State tracking does **not** provide guards, entry/exit actions, parallel states, or hierarchical states. If a transition should perform work, invoke an action separately. See [State Machines](https://github.com/daptin/daptin/wiki/State-Machines) for the exact supported model.

## Choose a starting point

| I want to… | Start here |
|---|---|
| Install and configure Daptin | [Installation](https://github.com/daptin/daptin/wiki/Installation) · [First admin setup](https://github.com/daptin/daptin/wiki/First-Admin-Setup) |
| Model resources and query them | [Core concepts](https://github.com/daptin/daptin/wiki/Core-Concepts) · [API reference](https://github.com/daptin/daptin/wiki/API-Reference) |
| Secure users, rows, and actions | [Permissions](https://github.com/daptin/daptin/wiki/Permissions) · [Authorization scenario](https://github.com/daptin/daptin/wiki/Authorization-Scenario-Action-Access-Gates) |
| Build reusable backend operations | [Actions](https://github.com/daptin/daptin/wiki/Actions-Overview) · [Custom actions](https://github.com/daptin/daptin/wiki/Custom-Actions) |
| Schedule work or track record states | [Task scheduling](https://github.com/daptin/daptin/wiki/Task-Scheduling) · [State machines](https://github.com/daptin/daptin/wiki/State-Machines) |
| Connect external services or LLMs | [Integrations](https://github.com/daptin/daptin/wiki/Integrations) · [LLM providers](https://github.com/daptin/daptin/wiki/LLM-Providers) |
| Store files or host a subsite | [Cloud storage](https://github.com/daptin/daptin/wiki/Cloud-Storage) · [Subsites](https://github.com/daptin/daptin/wiki/Subsites) |
| Meter or run Daptin in production | [API metering](https://github.com/daptin/daptin/wiki/API-Metering) · [Production deployment](https://github.com/daptin/daptin/wiki/Production-Deployment) · [Database setup](https://github.com/daptin/daptin/wiki/Database-Setup) |

## Ecosystem

| Project | Purpose |
|---|---|
| [Daptin CLI](https://github.com/daptin/daptin-cli) | Manage data, actions, OAuth, integrations, storage, and assets. |
| [JavaScript client](https://github.com/daptin/daptin-js-client) · [Go client](https://github.com/daptin/daptin-go-client) | Connect applications to Daptin. |
| [Schema samples](https://github.com/daptin/daptin-schema-samples) | Start from working schema examples. |
| [LLM demo](https://github.com/daptin/daptin-llm-demo) · [Metering demo](https://github.com/daptin/daptin-metering-credit-demo) | Explore metered AI and API products. |
| [Integration auth demo](https://github.com/daptin/daptin-integration-auth-demo) · [OAuth provider demo](https://github.com/daptin/daptin-oauth-provider-demo) | Explore integrations and identity. |
| [Dadadash](https://github.com/daptin/dadadash) | A larger application built on Daptin. |

---

<div align="center">
  <strong><a href="https://github.com/daptin/daptin/wiki">Documentation</a></strong>
  · <a href="https://discord.gg/t564q8SQVk">Discord</a>
  · <a href="https://github.com/daptin/daptin/issues">Issues</a>
  · <a href="https://github.com/daptin/daptin/releases">Releases</a>
  <br><br>
  LGPL v3
</div>
