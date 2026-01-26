# Template Rendering

**Tested ✓ 2026-01-26**

Dynamic content generation using Go templates with Daptin's template system.

## Overview

Daptin's template system provides:
- Go template syntax with extended functions
- Reusable templates stored in database
- Dynamic variable substitution
- Reference files from subsites
- MIME type and header control

## Important: Internal vs API Actions

⚠️ **`template.render` is NOT directly callable via API**

**template.render** is an **internal outcome** - it can only be used within custom action definitions, NOT called directly at `/action/entity/template.render`.

### Action Types in Daptin

**API Actions** (Directly Callable):
- Exposed at `/action/{entity}/{action_name}`
- Example: `become_an_administrator`, `upload_file`

**Internal Outcomes** (NOT Directly Callable):
- Used within `OutFields` of custom actions
- Example: `template.render`, `cloudstore.file.upload`

### How to Use template.render

You must create a **custom action** that calls template.render as an outcome:

```yaml
Actions:
  - Name: generate_welcome_email
    OnType: user_account
    InFields:
      - Name: user_name
        ColumnName: user_name
        ColumnType: label
    OutFields:
      - Type: template.render      # Internal outcome
        Method: EXECUTE
        Attributes:
          template: welcome_email_template
          name: ~user_name
```

Then call: `/action/user_account/generate_welcome_email`

See [Custom Actions](Custom-Actions.md) for full guide.

## Template Table

Templates are stored in the `template` table:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | varchar(500) | Yes | Unique template identifier |
| `content` | text | Yes | Template content with {{.variable}} syntax |
| `mime_type` | varchar(500) | Yes | Output MIME type (e.g., text/html) |
| `url_pattern` | text (JSON) | Yes | URL routing patterns |
| `headers` | text (JSON) | No | Additional HTTP headers |
| `action_config` | text (JSON) | No | Pre/post-processing config |
| `cache_config` | text (JSON) | No | Caching behavior |

## Creating Templates

### Via API

```bash
TOKEN=$(cat /tmp/daptin-token.txt)

curl -X POST http://localhost:6336/api/template \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "template",
      "attributes": {
        "name": "welcome_email_template",
        "content": "<h1>Hello {{.name}}!</h1><p>Welcome to {{.service}}.</p><p>{{.message}}</p>",
        "mime_type": "text/html",
        "url_pattern": "{}"
      }
    }
  }'
```

**Note**: `url_pattern` field is REQUIRED (even if empty `{}`)

### Template Content Sources

The `content` field supports multiple sources:

1. **Direct Content**: Plain template text
   ```
   "content": "Hello {{.name}}, your order {{.id}} is ready."
   ```

2. **Base64 Encoded**: Automatically decoded
   ```
   "content": "SGVsbG8ge3submFtZX19..."
   ```

3. **Subsite File Reference**:
   ```
   "content": "subsite://<site_reference_id>/templates/welcome.html"
   ```

4. **Site File Reference**:
   ```
   "content": "site://<site_reference_id>/emails/notification.html"
   ```

## Go Template Syntax

### Variables

```go
Hello {{.name}}!
Your order #{{.order_id}} total: ${{.total}}
```

### Conditionals

```go
{{if .isPremium}}
  <p>Premium member benefits...</p>
{{else}}
  <p>Upgrade to premium!</p>
{{end}}
```

### Loops

```go
<ul>
{{range .items}}
  <li>{{.name}}: ${{.price}}</li>
{{end}}
</ul>
```

### Extended Functions (soha)

Daptin uses [flysnow-org/soha](https://github.com/flysnow-org/soha) for extended template functions:

```go
{{.text | upper}}           - Uppercase
{{.text | lower}}           - Lowercase
{{.html | safe}}            - Unescaped HTML
{{.timestamp | date "2006-01-02"}}  - Format date
```

See soha documentation for complete function list.

## Using Templates in Custom Actions

### Example: Email Generation Action

**1. Create template:**
```bash
curl -X POST http://localhost:6336/api/template \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "template",
      "attributes": {
        "name": "order_confirmation",
        "content": "<h1>Order Confirmed</h1><p>Hi {{.customer_name}},</p><p>Your order #{{.order_id}} for ${{.total}} has been confirmed.</p>",
        "mime_type": "text/html",
        "url_pattern": "{}"
      }
    }
  }'
```

**2. Create custom action in schema:**
```yaml
Actions:
  - Name: send_order_confirmation
    OnType: order
    InstanceOptional: false
    InFields:
      - Name: customer_name
        ColumnName: customer_name
        ColumnType: label
      - Name: customer_email
        ColumnName: customer_email
        ColumnType: email
    OutFields:
      # Step 1: Render template
      - Type: template.render
        Method: EXECUTE
        Reference: rendered_email
        Attributes:
          template: order_confirmation
          customer_name: ~customer_name
          order_id: $.reference_id
          total: $.total
      
      # Step 2: Send email
      - Type: mail.send
        Method: EXECUTE
        Attributes:
          to: ~customer_email
          subject: "Order Confirmation"
          body: $rendered_email.content
          mime_type: $rendered_email.mime_type
```

**3. Call action:**
```bash
curl -X POST "http://localhost:6336/action/order/send_order_confirmation/$ORDER_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "attributes": {
      "customer_name": "John Doe",
      "customer_email": "john@example.com"
    }
  }'
```

## Template Response Format

When template.render executes (within custom action), it returns:

```json
{
  "content": "BASE64_ENCODED_RENDERED_OUTPUT",
  "mime_type": "text/html",
  "headers": {}
}
```

Use `$reference.content` to access the rendered output in subsequent outcomes.

## Referencing Subsite Files

Templates can load content from subsite files:

```bash
# 1. Create site and upload template file
mkdir -p ./storage/email-templates
cat > ./storage/email-templates/welcome.html << 'EOF'
<h1>Welcome {{.name}}!</h1>
<p>Thank you for joining {{.service}}.</p>
EOF

# 2. Get site reference_id
SITE_ID=$(curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/api/site" | jq -r '.data[] | select(.attributes.name == "email-templates") | .id')

# 3. Create template referencing subsite file
curl -X POST http://localhost:6336/api/template \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "template",
      "attributes": {
        "name": "welcome_from_site",
        "content": "subsite://'$SITE_ID'/welcome.html",
        "mime_type": "text/html",
        "url_pattern": "{}"
      }
    }
  }'
```

## Best Practices

1. **Use descriptive names** - Template names should clearly indicate purpose
2. **Set correct MIME types** - Ensures proper content type headers
3. **Escape user input** - Use `{{.var}}` not `{{.var | safe}}` for user data
4. **Version templates** - Include version in name for email templates that need history
5. **Test before production** - Create test custom action to verify template rendering
6. **Store complex templates in files** - Use subsite reference for large templates

## Common Use Cases

### 1. Email Templates

```yaml
Actions:
  - Name: send_welcome_email
    OnType: user_account
    OutFields:
      - Type: template.render
        Reference: email_content
        Attributes:
          template: welcome_email
          name: $.name
          activation_link: $.activation_url
      - Type: mail.send
        Attributes:
          to: $.email
          subject: "Welcome!"
          body: $email_content.content
```

### 2. PDF Generation

```yaml
Actions:
  - Name: generate_invoice_pdf
    OnType: invoice
    OutFields:
      - Type: template.render
        Reference: html
        Attributes:
          template: invoice_template
          invoice_number: $.number
          items: $.line_items
      - Type: html.to.pdf
        Attributes:
          html: $html.content
```

### 3. Dynamic API Responses

```yaml
Actions:
  - Name: get_formatted_data
    OnType: data
    OutFields:
      - Type: template.render
        Attributes:
          template: json_response_template
          data: $.attributes
```

## Troubleshooting

### Template Creation Fails: NOT NULL Constraint

**Error**: `NOT NULL constraint failed: template.url_pattern`

**Solution**: Always include `url_pattern` field:
```json
{
  "attributes": {
    "name": "my-template",
    "content": "...",
    "mime_type": "text/html",
    "url_pattern": "{}"  // REQUIRED
  }
}
```

### Cannot Call template.render Directly

**Error**: 403 Forbidden or "no reference id"

**Solution**: template.render is internal-only. Create custom action with template.render in OutFields.

### Template Variables Not Substituting

**Check**:
1. Variable names match between action and template
2. Using correct syntax: `{{.variable}}` not `{{variable}}`
3. Variables passed in `Attributes` of outcome

**Debug**:
```yaml
# Add logging outcome before template.render
OutFields:
  - Type: response.create
    Attributes:
      message: "Debug: name={{~name}}, email={{~email}}"
  - Type: template.render
    ...
```

## Related

- [Custom Actions](Custom-Actions.md) - How to create actions with template.render
- [Email Actions](Email-Actions.md) - Sending templated emails
- [Subsites](Subsites.md) - Hosting template files in subsites
- [Action Reference](Action-Reference.md) - All available internal outcomes
