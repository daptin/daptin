# SMTP Server

Built-in SMTP email server powered by go-guerrilla.

## Overview

Daptin includes a full SMTP server for:
- Receiving emails
- DKIM signing
- TLS encryption

**Note:** Direct email sending via REST API is not available. The `mail.send` and `aws.mail.send` are internal performers used by actions like password reset. To send emails programmatically, create a custom action that uses these performers in OutFields.

## Ports

| Port | Protocol | Description |
|------|----------|-------------|
| 25 | SMTP | Standard (often blocked) |
| 465 | SMTPS | SSL/TLS |
| 587 | Submission | TLS via STARTTLS |

## Creating Mail Server

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

## Mail Server Properties

| Property | Type | Description |
|----------|------|-------------|
| hostname | string | Server hostname |
| is_enabled | bool | Enable/disable |
| listen_interface | string | Bind address:port |
| always_on_tls | bool | Require TLS |
| max_size | int | Max message size (bytes) |
| max_clients | int | Max concurrent connections |

## Creating Mail Account

**Important:** The `password_md5` field is required for SMTP authentication. Set it to the same value as `password` - Daptin will automatically hash it (MD5 then bcrypt).

```bash
curl -X POST http://localhost:6336/api/mail_account \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "mail_account",
      "attributes": {
        "username": "noreply@example.com",
        "password": "account-password",
        "password_md5": "account-password"
      },
      "relationships": {
        "mail_server_id": {
          "data": {"type": "mail_server", "id": "SERVER_ID"}
        }
      }
    }
  }'
```

## Sending Email

### About Email Sending

Daptin's mail functionality (`mail.send` and `aws.mail.send`) are **internal performers**, not standalone REST API endpoints. They are designed to be used within action OutFields.

### Built-in Usage

The `mail.send` performer is used internally by:
- `reset-password` action - sends OTP verification code
- `reset-password-verify` action - sends new password

### Custom Actions

To send emails programmatically, create a custom action with `mail.send` in its OutFields:

```json
{
  "OutFields": [
    {
      "Type": "mail.send",
      "Method": "EXECUTE",
      "Attributes": {
        "to": "~recipient_email",
        "subject": "Your Subject",
        "body": "Email body content",
        "from": "noreply@example.com",
        "mail_server_hostname": "mail.example.com"
      }
    }
  ]
}
```

### mail.send Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| from | string | Yes | Sender address |
| to | array | Yes | Recipients |
| subject | string | Yes | Subject line |
| body | string | Yes | Email content |
| mail_server_hostname | string | No | Use specific mail server |

### aws.mail.send Parameters

For AWS SES, the performer expects a stored credential reference:

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| from | string | Yes | Sender address |
| to | array | Yes | Recipients |
| cc | array | No | CC recipients |
| bcc | array | No | BCC recipients |
| subject | string | Yes | Subject line |
| body | string | Yes | Email content |
| credential | string | Yes | Name of stored AWS credential |

**Note:** Create an AWS credential first with `access_key`, `secret_key`, `region`, and `token` fields.

## DKIM Signing

Daptin automatically generates DKIM keys.

### Get DKIM Public Key

```bash
curl http://localhost:6336/api/mail_server/SERVER_ID \
  -H "Authorization: Bearer $TOKEN"
```

### DNS Configuration

Add DKIM record:

```
d1._domainkey.example.com IN TXT "v=DKIM1; k=rsa; p=PUBLIC_KEY_HERE"
```

## SPF Configuration

Add SPF record to DNS:

```
example.com IN TXT "v=spf1 ip4:YOUR_SERVER_IP -all"
```

## DMARC Configuration

```
_dmarc.example.com IN TXT "v=DMARC1; p=quarantine; rua=mailto:dmarc@example.com"
```

## TLS Certificates

### Generate Self-Signed

**Note:** Certificate actions require an existing certificate record. First create a certificate record, then use this action to generate the actual certificate.

```bash
# First create a certificate record
curl -X POST http://localhost:6336/api/certificate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "certificate",
      "attributes": {
        "hostname": "mail.example.com"
      }
    }
  }'

# Then generate the certificate (use the certificate ID from above)
curl -X POST http://localhost:6336/action/certificate/generate_self_certificate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"attributes": {"hostname": "mail.example.com"}, "certificate_id": "CERTIFICATE_ID"}'
```

### Let's Encrypt

```bash
# First create a certificate record (if not already created)
curl -X POST http://localhost:6336/api/certificate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "certificate",
      "attributes": {
        "hostname": "mail.example.com"
      }
    }
  }'

# Then generate ACME certificate (use the certificate ID from above)
curl -X POST http://localhost:6336/action/certificate/generate_acme_certificate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "attributes": {
      "hostname": "mail.example.com",
      "email": "admin@example.com"
    },
    "certificate_id": "CERTIFICATE_ID"
  }'
```

## Sync Mail Servers

Reload mail server configuration after changes.

**Important:** The SMTP daemon is initialized at Daptin startup. If no mail servers existed when Daptin started, the `sync_mail_servers` action will not start the SMTP daemon - you must restart Daptin.

```bash
curl -X POST http://localhost:6336/action/mail_server/sync_mail_servers \
  -H "Authorization: Bearer $TOKEN"
```

**When to use:**
- After updating mail server settings (hostname, ports, TLS)
- After adding/removing mail accounts
- After certificate changes

## Email Tables

| Table | Purpose |
|-------|---------|
| mail_server | Server configuration |
| mail_account | User accounts |
| mail_box | Mailboxes (inbox, sent) |
| mail | Stored messages |

## Mailbox Management

### Create Mailbox

**Required fields:** `name`, `attributes`, `flags`, `permanent_flags`

```bash
curl -X POST http://localhost:6336/api/mail_box \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "mail_box",
      "attributes": {
        "name": "INBOX",
        "subscribed": true,
        "uidvalidity": 1,
        "nextuid": 1,
        "attributes": "\\HasNoChildren",
        "flags": "\\Seen \\Answered \\Flagged \\Deleted \\Draft",
        "permanent_flags": "\\Seen \\Answered \\Flagged \\Deleted \\Draft \\*"
      },
      "relationships": {
        "mail_account_id": {
          "data": {"type": "mail_account", "id": "ACCOUNT_ID"}
        }
      }
    }
  }'
```

### List Mailboxes

```bash
curl 'http://localhost:6336/api/mail_box?query=[{"column":"mail_account_id","operator":"is","value":"ACCOUNT_ID"}]' \
  -H "Authorization: Bearer $TOKEN"
```

## Disable SMTP

Via environment variable:

```bash
DAPTIN_DISABLE_SMTP=true ./daptin
```

## Troubleshooting

### Check Server Status

```bash
curl http://localhost:6336/api/mail_server \
  -H "Authorization: Bearer $TOKEN"
```

### Test Connection

```bash
openssl s_client -connect localhost:465
```

### View Logs

Check server logs for SMTP activity.
