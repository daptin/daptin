# Production Mail Delivery

This page describes outbound production email with Daptin `mail.send`, DKIM,
DNS, cloud-backed outbox storage, immediate delivery, and retries.

`mail.send` and `aws.mail.send` are action performers. They are used from
custom action `OutFields`; they are not standalone REST endpoints.

## Delivery Model

`mail.send` creates an `outbox` row for each recipient. By default that row is
queued and later processed by the scheduled `process_outbox` task.

For login, OTP, and password reset flows, set `send_immediately: true` or
`attempt_delivery: true` to attempt delivery before the action returns:

```yaml
OutFields:
  - Type: mail.send
    Method: EXECUTE
    Attributes:
      from: "login@example.com"
      to: "![email]"
      subject: "Your sign-in code"
      body: "~body"
      mail_server_hostname: "mail.example.com"
      send_immediately: true
```

Immediate delivery still uses the outbox:

1. `mail.send` creates an `outbox` row.
2. The row is committed before SMTP delivery begins.
3. If `outbox.mail` is cloud-store-backed, Daptin reloads the committed row
   with `mail` included so the `.eml` content is hydrated.
4. SMTP delivery runs without holding a database transaction open.
5. On success, `sent=true` stops future retries.
6. On failure, the row remains pending and `retry_count`, `last_error`, and
   `next_retry_at` are updated for scheduled retry.

The scheduled `process_outbox` task retries rows where `sent=false`,
`retry_count < 5`, and `next_retry_at` is due.

## Hostnames And Domains

Separate these names explicitly:

| Name | Example | Purpose |
|------|---------|---------|
| SMTP host | `mail.example.com` | Server identity, MX target, PTR target, `mail_server.hostname` |
| Visible sender | `login@example.com` | `From` address shown to recipients |
| DKIM domain | `example.com` | Domain in the DKIM `d=` value |

When `mail_server_hostname` is set, Daptin looks up that configured mail server
but signs the outgoing mail with the domain from the `From` address.

Example:

```yaml
from: "login@example.com"
mail_server_hostname: "mail.example.com"
```

This requires a Daptin certificate/private key for `example.com` because DKIM
signing uses the `From` domain. The SMTP host may still be `mail.example.com`.

## DNS Checklist

For direct outbound SMTP, configure DNS for both the SMTP host and the visible
sender domain.

SMTP host:

- `mail.example.com A <server-ip>`
- `example.com MX 10 mail.example.com`
- Reverse DNS/PTR for `<server-ip>` points to `mail.example.com`
- Forward-confirmed PTR: `mail.example.com` resolves back to the same IP

Sender domain:

- SPF authorizes the sending IP, for example:
  `example.com TXT "v=spf1 ip4:<server-ip> -all"`
- DKIM record for the signing domain:
  `d1._domainkey.example.com TXT "v=DKIM1; k=rsa; p=<public-key>"`
- DMARC policy aligned with the `From` domain, for example:
  `_dmarc.example.com TXT "v=DMARC1; p=quarantine; rua=mailto:dmarc@example.com"`

Gmail and other large providers may still reject technically valid mail from a
new or low-reputation IP. PTR, SPF, DKIM, and DMARC are necessary but not always
sufficient for inbox placement.

## Cloud-Backed Outbox Mail

The built-in `outbox.mail` column may be backed by a `cloud_store`:

```yaml
Tables:
  - TableName: outbox
    Columns:
      - Name: mail
        ColumnName: mail
        DataType: blob
        ColumnType: gzip
        IsForeignKey: true
        ForeignKeyData:
          DataSource: cloud_store
          Namespace: mail-storage
          KeyName: outbox-messages
```

When this is configured, Daptin stores the raw RFC 822 message as a
`message/rfc822` `.eml` object in the selected cloud store. SQL keeps the
delivery metadata. API reads need `included_relations=mail` to include file
contents:

```bash
curl "http://localhost:6336/api/outbox/$OUTBOX_ID?included_relations=mail" \
  -H "Authorization: Bearer $TOKEN"
```

## Inspecting The Outbox

Use the API:

```bash
curl "http://localhost:6336/api/outbox?query=[{\"column\":\"sent\",\"operator\":\"is\",\"value\":false}]" \
  -H "Authorization: Bearer $TOKEN"
```

Useful SQL during incident response:

```sql
SELECT id, from_address, to_address, sent, retry_count, next_retry_at, last_error
FROM outbox
ORDER BY id DESC
LIMIT 20;
```

To stop retrying a known stale row, mark it sent or delete it intentionally:

```sql
UPDATE outbox SET sent = 1 WHERE id = <id>;
```

Do not bulk-delete pending rows during an incident unless you have confirmed
they are not valid customer mail.

## Password Reset And OTP

The built-in `reset-password` action uses `otp.generate` followed by
`mail.send` with `send_immediately: true`.

The legacy/internal `password.reset.begin` performer stores mail through
`TaskSaveMail` in the local Daptin mailbox path. These are different flows, so
check which action your application is invoking before debugging delivery.

## SMTP Testing

Port `465` is implicit TLS. Use `openssl s_client`, not plaintext `nc`:

```bash
openssl s_client -connect localhost:465 -servername mail.example.com -quiet
```

For plaintext plus STARTTLS, use port `587` and issue `STARTTLS` before SMTP
AUTH.

## Troubleshooting

| Symptom | Check |
|---------|-------|
| Action returns but no mail arrives | Check `outbox.sent`, `retry_count`, `last_error`, and Daptin logs |
| DKIM lookup fails | Confirm record is under the `From` domain, for example `d1._domainkey.example.com` |
| Gmail rejects direct mail | Check PTR, forward-confirmed PTR, SPF, DKIM, DMARC, port 25 policy, and IP reputation |
| Duplicate OTP mail | Look for old `sent=false` rows retried by `process_outbox` |
| Cloud-backed outbox read fails | Read with `included_relations=mail` and verify cloud-store access |
