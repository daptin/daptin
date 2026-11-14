# Admin Actions

System administration actions requiring administrator privileges.

## become_an_administrator

**One-time action:** First user becomes system administrator.

```bash
curl -X POST http://localhost:6336/action/world/become_an_administrator \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"attributes": {}}'
```

**Effects:**
1. Creates `administrators` usergroup
2. Adds requesting user to administrators
3. Locks down all tables (admin-only access by default)
4. Disables action for future use

Daptin restarts automatically after the claim; see [[First-Admin-Setup]] for
the response and readiness procedure.

**Response:**
```json
[
  {"ResponseType": "client.notify", "Attributes": {"message": "You are now the administrator", "type": "success"}}
]
```

## Restarting Daptin

Available in `v0.13.51` and later.

Use the administrator-only `world/restart_daptin` action to apply startup-loaded
configuration or schema changes:

```bash
curl http://localhost:6336/action/world/restart_daptin \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"attributes":{}}'
```

A successful call returns HTTP 200 with `ResponseType: "Restart"`. The action's
`__restart` performer requests the existing process lifecycle: readiness becomes
unavailable, accepted HTTP requests drain, runtime services shut down, and Daptin
starts again with the same arguments and environment. On Unix, process replacement
preserves the PID, including PID 1 in Docker. No Docker restart policy is required.

The action has no broad execution grant and is shared with `administrators`.
Guests and ordinary accounts receive HTTP 403. First-administrator onboarding
uses the same lifecycle after committing its changes.

During restart, `/ready` can return 503 or the connection can be unavailable.
Wait for the new runtime to become ready before sending application traffic;
check the configuration you changed to confirm it was loaded. The restart
request acknowledges the request, not completion of the restart.

For example, startup-loaded `limit.rate` needs a restart. Normal plan, model
catalog, and relationship permission edits do not. Process supervisors can still
restart Daptin through its normal shutdown signal. There is no bare `/restart`
route; use the named action above.

## Enable GraphQL

Set the administrator-only backend configuration value:

```bash
curl -X POST http://localhost:6336/_config/backend/graphql.enable \
  -H "Authorization: Bearer $TOKEN" \
  --data 'true'
```

Restart Daptin with its process supervisor because GraphQL routes are composed
at startup. There is no public `__enable_graphql` action.

GraphQL endpoint: `http://localhost:6336/graphql`

## download_system_schema

Export complete system configuration.

```bash
curl -X POST http://localhost:6336/action/world/download_system_schema \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"attributes": {}}'
```

**Response:**
```json
[
  {
    "ResponseType": "client.file.download",
    "Attributes": {
      "content": "base64-encoded-json",
      "name": "schema_exported.json",
      "contentType": "application/json"
    }
  }
]
```

Exports:
- Table definitions
- Column configurations
- Relationships
- Actions
- State machines
- Integrations

## remove_table

Remove a table and its data. Get the table's `world` reference ID from
`/api/world`.

```bash
curl -X POST http://localhost:6336/action/world/remove_table \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "world_id": "WORLD_REFERENCE_ID"
    }
  }'
```

**Warning:** This permanently deletes the table and all data.

## rename_column

Rename a column in a table.

Table and column identifiers must match `[A-Za-z_][A-Za-z0-9_]*`. Spaces in the
new name are converted to underscores before validation. Reserved words and
names already used by another column are rejected before the rename runs.

```bash
curl -X POST http://localhost:6336/action/world/rename_column \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "table_name": "product",
      "column_name": "price",
      "new_column_name": "unit_price"
    }
  }'
```

Requires server restart to fully apply.

## remove_column

Remove a column from a table using its `world` reference ID.

```bash
curl -X POST http://localhost:6336/action/world/remove_column \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "world_id": "WORLD_REFERENCE_ID",
      "column_name": "deprecated_field"
    }
  }'
```

**Warning:** Data in the column is permanently lost.

## generate_self_certificate

Generate a self-signed TLS certificate for an existing `certificate` row.

```bash
curl -X POST http://localhost:6336/action/certificate/generate_self_certificate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  --data-binary '{"attributes":{},"certificate_id":"CERTIFICATE_REFERENCE_ID"}'
```

**Certificate properties:**
- RSA 2048-bit key
- 365-day validity
- Self-signed

## generate_acme_certificate

Get a Let's Encrypt production certificate via the instance action on an
existing `certificate` resource.

```bash
curl -X POST http://localhost:6336/action/certificate/generate_acme_certificate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "email": "admin@example.com"
    },
    "certificate_id": "CERTIFICATE_REFERENCE_ID"
  }'
```

**Requirements:**
- Port 80 accessible from internet
- Valid DNS pointing to server
- Email for Let's Encrypt notifications
- Existing `certificate` row whose hostname is the requested DNS name
- Awareness that the ACME action uses Let's Encrypt's production directory

## download_certificate

Export TLS certificate.

```bash
curl -X POST http://localhost:6336/action/certificate/download_certificate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "certificate_id": "CERT_REFERENCE_ID"
    }
  }'
```

## Adding Admin Users

As an existing administrator, open the intended account's **Groups** tab and
attach `administrators`. Verify the account and group before granting this
access. See [[Users-and-Groups#add-another-admin]] for the membership workflow;
there is no special `user_account_administrators` application resource.
