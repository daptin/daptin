# Custom Actions

**Tested ✓** - All examples on this page were verified against a running Daptin instance.

Custom actions let you define business logic that executes when called via API. Actions can send notifications, make HTTP requests, create/update records, or chain multiple operations together.

---

## Quick Start

### Create an Action via Schema File

Create a file named `schema_myapp.yaml` in your Daptin directory:

```yaml
Actions:
  - Name: greet_user
    Label: Greet User
    OnType: user_account
    InstanceOptional: true
    InFields:
      - Name: name
        ColumnName: name
        ColumnType: label
        IsNullable: false
    OutFields:
      - Type: client.notify
        Method: ACTIONRESPONSE
        Attributes:
          type: success
          title: Greeting
          message: "~name"
```

Restart Daptin to load the schema file.

### Call the Action

```bash
curl -X POST http://localhost:6336/action/user_account/greet_user \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"attributes": {"name": "World"}}'
```

**Response:**
```json
[
  {
    "ResponseType": "client.notify",
    "Attributes": {
      "type": "success",
      "title": "Greeting",
      "message": "World"
    }
  }
]
```

---

## Two Ways to Create Actions

### 1. Schema Files (Recommended for Production)

Place `schema_*.yaml`, `schema_*.json`, or `schema_*.toml` files in your Daptin directory. Actions are loaded on startup.

**Set schema folder:**
```bash
export DAPTIN_SCHEMA_FOLDER=/path/to/schemas
```

### 2. API (Runtime)

Create actions via the API. Requires server restart to take effect.

```bash
# Get the world_id for the target table
WORLD_ID=$(curl -s http://localhost:6336/api/world?page%5Bsize%5D=100 \
  -H "Authorization: Bearer $TOKEN" | \
  jq -r '.data[] | select(.attributes.table_name == "user_account") | .id')

# Create the action
curl -X POST http://localhost:6336/api/action \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "action",
      "attributes": {
        "action_name": "my_action",
        "label": "My Action",
        "instance_optional": 1,
        "action_schema": "{...JSON schema...}"
      },
      "relationships": {
        "world_id": {
          "data": {"type": "world", "id": "'$WORLD_ID'"}
        }
      }
    }
  }'

# IMPORTANT: Set instance_optional separately (API quirk)
curl -X PATCH http://localhost:6336/api/action/$ACTION_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "action",
      "id": "'$ACTION_ID'",
      "attributes": {"instance_optional": 1}
    }
  }'

# Restart server to load the action
```

**Important:** After creating or modifying actions via API, you must restart Daptin for changes to take effect.

---

## Action Schema Structure

Every action has this JSON structure (stored in `action_schema` column):

```json
{
  "Name": "action_name",
  "Label": "Human Readable Name",
  "OnType": "table_name",
  "InstanceOptional": true,
  "RequestSubjectRelations": null,
  "ReferenceId": "00000000-0000-0000-0000-000000000000",
  "InFields": [...],
  "OutFields": [...],
  "Validations": null,
  "Conformations": null
}
```

### Required Properties

| Property | Type | Description |
|----------|------|-------------|
| `Name` | string | Unique identifier (used in API URL) |
| `Label` | string | Display name for UI |
| `OnType` | string | Table this action operates on |
| `InstanceOptional` | boolean | `true` = no record needed, `false` = requires record ID |
| `InFields` | array | Input parameters (see below) |
| `OutFields` | array | Operations to execute (see below) |

---

## Input Fields (InFields)

Define what parameters the action accepts.

### Full Field Structure

```json
{
  "Name": "field_name",
  "ColumnName": "field_name",
  "ColumnDescription": "Help text for this field",
  "ColumnType": "label",
  "IsPrimaryKey": false,
  "IsAutoIncrement": false,
  "IsIndexed": false,
  "IsUnique": false,
  "IsNullable": false,
  "Permission": 0,
  "IsForeignKey": false,
  "ExcludeFromApi": false,
  "ForeignKeyData": {"DataSource": "", "Namespace": "", "KeyName": ""},
  "DataType": "",
  "DefaultValue": "",
  "Options": null
}
```

### Common ColumnTypes for Actions

| ColumnType | Description | Example Use |
|------------|-------------|-------------|
| `label` | Short text | Names, titles |
| `content` | Long text | Descriptions, messages |
| `email` | Email address | User email |
| `password` | Password (masked) | Login credentials |
| `url` | URL | Webhook endpoints |
| `file.*` | File upload (base64) | Attachments |
| `truefalse` | Boolean | Flags, options |
| `datetime` | Date/time | Scheduled dates |

### Minimal Field Definition

For quick testing, you can use minimal fields:

```json
{
  "Name": "message",
  "ColumnName": "message",
  "ColumnType": "label",
  "IsNullable": false
}
```

---

## Output Fields (OutFields)

OutFields define what the action does. Each OutField executes an operation.

### OutField Structure

```json
{
  "Type": "performer_name",
  "Method": "EXECUTE",
  "Reference": "",
  "LogToConsole": false,
  "SkipInResponse": false,
  "Condition": "",
  "Attributes": {...},
  "ContinueOnError": false
}
```

### Methods

| Method | Description | Type Value |
|--------|-------------|------------|
| `ACTIONRESPONSE` | Return data directly to client | Response type (client.notify, client.redirect, etc.) |
| `EXECUTE` | Run a registered performer | Performer name ($network.request, jwt.token, etc.) |
| `POST` | Create a new record | Table name |
| `PATCH` | Update existing record | Table name |
| `GET` | Retrieve records | Table name |
| `DELETE` | Delete a record | Table name |
| `GET_BY_ID` | Get specific record | Table name |

---

## Response Types (ACTIONRESPONSE)

Return data directly to the client.

### client.notify

**Tested ✓** - Show a notification message.

```json
{
  "Type": "client.notify",
  "Method": "ACTIONRESPONSE",
  "Attributes": {
    "type": "success",
    "title": "Done",
    "message": "Operation completed"
  }
}
```

Notification types: `success`, `error`, `warning`, `info`

### client.redirect

**Tested ✓** - Navigate to a URL.

```json
{
  "Type": "client.redirect",
  "Method": "ACTIONRESPONSE",
  "Attributes": {
    "location": "/dashboard",
    "delay": 2000,
    "window": "self"
  }
}
```

### client.file.download

Return a file for download.

```json
{
  "Type": "client.file.download",
  "Method": "ACTIONRESPONSE",
  "Attributes": {
    "content": "base64-encoded-content",
    "name": "report.pdf",
    "contentType": "application/pdf"
  }
}
```

### client.store.set

Store a value (for frontend localStorage).

```json
{
  "Type": "client.store.set",
  "Method": "ACTIONRESPONSE",
  "Attributes": {
    "key": "token",
    "value": "jwt-token-here"
  }
}
```

---

## Performers (EXECUTE)

Registered handlers that perform operations.

### $network.request

**Tested ✓** - Make HTTP requests to external APIs.

```yaml
OutFields:
  - Type: $network.request
    Method: EXECUTE
    Attributes:
      Url: "https://httpbin.org/post"
      Method: "POST"
      Headers:
        Content-Type: "application/json"
        Authorization: "Bearer ~api_token"
      Body:
        name: "~name"
        email: "~email"
```

**Response:**
```json
{
  "ResponseType": "$network.request",
  "Attributes": {
    "__type": "$network.response",
    "body": {...},
    "headers": {...}
  }
}
```

### Complete Performer List

| Performer | Purpose |
|-----------|---------|
| `$network.request` | HTTP requests to external APIs |
| `$transaction` | Database transaction wrapper |
| `jwt.token` | Generate JWT authentication token |
| `otp.generate` | Generate OTP for 2FA |
| `otp.login.verify` | Verify OTP code |
| `mail.send` | Send email via SMTP |
| `aws.mail.send` | Send email via AWS SES |
| `cloudstore.file.upload` | Upload to cloud storage |
| `cloudstore.folder.create` | Create folder in cloud storage |
| `cloudstore.path.move` | Move/rename in cloud storage |
| `cloudstore.site.create` | Create a subsite |
| `site.file.list` | List site files |
| `site.file.get` | Get site file content |
| `site.file.delete` | Delete site file |
| `site.storage.sync` | Sync site storage |
| `column.storage.sync` | Sync column storage |
| `password.reset.begin` | Start password reset flow |
| `password.reset.verify` | Complete password reset |
| `self.tls.generate` | Generate self-signed certificate |
| `acme.tls.generate` | Generate Let's Encrypt certificate |
| `integration.install` | Install OpenAPI integration |
| `template.render` | Render template |
| `response.create` | Create custom response |
| `random.generate` | Generate random value |
| `generate.random.data` | Generate random data |
| `command.execute` | Execute system command |
| `oauth.login.response` | Handle OAuth callback |
| `oauth.profile.exchange` | Exchange OAuth profile |
| `oauth.token` | Generate OAuth token |
| `oauth.client.redirect` | Redirect for OAuth |
| `mail.servers.sync` | Sync mail servers |
| `cloud_store.files.import` | Import files from cloud storage |
| `world.delete` | Delete a table |
| `world.column.delete` | Delete a column |
| `world.column.rename` | Rename a column |
| `__become_admin` | Become administrator |
| `__restart` | Restart server |
| `__enable_graphql` | Enable GraphQL |
| `__data_export` | Export table data |
| `__data_import` | Import table data |
| `__csv_data_export` | Export as CSV |
| `__upload_csv_file_to_entity` | Import CSV data |
| `__upload_xlsx_file_to_entity` | Import Excel data |
| `__download_cms_config` | Download CMS configuration |

---

## Value Substitution

Use special prefixes to substitute values dynamically.

### Input Field Values (~)

**Tested ✓** - Use `~field_name` to insert the value of an input field.

```yaml
InFields:
  - Name: greeting_name
    ColumnType: label

OutFields:
  - Type: client.notify
    Method: ACTIONRESPONSE
    Attributes:
      message: "~greeting_name"
```

When called with `{"attributes": {"greeting_name": "Claude"}}`, the message becomes "Claude".

### Entity Column Values ($.)

Use `$.column_name` to get values from the target record (instance actions only).

```yaml
OutFields:
  - Type: $network.request
    Method: EXECUTE
    Attributes:
      Body:
        order_id: "$.reference_id"
        total: "$.total"
        customer_email: "$.email"
```

### JavaScript Expressions (!)

Use `!expression` for dynamic JavaScript evaluation.

```yaml
OutFields:
  - Type: $network.request
    Method: EXECUTE
    Attributes:
      Url: "!subject.webhook_url"
      Body:
        calculated: "!subject.price * subject.quantity"
        timestamp: "!new Date().toISOString()"
```

The `subject` object contains the target record's fields.

---

## Chaining Multiple Operations

**Tested ✓** - Actions can have multiple OutFields that execute sequentially.

```yaml
Actions:
  - Name: notify_and_redirect
    Label: Notify and Redirect
    OnType: user_account
    InstanceOptional: true
    InFields:
      - Name: message
        ColumnType: label
    OutFields:
      # First: Show notification
      - Type: client.notify
        Method: ACTIONRESPONSE
        Attributes:
          type: success
          title: Notice
          message: "~message"
      # Second: Redirect
      - Type: client.redirect
        Method: ACTIONRESPONSE
        Attributes:
          location: "/dashboard"
          delay: 1000
```

**Response includes both:**
```json
[
  {"ResponseType": "client.notify", "Attributes": {...}},
  {"ResponseType": "client.redirect", "Attributes": {...}}
]
```

---

## Conditional Execution

Use the `Condition` field to conditionally execute an OutField.

```yaml
OutFields:
  - Type: otp.generate
    Method: EXECUTE
    Condition: "!mobile != null && mobile != undefined && mobile != ''"
    Attributes:
      mobile: "~mobile"
```

The condition is a JavaScript expression. If it evaluates to false, the OutField is skipped.

---

## CRUD Operations in Actions

### Create a Record (POST)

```yaml
OutFields:
  - Type: user_account
    Method: POST
    Reference: "new_user"
    SkipInResponse: true
    Attributes:
      email: "~email"
      name: "~name"
      password: "~password"
```

### Update a Record (PATCH)

```yaml
OutFields:
  - Type: order
    Method: PATCH
    Attributes:
      reference_id: "$.reference_id"
      status: "shipped"
```

### Get Records (GET)

```yaml
OutFields:
  - Type: user_account
    Method: GET
    Attributes:
      query: '[{"column":"email","operator":"is","value":"~email"}]'
```

---

## Input Validation

Validate input fields before processing.

```yaml
Validations:
  - ColumnName: email
    Tags: email
  - ColumnName: password
    Tags: required,min=8
  - ColumnName: password_confirm
    Tags: eqfield=password
```

### Common Validation Tags

| Tag | Description |
|-----|-------------|
| `required` | Field must not be empty |
| `email` | Must be valid email format |
| `min=N` | Minimum length/value |
| `max=N` | Maximum length/value |
| `eqfield=X` | Must equal another field |
| `url` | Must be valid URL |

---

## Input Conformations

Transform input values before processing.

```yaml
Conformations:
  - ColumnName: email
    Tags: email,lowercase
  - ColumnName: name
    Tags: trim
  - ColumnName: phone
    Tags: trim
```

### Common Conformation Tags

| Tag | Description |
|-----|-------------|
| `trim` | Remove leading/trailing whitespace |
| `lowercase` | Convert to lowercase |
| `uppercase` | Convert to uppercase |
| `email` | Normalize email format |

---

## Instance vs Collection Actions

### Collection Action (InstanceOptional: true)

Operates on the table, no specific record needed.

```bash
POST /action/user_account/signup
```

### Instance Action (InstanceOptional: false)

Requires a specific record ID in the URL.

```bash
POST /action/order/abc-123-def/mark_shipped
```

The target record is available via `$.column_name` in OutFields.

---

## Error Handling

### ContinueOnError

By default, if an OutField fails, execution stops. Set `ContinueOnError: true` to continue.

```yaml
OutFields:
  - Type: $network.request
    Method: EXECUTE
    ContinueOnError: true
    Attributes:
      Url: "https://optional-service.com/notify"
```

### Error Response

Failed actions return error notifications:

```json
[
  {
    "ResponseType": "client.notify",
    "Attributes": {
      "type": "error",
      "title": "failed",
      "message": "Validation failed: email is required"
    }
  }
]
```

---

## Complete Example: User Signup with Webhook

```yaml
Actions:
  - Name: signup_with_webhook
    Label: Sign Up with Webhook
    OnType: user_account
    InstanceOptional: true
    InFields:
      - Name: name
        ColumnName: name
        ColumnType: label
        IsNullable: false
      - Name: email
        ColumnName: email
        ColumnType: email
        IsNullable: false
      - Name: password
        ColumnName: password
        ColumnType: password
        IsNullable: false
    Validations:
      - ColumnName: email
        Tags: required,email
      - ColumnName: password
        Tags: required,min=8
    Conformations:
      - ColumnName: email
        Tags: lowercase,trim
      - ColumnName: name
        Tags: trim
    OutFields:
      # 1. Create user account
      - Type: user_account
        Method: POST
        Reference: new_user
        SkipInResponse: true
        Attributes:
          email: "~email"
          name: "~name"
          password: "~password"
      # 2. Send webhook notification
      - Type: $network.request
        Method: EXECUTE
        ContinueOnError: true
        Attributes:
          Url: "https://hooks.example.com/new-user"
          Method: POST
          Headers:
            Content-Type: "application/json"
          Body:
            event: "user.created"
            email: "~email"
            name: "~name"
      # 3. Show success message
      - Type: client.notify
        Method: ACTIONRESPONSE
        Attributes:
          type: success
          title: Welcome
          message: "Account created for ~name"
      # 4. Redirect to login
      - Type: client.redirect
        Method: ACTIONRESPONSE
        Attributes:
          location: "/auth/signin"
          delay: 2000
```

---

## Permissions

See [Actions-Overview](Actions-Overview.md#action-permissions) for permission configuration.

Key points:
- Actions use the same permission system as tables
- Both entity permission AND action permission must allow execution
- Permission changes require server restart

---

## Troubleshooting

### Action not found after creation via API

**Cause:** Action cache not refreshed.
**Solution:** Restart Daptin server.

### "required reference id not provided"

**Cause:** Action has `InstanceOptional: false` but no record ID in URL.
**Solutions:**
1. Call with record ID: `/action/entity/{record_id}/action_name`
2. Set `instance_optional: 1` via PATCH and restart

### Input field substitution (~field) not working

**Cause:** Field name mismatch between InFields and OutFields.
**Solution:** Ensure `Name` and `ColumnName` in InFields match what you reference in OutFields.

### Empty response array

**Cause:** All OutFields have `SkipInResponse: true` or conditions failed.
**Solution:** Add at least one OutField without SkipInResponse or check conditions.

---

## See Also

- [Actions-Overview](Actions-Overview.md) - Built-in actions and permissions
- [State-Machines](State-Machines.md) - Trigger actions on state changes
- [Task-Scheduling](Task-Scheduling.md) - Run actions on schedule
- [Integrations](Integrations.md) - OpenAPI integrations for external APIs
