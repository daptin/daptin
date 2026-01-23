# IMAP Support

Built-in IMAP server for email retrieval.

## Overview

Daptin includes an IMAP server that allows email clients to:
- Read emails
- Search mailboxes
- Manage folders
- Support IDLE for push notifications

## Ports

| Port | Protocol | Description |
|------|----------|-------------|
| 143 | IMAP | Standard (with STARTTLS) |
| 993 | IMAPS | SSL/TLS |
| 1143 | IMAP | Default Daptin port |

## Enable IMAP

### Configure via API

```bash
# Enable IMAP
curl -X POST 'http://localhost:6336/_config/backend/imap.enabled' \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer $TOKEN' \
  -d 'true'

# Set listen interface (default: :1143)
curl -X POST 'http://localhost:6336/_config/backend/imap.listen_interface' \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer $TOKEN' \
  -d '"0.0.0.0:993"'

# Set hostname
curl -X POST 'http://localhost:6336/_config/backend/hostname' \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer $TOKEN' \
  -d '"imap.example.com"'
```

### Restart to Apply

```bash
curl -X POST 'http://localhost:6336/action/world/restart_daptin' \
  -H 'Authorization: Bearer $TOKEN' \
  -H 'Content-Type: application/json' \
  -d '{"attributes":{}}'
```

## Prerequisites

Before IMAP works, you need:

1. **Mail Server** - Create a mail server entry
2. **Mail Account** - Create mail accounts
3. **TLS Certificate** - Required for secure connections

### Create Mail Server

```bash
curl -X POST 'http://localhost:6336/api/mail_server' \
  -H 'Content-Type: application/vnd.api+json' \
  -H 'Authorization: Bearer $TOKEN' \
  -d '{
    "data": {
      "type": "mail_server",
      "attributes": {
        "hostname": "mail.example.com",
        "is_enabled": true,
        "listen_interface": "0.0.0.0:465",
        "always_on_tls": true,
        "authentication_required": true,
        "max_clients": 20,
        "max_size": 10000000
      }
    }
  }'
```

### Create Mail Account

```bash
curl -X POST 'http://localhost:6336/api/mail_account' \
  -H 'Content-Type: application/vnd.api+json' \
  -H 'Authorization: Bearer $TOKEN' \
  -d '{
    "data": {
      "type": "mail_account",
      "attributes": {
        "username": "user@example.com",
        "password": "secure-password"
      },
      "relationships": {
        "mail_server_id": {
          "data": {"type": "mail_server", "id": "SERVER_ID"}
        }
      }
    }
  }'
```

## Client Configuration

### Thunderbird

1. Account Settings → Server Settings
2. Server Type: IMAP Mail Server
3. Server Name: `imap.example.com`
4. Port: `993` (SSL) or `143` (STARTTLS)
5. Connection Security: SSL/TLS
6. Username: Full email address

### macOS Mail

1. Mail → Add Account → Other Mail Account
2. IMAP Server: `imap.example.com`
3. Port: `993`
4. SSL: Enabled

### iOS Mail

1. Settings → Mail → Accounts → Add Account
2. Choose Other → Add Mail Account
3. Incoming Mail Server: `imap.example.com`
4. Port: `993`

## IMAP Features

### Supported Commands

| Command | Description |
|---------|-------------|
| LOGIN | Authenticate with username/password |
| SELECT | Open mailbox for read/write |
| EXAMINE | Open mailbox read-only |
| LIST | List available mailboxes |
| LSUB | List subscribed mailboxes |
| STATUS | Get mailbox status (EXISTS, RECENT, UNSEEN) |
| FETCH | Retrieve message content and metadata |
| SEARCH | Search messages by criteria |
| STORE | Modify message flags |
| COPY | Copy messages to another mailbox |
| EXPUNGE | Permanently delete flagged messages |
| IDLE | Real-time push notifications |
| CREATE | Create new mailbox |
| DELETE | Delete mailbox |
| RENAME | Rename mailbox |
| SUBSCRIBE | Subscribe to mailbox |
| UNSUBSCRIBE | Unsubscribe from mailbox |

### Folder Structure

Default mailboxes created automatically:
- INBOX - Incoming mail
- Spam - Messages with high spam score (>299)

Additional folders can be created via IMAP or REST API.

## Command Line Testing

### Basic Connection Test

```bash
# Without TLS (shows LOGINDISABLED)
echo "a CAPABILITY" | nc localhost 1143

# Expected output:
# * OK [CAPABILITY IMAP4rev1 ... LOGINDISABLED] IMAP4rev1 Service Ready
```

### Full IMAP Session with STARTTLS

```bash
printf 'a LOGIN user@example.com password\r\n\
b SELECT INBOX\r\n\
c SEARCH ALL\r\n\
d FETCH 1 (FLAGS ENVELOPE BODY[HEADER.FIELDS (FROM TO SUBJECT DATE)])\r\n\
e LOGOUT\r\n' | openssl s_client -connect localhost:1143 -starttls imap -quiet -ign_eof 2>/dev/null
```

### SEARCH Criteria

```bash
# Search all messages
c SEARCH ALL

# Search unseen messages
c SEARCH UNSEEN

# Search by sender
c SEARCH FROM "sender@example.com"

# Search by subject
c SEARCH SUBJECT "keyword"

# Search by date
c SEARCH SINCE 01-Jan-2024

# Combined search
c SEARCH UNSEEN FROM "sender@example.com" SINCE 01-Jan-2024
```

### FETCH Items

```bash
# Fetch flags only
d FETCH 1 FLAGS

# Fetch envelope (parsed headers)
d FETCH 1 ENVELOPE

# Fetch specific headers
d FETCH 1 BODY[HEADER.FIELDS (FROM TO SUBJECT DATE)]

# Fetch full message
d FETCH 1 BODY[]

# Fetch by UID
d UID FETCH 1 (FLAGS BODY[])
```

### STORE Flags

```bash
# Mark as seen
e STORE 1 +FLAGS (\Seen)

# Mark as deleted
e STORE 1 +FLAGS (\Deleted)

# Remove flag
e STORE 1 -FLAGS (\Seen)

# Replace all flags
e STORE 1 FLAGS (\Seen \Flagged)
```

## TLS Certificate

IMAP requires a valid TLS certificate for the hostname:

```bash
curl -X POST http://localhost:6336/action/world/generate_acme_tls_certificate \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "attributes": {
      "hostname": "imap.example.com",
      "email": "admin@example.com"
    }
  }'
```

## Troubleshooting

### Connection Refused

1. Check IMAP is enabled in config
2. Verify port is not blocked by firewall
3. Restart Daptin after config changes

### Authentication Failed

1. Verify mail account exists
2. Check username (full email address)
3. Verify password

### Certificate Error

1. Ensure certificate exists for hostname
2. Check certificate validity dates
3. Import self-signed cert to client (if applicable)

### Check IMAP Status

```bash
# Test connection with OpenSSL
openssl s_client -connect imap.example.com:993

# Test with telnet (non-SSL)
telnet imap.example.com 143
```

## Security

- TLS required for authentication (AllowInsecureAuth: false)
- Password stored securely
- Supports IDLE extension for real-time updates
