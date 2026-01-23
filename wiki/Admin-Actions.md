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

## restart_daptin

Restart the Daptin server.

```bash
curl -X POST http://localhost:6336/action/world/restart_daptin \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"attributes": {}}'
```

**Use cases:**
- Apply schema changes
- Enable/disable GraphQL
- Load new configuration
- Clear caches

**Response:**
```json
[
  {"ResponseType": "client.notify", "Attributes": {"message": "Server restarting...", "type": "info"}}
]
```

Server restarts gracefully, maintaining in-flight requests.

## enable_graphql

Enable the GraphQL API endpoint.

```bash
curl -X POST http://localhost:6336/action/world/__enable_graphql \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"attributes": {}}'
```

Alternative via config:

```bash
curl -X POST http://localhost:6336/_config/backend/graphql.enable \
  -H "Authorization: Bearer $TOKEN" \
  -d 'true'

# Restart required
curl -X POST http://localhost:6336/action/world/restart_daptin \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"attributes": {}}'
```

GraphQL endpoint: `http://localhost:6336/graphql`

## download_cms_config

Export complete system configuration.

```bash
curl -X POST http://localhost:6336/action/world/download_cms_config \
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

## delete_table

Drop a table from the database.

```bash
curl -X POST http://localhost:6336/action/world/delete_table \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "table_name": "old_table"
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
      "old_column_name": "price",
      "new_column_name": "unit_price"
    }
  }'
```

Requires server restart to fully apply.

## delete_column

Remove a column from a table.

```bash
curl -X POST http://localhost:6336/action/world/delete_column \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "table_name": "product",
      "column_name": "deprecated_field"
    }
  }'
```

**Warning:** Data in the column is permanently lost.

## generate_self_tls_certificate

Generate self-signed TLS certificate.

```bash
curl -X POST http://localhost:6336/action/world/generate_self_tls_certificate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "hostname": "api.example.com"
    }
  }'
```

**Certificate properties:**
- RSA 2048-bit key
- 365-day validity
- Self-signed

## generate_acme_tls_certificate

Get Let's Encrypt certificate via ACME.

```bash
curl -X POST http://localhost:6336/action/world/generate_acme_tls_certificate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "hostname": "api.example.com",
      "email": "admin@example.com"
    }
  }'
```

**Requirements:**
- Port 80 accessible from internet
- Valid DNS pointing to server
- Email for Let's Encrypt notifications

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

## transaction

Control database transactions.

```bash
# Begin transaction
curl -X POST http://localhost:6336/action/world/transaction \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"attributes": {"action": "begin"}}'

# Commit transaction
curl -X POST http://localhost:6336/action/world/transaction \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"attributes": {"action": "commit"}}'

# Rollback transaction
curl -X POST http://localhost:6336/action/world/transaction \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"attributes": {"action": "rollback"}}'
```

## execute_process

Run external process (dangerous - use carefully).

```bash
curl -X POST http://localhost:6336/action/world/execute_process \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "command": "ls",
      "args": ["-la", "/var/log"]
    }
  }'
```

**Security:** Only available to administrators.

## Adding Admin Users

Add users to administrators group:

```bash
# Get user ID
USER_ID=$(curl 'http://localhost:6336/api/user_account?query=[{"column":"email","operator":"is","value":"newadmin@example.com"}]' \
  -H "Authorization: Bearer $TOKEN" | jq -r '.data[0].id')

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
