# GraphQL API

**Tested ✓ 2026-01-26**

Complete GraphQL API for querying and mutating your Daptin data with full introspection support.

## Overview

Daptin auto-generates a complete GraphQL schema from your table definitions:

- **Queries** - Read operations for all tables
- **Mutations** - Create, update, delete operations
- **Actions** - Execute system actions via GraphQL
- **Aggregations** - Grouped queries with projections
- **Relationships** - Nested queries for foreign keys
- **Introspection** - Self-documenting schema

GraphQL endpoint: `http://localhost:6336/graphql`

GraphiQL playground included for interactive exploration.

## Enabling GraphQL

GraphQL must be enabled before use:

**Option 1: Direct Database (Requires Restart)**
```bash
sqlite3 daptin.db "UPDATE _config SET value='true' WHERE name='graphql.enable';"
./scripts/testing/test-runner.sh stop && ./scripts/testing/test-runner.sh start
```

**Option 2: Via Action (If Permissions Allow)**
```bash
TOKEN=$(cat /tmp/daptin-token.txt)
curl -X POST http://localhost:6336/action/world/__enable_graphql \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"attributes":{}}'
```

**Verification:**
```bash
# Open browser to GraphiQL playground
open http://localhost:6336/graphql

# Or test via curl
curl -X POST http://localhost:6336/graphql \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"query": "{ __schema { types { name } } }"}'
```

## Quick Start

### List All Tables

**Query:**
```graphql
{
  __schema {
    types {
      name
    }
  }
}
```

### Query a Table

**Query:**
```graphql
{
  user_account {
    name
    email
    reference_id
  }
}
```

**curl:**
```bash
TOKEN=$(cat /tmp/daptin-token.txt)
curl -s -X POST http://localhost:6336/graphql \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"query": "{ user_account { name email reference_id } }"}' | jq .
```

**Response:**
```json
{
  "data": {
    "user_account": [
      {
        "email": "admin@admin.com",
        "name": "Admin",
        "reference_id": "019bf973-4c1f-7e27-993a-7f98c638daaa"
      }
    ]
  }
}
```

## Queries

### Basic Query

```graphql
{
  task {
    reference_id
    name
    schedule
    active
  }
}
```

### Query with Pagination

```graphql
{
  task(page: {size: 10, number: 0}) {
    name
    schedule
  }
}
```

- `page.size` - Results per page
- `page.number` - Page number (0-indexed)

### Query with Keyword Filter

```graphql
{
  user_account(filter: "admin") {
    name
    email
  }
}
```

Searches across all text fields for the keyword.

### Query with Structured Filters

```graphql
{
  task(query: [
    {column: "active", operator: "is", value: "1"},
    {column: "name", operator: "like", value: "test-%"}
  ]) {
    name
    schedule
  }
}
```

**Available Operators:**
- `is` - Exact match
- `not` - Negation
- `like` - Pattern matching with % wildcards

**Multiple Conditions:** Combined with AND logic

### Query with Relationships

```graphql
{
  task {
    name
    schedule
    as_user_id {
      name
      email
    }
  }
}
```

Nested queries fetch related data in single request.

## Mutations

### Create Record

```graphql
mutation {
  addTask(
    name: "daily-backup"
    action_name: "backup_database"
    entity_name: "world"
    schedule: "@daily"
    active: true
    attributes: "{}"
    job_type: "backup"
    as_user_id: "USER_REFERENCE_ID"
  ) {
    reference_id
    name
    schedule
  }
}
```

**curl:**
```bash
TOKEN=$(cat /tmp/daptin-token.txt)
USER_ID="019bf973-4c20-75f7-b5b1-e9d26c398eee"

curl -s -X POST http://localhost:6336/graphql \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "mutation { addTask(name: \"daily-backup\", action_name: \"backup_database\", entity_name: \"world\", schedule: \"@daily\", active: true, attributes: \"{}\", job_type: \"backup\", as_user_id: \"'$USER_ID'\") { reference_id name schedule } }"
  }' | jq .
```

### Update Record

```graphql
mutation {
  updateTask(
    reference_id: "019bf9a6-7326-75ec-a94a-c12aed3d0e07"
    schedule: "@hourly"
    active: true
  ) {
    reference_id
    name
    schedule
  }
}
```

### Delete Record

```graphql
mutation {
  deleteTask(reference_id: "019bf9a6-7326-75ec-a94a-c12aed3d0e07") {
    reference_id
    name
  }
}
```

**Important:** Delete mutations MUST include field selection.

### Batch Mutations

Execute multiple mutations in a single request using aliases:

```graphql
mutation {
  backup: addTask(
    name: "backup-task"
    action_name: "backup"
    entity_name: "world"
    schedule: "@daily"
    active: true
    attributes: "{}"
    job_type: "backup"
  ) {
    name
  }

  cleanup: addTask(
    name: "cleanup-task"
    action_name: "cleanup"
    entity_name: "world"
    schedule: "@weekly"
    active: true
    attributes: "{}"
    job_type: "maintenance"
  ) {
    name
  }
}
```

**Note:** Partial success is supported - one mutation failing doesn't rollback others.

## Actions

All Daptin actions are available as GraphQL mutations.

### List Available Actions

```graphql
{
  __schema {
    mutationType {
      fields {
        name
      }
    }
  }
}
```

**Filter for actions:**
```bash
curl -s -X POST http://localhost:6336/graphql \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"query": "{ __schema { mutationType { fields { name } } } }"}' | \
  jq '.data.__schema.mutationType.fields[].name' | grep execute | head -10
```

**Result:**
```
"executeAddExchangeOnWorld"
"executeBecomeAnAdministratorOnWorld"
"executeCreateFolderOnCloudStore"
"executeCreateSiteOnCloudStore"
"executeDeleteFileOnSite"
```

### Execute Action

**Pattern:** `execute{ActionName}On{EntityName}`

**Example - Download Schema:**
```graphql
mutation {
  executeDownloadSystemSchemaOnWorld {
    ResponseType
    Attributes {
      message
    }
  }
}
```

**curl:**
```bash
curl -s -X POST http://localhost:6336/graphql \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"query": "mutation { executeDownloadSystemSchemaOnWorld { ResponseType Attributes { message } } }"}' | jq .
```

**Response:**
```json
{
  "data": {
    "executeDownloadSystemSchemaOnWorld": [
      {
        "Attributes": {
          "message": "Downloading system schema"
        },
        "ResponseType": "client.file.download"
      }
    ]
  }
}
```

**Action Response Fields:**
- `ResponseType` - Action response type (e.g., client.notify, client.file.download)
- `Attributes` - Action-specific response data
  - `message` - Status message
  - `value` - Return value
  - `title` - Alert title
  - `key` - Data key
  - `location` - Redirect location
  - `token` - Auth token

## Aggregations

Group and aggregate data:

```graphql
{
  aggregateTask {
    active
  }
}
```

## Introspection

### Inspect Table Schema

```graphql
{
  __type(name: "task") {
    fields {
      name
      type {
        name
        kind
      }
    }
  }
}
```

### Inspect Query Arguments

```graphql
{
  __type(name: "RootQuery") {
    fields {
      name
      args {
        name
        type {
          name
        }
      }
    }
  }
}
```

## GraphQL vs REST API

### When to Use GraphQL

**✅ Use GraphQL When:**
- Fetching related data (single request vs multiple REST calls)
- Need precise field selection (reduce payload size)
- Building complex UIs with nested data
- Want type-safe API with introspection
- Need batch operations in single request

**Example - GraphQL Advantage:**
```graphql
# Single GraphQL request
{
  task {
    name
    schedule
    as_user_id {
      name
      email
    }
  }
}
```

```bash
# Multiple REST requests required
GET /api/task
GET /api/user_account/:id  # For each task's user
GET /api/user_account/:id  # For each task's user
```

### When to Use REST API

**✅ Use REST When:**
- Need sorting/ordering (GraphQL doesn't support `sort` argument yet)
- Working with HTTP caching infrastructure
- Uploading files (simpler with REST multipart)
- Building simple CRUD interfaces
- Client doesn't support GraphQL

## Complete Examples

### Example 1: Task Management Dashboard

**Fetch tasks with user info and filter:**
```graphql
{
  activeTasks: task(
    query: [{column: "active", operator: "is", value: "1"}]
    page: {size: 10}
  ) {
    reference_id
    name
    schedule
    as_user_id {
      name
      email
    }
  }

  inactiveTasks: task(
    query: [{column: "active", operator: "is", value: "0"}]
    page: {size: 5}
  ) {
    reference_id
    name
  }
}
```

### Example 2: User Account with Groups

```graphql
{
  user_account(filter: "admin") {
    reference_id
    name
    email
    permission
  }
}
```

### Example 3: Create and Link Records

```graphql
mutation {
  newTask: addTask(
    name: "weekly-report"
    action_name: "generate_report"
    entity_name: "analytics"
    schedule: "@weekly"
    active: true
    attributes: "{\"format\": \"pdf\"}"
    job_type: "reporting"
    as_user_id: "USER_ID_HERE"
  ) {
    reference_id
    name
    as_user_id {
      name
      email
    }
  }
}
```

## Common Pitfalls

### 1. The "id" Field Returns Null

**Problem:**
```graphql
{ task { id name } }  # Error: Cannot return null for non-nullable field
```

**Solution:** Use `reference_id` instead:
```graphql
{ task { reference_id name } }  # ✅ Works
```

### 2. Delete Mutation Needs Field Selection

**Problem:**
```graphql
mutation { deleteTask(reference_id: "...") }  # Error: must have sub selection
```

**Solution:**
```graphql
mutation { deleteTask(reference_id: "...") { reference_id } }  # ✅ Works
```

### 3. Action Attributes Needs Sub-Selection

**Problem:**
```graphql
mutation { executeAction { ResponseType Attributes } }  # Error
```

**Solution:**
```graphql
mutation { executeAction { ResponseType Attributes { message value } } }  # ✅
```

### 4. Query Operator "contains" Doesn't Work

**Problem:**
```graphql
{ task(query: [{column: "name", operator: "contains", value: "test"}]) {...} }
# Returns empty
```

**Solution:** Use "like" with % wildcards:
```graphql
{ task(query: [{column: "name", operator: "like", value: "%test%"}]) {...} }
```

## Troubleshooting

### GraphQL Endpoint Returns HTML

**Symptoms:** `/graphql` returns admin UI instead of GraphQL response

**Solution:** Enable GraphQL (see [Enabling GraphQL](#enabling-graphql))

### Permission Errors

**Symptoms:** 403 Forbidden on GraphQL requests

**Solution:**
1. Verify JWT token is valid
2. Check user has required permissions
3. Become administrator if needed:
   ```bash
   curl -X POST http://localhost:6336/action/world/become_an_administrator \
     -H "Authorization: Bearer $TOKEN"
   ```

### Empty Query Results

**Check Data Exists:**
```bash
sqlite3 daptin.db "SELECT reference_id, name FROM task LIMIT 5;"
```

**Verify Filter Logic:**
```graphql
# Try without filters first
{ task { reference_id name } }

# Then add filters one by one
{ task(query: [{column: "active", operator: "is", value: "1"}]) { name } }
```

## Best Practices

1. **Use reference_id not id** - The `id` field returns null, use `reference_id`
2. **Filter with "is" not "eq"** - "is" operator is more reliable for exact matches
3. **Use "like" for patterns** - Pattern matching with SQL-style % wildcards
4. **Batch with aliases** - Combine multiple mutations using aliases
5. **Select minimal fields** - Only request fields you need to reduce payload
6. **Leverage relationships** - Fetch related data in single request
7. **Use introspection** - Explore schema with `__schema` and `__type` queries
8. **Test in GraphiQL** - Use the playground at `/graphql` for development

## Limitations

Current limitations (as of 2026-01-26):

1. **No Sorting** - GraphQL doesn't support `sort` argument (use REST API for sorting)
2. **No OR Logic** - Multiple query conditions use AND only
3. **Has-Many Relationships** - May not work as expected, use belongs-to instead
4. **"in" Operator** - Doesn't accept arrays, only string values
5. **Aggregations** - Basic support, full functionality unclear

## Related

- [REST API](Getting-Started-Guide.md#rest-api) - Traditional REST endpoints
- [Actions Overview](Actions-Overview.md) - Available actions via GraphQL
- [Permissions](Permissions.md) - Access control for GraphQL
- [Task Scheduling](Task-Scheduling.md) - Automate actions tested via GraphQL
