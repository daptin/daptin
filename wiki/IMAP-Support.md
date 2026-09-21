# IMAP Support

The `/_config` API stores text exactly as submitted. Send IMAP scalar values as
raw `text/plain` without JSON quotes. JSON-valued settings remain JSON text and
are interpreted by their owning feature. See [[Configuration]] for the
type/encoding table.

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
  -H 'Content-Type: text/plain' \
  -H 'Authorization: Bearer $TOKEN' \
  --data-binary 'true'

# Set listen interface (default: :1143)
curl -X POST 'http://localhost:6336/_config/backend/imap.listen_interface' \
  -H 'Content-Type: text/plain' \
  -H 'Authorization: Bearer $TOKEN' \
  --data-binary '0.0.0.0:993'

# Set IMAP hostname independently from the backend/API hostname
curl -X POST 'http://localhost:6336/_config/backend/imap.hostname' \
  -H 'Content-Type: text/plain' \
  -H 'Authorization: Bearer $TOKEN' \
  --data-binary 'imap.example.com'
```

Read the values back before restarting and confirm that no surrounding quote
characters were stored:

```bash
curl 'http://localhost:6336/_config/backend/imap.enabled' \
  -H 'Authorization: Bearer $TOKEN'
curl 'http://localhost:6336/_config/backend/imap.listen_interface' \
  -H 'Authorization: Bearer $TOKEN'
curl 'http://localhost:6336/_config/backend/imap.hostname' \
  -H 'Authorization: Bearer $TOKEN'
```

If `imap.hostname` is not set, Daptin falls back to the legacy derived hostname
`imap.{hostname}`, where `hostname` is the global backend/API hostname.

If the IMAP address or certificate configuration is invalid, Daptin leaves
IMAP disabled for that runtime while keeping the HTTP API available. Correct
the value through `/_config` and restart Daptin.

### Restart to Apply

```bash
docker restart daptin
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
        "password": "secure-password",
        "password_md5": "secure-password"
      },
      "relationships": {
        "mail_server_id": {
          "data": {"type": "mail_server", "id": "SERVER_ID"}
        }
      }
    }
  }'
```

The creating user owns the new mail account. When an administrator provisions
mail for another user, update the `user_account_id` relationship after creation:

```bash
curl -X PATCH 'http://localhost:6336/api/mail_account/MAIL_ACCOUNT_ID/relationships/user_account_id' \
  -H 'Content-Type: application/vnd.api+json' \
  -H 'Authorization: Bearer ADMIN_TOKEN' \
  -d '{"data":{"type":"user_account","id":"USER_ID"}}'
```

Supply the same initial password to both password fields; their column
conformations store the forms used by the supported mail authentication
mechanisms.

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

IMAP adapts mailbox operations to Daptin's resource lifecycle. `APPEND` and
`COPY` create `mail` resources, flag changes update them, and `EXPUNGE` deletes
them through the same create, update, and delete paths used by the JSON:API.
`FETCH`, `STORE`, `COPY`, and the candidate rows returned by `SEARCH` are read
through the same resource-read path used by the JSON:API; IMAP sequence and UID
selection only determines the ordered public reference IDs to read.
IMAP authorization is based on the authenticated mail account and the mailbox
being operated on; changing JSON:API table permissions does not grant or revoke
mail protocol access. After that protocol authorization, validation, ownership,
relations, exchanges, events, metering, audit, and asset handling use the same
resource lifecycle as other Daptin surfaces. See
[[Data-Exchange|Data Exchange]] for the canonical execution behavior.

### Mail Deletion Lifecycle

IMAP uses a two-phase deletion model:

1. **Mark for deletion**: `STORE +FLAGS (\Deleted)` sets `deleted=true` in database
2. **Permanent removal**: `EXPUNGE` command removes marked messages

```bash
# Mark message 1 as deleted
e STORE 1 +FLAGS (\Deleted)

# Permanently remove all deleted messages
f EXPUNGE
```

**Database changes:**
- STORE: Sets `deleted=true` on mail record
- EXPUNGE: Deletes mail record and usergroup relations

### Cloud Store Backed Message Bodies

IMAP reads and writes message bodies through the `mail.mail` column. To store
message bodies in a configured cloud store, configure that built-in column the
same way as any cloud-backed asset column:

```yaml
Tables:
  - TableName: mail
    Columns:
      - Name: mail
        ColumnName: mail
        DataType: blob
        ColumnType: gzip
        IsForeignKey: true
        ForeignKeyData:
          DataSource: cloud_store
          Namespace: mail-storage
          KeyName: mail-messages
```

After the schema is applied and the server is restarted, IMAP `FETCH`, `COPY`,
`APPEND`, and `EXPUNGE` continue to use the normal mailbox behavior. The SQL
tables keep message metadata, flags, UID state, and mailbox relations. The raw
RFC 822 message body is stored as a `message/rfc822` `.eml` object in the
configured `cloud_store`.

If you enable cloud-store backing after messages already exist, IMAP can still
read existing database-backed base64 message bodies from the built-in mail
column. `COPY` writes the copied message through the current column storage
configuration, so copied messages move onto the configured cloud-store path.

To fetch the same message body through the JSON:API, include the `mail`
relation:

```bash
curl "http://localhost:6336/api/mail/$MAIL_ID?included_relations=mail" \
  -H "Authorization: Bearer $TOKEN"
```

### IDLE Extension

IMAP IDLE allows clients to receive real-time notifications without polling.

```bash
# Enter IDLE mode
a IDLE
# Server will send EXISTS/EXPUNGE notifications
# Type "DONE" to exit IDLE mode
```

**Use cases:**
- Push notifications for new mail
- Real-time folder synchronization
- Mobile app background sync

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

# Search by the stable mailbox UID rather than the current sequence number
c UID SEARCH UID 100:200
```

`SEARCH` returns the current sequence numbers in the selected mailbox. `UID
SEARCH` returns stable mailbox UIDs. Messages carrying `\Deleted` remain in
the sequence-number space and can be searched until `EXPUNGE` permanently
removes them; messages after an expunged message then receive lower sequence
numbers, while their UIDs do not change.

Daptin evaluates search criteria against persisted mail metadata in the SQL
database. This includes sequence and UID sets, flags and keywords, internal and
sent dates, sizes, `FROM`, `TO`, `CC`, `BCC`, `SUBJECT`, `Message-ID`, `BODY`,
`TEXT`, and combinations using `OR` and `NOT`. Searching another arbitrary
header returns an unsupported-search error instead of silently ignoring the
criterion. Search does not download cloud-backed RFC822 objects.

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

Create a `certificate` row for `imap.example.com`, then invoke
`POST /action/certificate/generate_acme_certificate` with that row's public
reference ID as `certificate_id`. See [[Certificate-Actions]] for the complete
request and the v0.13.14 production-only ACME limitation.

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
