# Daptin Quick Start Guide for New Users and LLMs

## Overview
Daptin is a self-discoverable GraphQL/REST API server with comprehensive OpenAPI documentation. This guide documents key findings from testing a fresh Daptin instance.

## Starting Daptin
```bash
# Start on custom port with SQLite
daptin -port 8081 -database sqlite3 -dbname test_discovery.db

# Default port is 6336
# Use daptin -h for all CLI options
```

## First Steps - Critical Admin Setup

### 1. Create First User
```bash
curl -X POST http://localhost:8081/action/user_account/signup \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "email": "admin@test.com",
      "password": "testpass123"  # Must be 8+ characters
    }
  }'
```

### 2. Sign In and Get JWT Token
```bash
TOKEN=$(curl -X POST http://localhost:8081/action/user_account/signin \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "email": "admin@test.com",
      "password": "testpass123"
    }
  }' | jq -r '.[0].Attributes.value')
```

### 3. Become Administrator (ONE-TIME ONLY!)
⚠️ **CRITICAL**: This is a one-time, irreversible action. The first user to invoke this becomes the permanent admin.

```bash
curl -X POST http://localhost:8081/action/world/become_an_administrator \
  -H "Authorization: Bearer $TOKEN"
```

## Key API Discovery Endpoints

### OpenAPI Documentation
```bash
# Download complete API specification
curl http://localhost:8081/openapi.yaml -o openapi.yaml
```

### List All Entities
```bash
curl http://localhost:8081/api/world \
  -H "Authorization: Bearer $TOKEN"
```

### List Available Actions
```bash
curl http://localhost:8081/api/action \
  -H "Authorization: Bearer $TOKEN"
```

### Download System Schema
```bash
curl -X POST http://localhost:8081/action/world/download_system_schema \
  -H "Authorization: Bearer $TOKEN"
```

## Creating New Entities

### 1. Define New Entity
```bash
curl -X POST http://localhost:8081/api/world \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "data": {
      "type": "world",
      "attributes": {
        "table_name": "book",
        "world_schema_json": "{\"Columns\":[{\"Name\":\"title\",\"DataType\":\"varchar(500)\",\"ColumnType\":\"label\",\"IsIndexed\":false,\"IsUnique\":false,\"IsNullable\":false,\"Permission\":2097151,\"DefaultValue\":\"\",\"ForeignKeyData\":{\"DataSource\":\"\",\"Namespace\":\"\",\"KeyName\":\"\",\"OnDelete\":\"\",\"OnUpdate\":\"\"}},{\"Name\":\"author\",\"DataType\":\"varchar(200)\",\"ColumnType\":\"label\",\"IsIndexed\":false,\"IsUnique\":false,\"IsNullable\":true,\"Permission\":2097151,\"DefaultValue\":\"\",\"ForeignKeyData\":{\"DataSource\":\"\",\"Namespace\":\"\",\"KeyName\":\"\",\"OnDelete\":\"\",\"OnUpdate\":\"\"}}],\"Relations\":[]}"
      }
    }
  }'
```

### 2. Restart Server (Required for Schema Changes)
```bash
curl -X POST http://localhost:8081/action/world/restart_daptin \
  -H "Authorization: Bearer $TOKEN"
```

## Authentication Best Practices

### Always Include Bearer Token
```bash
# Correct
curl http://localhost:8081/api/user_account \
  -H "Authorization: Bearer $TOKEN"

# Wrong - will get 403 or empty results
curl http://localhost:8081/api/user_account
```

### Token Validity
- JWT tokens are valid for 3 days
- Store token securely for reuse
- Re-authenticate when token expires

## Multi-Admin Support

### Add User to Administrators Group
1. Find the administrators usergroup ID
2. Add user to the group via usergroup relationship
3. Any user in "administrators" group has admin permissions

## Common Issues and Solutions

### Password Validation Error
- Error: "min and 0 more errors, invalid value for password"
- Solution: Use password with 8+ characters

### Empty API Responses
- Cause: Missing or incorrect Authorization header
- Solution: Ensure TOKEN variable is set and passed correctly

### 403 Forbidden Errors
- Cause: User not authenticated or lacks permissions
- Solution: Check token is valid and user has appropriate permissions

### Schema Changes Not Reflected
- Cause: Server needs restart after entity creation
- Solution: Use `/action/world/restart_daptin` action

## Restricted Actions (Even for Admins)
Some actions remain restricted:
- `generate_random_data` - Returns 403
- `get_action_schema` - Returns 403

## Key Findings Summary

### Self-Discoverability (9/10)
✅ Comprehensive OpenAPI documentation at `/openapi.yaml`
✅ Meta-endpoints for discovering entities and actions
✅ Clear JSON:API specification compliance
✅ Consistent CRUD patterns across all entities

### Self-Management (7/10)
✅ Dynamic entity creation via API
✅ Programmatic server restart capability
✅ Multi-admin support via usergroups
✅ Schema export/import functionality
❌ Some admin actions still restricted
❌ Most schema changes require restart

## Permission Model
- Default permission value: 2097151 (full access)
- Before admin setup: ALL users have full access
- After admin setup: Only admins retain full access
- Use usergroups for role-based access control

## Column Types Reference
Daptin supports various column types with validations:
- `id`, `alias` - Identifiers
- `email` - Email with validation
- `password`, `bcrypt`, `md5` - Password types
- `date`, `time`, `datetime` - Temporal types
- `json` - JSON data
- `file`, `image`, `video` - Binary data
- `location` - Geographic coordinates
- See `/server/resource/column_types.go` for full list

## Next Steps
1. Explore GraphQL endpoint at `/graphql`
2. Set up cloud storage integrations
3. Configure state machines for workflows
4. Create custom actions for business logic
5. Set up data exchanges for integrations

## Resources
- OpenAPI Spec: http://localhost:8081/openapi.yaml
- Admin Email: admin@test.com (after setup)
- Default Permission: 2097151 (full access)
- Token Validity: 3 days

---
Generated from testing Daptin instance on port 8081 with SQLite database.