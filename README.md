<div align="center">
  <img src="docs/images/logo.png" width="112" alt="Daptin logo">
  <h1>Daptin</h1>
  <p><strong>Turn a data model into a production-ready backend.</strong></p>
  <p>APIs, auth, permissions, files, workflows, integrations, realtime, and operations—one server.</p>

  <p>
    <a href="https://github.com/daptin/daptin/releases/latest"><img src="https://img.shields.io/github/v/release/daptin/daptin?style=flat-square" alt="Release"></a>
    <a href="LICENSE"><img src="https://img.shields.io/badge/license-LGPL%20v3-brightgreen?style=flat-square" alt="License: LGPL v3"></a>
    <a href="https://goreportcard.com/report/github.com/daptin/daptin"><img src="https://goreportcard.com/badge/github.com/daptin/daptin?style=flat-square" alt="Go Report"></a>
    <a href="https://discord.gg/t564q8SQVk"><img src="https://img.shields.io/badge/chat-Discord-5865F2?style=flat-square&amp;logo=discord&amp;logoColor=white" alt="Discord"></a>
  </p>
  <p>
    <a href="#run-it">Get started</a> · <a href="#how-it-works">How it works</a> · <a href="https://github.com/daptin/daptin/wiki">Documentation</a> · <a href="https://github.com/daptin/daptin-schema-samples">Examples</a>
  </p>
</div>

---

## From idea to working product

```mermaid
flowchart LR
    IDEA["1 · Describe your business<br/>Products · Orders · Customers"]
    BASE["2 · Get the foundation<br/>Database · APIs · Login · Permissions"]
    BEHAVIOR["3 · Add business behavior<br/>Approve · Notify · Publish · Bill"]
    APP["4 · Connect your product<br/>Website · Mobile app · Internal tool"]
    RUN["5 · Run it reliably<br/>Files · Realtime · Limits · Audit"]

    IDEA -->|schema| BASE
    BASE -->|you add actions + rules| BEHAVIOR
    BEHAVIOR -->|a working backend| APP
    APP -->|real usage| RUN
```

The steps build on each other, but you do not need every feature. Start with data and login; add automation only where your product needs it.

### What do these words mean?

| Daptin term | In plain English | Example |
|---|---|---|
| **Schema** | A description of the information your product keeps | An `order` has a customer, items, total, and status |
| **Action** | A named job that runs when asked | “Approve order”, “send receipt”, or “generate invoice” |
| **State machine** | A record of the allowed stages and moves; actions that do the work stay separate | An order may move **new → paid → shipped**, but not **new → shipped** |
| **Schedule** | A timer that starts an action automatically | Every night, find overdue invoices and send reminders |
| **Integration** | A safe connection to another service | Charge through Stripe, create a GitHub issue, or call an AI model |

### How automation fits together

```mermaid
flowchart LR
    TRIGGER{"What starts the work?"}
    PERSON["A person clicks<br/>Approve order"] --> TRIGGER
    APPREQ["Your app asks<br/>Create order through the API"] --> TRIGGER
    CLOCK["A schedule fires<br/>Every day at 9:00"] --> TRIGGER

    TRIGGER --> ACTION["Action<br/>A reusable business task"]
    ACTION --> CHECK["Check rules<br/>Is this user allowed?"]
    CHECK --> CHANGE["Change data<br/>Update the order"]
    CHECK --> CONNECT["Contact a service<br/>Send email or take payment"]
    CHANGE --> RESULT["Return a result<br/>and publish a live update"]
    CONNECT --> RESULT
```

An **action** is the work. A **schedule**, button click, or API request decides when that work starts.

## What is inside?

| | Capability | Highlights |
|---|---|---|
| 🧱 | **Data + APIs** | SQLite, PostgreSQL, MySQL/MariaDB · JSON:API · GraphQL · OpenAPI · import/export |
| 🔐 | **Identity + policy** | JWT sessions · OAuth client/provider · users, groups, tenants · entity and row permissions |
| ⚙️ | **Backend logic** | Actions · state machines · scheduled tasks · validation · data exchange |
| 🗂️ | **Files + sites** | Asset columns · local/cloud storage · uploads · static sites · caching |
| ⚡ | **Realtime + protocols** | WebSockets · YJS · feeds · SMTP/IMAP · FTP · CalDAV/CardDAV |
| 🔌 | **Integrations + AI** | OpenAPI integrations · encrypted credentials · OpenAI-compatible LLM routing |
| 📊 | **Product + operations** | Metering · quotas · rate limits · audit · TLS · health · monitoring |

> **One policy boundary:** APIs, actions, files, integrations, and events all use the same users, groups, ownership, and permission model.

## Run it

### Binary

```bash
curl -L -o daptin https://github.com/daptin/daptin/releases/latest/download/daptin-linux-amd64
chmod +x daptin
./daptin -port=6336
```

### Docker

```bash
docker run --rm -p 6336:8080 -p 6443:6443 daptin/daptin:v0.12.31
```

Open **http://localhost:6336**, create the first admin user, and follow the [Getting Started Guide](https://github.com/daptin/daptin/wiki/Getting-Started-Guide).

## Build your first API

Create `schema_product.yaml` beside the Daptin binary:

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

Restart Daptin. That short description becomes a backend your product can use:

```mermaid
flowchart LR
    S["You describe a product<br/>name · price · published"] --> D{"Daptin builds"}
    D --> STORE["A place to store products"]
    D --> API["A secured way for apps<br/>to create, read, edit, and delete"]
    D --> ADMIN["An admin screen<br/>to manage the data"]
    D --> DOCS["Live API documentation<br/>for developers and tools"]

    API --> USE["Your website, mobile app,<br/>dashboard, or automation"]
```

```http
GET    /api/product
POST   /api/product
PATCH  /api/product/{reference_id}
DELETE /api/product/{reference_id}
```

Next, add [relations](https://github.com/daptin/daptin/wiki/Relationships), [permissions](https://github.com/daptin/daptin/wiki/Permissions), [actions](https://github.com/daptin/daptin/wiki/Actions-Overview), or [file storage](https://github.com/daptin/daptin/wiki/Cloud-Storage).

## What happens when someone uses your app?

```mermaid
flowchart LR
    USER["1 · Someone does something<br/>Place order · Upload file · Approve request"]
    ID["2 · Daptin identifies them<br/>Guest · Customer · Team member · Admin"]
    RULE["3 · Daptin checks the rules<br/>Can they see or change this specific item?"]
    WORK["4 · Daptin does the work<br/>Save data · Run action · Move to next stage"]
    EFFECT["5 · Other things can happen<br/>Send email · Call service · Notify live app"]
    RECORD["6 · The result is recorded<br/>Response · Usage · History · Audit"]

    USER --> ID --> RULE
    RULE -->|allowed| WORK --> EFFECT --> RECORD
    RULE -->|not allowed| STOP["Safe rejection<br/>Nothing is changed"]
```

The same safety checks apply whether the request comes from a website, mobile app, command line, scheduled job, or another service.

## See it

| Admin dashboard | Visual data modeling |
| :---: | :---: |
| [![Daptin dashboard](docs/images/dashboard/index-page.png)](docs/images/dashboard/index-page.png) | [![Create a table](docs/images/dashboard/create-new-table.png)](docs/images/dashboard/create-new-table.png) |
| **Generated OpenAPI** | **GraphQL explorer** |
| [![OpenAPI specification](docs/images/dashboard/openapi-spec-page.png)](docs/images/dashboard/openapi-spec-page.png) | [![GraphQL interface](docs/images/dashboard/graphql-web-interface.png)](docs/images/dashboard/graphql-web-interface.png) |

## Choose your path

| I want to… | Start here |
|---|---|
| Install and configure Daptin | [Installation](https://github.com/daptin/daptin/wiki/Installation) · [First admin setup](https://github.com/daptin/daptin/wiki/First-Admin-Setup) |
| Model data and expose APIs | [Core concepts](https://github.com/daptin/daptin/wiki/Core-Concepts) · [API reference](https://github.com/daptin/daptin/wiki/API-Reference) |
| Secure users, rows, and actions | [Permissions](https://github.com/daptin/daptin/wiki/Permissions) · [OAuth](https://github.com/daptin/daptin/wiki/OAuth-Authentication) |
| Automate backend behavior | [Actions](https://github.com/daptin/daptin/wiki/Actions-Overview) · [State machines](https://github.com/daptin/daptin/wiki/State-Machines) |
| Connect services or LLMs | [Integrations](https://github.com/daptin/daptin/wiki/Integrations) · [LLM providers](https://github.com/daptin/daptin/wiki/LLM-Providers) |
| Deploy to production | [Production deployment](https://github.com/daptin/daptin/wiki/Production-Deployment) · [Database setup](https://github.com/daptin/daptin/wiki/Database-Setup) |

<details>
<summary><strong>Production checklist</strong></summary>

- Use PostgreSQL or MySQL/MariaDB instead of development SQLite.
- Set stable `jwt.secret` and `encryption.secret` values.
- Configure HTTPS/TLS, hostnames, backups, restore tests, and durable file storage.
- Enable only the protocols your app needs.
- Configure rate limits, metering, monitoring, and audit behavior.

</details>

## Ecosystem

| Project | Use it for |
|---|---|
| [Daptin CLI](https://github.com/daptin/daptin-cli) | Manage CRUD, actions, OAuth, integrations, storage, and assets |
| [JavaScript client](https://github.com/daptin/daptin-js-client) · [Go client](https://github.com/daptin/daptin-go-client) | Connect applications to Daptin |
| [Schema samples](https://github.com/daptin/daptin-schema-samples) | Start from blogs, stores, task lists, FAQs, payments, and more |
| [LLM demo](https://github.com/daptin/daptin-llm-demo) · [Metering demo](https://github.com/daptin/daptin-metering-credit-demo) | Build metered AI and API products |
| [Integration auth demo](https://github.com/daptin/daptin-integration-auth-demo) · [OAuth provider demo](https://github.com/daptin/daptin-oauth-provider-demo) | Explore secure external integrations and identity |
| [Dadadash](https://github.com/daptin/dadadash) | See a larger app built on Daptin |

---

<div align="center">
  <strong><a href="https://github.com/daptin/daptin/wiki">Read the docs</a></strong>
  · <a href="https://discord.gg/t564q8SQVk">Discord</a>
  · <a href="https://github.com/daptin/daptin/issues">Issues</a>
  · <a href="https://github.com/daptin/daptin/releases">Releases</a>
  <br><br>
  LGPL v3 · Build your app. Let Daptin run the backend.
</div>
