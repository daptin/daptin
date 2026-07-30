# Production Mail Delivery

This page describes outbound production email with Daptin `mail.send`, DKIM,
DNS, cloud-backed outbox storage, immediate delivery, and retries.

`mail.send` and `aws.mail.send` are action performers. They are used from
custom action `OutFields`; they are not standalone REST endpoints.

## Delivery model

`mail.send` requires the `from` address to match a configured
`mail_account.username`. It creates one `Sent` mailbox copy for that sender at
queue time, then creates an `outbox` row for each recipient using a configured
`mail_server`. By default outbox rows are queued and later processed by the
scheduled `process_outbox` task.

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

1. `mail.send` resolves the action `mail_server_hostname` or backend
   `mail.default_server_hostname` to a `mail_server` row and creates an
   `outbox` row with `mail_server_id`.
2. `mail.send` appends one message to the sender's `Sent` mailbox.
3. The row is committed before SMTP delivery begins.
4. If `outbox.mail` is cloud-store-backed, Daptin reloads the committed row
   with `mail` included so the `.eml` content is hydrated.
5. `process_outbox` uses `mail_server.hostname` as the SMTP EHLO identity and
   the recipient domain only for MX lookup.
6. SMTP delivery runs without holding a database transaction open.
7. On success, `sent=true` stops future retries.
8. On failure, the row remains pending and `retry_count`, `last_error`, and
   `next_retry_at` are updated for scheduled retry.

The scheduled `process_outbox` task retries rows where `sent=false`,
`retry_count < 5`, and `next_retry_at` is due.
Outbox retries do not create more `Sent` rows because the mailbox copy is
created before delivery attempts begin.

## Hostnames and domains

| Name | Example | Purpose |
|------|---------|---------|
| SMTP host | `mail.example.com` | Server identity, EHLO name, PTR target, `mail_server.hostname` |
| Visible sender | `login@example.com` | `From` address shown to recipients |
| DKIM domain | `example.com` | Domain in the DKIM `d=` value |

`mail.send` resolves a configured mail server by hostname. Use
`mail_server_hostname` in the action attributes, or set backend config
`mail.default_server_hostname` as the fallback for actions that do not specify
one. Daptin stores the selected row on `outbox.mail_server_id` and uses
`mail_server.hostname` as the SMTP EHLO identity for immediate delivery and
scheduled retries. Daptin signs the outgoing mail with the domain from the
`From` address. The `From` address must exist as a Daptin `mail_account`
username so Daptin can append the sender's `Sent` mailbox copy.

Example:

```yaml
from: "login@example.com"
mail_server_hostname: "mail.example.com"
```

This requires a Daptin certificate/private key for `example.com` because DKIM
signing uses the `From` domain. The SMTP EHLO host is `mail.example.com`.

## DNS checklist

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

Large providers may still reject technically valid mail from a new or
low-reputation IP. PTR, SPF, DKIM, and DMARC are necessary but not always
sufficient for inbox placement.

## Cloud-backed outbox mail

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

## Inspecting the outbox

```sql
SELECT id, from_address, to_address, mail_server_id, sent, retry_count, next_retry_at, last_error
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

## Password reset and OTP

The schema-managed `user_account/reset-password` and
`user_account/reset-password-verify` actions are basic starter examples built
with the same Daptin action framework available to applications. They are not
intended to encode a production deployment's mail identity, templates, or
password-recovery policy.

Both starter actions currently set `from: no-reply@localhost` directly in
their `mail.send` outcome. `mail.default_server_hostname` can select the mail
server used for delivery, but it does **not** replace that `from` address.
Production deployments should not provision a fake localhost mailbox or
certificate to accommodate the example.

Instead, create an application-owned action based on the starter sequence and
set its sender and mail server in server-managed action attributes. Do not add
`from` or `mail_server_hostname` as guest-supplied action inputs.

For example, a production verification action can use this shape:

```yaml
Actions:
  - Name: verify_example_password_reset
    Label: Verify password reset
    OnType: user_account
    InstanceOptional: true
    InFields:
      - Name: email
        ColumnName: email
        ColumnType: email
        IsNullable: false
      - Name: otp
        ColumnName: otp
        ColumnType: value
        IsNullable: false
    Validations:
      - ColumnName: email
        Tags: email
    Conformations:
      - ColumnName: email
        Tags: email
    OutFields:
      - Type: user_account
        Method: GET
        Reference: user
        SkipInResponse: true
        Attributes:
          query: '[{"column":"email","operator":"is","value":"$email"}]'
      - Type: otp.login.verify
        Method: EXECUTE
        Attributes:
          otp: "~otp"
          email: "~email"
          purpose: password_reset
      - Type: random.generate
        Method: EXECUTE
        Reference: newPassword
        SkipInResponse: true
        Attributes:
          type: password
      - Type: user_account
        Method: PATCH
        SkipInResponse: true
        Attributes:
          reference_id: "$user[0].reference_id"
          password: "!newPassword.value"
      - Type: mail.send
        Method: EXECUTE
        SkipInResponse: true
        Attributes:
          from: "login@example.com"
          to: "~email"
          subject: "Your password was reset"
          body: "Your new password is: $newPassword.value"
          mail_server_hostname: "mail.example.com"
          send_immediately: true
```

This example intentionally preserves the starter behavior of generating and
mailing a password. Applications may instead let the user choose a new password
after verification, use a one-time continuation token, render a template, or
add application-specific auditing. Review guest execution permission carefully
and keep OTP verification before any password change.

The related starter request action follows `otp.generate` with `mail.send` and
should be customized in the same way so both halves of the flow use the same
production identity and policy.

The legacy/internal `password.reset.begin` performer stores mail through
`TaskSaveMail` in the local Daptin mailbox path. These are different flows, so
check which action your application is invoking before debugging delivery.

## SMTP testing

Port `465` is implicit TLS. Use `openssl s_client`, not plaintext `nc`:

```bash
openssl s_client -connect localhost:465 -servername mail.example.com -quiet
```

For plaintext plus STARTTLS, use port `587` and issue `STARTTLS` before SMTP
AUTH.
