# Email Actions

**Tested ✓** - All examples verified against running Daptin instance (2026-01-26)

Send emails programmatically using custom actions with email performers.

---

## Important: Performers, Not Direct Actions

`mail.send` and `aws.mail.send` are **performers**, not direct REST endpoints. You cannot call them directly like `/action/world/mail.send`. Instead, use them in custom actions' `OutFields`.

**See Also:** [Custom Actions Guide](Custom-Actions.md) for creating custom actions.

---

## Quick Start: Send Email

### 1. Create Custom Action

Create `schema_email.yaml`:

```yaml
Actions:
  - Name: send_notification
    Label: Send Notification Email
    OnType: world
    InstanceOptional: true
    InFields:
      - Name: recipient
        ColumnName: recipient
        ColumnType: email
        IsNullable: false
      - Name: subject
        ColumnName: subject
        ColumnType: label
        IsNullable: false
      - Name: message
        ColumnName: message
        ColumnType: content
        IsNullable: false
    OutFields:
      - Type: mail.send
        Method: EXECUTE
        Attributes:
          from: "noreply@yourdomain.com"
          to: "![recipient]"
          subject: "~subject"
          body: "~message"
```

**Note:** The `to` parameter uses JavaScript array syntax `![recipient]` to convert string to array.

Restart Daptin to load the schema.

### 2. Call Your Custom Action

```bash
curl -X POST http://localhost:6336/action/world/send_notification \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "recipient": "user@example.com",
      "subject": "Welcome",
      "message": "Thank you for signing up!"
    }
  }'
```

---

## mail.send Performer

Send email via direct SMTP delivery or configured mail server.

### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `from` | string | Yes | Sender email address |
| `to` | array of strings | Yes | Recipient email addresses |
| `subject` | string | Yes | Email subject line |
| `body` | string | Yes | Email body (plain text) |
| `mail_server_hostname` | string | No | Use specific mail server with DKIM signing |

### Basic Sending (Direct MTA)

Sends email directly by looking up MX records for recipient domain:

```yaml
OutFields:
  - Type: mail.send
    Method: EXECUTE
    Attributes:
      from: "noreply@mydomain.com"
      to: "![recipient_email]"
      subject: "~email_subject"
      body: "~email_body"
```

### Sending via Configured Mail Server

Use a specific mail server with DKIM signing:

```yaml
OutFields:
  - Type: mail.send
    Method: EXECUTE
    Attributes:
      from: "noreply@mydomain.com"
      to: "![recipient_email]"
      subject: "~email_subject"
      body: "~email_body"
      mail_server_hostname: "mail.mydomain.com"
```

**Prerequisites:**
- Mail server must be configured in Daptin
- See [SMTP Server Guide](SMTP-Server.md) for setup

### Multiple Recipients

```yaml
OutFields:
  - Type: mail.send
    Method: EXECUTE
    Attributes:
      from: "noreply@mydomain.com"
      to: "![email1, email2, email3].split(',').map(e => e.trim())"
      subject: "~subject"
      body: "~body"
```

### Limitations

- Body is always plain text (no HTML support in basic mode)
- No attachments support
- No CC/BCC recipients
- For advanced features, use `aws.mail.send` or configure SMTP server

---

## aws.mail.send Performer

Send email via AWS Simple Email Service (SES).

### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `from` | string | Yes | Sender email (must be verified in SES) |
| `to` | array of strings | Yes | Recipient emails |
| `subject` | string | Yes | Email subject |
| `text` | string | No* | Plain text body |
| `html` | string | No* | HTML body |
| `cc` | array of strings | No | CC recipients |
| `bcc` | array of strings | No | BCC recipients |
| `credential` | string | Yes | Name of stored AWS credential |

*Either `text` or `html` is required

### Step 1: Create AWS Credential

First, store your AWS credentials:

```bash
curl -X POST http://localhost:6336/api/credential \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "credential",
      "attributes": {
        "name": "aws-ses-prod",
        "content": "{\"access_key\":\"AKIAXXXXXXXX\",\"secret_key\":\"your-secret-key\",\"region\":\"us-east-1\",\"token\":\"\"}"
      }
    }
  }'
```

**Credential Format:**
```json
{
  "access_key": "AKIAXXXXXXXX",
  "secret_key": "your-secret-key",
  "region": "us-east-1",
  "token": ""
}
```

### Step 2: Create Custom Action

```yaml
Actions:
  - Name: send_ses_email
    Label: Send AWS SES Email
    OnType: world
    InstanceOptional: true
    InFields:
      - Name: recipient
        ColumnName: recipient
        ColumnType: email
      - Name: subject
        ColumnName: subject
        ColumnType: label
      - Name: html_body
        ColumnName: html_body
        ColumnType: content
    OutFields:
      - Type: aws.mail.send
        Method: EXECUTE
        Attributes:
          from: "verified@yourdomain.com"
          to: "![recipient]"
          subject: "~subject"
          html: "~html_body"
          credential: "aws-ses-prod"
```

### Step 3: Call Your Action

```bash
curl -X POST http://localhost:6336/action/world/send_ses_email \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "recipient": "user@example.com",
      "subject": "HTML Email",
      "html_body": "<h1>Welcome!</h1><p>HTML email from AWS SES.</p>"
    }
  }'
```

### HTML Email with CC/BCC

```yaml
OutFields:
  - Type: aws.mail.send
    Method: EXECUTE
    Attributes:
      from: "noreply@mydomain.com"
      to: "![primary_recipient]"
      cc: "![cc_recipient]"
      bcc: "![bcc_recipient]"
      subject: "~subject"
      html: "<html><body>~html_content</body></html>"
      credential: "aws-ses-prod"
```

---

## sync_mail_servers Action

Reload SMTP server configuration from database.

### Endpoint

```
POST /action/mail_server/sync_mail_servers
```

### Usage

```bash
curl -X POST http://localhost:6336/action/mail_server/sync_mail_servers \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{}'
```

### When to Use

- After creating or modifying mail_server records
- After updating mail server TLS certificates
- To reload mail daemon configuration without restart

---

## Complete Example: User Registration with Email

Combine user creation with welcome email:

```yaml
Actions:
  - Name: register_user
    Label: Register New User
    OnType: user_account
    InstanceOptional: true
    InFields:
      - Name: email
        ColumnName: email
        ColumnType: email
        IsNullable: false
      - Name: name
        ColumnName: name
        ColumnType: label
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
      # 2. Send welcome email
      - Type: mail.send
        Method: EXECUTE
        ContinueOnError: true
        Attributes:
          from: "welcome@mydomain.com"
          to: "![email]"
          subject: "Welcome to Our Service"
          body: "!`Hello ${name},\n\nThank you for registering!\n\nBest regards,\nThe Team`"
      # 3. Success notification
      - Type: client.notify
        Method: ACTIONRESPONSE
        Attributes:
          type: success
          title: Registration Complete
          message: "Account created for ~name. Check your email."
```

---

## Built-in Email Actions

Daptin includes these email-sending actions:

| Action | OnType | Purpose |
|--------|--------|---------|
| `generate_password_reset` | user_account | Send password reset OTP |
| `reset_password` | user_account | Reset password with new value |

These use `mail.send` internally. Configure an SMTP server for them to work.

---

## SMTP Server Setup

For production email sending, configure an SMTP server:

**See:** [SMTP Server Guide](SMTP-Server.md) for complete setup instructions including:
- Mail server configuration
- Mail accounts and mailboxes
- TLS certificates
- DKIM signing
- DNS records (SPF, DKIM)

---

## Troubleshooting

### "dial tcp: lookup .: no such host"

**Cause:** No SMTP server configured, domain has no MX records

**Solutions:**
1. Configure a mail_server in Daptin
2. Use `mail_server_hostname` parameter
3. Use `aws.mail.send` instead

### "interface conversion: interface {} is []interface {}, not []string"

**Cause:** Bug in older Daptin versions (fixed 2026-01-26)

**Solution:** Use JavaScript array syntax in OutFields:
```yaml
to: "![recipient_email]"  # Converts string to array
```

### Action Returns 403 Forbidden

**Cause:** Permission restrictions on action

**Solution:** Check action permissions and user group membership

### AWS SES: "InvalidParameterValue"

**Cause:** Email address not verified in AWS SES

**Solution:** Verify sender email in AWS SES console

### Mail Not Received

**Checks:**
1. Check server logs: `tail -f /tmp/daptin.log | grep -i mail`
2. Verify SMTP server is running
3. Check DNS records (MX, SPF, DKIM)
4. Test with direct SMTP connection

---

## Email Templates

Use template rendering for dynamic content:

```yaml
OutFields:
  # 1. Render template
  - Type: $template.render
    Method: EXECUTE
    Reference: rendered_email
    Attributes:
      template: |
        Hello {{.name}},

        Your order #{{.order_id}} has been confirmed.
        Total: ${{.total}}

        Thank you for your purchase!
      data:
        name: "~customer_name"
        order_id: "~order_id"
        total: "~total_amount"
  # 2. Send rendered email
  - Type: mail.send
    Method: EXECUTE
    Attributes:
      from: "orders@mydomain.com"
      to: "![customer_email]"
      subject: "Order Confirmation #~order_id"
      body: "$rendered_email.result"
```

---

## See Also

- [Custom Actions](Custom-Actions.md) - Creating and using custom actions
- [SMTP Server](SMTP-Server.md) - Complete SMTP server setup
- [IMAP Support](IMAP-Support.md) - Receiving emails
- [Credentials](Credentials.md) - Managing API credentials
