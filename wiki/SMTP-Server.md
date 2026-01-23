# SMTP Server

Built-in SMTP email server powered by go-guerrilla.

## Overview

Daptin includes a full SMTP server for:
- Receiving emails
- DKIM signing
- TLS encryption
- Authenticated SMTP relay

**Note:** Direct email sending via REST API is not available. The `mail.send` and `aws.mail.send` are internal performers used by actions like password reset. To send emails programmatically, create a custom action that uses these performers in OutFields.


## Complete Setup Guide

The SMTP server requires proper setup order. Follow these steps:

### Step 1: Create TLS Certificate

```bash
# Create certificate record
CERT_RESPONSE=$(curl -s -X POST http://localhost:6336/api/certificate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "certificate",
      "attributes": {
        "hostname": "mail.example.com"
      }
    }
  }')
CERT_ID=$(echo $CERT_RESPONSE | jq -r '.data.id')

# Generate self-signed certificate
curl -X POST http://localhost:6336/action/certificate/generate_self_certificate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"attributes\":{}, \"certificate_id\": \"$CERT_ID\"}"
```

### Step 2: Create Mail Server

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
        "always_on_tls": false,
        "max_size": 10485760,
        "max_clients": 100
      }
    }
  }'
```

### Step 3: Restart Daptin

**Important:** The SMTP daemon is initialized at Daptin startup. After creating your first mail server, you must restart Daptin.

### Step 4: Create Mail Account

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

### Step 5: Create Mailbox

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

**Password Reset Flow:**
The `generate_password_reset_flow` action stores the reset email in the user's **local Daptin mailbox** (via `TaskSaveMail`), not sent externally. Users retrieve it via IMAP.

### Custom Actions

To send emails externally, create a custom action with `mail.send` in its OutFields:

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
| mail_server_hostname | string | No | Use specific mail server for DKIM signing |

### mail.send Operation Modes

**Mode 1: Direct Send** (no `mail_server_hostname`)
- Sends via MTA directly to recipient's mail server
- No DKIM signing
- Simpler but may have lower deliverability

**Mode 2: Via Mail Server** (with `mail_server_hostname`)
- Uses configured mail server settings
- Signs outgoing mail with DKIM
- Requires valid certificate for sender's domain
- Better deliverability and authenticity

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

## Spam Scoring Algorithm

Incoming emails are scored for spam based on SPF and DKIM verification.

### Score Calculation

| Check | Result | Score |
|-------|--------|-------|
| SPF Valid | Sender authorized | 0 |
| SPF Error | Uncertain (network/lookup issue) | 50 |
| SPF Invalid | Sender not authorized/blacklisted | 200 |
| DKIM | Each failed/missing signature | +100 |

**Routing by Score:**
- `score > 299` → Spam folder
- `score > 50` → INBOX with `\Spam` flag
- `score ≤ 50` → INBOX with `\Recent` flag

### Example Scores

| Scenario | Score | Destination |
|----------|-------|-------------|
| Valid SPF, valid DKIM | 0 | INBOX |
| SPF error, no DKIM | 150 | INBOX (\Spam flag) |
| Invalid SPF, no DKIM | 300 | Spam folder |
| Valid SPF, 2 failed DKIM | 200 | INBOX (\Spam flag) |

### Spam Fields in Mail Table

| Field | Type | Description |
|-------|------|-------------|
| spam_score | int | Calculated spam score |
| spam | bool | true if score > 50 |
| flags | varchar | IMAP flags including `\Spam` |

## Mail Forwarding (Relay)

Authenticated users can send emails to external recipients (not local accounts).

### Requirements

1. **Authentication** - Sender must be logged in via SMTP AUTH
2. **Certificate** - Valid TLS certificate for sender's domain (for DKIM signing)
3. **DKIM** - Outgoing mail is signed with domain's private key

### Flow

```
Authenticated User → SMTP Server → DKIM Sign → External MTA
```

### Error: "private key not found for signing outgoing email"

This occurs when forwarding mail from a domain without a certificate:

```bash
# Create certificate for your domain
curl -X POST http://localhost:6336/api/certificate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{"data":{"type":"certificate","attributes":{"hostname":"yourdomain.com"}}}'

# Generate the certificate
curl -X POST http://localhost:6336/action/certificate/generate_self_certificate \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"attributes":{},"certificate_id":"CERT_ID"}'
```

### Unauthenticated External Sender

Without authentication, external senders are rejected:
```
554 5.7.1 Client host rejected: Access denied
```

This prevents open relay abuse.

## Email Tables

| Table | Purpose |
|-------|---------|
| mail_server | Server configuration |
| mail_account | User accounts |
| mail_box | Mailboxes (INBOX, Spam, etc.) |
| mail | Stored messages |
| outbox | Reserved for mail queue (not currently used) |

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

## SMTP Authentication

The SMTP server supports LOGIN authentication. Credentials are base64-encoded.

### Testing Authenticated SMTP

```bash
# Base64 encode credentials
USERNAME_B64=$(echo -n "noreply@example.com" | base64)
PASSWORD_B64=$(echo -n "account-password" | base64)

# Send authenticated email
{
  echo "EHLO client.example.com"
  sleep 0.2
  echo "AUTH LOGIN"
  sleep 0.2
  echo "$USERNAME_B64"
  sleep 0.2
  echo "$PASSWORD_B64"
  sleep 0.2
  echo "MAIL FROM:<noreply@example.com>"
  sleep 0.2
  echo "RCPT TO:<recipient@external.com>"
  sleep 0.2
  echo "DATA"
  sleep 0.2
  echo "Subject: Test Email"
  echo "From: noreply@example.com"
  echo "To: recipient@external.com"
  echo ""
  echo "Email body content."
  echo "."
  sleep 0.2
  echo "QUIT"
} | nc localhost 465
```

**Expected responses:**
- `334 VXNlcm5hbWU6` - Server requesting username
- `334 UGFzc3dvcmQ6` - Server requesting password
- `235 Authentication succeeded` - Login successful
- `250 2.0.0 OK: queued as HASH` - Email queued

## Manual Email Creation

Due to the inbound storage bug, create emails via REST API:

```bash
curl -X POST http://localhost:6336/api/mail \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "mail",
      "attributes": {
        "message_id": "<unique-id@example.com>",
        "mail_id": "unique-hash",
        "from_address": "sender@example.com",
        "to_address": "noreply@example.com",
        "sender_address": "sender@example.com",
        "subject": "Email Subject",
        "body": "Plain text body",
        "mail": "From: sender@example.com\r\nTo: noreply@example.com\r\nSubject: Email Subject\r\n\r\nEmail body.",
        "spam_score": 0,
        "spam": false,
        "hash": "unique-hash",
        "content_type": "text/plain",
        "reply_to_address": "sender@example.com",
        "internal_date": "2026-01-23T12:00:00Z",
        "recipient": "noreply@example.com",
        "has_attachment": false,
        "ip_addr": "127.0.0.1",
        "return_path": "sender@example.com",
        "is_tls": false,
        "seen": false,
        "recent": true,
        "flags": "\\Recent",
        "size": 100
      },
      "relationships": {
        "mail_box_id": {"data": {"type": "mail_box", "id": "MAILBOX_ID"}},
        "user_account_id": {"data": {"type": "user_account", "id": "USER_ID"}}
      }
    }
  }'
```

**Required mail fields:**
| Field | Description |
|-------|-------------|
| message_id | Unique message identifier |
| mail_id | Hash identifier |
| from_address | Sender email |
| to_address | Recipient email |
| sender_address | Sender for SMTP envelope |
| subject | Email subject |
| body | Plain text body |
| mail | Full RFC 822 message (can be gzip compressed) |
| reply_to_address | Reply-to address |
| ip_addr | Sender IP |
| return_path | Return path address |
| mail_box_id | Relationship to mailbox |
| user_account_id | Relationship to user |

## IMAP Server

Daptin includes an IMAP server for email retrieval.

### Enable IMAP

**Important:** Config values must be sent as plain text, not JSON-quoted strings.

```bash
# Enable IMAP via config API (use Content-Type: text/plain)
curl -X POST 'http://localhost:6336/_config/backend/imap.enabled' \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: text/plain' \
  -d 'true'

# Set hostname (plain text)
curl -X POST 'http://localhost:6336/_config/backend/hostname' \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: text/plain' \
  -d 'mail.example.com'

# Optionally set listen interface (default :1143)
curl -X POST 'http://localhost:6336/_config/backend/imap.listen_interface' \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: text/plain' \
  -d ':1143'
```

**Restart Daptin after enabling IMAP.**

### IMAP Prerequisites

1. Create certificate for `imap.{hostname}` (e.g., `imap.mail.example.com`)
2. Set `imap.enabled` to `true` (plain text, not `"true"`)
3. Set `hostname` config value
4. Restart Daptin

### IMAP Authentication

- Requires TLS (STARTTLS or port 993)
- Without TLS, server shows `LOGINDISABLED`
- Uses mail_account credentials (username and plain password)

### IMAP Ports

| Port | Protocol |
|------|----------|
| 143 | IMAP |
| 993 | IMAPS (TLS) |
| 1143 | Default Daptin IMAP |

### Testing IMAP Connection

```bash
# Test capabilities (without TLS)
echo "a001 CAPABILITY" | nc localhost 1143

# Test with STARTTLS
openssl s_client -starttls imap -connect localhost:1143 -quiet
```

## Disable SMTP

Via environment variable:

```bash
DAPTIN_DISABLE_SMTP=true ./daptin
```

## Complete SMTP → IMAP Flow (Verified Working)

This section documents the verified working flow from receiving email via SMTP to reading it via IMAP.

### 1. Send Email via SMTP (using swaks)

```bash
swaks --to test@test.com --from sender@example.com \
  --server localhost --port 2525 \
  --auth LOGIN --auth-user test@test.com --auth-password testpass123 \
  --header "Subject: Test email" \
  --body "This is a test email body"
```

**Expected output:**
```
<- 235 Authentication succeeded
<- 250 2.0.0 OK: queued as <hash>
```

### 2. Retrieve via IMAP (using openssl)

```bash
printf 'a LOGIN test@test.com testpass123\r\n\
b SELECT INBOX\r\n\
c SEARCH ALL\r\n\
d FETCH 1 (FLAGS BODY[HEADER.FIELDS (FROM SUBJECT)])\r\n\
e LOGOUT\r\n' | openssl s_client -connect localhost:1143 -starttls imap -quiet -ign_eof 2>/dev/null
```

**Expected output:**
```
a OK LOGIN completed
* 1 EXISTS
* 1 RECENT
b OK SELECT completed
* SEARCH 1
c OK SEARCH completed
* 1 FETCH (FLAGS (\Recent) BODY[HEADER.FIELDS (FROM SUBJECT)] {64}
From: sender@example.com
Subject: Test email
)
d OK FETCH completed
e OK LOGOUT completed
```

### 3. Verify in Database

```bash
sqlite3 daptin.db "SELECT id, subject, from_address, to_address FROM mail ORDER BY id DESC LIMIT 5;"
```

## Troubleshooting

### Check Server Status

```bash
curl http://localhost:6336/api/mail_server \
  -H "Authorization: Bearer $TOKEN"
```

### Test SMTP Connection

```bash
# Non-TLS test
{
  echo "EHLO test.local"
  sleep 0.2
  echo "QUIT"
} | nc localhost 2525

# TLS test
openssl s_client -connect localhost:465
```

**Expected EHLO response:**
```
250-mail.example.com Hello
250-SIZE 10485760
250-PIPELINING
250-STARTTLS
250-AUTH LOGIN
250-ENHANCEDSTATUSCODES
250 HELP
```

### View Logs

Check server logs for SMTP activity. Key log messages:

```
# Successful startup
INFO Setup SMTP server at [0.0.0.0:2525] for hostname [mail.example.com]
INFO Starting: 0.0.0.0:2525
INFO Listening on TCP 0.0.0.0:2525

# SMTP disabled (no servers configured)
INFO SMTP server is disabled since DAPTIN_DISABLE_SMTP=true or no servers configured
```

### Common Issues

**Issue: SMTP server doesn't start after creating mail_server**
- Cause: SMTP daemon initializes at startup only
- Solution: Restart Daptin after creating your first mail server

**Issue: `sync_mail_servers` action returns empty response**
- Cause: SMTP daemon was not initialized (no servers existed at startup)
- Solution: Restart Daptin

**Issue: `554 5.7.1 Client host rejected: Access denied`**
- Cause: Unauthenticated sender fails SPF validation
- Solution: Use SMTP authentication (AUTH LOGIN)

**Issue: `NOT NULL constraint failed: mail_account.password_md5`**
- Cause: `password_md5` field is required
- Solution: Include `password_md5` with same value as `password`

**Issue: `NOT NULL constraint failed: mail_box.attributes`**
- Cause: `attributes`, `flags`, `permanent_flags` fields are required
- Solution: Include all required mailbox fields (see Step 5 in setup guide)

**Issue: IMAP shows `LOGINDISABLED`**
- Cause: IMAP requires TLS for authentication
- Solution: Use STARTTLS or connect to port 993

**Issue: IMAP FETCH hangs or returns internal error**
- Cause: SQLite transaction deadlock (fixed in recent commits)
- Solution: Update to latest Daptin version
