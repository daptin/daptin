# Documentation Quality Checklist

Use this checklist when creating or updating documentation to ensure accuracy and usability.

---

## Before Publishing: Test Everything

### ✅ All Code Examples Must Be Tested

**Rule**: NEVER publish a code example without testing it against a running Daptin instance.

**Process**:
1. Set up test environment (fresh database if possible)
2. Copy/paste the exact command from documentation
3. Verify it produces the expected output
4. Add "**Tested ✓**" marker to the documentation section

**Example marker**:
```markdown
### Create a Todo

**Tested ✓** - Verified on 2026-01-25.

\```bash
curl -X POST http://localhost:6336/api/todo...
\```
```

---

## Syntax Standards

### 1. Filter/Query Syntax (CRITICAL)

**❌ WRONG** - Never use this (doesn't work in most cases):
```bash
curl "http://localhost:6336/api/world?filter[table_name]=product"
```

**✅ CORRECT** - Always use curl --get with --data-urlencode:
```bash
curl --get \
  --data-urlencode 'query=[{"column":"table_name","operator":"is","value":"product"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/world"
```

**Why**: The query parameter expects JSON, which must be URL-encoded. The `--data-urlencode` flag handles this automatically.

---

### 2. Pagination Syntax

**✅ CORRECT** - Use single quotes or URL-encoded brackets:

```bash
# Method 1: Single quotes (easiest)
curl 'http://localhost:6336/api/world?page[size]=100'

# Method 2: URL-encoded
curl "http://localhost:6336/api/world?page%5Bsize%5D=100"

# Method 3: With --data-urlencode
curl --get \
  --data-urlencode 'page[size]=100' \
  "http://localhost:6336/api/world"
```

**Default page size**: 10 records
**Getting all world records**: Use `page[size]=100` (there are ~60 world records for all tables)

---

### 3. Token Extraction from Signin

**✅ CORRECT** - Extract using jq:

```bash
TOKEN=$(curl -s -X POST http://localhost:6336/action/user_account/signin \
  -H "Content-Type: application/json" \
  -d '{"attributes":{"email":"admin@admin.com","password":"adminadmin"}}' | \
  jq -r '.[] | select(.ResponseType == "client.store.set") | .Attributes.value')

echo "$TOKEN" > /tmp/daptin-token.txt
```

**❌ WRONG** - Don't tell users to manually copy/paste tokens from JSON output.

---

### 4. Cloud Storage Credentials

**✅ CORRECT** - Use `content` field with rclone JSON:

```json
{
  "name": "my-creds",
  "content": "{\"type\":\"s3\",\"provider\":\"AWS\",\"access_key_id\":\"...\",\"secret_access_key\":\"...\"}"
}
```

**❌ WRONG** - These fields don't exist:
- `credential_type`
- `credential_value`

**Required fields in content**:
- `"type"`: rclone remote type
- `"provider"`: Provider name
- Provider-specific fields (access keys, endpoints, etc.)

---

### 5. File Upload Format

**✅ CORRECT** - Files must be an array of objects:

```json
{
  "photo": [
    {
      "name": "filename.jpg",
      "file": "data:image/jpeg;base64,...",
      "type": "image/jpeg"
    }
  ]
}
```

**❌ WRONG**:
```json
// Not a string
{"photo": "data:image/jpeg;base64,..."}

// Not a single object
{"photo": {"name": "file.jpg", "file": "data:..."}}
```

---

### 6. Join Table Operations

**✅ CORRECT** - POST then PATCH for permissions:

```bash
# Step 1: Create join record (permission will be ignored)
JOIN_ID=$(curl -s -X POST http://localhost:6336/api/user_account_user_account_id_has_usergroup_usergroup_id \
  -d '{"data":{"type":"...","attributes":{"user_account_id":"...","usergroup_id":"..."}}}' | \
  jq -r '.data.id')

# Step 2: PATCH to set permission
curl -X PATCH "http://localhost:6336/api/user_account_user_account_id_has_usergroup_usergroup_id/$JOIN_ID" \
  -d '{"data":{"type":"...","id":"'$JOIN_ID'","attributes":{"permission":688128}}}'
```

**Important**: POST ignores `permission` attribute on join tables. Must PATCH after creation.

**Join table permissions use bit-shifted format**:
- Group Read (2): `2 << 14 = 32768`
- Group Update (8): `8 << 14 = 131072`
- Group Read+Update+Execute (42): `42 << 14 = 688128`

---

### 7. Custom Actions

**✅ CORRECT** - Define in schema file:

```yaml
Actions:
  - Name: my_action
    Label: My Action
    OnType: table_name
    InstanceOptional: false
    InFields: []
    OutFields:
      - Type: table_name
        Method: PATCH
        Attributes:
          reference_id: $subject.reference_id
          field: '!subject.field ? 0 : 1'
```

**Value substitution**:
- `$subject.field_name` - Direct value from target record
- `!javascript_expression` - Evaluated JavaScript (use `subject.field` inside)
- `~input_field` - Value from InFields parameter
- `$previous_outfield[0].result` - Result from previous OutField

**Action execution URL**:
- Instance actions: `/action/{table}/{action_name}` with `{table}_id` in attributes
- Collection actions: `/action/{table}/{action_name}` (no ID needed)

---

## Required Knowledge Checks

Before documenting a feature, verify you understand:

### Permissions

- [ ] Three-tier model: Guest (0-6) | Owner (7-13) | Group (14-20)
- [ ] Bit values: Peek(1), Read(2), Create(4), Update(8), Delete(16), Execute(32), Refer(64), Full(127)
- [ ] Formula: `guest + (owner * 128) + (group * 16384)`
- [ ] Two-level check: Table (world) AND record must both be shared with group
- [ ] POST ignores permission on join tables - must PATCH
- [ ] Server restart clears Olric cache (10-minute TTL otherwise)

### Cloud Storage

- [ ] Credential `content` is rclone JSON format
- [ ] Must include `"type"` and `"provider"` fields
- [ ] Credential must be linked via relationship PATCH (credential_name doesn't auto-link)
- [ ] Server restart required after creating cloud_store
- [ ] ForeignKeyData.Namespace must match cloud_store name field
- [ ] File uploads must be array of objects: `[{name, file, type}]`

### Actions

- [ ] Schema file method vs API method (schema file recommended)
- [ ] Server restart required after creating actions
- [ ] Action schema is single JSON field, not separate in_fields/out_fields
- [ ] JavaScript expressions use `subject.field`, not `$.field`
- [ ] Instance actions need `{table}_id` in attributes, not URL path

### API Queries

- [ ] Use `curl --get --data-urlencode 'query=[...]'` for filters
- [ ] Use `page[size]=100` to get all world records (~60 tables)
- [ ] Single quotes preserve brackets: `'page[size]=20'`
- [ ] URL-encoded: `page%5Bsize%5D` where %5B=[, %5D=]

---

## Testing Workflow

### 1. Setup Test Environment

```bash
# Kill old processes
pkill -9 -f daptin
sleep 2

# Fresh database
rm -f daptin.db

# Start server
nohup go run main.go > /tmp/daptin.log 2>&1 &
sleep 20

# Create admin
curl -X POST http://localhost:6336/action/user_account/signup \
  -H "Content-Type: application/json" \
  -d '{"attributes":{"name":"Admin","email":"admin@admin.com","password":"adminadmin","passwordConfirm":"adminadmin"}}'

# Get token
TOKEN=$(curl -s -X POST http://localhost:6336/action/user_account/signin \
  -H "Content-Type: application/json" \
  -d '{"attributes":{"email":"admin@admin.com","password":"adminadmin"}}' | \
  jq -r '.[] | select(.ResponseType == "client.store.set") | .Attributes.value')
echo "$TOKEN" > /tmp/daptin-token.txt

# Become admin
curl -s -X POST http://localhost:6336/action/world/become_an_administrator \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{}'
sleep 5

# Refresh token
TOKEN=$(curl -s -X POST http://localhost:6336/action/user_account/signin \
  -H "Content-Type: application/json" \
  -d '{"attributes":{"email":"admin@admin.com","password":"adminadmin"}}' | \
  jq -r '.[] | select(.ResponseType == "client.store.set") | .Attributes.value')
echo "$TOKEN" > /tmp/daptin-token.txt
```

### 2. Test Each Example

For each code example in documentation:
1. Copy the exact command
2. Run it in terminal
3. Verify output matches expected result
4. If fails, update the documentation
5. Re-test until it works

### 3. Mark as Tested

Add this marker at the top of tested sections:

```markdown
**Tested ✓** - Verified on YYYY-MM-DD
```

Or for known broken features:

```markdown
**Not Working** - See [GitHub Issue #XXX](https://github.com/daptin/daptin/issues/XXX)
```

---

## Common Mistakes to Avoid

### ❌ Don't Document Without Testing

**Bad**:
```markdown
You can filter by status:
\```bash
curl "http://localhost:6336/api/todo?filter[status]=completed"
\```
```

**Good**:
```markdown
**Tested ✓** - Filter by status:
\```bash
curl --get \
  --data-urlencode 'query=[{"column":"status","operator":"is","value":"completed"}]' \
  "http://localhost:6336/api/todo"
\```
```

### ❌ Don't Use Old Filter Syntax

**Bad**: `?filter[field]=value`
**Good**: `curl --get --data-urlencode 'query=[...]'`

### ❌ Don't Forget Server Restarts

**Bad**:
```markdown
Create a cloud_store, then use it immediately.
```

**Good**:
```markdown
Create a cloud_store, then **restart the server** to load the configuration.
```

### ❌ Don't Use Non-Existent Fields

Always verify field names in the code or API before documenting them.

**Common wrong fields**:
- `credential_type` (doesn't exist - use `content`)
- `credential_value` (doesn't exist - use `content`)
- `in_fields` / `out_fields` as attributes (doesn't exist - use `action_schema`)

---

## Documentation Structure

### Page Template

```markdown
# Page Title

Brief description of what this feature does.

**Related**: [Link](Other.md) | [Link](Another.md)

---

## Quick Start

**Tested ✓** - Simplest working example.

\```bash
# Working example with comments
\```

---

## Detailed Examples

### Use Case 1

**Tested ✓** - Description of what this does.

\```bash
# Example with explanation
\```

**Expected output:**
\```json
{...}
\```

### Use Case 2

...

---

## Troubleshooting

### Common Error Message

**Cause:** Why this happens

**Solution:**
\```bash
# How to fix it
\```

---

## See Also

- [Related Doc](Link.md) - What it covers
```

---

## Cross-Reference Validation

Before publishing, verify:

### Links Work

- [ ] All `[Link Text](File.md)` links point to existing files
- [ ] All `#section-anchors` point to existing sections
- [ ] No broken links in "See Also" sections

### Consistent Terminology

Use these exact terms:

| Use This | Not This |
|----------|----------|
| reference_id | id, uuid, ref_id |
| world record | table metadata, table definition |
| join table | junction table, relationship table |
| usergroup | user group, group |
| cloud_store | cloud storage, storage backend |

### Consistent Examples

- **Email**: `admin@admin.com`, `user@example.com`
- **Password**: `adminadmin`, `password123`
- **Table**: `todo`, `product`, `user_account`
- **Port**: `6336` (not 8080, 8181, etc.)
- **Token file**: `/tmp/daptin-token.txt`

---

## Adding New Features

When documenting a new feature:

### 1. Find the Source Code

```bash
# Find action files
ls server/actions/action_*.go

# Find the action name in columns.go
grep "ActionName:" server/columns.go | grep -i keyword

# Find performer name in action file
grep "Name:" server/actions/action_xxx.go
```

### 2. Verify Action vs Performer Names

| Name Type | Where Used | Example |
|-----------|------------|---------|
| Action Name | REST API endpoint | `/action/user_account/signin` |
| Performer Name | Inside action OutFields | `Type: "$network.request"` |

**Common confusion**: The action file name (action_mail_send.go) may use the performer name, but the REST endpoint uses the action name from columns.go.

### 3. Test the Feature

Create a minimal working example:

```bash
# 1. Find the action
curl --get \
  --data-urlencode 'query=[{"column":"action_name","operator":"is","value":"ACTION_NAME"}]' \
  -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/action" | jq

# 2. Check input fields
# (Look at InFields in response)

# 3. Test execution
curl -X POST "http://localhost:6336/action/{entity}/{action_name}" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"attributes":{...}}'

# 4. Verify result
```

### 4. Document

- Explain what it does
- Show working example
- List all parameters
- Show expected output
- Add troubleshooting section

---

## Permission Documentation Standards

### Always Include Permission Calculation

When showing permission values, explain how they're calculated:

**✅ Good**:
```markdown
Permission 704385 means:
- Guest: Peek (1)
- Owner: Full (127) → 127 × 128 = 16,256
- Group: Read+Update+Execute (42) → 42 × 16,384 = 688,128
- Total: 1 + 16,256 + 688,128 = 704,385
```

**❌ Bad**:
```markdown
Set permission to 704385.
```

### Always Show Bit-Shift for Join Tables

**✅ Good**:
```markdown
Group permissions on join tables use bit-shifted format:
- Group Read (2): `2 << 14 = 32768`
- Group Update (42): `42 << 14 = 688128`
```

**❌ Bad**:
```markdown
Set permission to 32768 for read-only.
```

### Always Mention Two-Level Check

When documenting group permissions, always mention:

1. **Table-level**: Share world record with group
2. **Record-level**: Share specific records with group (optional)

---

## Action Documentation Standards

### Schema File Examples

Always show the schema file format (recommended method):

```yaml
Actions:
  - Name: action_name
    Label: Display Name
    OnType: table_name
    InstanceOptional: false
    InFields: []
    OutFields:
      - Type: table_name
        Method: PATCH
        Attributes:
          reference_id: $subject.reference_id
          field: '!subject.field ? 0 : 1'
```

### Value Substitution Reference

Always include this table:

| Syntax | Description | Example |
|--------|-------------|---------|
| `$var` | Direct value | `$subject.reference_id` |
| `!expr` | JavaScript | `'!subject.price * 1.1'` |
| `~field` | Input parameter | `~email` |

### Execution Examples

Show both instance and collection actions:

```bash
# Instance action (requires record ID)
curl -X POST "http://localhost:6336/action/product/toggle_publish" \
  -d '{"attributes":{"product_id":"PRODUCT_UUID"}}'

# Collection action (no record needed)
curl -X POST "http://localhost:6336/action/user_account/signup" \
  -d '{"attributes":{...}}'
```

---

## Cloud Storage Documentation Standards

### Always Include Three Steps

1. **Create credential** (with rclone content format)
2. **Create cloud_store**
3. **Link credential via relationship PATCH**
4. **Restart server**

Don't skip any of these steps - all are required.

### Always Show ForeignKeyData

When documenting asset columns:

```yaml
Columns:
  - Name: photo
    ColumnType: file
    IsForeignKey: true
    ForeignKeyData:
      DataSource: cloud_store
      Namespace: my-cloud-store  # Must match cloud_store.name
      KeyName: subfolder          # Folder within root_path
```

---

## Response Examples

### Show Real Responses

**✅ Good** - Copy actual API response:
```json
{
  "data": {
    "type": "todo",
    "id": "019bf528-...",
    "attributes": {
      "title": "Buy milk",
      "completed": 0
    }
  }
}
```

**❌ Bad** - Fake/simplified response:
```json
{"success": true}
```

### Show Error Responses

Document what happens when things fail:

```json
{
  "errors": [
    {
      "status": "403",
      "title": "TableAccessPermissionChecker",
      "detail": "access not allowed"
    }
  ]
}
```

---

## Troubleshooting Section Requirements

Every major feature page must have a Troubleshooting section with:

### Common Errors

| Error Message | Cause | Solution |
|---------------|-------|----------|
| ... | ... | `command to fix` |

### Debug Commands

```bash
# Check server logs
tail -f /tmp/daptin.log

# Check database
sqlite3 daptin.db "SELECT ..."

# Check API
curl http://localhost:6336/api/...
```

---

## Review Checklist

Before merging documentation changes:

- [ ] All commands tested against running Daptin
- [ ] All filter/query syntax uses `curl --get --data-urlencode`
- [ ] All pagination uses correct `page[size]` format
- [ ] Token extraction uses jq with proper selector
- [ ] Permission values include calculation explanation
- [ ] Join table examples use POST+PATCH pattern
- [ ] Cloud storage examples include credential linking
- [ ] Action examples use schema file approach
- [ ] Server restart mentioned where required
- [ ] All links verified
- [ ] Consistent terminology
- [ ] "Tested ✓" markers added
- [ ] Expected outputs shown
- [ ] Troubleshooting section included
- [ ] Cross-referenced to related docs

---

## Tools for Testing

### Recommended Setup

```bash
# Helper script
./scripts/testing/test-runner.sh

# Or manual
export TOKEN=$(cat /tmp/daptin-token.txt)
alias dapi='curl -s -H "Authorization: Bearer $TOKEN"'

# Use it
dapi http://localhost:6336/api/world | jq '.data | length'
```

### Quick Test Script

Create `test-doc-example.sh`:

```bash
#!/bin/bash
set -e

TOKEN=$(cat /tmp/daptin-token.txt)

# Your example command here
curl -X POST http://localhost:6336/api/todo \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{...}'

echo "✓ Example works!"
```

Make it executable: `chmod +x test-doc-example.sh`

---

## See Also

- [Documentation Guidelines](Documentation-Guidelines.md) - Writing style guide
- [Documentation TODO](Documentation-TODO.md) - What needs documentation
- [WIKI Audit Report](WIKI_AUDIT_REPORT.md) - Known documentation issues
- [Testing Onboarding Journey](Testing-Onboarding-Journey.md) - End-to-end testing guide
