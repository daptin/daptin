# Daptin Getting Started Guide

A practical onboarding guide to get you up and running with Daptin.

---

## Quick Start

### 1. Run Daptin

```bash
# From source
go run main.go

# With custom port
go run main.go -port :8080

# With PostgreSQL
go run main.go -db_type postgres -db_connection_string "host=localhost port=5432 user=postgres password=secret dbname=daptin sslmode=disable"
```

### 2. Access Dashboard

Open `http://localhost:6336` in your browser.

### 3. Health Check

```bash
# Quick health check
curl http://localhost:6336/ping
# Returns: pong

# Full system statistics
curl http://localhost:6336/statistics
```

---

## Core Concepts

### Tables (Entities)

Daptin stores data in tables. Define tables via YAML schema:

```yaml
Tables:
  - TableName: todo
    Columns:
      - Name: title
        DataType: varchar(500)
        ColumnType: label
      - Name: completed
        DataType: bool
        ColumnType: truefalse
        DefaultValue: "false"
```

### API Access

All tables get automatic REST API:

```bash
# List all todos
curl http://localhost:6336/api/todo

# Create todo
curl -X POST http://localhost:6336/api/todo \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "todo",
      "attributes": {
        "title": "My first task"
      }
    }
  }'

# Update todo
curl -X PATCH http://localhost:6336/api/todo/REFERENCE_ID \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "todo",
      "id": "REFERENCE_ID",
      "attributes": {
        "completed": true
      }
    }
  }'

# Delete todo
curl -X DELETE http://localhost:6336/api/todo/REFERENCE_ID
```

---

## Authentication

### First Admin (CRITICAL BOOTSTRAPPING)

**⚠️ Security Note**: Until an admin exists, ALL authenticated users have full system access. Secure your system immediately.

```bash
# Step 1: Sign up a new user
curl -X POST http://localhost:6336/action/user_account/signup \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "name": "Admin User",
      "email": "admin@example.com",
      "password": "securepass123",
      "passwordConfirm": "securepass123"
    }
  }'

# Step 2: Sign in to get JWT token
curl -X POST http://localhost:6336/action/user_account/signin \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "email": "admin@example.com",
      "password": "securepass123"
    }
  }'
# Extract token from response: .[] | select(.ResponseType == "client.store.set") | .Attributes.value

# Step 3: Become administrator (IMPORTANT: action is on "world" table, not "user_account")
curl -X POST http://localhost:6336/action/world/become_an_administrator \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{}'
# Server will restart. Sign in again to get admin-privileged token.

# Step 4: Re-signin after restart
curl -X POST http://localhost:6336/action/user_account/signin \
  -H "Content-Type: application/json" \
  -d '{"attributes":{"email":"admin@example.com","password":"securepass123"}}'
```

**How it works**:
- The `become_an_administrator` action checks if the "administrators" usergroup has any members
- If no admin exists, it adds the current user to the administrators group
- The server restarts to apply permission changes
- After restart, only users in "administrators" group have admin privileges

**Verification**: Check admin status via database:
```sql
SELECT ua.name, ug.name as group_name
FROM user_account ua
JOIN user_account_user_account_id_has_usergroup_usergroup_id j ON ua.id = j.user_account_id
JOIN usergroup ug ON j.usergroup_id = ug.id
WHERE ug.name = 'administrators';
```

### Using JWT Token

```bash
export TOKEN="your-jwt-token"

curl http://localhost:6336/api/todo \
  -H "Authorization: Bearer $TOKEN"
```

---

## Permissions

Daptin uses a 21-bit permission system. **Important: The order is Guest → User → Group**

### Permission Bits

| Scope | Bits | Base Value |
|-------|------|------------|
| Guest | 0-6 | 1 |
| User | 7-13 | 128 |
| Group | 14-20 | 16384 |

### Permission Types

| Type | Offset | Guest | User | Group |
|------|--------|-------|------|-------|
| Peek | 0 | 1 | 128 | 16384 |
| Read | 1 | 2 | 256 | 32768 |
| Create | 2 | 4 | 512 | 65536 |
| Update | 3 | 8 | 1024 | 131072 |
| Delete | 4 | 16 | 2048 | 262144 |
| Execute | 5 | 32 | 4096 | 524288 |
| Refer | 6 | 64 | 8192 | 1048576 |

### Common Permission Values

```javascript
// Calculate permission
function calcPerm(guest, user, group) {
  return guest + (user << 7) + (group << 14);
}

// Examples:
// User full access only: calcPerm(0, 127, 0) = 16256
// User + Group read: calcPerm(0, 2, 2) = 33024
// Public read: calcPerm(2, 2, 2) = 33026
// Full access all: calcPerm(127, 127, 127) = 2097151
```

---

## Filtering & Querying

### JSON Query Syntax

```bash
# Filter by exact value
curl 'http://localhost:6336/api/todo?query=[{"column":"completed","operator":"is","value":"true"}]'

# Multiple filters (AND)
curl 'http://localhost:6336/api/todo?query=[{"column":"completed","operator":"is","value":"false"},{"column":"title","operator":"contains","value":"urgent"}]'
```

### Operators

| Operator | Description |
|----------|-------------|
| `is`, `eq` | Exact match |
| `is not`, `neq` | Not equal |
| `contains` | Substring match |
| `begins with` | Starts with |
| `ends with` | Ends with |
| `before`, `less than` | Less than |
| `after`, `more than` | Greater than |
| `any of`, `in` | In list |
| `none of` | Not in list |
| `is empty` | Is null |

### Pagination

```bash
curl "http://localhost:6336/api/todo?page[number]=1&page[size]=10"
```

### Sorting

```bash
# Ascending
curl "http://localhost:6336/api/todo?sort=created_at"

# Descending
curl "http://localhost:6336/api/todo?sort=-created_at"
```

---

## Actions

Actions are custom business logic endpoints.

### Built-in Actions

```bash
# User signup
curl -X POST http://localhost:6336/action/user_account/signup \
  -H "Content-Type: application/json" \
  -d '{"attributes": {"name": "User", "email": "user@example.com", "password": "pass", "passwordConfirm": "pass"}}'

# User signin
curl -X POST http://localhost:6336/action/user_account/signin \
  -H "Content-Type: application/json" \
  -d '{"attributes": {"email": "user@example.com", "password": "pass"}}'

# Generate JWT token for current user
curl -X POST http://localhost:6336/action/user_account/jwt.token \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{}'

# Register OTP for 2FA (creates TOTP secret)
curl -X POST http://localhost:6336/action/user_account/register_otp \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"attributes": {"email": "user@example.com"}}'
```

### Action Reference

| Action Name | Entity | Purpose |
|-------------|--------|---------|
| `signup` | user_account | Create new user |
| `signin` | user_account | Authenticate user |
| `jwt.token` | user_account | Generate JWT |
| `register_otp` | user_account | Generate OTP for 2FA |
| `verify_otp` | user_account | Verify OTP |
| `become_an_administrator` | world | Claim admin (first user only) |
| `restart_daptin` | world | Restart Daptin |
| `__enable_graphql` | world | Enable GraphQL API |
| `generate_self_certificate` | certificate | Generate self-signed cert |
| `generate_acme_certificate` | certificate | Generate Let's Encrypt cert |

**Note**: `mail.send` and `aws.mail.send` are **internal performers**, not REST actions. They are used in action OutFields (e.g., password reset flows).

---

## Aggregation

SQL-like aggregations via REST API:

```bash
# Count all
curl "http://localhost:6336/aggregate/todo?column=count"

# Group by status
curl "http://localhost:6336/aggregate/order?group=status&column=status,count,sum(total)"

# With filter
curl "http://localhost:6336/aggregate/order?filter=gt(total,100)&column=count,sum(total)"
```

### Aggregate Functions

- `count` - Count records
- `sum(column)` - Sum values
- `avg(column)` - Average
- `min(column)` - Minimum
- `max(column)` - Maximum
- `first(column)` - First value
- `last(column)` - Last value

---

## Real-time Updates

### WebSocket Connection

```javascript
const ws = new WebSocket(`ws://localhost:6336/live?token=${TOKEN}`);

// Subscribe to table changes
ws.send(JSON.stringify({
  method: 'subscribe',
  attributes: { topicName: 'todo' }
}));

// Receive events
ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log('Event:', data.event, data.data);
};
```

---

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DAPTIN_PORT` | HTTP port | `:6336` |
| `DAPTIN_HTTPS_PORT` | HTTPS port | `:6443` |
| `DAPTIN_DB_TYPE` | Database type | `sqlite3` |
| `DAPTIN_DB_CONNECTION_STRING` | DB connection | `daptin.db` |
| `DAPTIN_LOG_LEVEL` | Log level | `info` |
| `DAPTIN_LOG_LOCATION` | Log file path | stdout |

### Command Line Flags

```bash
./daptin \
  -db_type postgres \
  -db_connection_string "host=localhost..." \
  -port :8080 \
  -https_port :8443 \
  -log_level debug \
  -runtime release
```

### Runtime Configuration

```bash
# Set config value
curl -X POST http://localhost:6336/_config/backend/graphql.enable \
  -H "Authorization: Bearer $TOKEN" \
  -d 'true'

# Get config value
curl http://localhost:6336/_config/backend/graphql.enable \
  -H "Authorization: Bearer $TOKEN"
```

---

## Common Tasks

### Import Data from CSV

```bash
curl -X POST http://localhost:6336/action/world/__upload_csv_file_to_entity \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@data.csv"
```

### Export Data

```bash
curl -X POST http://localhost:6336/action/world/__data_export \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"attributes": {"table_name": "todo", "format": "csv"}}'
```

### Enable GraphQL

```bash
curl -X POST http://localhost:6336/action/world/__enable_graphql \
  -H "Authorization: Bearer $TOKEN"
```

Then access GraphQL at `http://localhost:6336/graphql`.

---

## Troubleshooting

### Check Server Status

```bash
# Quick health
curl http://localhost:6336/ping

# Full statistics
curl http://localhost:6336/statistics
```

### Common Issues

1. **API returns HTML** - Add `Accept: application/vnd.api+json` header
2. **401 Unauthorized** - Check JWT token is valid
3. **Permission denied** - Verify permission values (Guest→User→Group order)
4. **Action not found** - Check exact action name (use dots: `mail.send` not `mail_send`)

---

## Next Steps

1. [Schema Definition](Schema-Definition.md) - Define your data model
2. [Permissions](Permissions.md) - Configure access control
3. [Actions Overview](Actions-Overview.md) - Create custom business logic
4. [WebSocket API](WebSocket-API.md) - Real-time features
5. [GraphQL API](GraphQL-API.md) - Alternative query interface
