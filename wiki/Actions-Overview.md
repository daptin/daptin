# Actions Overview

Actions are named operations you can invoke via API to perform specific tasks. Unlike CRUD operations (create, read, update, delete) which work on individual records, actions execute business logic - sending emails, uploading files, generating tokens, or calling external APIs.

---

## Quick Start

**Tested ✓** - All examples on this page were verified against a running Daptin instance.

### Call an Action

```bash
# Entity-level action (no specific record needed)
curl -X POST http://localhost:6336/action/{entity}/{action_name} \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"attributes": {...}}'

# Instance action (operates on a specific record)
curl -X POST http://localhost:6336/action/{entity}/{record_id}/{action_name} \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"attributes": {...}}'
```

### Example: Download System Schema

```bash
curl -X POST http://localhost:6336/action/world/download_system_schema \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"attributes": {}}'
```

Response:
```json
[
  {
    "ResponseType": "client.file.download",
    "Attributes": {
      "content": "base64-encoded-schema-json",
      "name": "schema.json",
      "contentType": "application/json"
    }
  }
]
```

---

## Understanding Response Types

Every action returns an array of responses. Each response has a `ResponseType` that tells clients what to do with the result.

| ResponseType | What It Does |
|--------------|--------------|
| `client.notify` | Show a message (success, error, info) |
| `client.redirect` | Navigate to a URL |
| `client.file.download` | Download a file (content is base64) |
| `client.store.set` | Store a value (for frontend localStorage) |
| `client.cookie.set` | Set a cookie |

### Response Examples

**Notification:**
```json
{
  "ResponseType": "client.notify",
  "Attributes": {
    "type": "success",
    "title": "Success",
    "message": "Operation completed"
  }
}
```

**File Download:**
```json
{
  "ResponseType": "client.file.download",
  "Attributes": {
    "content": "SGVsbG8gV29ybGQ=",
    "name": "data.txt",
    "contentType": "text/plain"
  }
}
```

**Redirect:**
```json
{
  "ResponseType": "client.redirect",
  "Attributes": {
    "location": "/dashboard",
    "delay": 2000
  }
}
```

---

## Built-in Actions by Category

Daptin includes 40+ built-in actions. Here are the most commonly used ones grouped by purpose.

### Authentication & Users

| Action | Entity | Description | Instance Required |
|--------|--------|-------------|-------------------|
| `signup` | user_account | Register new user | No |
| `signin` | user_account | Get JWT token | No |
| `reset-password` | user_account | Request password reset email | No |
| `reset-password-verify` | user_account | Complete password reset | No |
| `generate_jwt_token` | user_account | Create API token for user | Yes |
| `otp_generate` | user_account | Enable 2FA | Yes |
| `otp_login_verify` | user_account | Verify 2FA code | No |

**Example: Sign In**
```bash
curl -X POST http://localhost:6336/action/user_account/signin \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "email": "admin@admin.com",
      "password": "adminadmin"
    }
  }'
```

### OAuth Integration

| Action | Entity | Description |
|--------|--------|-------------|
| `oauth_login_begin` | oauth_connect | Start OAuth flow (returns redirect URL) |
| `oauth.login.response` | oauth_token | Handle OAuth callback |

See [[Authentication|Authentication]] for OAuth setup.

### Administration

| Action | Entity | Description |
|--------|--------|-------------|
| `become_an_administrator` | world | First user claims admin role |
| `download_system_schema` | world | Export full schema as JSON |
| `upload_csv_to_system_schema` | world | Create/update tables from CSV |
| `restart` | world | Restart Daptin server |
| `enable_graphql` | world | Enable GraphQL endpoint |

**Example: Export Schema**
```bash
curl -X POST http://localhost:6336/action/world/download_system_schema \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"attributes": {}}'
```

### Data Import/Export

| Action | Entity | Description |
|--------|--------|-------------|
| `__data_import` | any table | Import JSON/CSV/XLSX data |
| `__data_export` | any table | Export table data |
| `__csv_data_export` | any table | Export as CSV file |

**Example: Export Data as CSV**
```bash
curl -X POST http://localhost:6336/action/user_account/__csv_data_export \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"attributes": {}}'
```

### Cloud Storage

All cloud storage actions require `cloud_store_id` in attributes.

| Action | Entity | Description |
|--------|--------|-------------|
| `upload_file` | cloud_store | Upload files (base64) |
| `create_folder` | cloud_store | Create directory |
| `delete_path` | cloud_store | Delete file or folder |
| `move_path` | cloud_store | Move/rename file or folder |
| `create_site` | cloud_store | Create a subsite |
| `list_files` | site | List site files |
| `get_file` | site | Get file content |

See [[Cloud-Storage|Cloud Storage]] for full examples.

### Email

| Action | Entity | Description |
|--------|--------|-------------|
| `mail.send` | mail_server | Send email via SMTP |
| `aws.mail.send` | mail_server | Send via AWS SES |

See [[Email-Actions|Email-Actions]] for setup.

### TLS Certificates

| Action | Entity | Description |
|--------|--------|-------------|
| `generate_self_tls_certificate` | world | Generate self-signed certificate |
| `generate_acme_tls_certificate` | world | Get Let's Encrypt certificate |
| `download_certificate` | certificate | Download certificate files |

### Data Exchange

| Action | Entity | Description |
|--------|--------|-------------|
| `add_exchange` | world | Create data sync job |

See [[Data-Exchange|Data-Exchange]] for details.

### Integrations

| Action | Entity | Description |
|--------|--------|-------------|
| `integration_install` | integration | Install OpenAPI integration |
| `integration_execute` | integration | Execute integration action |

See [[Integrations|Integrations]] for setup.

---

## List Available Actions

Query the `action` table to see all registered actions.

```bash
# List all actions
curl http://localhost:6336/api/action \
  -H "Authorization: Bearer $TOKEN"
```

Response includes:
- `action_name` - API name to call
- `label` - Human-readable name
- `instance_optional` - 1 = no record needed, 0 = requires record ID
- `world_id` - Which entity this action belongs to

---

## Instance vs Entity Actions

**Entity Actions** (instance_optional=1):
- Called on the entity itself
- No specific record needed
- Example: `POST /action/user_account/signup`

**Instance Actions** (instance_optional=0):
- Called on a specific record
- Requires record ID in URL
- Example: `POST /action/user_account/{user_id}/generate_jwt_token`

---

## Defining Custom Actions

You can define your own actions in your schema YAML file.

### Basic Action

```yaml
Tables:
  - TableName: order
    Columns:
      - Name: status
        DataType: varchar(100)

    Actions:
      - Name: mark_shipped
        Label: Mark as Shipped
        OnType: order
        InstanceOptional: false
        InFields:
          - Name: tracking_number
            ColumnType: label
            IsNullable: false
        OutFields:
          - Type: client.notify
            Attributes:
              type: success
              message: Order marked as shipped
        Conformations:
          - Name: status
            Value: "shipped"
```

### Action Properties

| Property | Type | Description |
|----------|------|-------------|
| `Name` | string | Unique identifier (used in API) |
| `Label` | string | Display name in UI |
| `OnType` | string | Entity this action belongs to |
| `InstanceOptional` | bool | `true` = no record needed |
| `InFields` | array | Input parameters user provides |
| `OutFields` | array | Responses to return |
| `Conformations` | array | Auto-set field values |
| `Validations` | array | Input validation rules |

### Input Field Types

```yaml
InFields:
  - Name: title
    ColumnType: label
    IsNullable: false

  - Name: priority
    ColumnType: label
    DataType: enum('low','medium','high')

  - Name: attachment
    ColumnType: file.document

  - Name: due_date
    ColumnType: datetime
```

### Auto-Set Values (Conformations)

```yaml
Conformations:
  - Name: status
    Value: "completed"

  - Name: completed_at
    Value: "~now"       # Current timestamp

  - Name: completed_by
    AttributeName: user_id  # Current user
```

---

## Calling Actions Programmatically

### JavaScript/Fetch

```javascript
const response = await fetch('http://localhost:6336/action/cloud_store/create_folder', {
  method: 'POST',
  headers: {
    'Authorization': 'Bearer ' + token,
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({
    attributes: {
      cloud_store_id: 'your-store-id',
      name: 'new-folder',
      path: ''
    }
  })
});

const results = await response.json();
// results is an array of ResponseType objects
```

### Python

```python
import requests

response = requests.post(
    'http://localhost:6336/action/user_account/signin',
    json={
        'attributes': {
            'email': 'user@example.com',
            'password': 'password123'
        }
    }
)

for result in response.json():
    if result['ResponseType'] == 'client.store.set':
        token = result['Attributes']['value']
```

---

## Action Permissions

Actions use the same permission system as tables. Each action has its own permission that controls who can execute it.

### Permission Levels

| Level | Who Can Execute | Use Case |
|-------|-----------------|----------|
| **Guest** | Anyone, no login required | Public actions (signup, signin) |
| **User** | The action's owner only | Personal actions |
| **Group** | Members of assigned groups | Team/role-based actions |
| **Admin** | Administrators always | All actions |

### Three Permission Checks

When a user tries to execute an action, Daptin checks **three things**:

1. **Entity Permission** - Can this user execute on this table type?
2. **Action Permission** - Can this user execute this specific action?
3. **Row Permission** (instance actions only) - Can this user execute on this specific record?

All three must pass for the action to run.

### Schema-Managed Action Permissions

Actions declared in schema files can define their row permission directly:

```yaml
Actions:
  - Name: post_gig
    Label: Post gig
    OnType: gig
    InstanceOptional: true
    Permission: 32
    OutFields:
      - Method: ACTIONRESPONSE
        Type: client.notify
        Attributes:
          type: success
          title: Success
          message: posted
```

On startup, Daptin syncs this value into the `action.permission` column for both new and existing schema-managed actions.

### Schema-Managed Action Usergroups

Action usergroup membership is configured at the `TableInfo` level because every entity has the default `has_many usergroup` relation. Do not put usergroup membership inside one action definition.

```yaml
Tables:
  - TableName: action
    DefaultGroups:
      - Name: administrators
        Permission: 524288
```

This writes rows into `action_action_id_has_usergroup_usergroup_id` for schema-managed actions. The optional `Permission` value is stored on the relation row; if omitted, Daptin uses the relation table default permission.

### Cache Refresh

Schema startup sync invalidates the action, action-permission, object-permission, and object-group caches for updated schema-managed actions. Manual permission changes made through the API may still require a restart or explicit cache invalidation depending on the path used.

### View Action Permissions

```bash
# List actions with their permissions
curl http://localhost:6336/api/action \
  -H "Authorization: Bearer $TOKEN"
```

Each action has a `permission` field (integer). Common values:

| Permission Value | Meaning |
|------------------|---------|
| 561441 | Generic public-action profile |
| 2085120 | Post-admin locked action profile (no guest execute) |

### Make an Action Public (Guest Accessible)

To allow guests (unauthenticated users) to execute an action:

```bash
# Get the action ID first
ACTION_ID=$(curl -s http://localhost:6336/api/action \
  -H "Authorization: Bearer $TOKEN" | \
  jq -r '.data[] | select(.attributes.action_name == "your_action") | .id')

# Update permission to include GuestExecute (561441)
curl -X PATCH "http://localhost:6336/api/action/$ACTION_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "action",
      "id": "'$ACTION_ID'",
      "attributes": {
        "permission": 561441
      }
    }
  }'
```

`561441` is the generic public-action profile. For `signup` after `become_an_administrator`, use `2085152` instead: the admin transition locks `signup` to `2085120`, and re-enabling public signup only requires adding `GuestExecute` (`32`).

### Restrict Action to Admin Only

To make an action admin-only, remove guest and group execute bits:

```bash
curl -X PATCH "http://localhost:6336/api/action/$ACTION_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "action",
      "id": "'$ACTION_ID'",
      "attributes": {
        "permission": 2085120
      }
    }
  }'
```

### Assign Action to User Group

**Tested ✓** - To restrict an action to specific user groups:

1. Get the action and usergroup reference IDs
2. Create a relation via the junction table using relationships (not attributes)

```bash
# Get action ID
ACTION_ID=$(curl -s http://localhost:6336/api/action \
  -H "Authorization: Bearer $TOKEN" | \
  jq -r '.data[] | select(.attributes.action_name == "your_action") | .id')

# Get usergroup ID
USERGROUP_ID=$(curl -s http://localhost:6336/api/usergroup \
  -H "Authorization: Bearer $TOKEN" | \
  jq -r '.data[] | select(.attributes.name == "editors") | .id')

# Link action to usergroup using relationships
curl -X POST http://localhost:6336/api/action_action_id_has_usergroup_usergroup_id \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "action_action_id_has_usergroup_usergroup_id",
      "attributes": {},
      "relationships": {
        "action_id": {
          "data": {"type": "action", "id": "'$ACTION_ID'"}
        },
        "usergroup_id": {
          "data": {"type": "usergroup", "id": "'$USERGROUP_ID'"}
        }
      }
    }
  }'
```

**Note:** Junction tables can be created via POST but cannot be listed via GET.

### Permission Bits Reference

For advanced users, here are the individual permission bits:

| Bit | Name | Value | Description |
|-----|------|-------|-------------|
| 5 | GuestExecute | 32 | Guests can execute |
| 12 | UserExecute | 4096 | Owner can execute |
| 19 | GroupExecute | 524288 | Group members can execute |

Common permission values:
- `561441` = GuestPeek + GuestExecute + UserRead + UserExecute + GroupRead + GroupExecute
- `2085120` = UserCRUD + UserExecute + GroupCRUD + GroupExecute (no guest access)
- `2085152` = `2085120` + GuestExecute. This is the current post-admin reopen value for `signup`.

### Default Action Permissions

Built-in actions have sensible defaults:

| Action | Default Access |
|--------|----------------|
| `signup` | Guest before first admin; disabled after admin setup until explicitly re-enabled |
| `signin` | Guest (public, including after admin setup) |
| `become_an_administrator` | Guest (first user only) |
| `download_system_schema` | Admin/Owner |
| `restart` | Admin only |
| Cloud storage actions | Owner/Group/Admin |

Schema actions without an explicit `Permission` keep the historical default of `ALLOW_ALL_PERMISSIONS` when inserted. Existing actions preserve their current permission unless the schema action includes `Permission`.

See [[Permissions|Permissions]] for the complete permission system.

---

## Error Handling

Failed actions return error notifications:

```json
[
  {
    "ResponseType": "client.notify",
    "Attributes": {
      "type": "error",
      "title": "failed",
      "message": "required reference id not provided"
    }
  }
]
```

Common errors:
- Missing required attributes
- Invalid record ID (for instance actions)
- Permission denied
- Validation failure

---

## See Also

- [[Custom-Actions|Custom-Actions]] - Creating actions with workflows
- [[Cloud-Storage|Cloud-Storage]] - Cloud storage action examples
- [[Authentication|Authentication]] - Auth action details
- [[State-Machines|State-Machines]] - Trigger actions on state changes
- [[Task-Scheduling|Task-Scheduling]] - Run actions on schedule
