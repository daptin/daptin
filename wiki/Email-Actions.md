# Email Actions

Actions for sending and managing emails.

## mail.send

Send email via configured SMTP server.

```bash
curl -X POST http://localhost:6336/action/world/mail.send \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "from": "noreply@example.com",
      "to": ["user@example.com"],
      "subject": "Welcome to our service",
      "body": "<h1>Welcome!</h1><p>Thank you for signing up.</p>",
      "contentType": "text/html"
    }
  }'
```

**Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| from | string | Sender email |
| to | array | Recipient emails |
| cc | array | CC recipients |
| bcc | array | BCC recipients |
| subject | string | Email subject |
| body | string | Email content |
| contentType | string | text/plain or text/html |
| attachments | array | File attachments |

### With Attachments

```bash
curl -X POST http://localhost:6336/action/world/mail.send \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "attributes": {
      "from": "reports@example.com",
      "to": ["manager@example.com"],
      "subject": "Monthly Report",
      "body": "Please find attached the monthly report.",
      "attachments": [{
        "name": "report.pdf",
        "file": "data:application/pdf;base64,..."
      }]
    }
  }'
```

## aws.mail.send

Send email via AWS SES.

```bash
curl -X POST http://localhost:6336/action/world/aws.mail.send \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "from": "noreply@example.com",
      "to": ["user@example.com"],
      "subject": "Notification",
      "body": "This email was sent via AWS SES.",
      "region": "us-east-1",
      "access_key": "AKIAXXXXXXXX",
      "secret_key": "secret"
    }
  }'
```

**Additional Parameters:**

| Parameter | Description |
|-----------|-------------|
| region | AWS region |
| access_key | AWS access key |
| secret_key | AWS secret key |

## mail_servers_sync

Synchronize mail server configuration.

```bash
curl -X POST http://localhost:6336/action/mail_server/mail_servers_sync \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"attributes": {}}'
```

Reloads all mail server configurations from database.

## Setting Up SMTP Server

### Create Mail Server

```bash
curl -X POST http://localhost:6336/api/mail_server \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "mail_server",
      "attributes": {
        "hostname": "mail.example.com",
        "is_enabled": true,
        "listen_interface": "0.0.0.0:465",
        "always_on_tls": true,
        "max_size": 10485760,
        "max_clients": 100
      }
    }
  }'
```

**Mail Server Fields:**

| Field | Description |
|-------|-------------|
| hostname | Server hostname |
| is_enabled | Enable/disable server |
| listen_interface | Bind address:port |
| always_on_tls | Require TLS |
| max_size | Max message size (bytes) |
| max_clients | Max concurrent connections |

### Create Mail Account

```bash
curl -X POST http://localhost:6336/api/mail_account \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "mail_account",
      "attributes": {
        "username": "user@example.com",
        "password": "accountpassword"
      },
      "relationships": {
        "mail_server": {"data": {"type": "mail_server", "id": "SERVER_ID"}}
      }
    }
  }'
```

### Create Mailbox

```bash
curl -X POST http://localhost:6336/api/mail_box \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "mail_box",
      "attributes": {
        "name": "inbox"
      },
      "relationships": {
        "mail_account": {"data": {"type": "mail_account", "id": "ACCOUNT_ID"}}
      }
    }
  }'
```

## Email Tables

| Table | Purpose |
|-------|---------|
| mail_server | SMTP server configuration |
| mail_account | Email accounts |
| mail_box | Mailboxes (inbox, sent, etc.) |
| mail | Stored emails |

## DKIM Signing

Daptin automatically generates DKIM keys for each mail server:

```bash
# Get DKIM public key
curl http://localhost:6336/api/mail_server/SERVER_ID \
  -H "Authorization: Bearer $TOKEN"
```

Add the DKIM record to your DNS:

```
selector._domainkey.example.com IN TXT "v=DKIM1; k=rsa; p=PUBLIC_KEY"
```

## IMAP Configuration

Enable IMAP for email retrieval:

```bash
# Enable IMAP
curl -X POST http://localhost:6336/_config/backend/imap.enabled \
  -H "Authorization: Bearer $TOKEN" \
  -d 'true'

# Set interface
curl -X POST http://localhost:6336/_config/backend/imap.listen_interface \
  -H "Authorization: Bearer $TOKEN" \
  -d '"0.0.0.0:993"'
```

## Email Templates

Use render_template action with email:

```bash
curl -X POST http://localhost:6336/action/world/render_template \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "attributes": {
      "template": "<h1>Hello {{.name}}</h1><p>Your order {{.order_id}} is confirmed.</p>",
      "data": {
        "name": "John",
        "order_id": "ORD-12345"
      }
    }
  }'
```

Then send the rendered output via mail.send.

## SPF Configuration

Add SPF record to DNS:

```
example.com IN TXT "v=spf1 ip4:YOUR_SERVER_IP -all"
```

## Troubleshooting

### Check Mail Server Status

```bash
curl http://localhost:6336/api/mail_server \
  -H "Authorization: Bearer $TOKEN"
```

### View Mail Queue

```bash
curl 'http://localhost:6336/api/mail?query=[{"column":"status","operator":"is","value":"queued"}]' \
  -H "Authorization: Bearer $TOKEN"
```

### Test SMTP Connection

```bash
# From server
telnet localhost 465
```
