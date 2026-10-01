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

**Response:**
```json
[
  {"ResponseType": "client.notify", "Attributes": {"message": "You are now the administrator", "type": "success"}}
]
```

## Restarting Daptin

Process restarts are managed by Kubernetes, Docker, systemd, or another process
supervisor rather than an administrative HTTP action.

**Use cases:**
- Apply schema changes
- Enable/disable GraphQL
- Load new configuration
- Clear caches

Use that supervisor's normal restart command so Daptin receives its standard
shutdown signal and drains through the runtime lifecycle.

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

Add users to administrators group:

```bash
# Get user ID
USER_ID=$(curl --get -s \
  --data-urlencode 'query=[{"column":"email","operator":"is","value":"newadmin@example.com"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/user_account" | jq -r '.data[0].id')

# Add to administrators
curl -X POST http://localhost:6336/api/user_account_administrators_has_usergroup_administrators \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d "{
    \"data\": {
      \"type\": \"user_account_administrators_has_usergroup_administrators\",
      \"attributes\": {
        \"user_account_id\": \"$USER_ID\"
      }
    }
  }"
```
