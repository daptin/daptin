# SMTP Server

Built-in SMTP email server powered by go-guerrilla.

## Overview

Daptin includes a full SMTP server for:
- Sending emails
- Receiving emails
- DKIM signing
- TLS encryption

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

```bash
curl -X POST http://localhost:6336/api/mail_account \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "mail_account",
      "attributes": {
        "username": "noreply@example.com",
        "password": "account-password"
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

### Via mail.send Action

```bash
curl -X POST http://localhost:6336/action/world/mail.send \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "attributes": {
      "from": "noreply@example.com",
      "to": ["user@example.com"],
      "subject": "Hello",
      "body": "<h1>Hello World</h1>",
      "mail_server_hostname": "mail.example.com"
    }
  }'
```

### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| from | string | Yes | Sender address |
| to | array | Yes | Recipients |
| cc | array | No | CC recipients |
| bcc | array | No | BCC recipients |
| subject | string | Yes | Subject line |
| body | string | Yes | Email content |
| mail_server_hostname | string | No | Specific server |

### AWS SES Alternative

```bash
curl -X POST http://localhost:6336/action/world/aws.mail.send \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "attributes": {
      "from": "noreply@example.com",
      "to": ["user@example.com"],
      "subject": "Hello",
      "body": "Plain text body",
      "region": "us-east-1",
      "access_key": "AKIAXXXXXXXX",
      "secret_key": "secret"
    }
  }'
```

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

```bash
curl -X POST http://localhost:6336/action/world/generate_self_tls_certificate \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"attributes": {"hostname": "mail.example.com"}}'
```

### Let's Encrypt

```bash
curl -X POST http://localhost:6336/action/world/generate_acme_tls_certificate \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "attributes": {
      "hostname": "mail.example.com",
      "email": "admin@example.com"
    }
  }'
```

## Sync Mail Servers

Reload configuration:

```bash
curl -X POST http://localhost:6336/action/mail_server/mail_servers_sync \
  -H "Authorization: Bearer $TOKEN"
```

## Email Tables

| Table | Purpose |
|-------|---------|
| mail_server | Server configuration |
| mail_account | User accounts |
| mail_box | Mailboxes (inbox, sent) |
| mail | Stored messages |

## Mailbox Management

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
