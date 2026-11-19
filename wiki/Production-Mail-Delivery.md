# Production Mail Delivery

This page describes outbound production email with Daptin `mail.send`, DKIM,
DNS, cloud-backed outbox storage, immediate delivery, and retries.

`mail.send` and `aws.mail.send` are action performers. They are used from
custom action `OutFields`; they are not standalone REST endpoints.

## Setup Order

For `login@example.com` sent through `mail.example.com`:

1. Provision a TLS certificate for `mail.example.com` through
   [[TLS-Certificates]]. Use a certificate trusted by SMTP clients for production
   submission; a self-signed certificate requires explicit client trust.
2. Create the `mail_server` and the `login@example.com` mail account, related
   through `mail_server_id`, following [[SMTP-Server#complete-setup-guide|SMTP setup]].
   An implicit TLS listener on port `465` requires `always_on_tls=true`.
3. Provision a separate `certificate` with `hostname=example.com` and an RSA
   private key for DKIM. Use the certificate resource and generation actions in
   [[TLS-Certificates]]. A self-signed certificate can supply this signing key:
   DKIM verification uses the DNS public key rather than the certificate's issuer.
4. Publish the SMTP host and sender-domain DNS records described below. Extract
   the DKIM public key from the `example.com` certificate, not the SMTP host's
   certificate.
5. If using cloud-backed message storage, configure the cloud store and column
   relationships before sending; see [[Cloud-Storage]] and the outbox example
   below. `mail.mail` stores mailbox messages; `outbox.mail` stores delivery
   payloads. Configure each column that should use cloud storage.
6. Define the sending action with `send_immediately: true`, verify the active
   outbox task, then check the outbox state and recipient SMTP result.

## Delivery Model

`mail.send` requires the `from` address to match a configured
`mail_account.username`. The action controls who may invoke the workflow;
the sender mailbox can belong to a service account. The mail account's
`mail_server_id` relationship selects the SMTP identity. The performer creates
one `Sent` mailbox copy owned by that mail account's owner at
queue time, then creates an `outbox` row for each recipient. By default outbox
rows are queued and later processed by the scheduled `process_outbox` task.

Authenticated SMTP relay requires the envelope sender to
match the authenticated mail account. A mismatch is rejected before DKIM key
access, Sent-mail storage, or outbox creation.

For login, OTP, and password reset flows, set `send_immediately: true` or
`attempt_delivery: true` to start a background delivery attempt after queueing,
without waiting for the scheduled task:

```yaml
OutFields:
  - Type: mail.send
    Method: EXECUTE
    Attributes:
      from: "login@example.com"
      to: "![email]"
      subject: "Your sign-in code"
      body: "~body"
      send_immediately: true
```

The action returns without waiting for SMTP delivery to finish. A successful
action response confirms queueing, not recipient acceptance or inbox placement.
Use one flag; if both are supplied, `send_immediately` takes precedence.

Immediate delivery still uses the outbox:

1. `mail.send` appends one message to the sender's `Sent` mailbox.
2. `mail.send` creates an `outbox` row for each recipient.
3. The row is committed before SMTP delivery begins.
4. If `outbox.mail` is cloud-store-backed, Daptin reloads the committed row
   with `mail` included so the `.eml` content is hydrated.
5. SMTP delivery runs without holding a database transaction open.
6. On success, `sent=true` stops future retries.
7. A confirmed failure updates `retry_count`, `last_error`, and `next_retry_at`
   for bounded retry. An uncertain SMTP result stops automatic retries.

Outbox retries do not create more `Sent` rows because the mailbox copy is
created before delivery attempts begin.

The built-in `__daptin_outbox_process` task runs `outbox/process_outbox` every
five minutes. Verify that the persisted `task` is present and active. See
[[Browser-Mail]] for system-task setup and administrator processing passes.

Both background delivery and scheduled processing use the same outbox processor.
A row is eligible only when `sent=false`, `retry_count < 5`, and `next_retry_at`
is null or due. Confirmed failures allow up to five attempts, with minimum
2/4/8/16-minute backoffs between attempts; the task cadence can delay them further.

Before SMTP begins, the processor commits a durable claim using `retry_count=5`
and a `last_error` starting with `SMTP delivery uncertain:`. While an attempt is
active, this prevents another worker from sending the same row. A confirmed
result replaces the claim. If the process crashes or the final SMTP DATA
acknowledgment is lost, the marker remains and automatic retries stop, even
after a restart. Review delivery evidence before deliberately sending a new
message; resetting queue flags can duplicate a message already accepted by the
recipient server.

## Hostnames And Domains

Separate these names explicitly:

| Name | Example | Purpose |
|------|---------|---------|
| SMTP host | `mail.example.com` | Server identity, MX target, PTR target, `mail_server.hostname` |
| Visible sender | `login@example.com` | `From` address shown to recipients |
| DKIM domain | `example.com` | Domain in the DKIM `d=` value |

The sender mail account's `mail_server_id` relationship selects the configured
mail server. An optional `mail_server_hostname` value only asserts that
server's hostname; it cannot override the relationship. Daptin signs outgoing
mail with the domain from the `From` address.

Example:

```yaml
from: "login@example.com"
mail_server_hostname: "mail.example.com"
```

This requires a Daptin certificate/private key for `example.com` because DKIM
signing uses the `From` domain. Provision that certificate and RSA private key
through the certificate resource and actions described in [[SMTP-Server]].
Configure the SMTP TLS certificate separately for `mail.example.com`.

The DKIM selector defaults to `d1`. If the backend setting `mail.dkim_selector`
is changed, publish the key at `<selector>._domainkey.example.com` instead.

### Extract The DKIM DNS Key

Set `DKIM_CERTIFICATE_ID` to the public reference ID of the certificate whose
`hostname` is `example.com`. With certificate read permission, extract the public
key from its certificate PEM (requires `jq` and OpenSSL):

```bash
curl --fail --silent --show-error \
  "http://localhost:6336/api/certificate/$DKIM_CERTIFICATE_ID" \
  -H "Authorization: Bearer $TOKEN" | \
  jq -er '.data.attributes.certificate_pem' | \
  openssl x509 -pubkey -noout | \
  openssl pkey -pubin -outform DER | \
  openssl base64 -A
```

Use the resulting single-line Base64 value as `p=` in the DKIM TXT record.
Publish only the public key. Replacing the signing key requires updating DNS
with the matching public key.

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

DMARC requires the visible `From` domain to align with either a passing SPF
identity or a passing DKIM signing domain. The SMTP hostname and PTR target do
not have to equal the visible sender domain. Daptin's DKIM signature uses that
sender domain directly.

Check that your hosting provider permits outbound port `25`; a working inbound
SMTP listener on `465` or `587` does not establish outbound connectivity to
recipient MX servers. The backend `outbox.smtp_port` setting defaults to `25`;
see [[Browser-Mail#outbound-smtp-port|Outbound SMTP port]] for controlled tests.

Gmail and other large providers may still reject mail from a new or
low-reputation IP. DNS and authentication do not guarantee inbox placement.
Check the exact SMTP response in `last_error` against the provider's guidance,
such as [Gmail's sender requirements](https://support.google.com/a/answer/81126).

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

Use an authenticated account with permission to read the outbox resources.
This query lists unsent rows, including exhausted and uncertain attempts:

```bash
curl --get "http://localhost:6336/api/outbox" \
  --data-urlencode 'query=[{"column":"sent","operator":"is","value":false}]' \
  -H "Authorization: Bearer $TOKEN"
```

Inspect each row using its public `reference_id` as `OUTBOX_ID`:

```bash
curl "http://localhost:6336/api/outbox/$OUTBOX_ID" \
  -H "Authorization: Bearer $TOKEN"
```

| Fields | Meaning |
|--------|---------|
| `sent=true` | Recipient SMTP server accepted the message; no further automatic attempts |
| `sent=false`, `retry_count < 5` | Queued or retryable; check `next_retry_at` and `last_error` |
| `sent=false`, `retry_count >= 5` | Automatic attempts stopped |
| `last_error` starts with `SMTP delivery uncertain:` | Active attempt or unresolved result; inspect logs before taking action |

For duplicate OTP reports, correlate the outbox references, recipients, message
content, timestamps, and SMTP logs. An old pending OTP can be delivered after a
later retry, but `sent=false` alone does not establish why multiple successful
deliveries occurred. Repeated action invocations can also create separate rows.

### Retiring Stale Mail

First confirm that the row is obsolete and preserve its diagnostic details.
Coordinate with operators to pause delivery work and let active attempts finish
before changing queue state. Editing or deleting a row cannot recall mail
already in flight.

An administrator with update permission can stop future attempts while retaining
the row and its error evidence through the resource API:

```bash
curl -X PATCH "http://localhost:6336/api/outbox/$OUTBOX_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d "{\"data\":{\"type\":\"outbox\",\"id\":\"$OUTBOX_ID\",\"attributes\":{\"retry_count\":5,\"next_retry_at\":null}}}"
```

Read the row again to verify the change before resuming processing. Keep
`sent=false` for undelivered mail. Do not reset retry counts on uncertain rows
or bulk-retire pending customer mail. Sender read access does not grant queue
update permission; see [[Browser-Mail]] for the queue permission boundary.

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
AUTH:

```bash
openssl s_client -starttls smtp -connect localhost:587 -servername mail.example.com -quiet
```

## Local MX Testing

Outbound `mail.send` delivery uses the recipient domain's MX records:

```text
recipient@local.test -> lookup MX for local.test -> connect to MX host on port 25
```

An `/etc/hosts` entry can make `mail.local.test` resolve to `127.0.0.1`, but it
cannot define an MX record for `local.test`. If that domain has no MX record,
Daptin can use the recipient domain's address records as an implicit MX. An
address entry for only `mail.local.test` does not resolve `local.test` itself.
For an explicit MX test with `user@local.test`, configure local DNS as below.

Small CoreDNS example:

```text
# Corefile
local.test:1053 {
  file /etc/coredns/local.test.zone
  log
  errors
}
```

```dns
; /etc/coredns/local.test.zone
$ORIGIN local.test.
@ 3600 IN SOA ns.local.test. hostmaster.local.test. 1 7200 3600 1209600 3600
@ 3600 IN NS ns.local.test.
ns 3600 IN A 127.0.0.1
mail 3600 IN A 127.0.0.1
@ 3600 IN MX 10 mail.local.test.
```

Point the test machine or container running Daptin at that DNS server, then
verify from the same network namespace as Daptin:

```bash
dig @127.0.0.1 -p 1053 MX local.test
dig @127.0.0.1 -p 1053 A mail.local.test
```

For direct outbox delivery, make sure an SMTP server is reachable at the MX host
on port `25`. Mapping only port `465` or `587` is not enough for this path.

## Troubleshooting

| Symptom | Check |
|---------|-------|
| Action returns but no mail arrives | Check `outbox.sent`, `retry_count`, `last_error`, and Daptin logs |
| Local test creates outbox rows but does not deliver | Verify MX DNS from Daptin's runtime environment and ensure the MX host accepts SMTP on port 25 |
| DKIM lookup fails | Confirm record is under the `From` domain, for example `d1._domainkey.example.com` |
| Gmail rejects direct mail | Check PTR, forward-confirmed PTR, SPF, DKIM, DMARC, port 25 policy, and IP reputation |
| Duplicate or late OTP mail | Correlate outbox references and SMTP logs; inspect retry eligibility and repeated action invocations |
| Unsent row never retries | Check the active task, retry limit, due time, and uncertainty marker |
| Cloud-backed outbox read fails | Read with `included_relations=mail` and verify cloud-store access |
