# Getting Started with Daptin

This is the shortest complete path from a new installation to an authenticated,
inspectable Daptin server. It keeps the first-run server on localhost, creates
the first administrator, verifies the API, and points to the next guide for each
kind of project.

## 1. Run Daptin locally

The container listens on port 8080. Pull the current `latest` image whenever
the container starts:

```bash
docker run --pull=always --rm \
  --name daptin \
  -p 127.0.0.1:6336:8080 \
  daptin/daptin:latest
```

Wait for `Listening at`, then verify the server from another terminal:

```bash
curl --fail http://localhost:6336/ping
# pong
```

Binding to `127.0.0.1` matters during first setup. A fresh instance has no
administrator yet; do not expose it publicly until the first administrator is
claimed.

Prefer a native binary? See [[Installation]] for release downloads, source
builds, database selection, and persistent storage.

## 2. Create and claim the first administrator

Choose your own email and a strong password. The examples below use shell
variables so the credentials do not need to be repeated in command history.

```bash
export DAPTIN_URL=http://localhost:6336
export DAPTIN_ADMIN_EMAIL=admin@example.test
read -s DAPTIN_ADMIN_PASSWORD
export DAPTIN_ADMIN_PASSWORD
```

Create the first account:

```bash
curl --fail-with-body -sS \
  -X POST "$DAPTIN_URL/action/user_account/signup" \
  -H "Content-Type: application/json" \
  --data "$(jq -n \
    --arg email "$DAPTIN_ADMIN_EMAIL" \
    --arg password "$DAPTIN_ADMIN_PASSWORD" \
    '{attributes:{name:"Administrator",email:$email,password:$password,passwordConfirm:$password}}')"
```

Sign in and keep the returned JWT in memory:

```bash
export DAPTIN_TOKEN="$(
  curl --fail-with-body -sS \
    -X POST "$DAPTIN_URL/action/user_account/signin" \
    -H "Content-Type: application/json" \
    --data "$(jq -n \
      --arg email "$DAPTIN_ADMIN_EMAIL" \
      --arg password "$DAPTIN_ADMIN_PASSWORD" \
      '{attributes:{email:$email,password:$password}}')" |
  jq -r '.[] | select(.ResponseType == "client.store.set" and .Attributes.key == "token") | .Attributes.value'
)"
test -n "$DAPTIN_TOKEN"
```

Claim administrator access:

```bash
curl --fail-with-body -sS \
  -X POST "$DAPTIN_URL/action/world/become_an_administrator" \
  -H "Authorization: Bearer $DAPTIN_TOKEN" \
  -H "Content-Type: application/json" \
  --data '{}'
```

This action is intentionally available only for the first administrator. It
locks down public signup and may restart the runtime. Wait for the server to
answer `/ping`, then run the sign-in command again to obtain a fresh token.

For recovery cases and the exact security transition, use
[[First-Admin-Setup]]. Avoid deleting databases or killing unrelated processes
as a first troubleshooting step.

## 3. Inspect the running resource model

Daptin represents its configured tables in the `world` resource. Verify the
authenticated JSON:API surface:

```bash
curl --fail-with-body -sS \
  "$DAPTIN_URL/api/world?page[size]=5" \
  -H "Authorization: Bearer $DAPTIN_TOKEN" |
  jq '.data[] | {type, id, table_name: .attributes.table_name}'
```

Open [http://localhost:6336](http://localhost:6336) for the administrative
interface. The API remains the system contract; the dashboard and clients use
the same resources and actions.

Useful discovery endpoints:

```text
GET /openapi.yaml       OpenAPI description
GET /meta               Runtime metadata
GET /jsmodel/{typename} Generated JavaScript model
GET /statistics         Operational statistics
GET /ping               Lightweight liveness response
```

## 4. Add your first application resource

Daptin loads schema files and turns each declared table into a
permission-aware JSON:API resource. Follow [[Schema-Definition]] for the schema
format and loading options, or use the tested [[Walkthrough-Product-Catalog]]
to build a complete product catalog with relationships and API calls.

A table named `product` becomes:

```http
GET    /api/product
POST   /api/product
PATCH  /api/product/{reference_id}
DELETE /api/product/{reference_id}
```

The same resource can participate in permissions, actions, events, asset
storage, audit history, metering, GraphQL, and realtime delivery without a
parallel application model.

## 5. Use the CLI when the API is familiar

Install the current CLI through Homebrew, Scoop, a release package, or Go. Then
create a context and inspect the server:

```bash
daptin-cli context add local http://localhost:6336
daptin-cli context set local
daptin-cli execute user_account signin \
  email="$DAPTIN_ADMIN_EMAIL" password="$DAPTIN_ADMIN_PASSWORD"
daptin-cli list --columns table_name,is_top_level world
daptin-cli describe action world import_data
```

See the [Daptin CLI repository](https://github.com/daptin/daptin-cli) for CRUD,
relations, actions, OAuth, integrations, storage, assets, logs, and WebSocket
commands.

## Choose the next path

| Goal | Continue with |
|---|---|
| Model tables, columns, and relationships | [[Core-Concepts]] · [[Schema-Definition]] · [[Relationships]] |
| Build and secure an API | [[API-Overview]] · [[Authentication]] · [[Permissions]] |
| Add backend behavior | [[Actions-Overview]] · [[Task-Scheduling]] · [[State-Machines]] |
| Store and deliver files | [[Cloud-Storage]] · [[Asset-Columns]] · [[Subsites]] |
| Connect external APIs or LLMs | [[Integrations]] · [[Credentials]] · [[LLM-Providers]] |
| Add plans, quotas, or usage accounting | [[API-Metering]] · [[Rate-Limiting]] |
| Deploy outside localhost | [[Production-Deployment]] · [[Database-Setup]] · [[TLS-Certificates]] |

## Before production

Do not carry the disposable first-run command directly into production.
Configure durable database and file storage, stable JWT and encryption secrets,
TLS, backups, monitoring, and only the protocols the application needs. The
[[Production-Deployment]] checklist covers the transition.

## If something fails

Start with the response body and the Daptin process log, then consult
[[Common-Errors]]. Include the Daptin release, database type, command or request,
HTTP status, response body, and relevant log lines when opening a
[GitHub issue](https://github.com/daptin/daptin/issues).
