# Browser Mail contracts

Browser clients reuse Daptin `mail_account`, `mail_box`, `mail`, MIME assets and
outbox resources. Public resource references and IMAP mailbox UIDs are distinct.
Do not use a UID as a JSON:API resource reference.

## Reading a message

Authenticated clients invoke the built-in action:

```http
POST /action/mail/unpack
Authorization: Bearer <token>
Content-Type: application/json

{"attributes":{"message_ref":"<mail reference_id>"}}
```

The action returns one `mail.unpack` response containing `reference_id`,
`revision` (SHA-256 of original MIME), `preferred_text_body`,
`optional_html_body`, and `parts`. Part fields and bounds follow
[Email Actions](Email-Actions.md). The action checks message, mailbox and
account table/row access before resolving MIME. It reloads the active account's
persisted memberships; temporary action execution rights do not grant access
to private mail or attachments. Revocation applies to each subsequent call.

`POST /action/mail/source` accepts the same attributes and performs the same
checks. Its `mail.source` response contains `reference_id`, `revision`, and
`raw_base64` for the bounded original RFC822 bytes. Render source as text or
download it; never insert it into an application DOM as HTML.

These named browser actions are distinct from the existing `mail.unpack`
performer, which parses trusted action-provided bytes and remains reusable by
custom action compositions. Do not expose arbitrary MIME asset references as
browser action inputs.

## Discovery, metadata and status

Use authenticated JSON:API sparse projections on `mail_account` and `mail_box`.
Filter folders by `mail_account_id`, then messages by `mail_box_id`; retain
public references as identity. Keep `mail` and `body` out of message-list
projections. Paginate rather than retrieving MIME to compute counts.

`POST /action/mail_box/status` accepts `mail_account_id`, optional
`mail_box_id`, `page_size` and `page_number`. It returns `mail_box.status`
with folder counts and UID state. Resource read permission and containing
account access are enforced using persisted active-user memberships.

SQL-like `contains` filters use explicit patterns, for example `%invoice%`.
Always retain the mailbox filter when adding search terms. Server permission
checks remain authoritative; client filtering is not access control.

Sanitize and isolate rendered email, deny automatic remote content, and
reauthorize downloads instead of treating cached decoded parts as enduring
authorization. Browser body-editing libraries do not implement durable draft,
sender-authority, MIME submission or delivery contracts.

## Durable draft operations

The authenticated `mail_account` actions `draft_save`, `draft_status` and
`draft_delete` accept `attributes.draft`. They require the active user to own
the mail account and hold its read/update permissions. Action executor
privileges do not grant ownership. Account provisioners must grant the owner
appropriate write rights; a read-only account remains read-only.

```json
{
  "attributes": {
    "draft": {
      "account_ref": "<mail_account reference_id>",
      "compose_key": "<stable UUID for this compose session>",
      "write_key": "<UUID for this particular save attempt>",
      "draft_ref": "<omit on create; canonical mail reference on update>",
      "expected_revision": "<omit on create; MIME SHA-256 on update>",
      "content": {
        "to": [{"name": "Recipient", "address": "recipient@example.test"}],
        "cc": [], "bcc": [], "subject": "Hello",
        "text": "Plain alternative", "html": "<p>HTML alternative</p>",
        "in_reply_to": "", "references": [], "parts": []
      }
    }
  }
}
```

Each part carries `filename`, `media_type`, optional `content_id`, and Base64
bytes in `base64`. CID inline images are bounded PNG/JPEG/GIF/WebP parts and
must match HTML image references. Blob/data preview URLs must be converted to
CID parts before saving. The server reads no caller-provided attachment paths
or URLs. SMTPUTF8 local parts, header injection and oversized bodies/parts are
rejected. Sender identity comes from the canonical account, not the request.

`draft_save` returns `mail.draft` with the canonical reference, compose/write
keys, SHA-256 revision and UID. One account transaction owns mailbox creation,
UID allocation and the normal resource lifecycle. Updates preserve identity
and UID. Repeating the same write key and identical content returns its
already-persisted result; stale revisions or a reused key with changed content
conflict. Native APPEND and Sent insertion use the same append routine.

After a lost response, stop subsequent autosaves and call `draft_status` with
account/compose key. Its `mail.draft.status` response identifies whether the
compose record exists, its last write key, current revision and draft state.
Reconcile that result instead of blindly creating another draft.

`draft_delete` requires the current revision and delete permission; its
`mail.draft.deleted` response confirms discard. Repeating an already-completed
discard under the authorized account is idempotent. These operations persist
draft MIME including Bcc for native-client editing. Outgoing serialization
omits the Bcc header and supplies Bcc recipients only through the separate
delivery envelope. Draft persistence is not a send or delivery acknowledgment.

An unclaimed native draft can be adopted by `draft_save` using its canonical
`draft_ref`, current MIME revision, and a new compose/write key. The server
checks that it is still a writable draft and has no other compose identity.
Updates retain the current mailbox, UID, internal date and native flags; they
do not silently move the message to the default Draft folder. Resource version
checks protect changes made while MIME is being prepared.

## Submitting and reconciling a saved draft

`POST /action/mail_account/submit` accepts this strict input:

```json
{"attributes":{"submission":{
  "account_ref":"<owned account reference>",
  "compose_key":"<saved draft compose UUID>",
  "submission_key":"<UUID for this deliberate submission>",
  "expected_revision":"<saved MIME SHA-256>"
}}}
```

The sender is the account's canonical username. The action requires account
ownership/read/update, draft update and Sent mailbox write rights, and valid
signing configuration. It submits the saved revision, not request-supplied
MIME. The original multipart body, attachment bytes, CID relationships and
reply headers remain intact. From, Date and Message-ID are set server-side;
Bcc and inherited transport/signature headers are removed before signing.
To/Cc/Bcc form a separate, deduplicated envelope. The same DKIM signer and
owner-scoped resource queue are used by the existing `mail.send` performer.

One transaction moves the canonical message to Sent with a newly allocated
Sent UID and creates each outbox recipient linked by `sent_mail_id`. The mail
resource reference and compose identity remain stable. The response is
`mail.submission`, with `submitted`, `reference_id`, `compose_key`,
`submission_key`, `draft_revision` and `recipients`. Repeating the same
submission key and original draft revision returns the original result.
Changing the key/revision after submission conflicts; submitted mail can no
longer be saved or discarded as a draft. Queueing is not SMTP acceptance.

After a lost response, stop dispatching submissions and call
`POST /action/mail_account/submission_status` with
`attributes.submission.account_ref` and `compose_key`. It returns the same
response type with `submitted:false` if no submission is recorded, or the
attributed result. A different key/revision requires reconciliation; do not
silently create another compose operation and send again.

Each recipient has a canonical outbox `reference_id`, `address` and `state`:
`queued`, `retrying`, `smtp_accepted`, `uncertain` or `failed`. Status reads only
bounded queue metadata, never per-recipient MIME. `smtp_accepted` means that
SMTP accepted the message, not that the destination user received or read it.
Known failures use the existing bounded retry schedule. Before SMTP begins,
the queue durably disables automatic retry and records an uncertainty marker.
A confirmed result replaces that marker; a process crash or failed state
commit leaves it uncertain even after its live lease disappears. Operator
reconciliation is required before deliberately retrying an uncertain attempt.
Each outbound attempt has a 30-second network deadline covering DNS, connection,
greeting, STARTTLS, commands and DATA. Deadline expiry closes the owned SMTP
connection; an unresolved final DATA acknowledgment remains uncertain.
The built-in `__daptin_outbox_process` system task runs this same action every
five minutes, using the persisted administrator identity. On a fresh server
started before the first administrator existed, restart after owner setup so
system tasks can be installed; then verify the active task through the normal
`task` resource. An administrator can invoke `outbox/process_outbox` for an
immediate processing pass. Configuration and external mail-server reachability
still determine whether delivery can succeed.

Retry policy is explicit: confirmed failures get at most five attempts with
2/4/8/16-minute minimum backoff before the remaining attempts; the five-minute
task cadence may make them later. `failed` is terminal for that queue record.
An uncertain attempt is never automatically retried, including at another MX
or after restart. Review server/recipient evidence before deliberately creating
another message; do not reset queue flags as a recovery shortcut. Office's
“Retry original request” recovers an idempotent HTTP operation and does not
start a new SMTP delivery attempt for an already submitted message.

The browser submit action only queues; it never bypasses the configured outbox
processor or accepts caller-controlled delivery routing.


The built-in `mail` and `mail_box` definitions default new records to private
owner CRUD/execute rights. These defaults apply to the shared native/web
creation paths after a restart as well as initial startup. Existing row grants
and explicit schema/access policy still control access; neither invoking a
browser action nor using an editor grants mailbox write permission. CardDAV
suggestions additionally require the administrator to enable CalDAV/CardDAV
(`caldav.enable`) and provision the Contacts resource permissions independently.


Draft status includes `write_revision`, the MIME SHA-256 produced by its last
browser write, separately from the current `revision`. Confirm an uncertain save
only when both the write key and MIME revision match. Native-deleted drafts are
not reported as active drafts. A different/missing write revision requires an
explicit reload/conflict decision; repeating the same key cannot acknowledge a
message subsequently changed by another resource/protocol writer. Older records
without this attribution can be read and saved with a fresh key/current revision.

Queued recipient records expose private sender read/reference rights. The sender
cannot edit signed MIME, sender/envelope fields or queue state through generic
outbox PATCH; use the attributed submission/status boundary and administrator
queue operations instead. This keeps canonical ownership without granting a raw
send bypass around structured sender authorization.

## Message and folder operations

Authenticated account owners invoke `mail_account` actions using public
references. These actions reload persisted permissions, require account and
row operation grants, and share the resource helpers used by native mail.

`message_state` accepts `attributes.message` with `account_ref` and
`message_ref`. Its `mail.message` response contains `exists`, `reference_id`,
`mail_box_id`, `version`, mailbox `uid`, `flags`, `seen`, `flagged` and `deleted`.
A missing message returns `exists: false`. State is for reconciliation, not
permission to repeat a write automatically.

`message_flags`, `message_move`, `message_archive`, `message_trash` and
`message_delete` accept the same object plus `expected_version` and
`expected_box_ref`. Flags additionally accept optional `seen` and `flagged`
booleans; move requires `destination_ref`. Stale versions or mailbox identity
return 409. Moves stay within the account, preserve canonical message identity
and original MIME, and allocate a destination UID. Same-folder moves do not
allocate a new UID. Archive and Trash are created as needed. Permanent deletion
requires Trash and delete permission. A linked queued or uncertain SMTP attempt
prevents deletion; deleting its attribution record cannot resolve delivery.

`folder_create`, `folder_rename` and `folder_delete` accept `attributes.folder`
with `account_ref`. Create requires `name`; rename additionally requires
`folder_ref` and `expected_version`; delete requires those preconditions and an
empty folder. Their `mail.folder` responses contain `exists`, `reference_id`,
`name`, `version` and `uidvalidity`, or `exists: false` after deletion. Slash
hierarchies create missing parents; renaming a parent renames descendants.
Deleting an empty parent retains its descendants. Office system folders cannot
be renamed or deleted through these browser actions. Recreated folders receive
a new durable UIDVALIDITY generation even within the same second.

After a network failure, refresh state or folder discovery and inspect the
result before choosing another operation. Do not blindly retry a move, rename
or deletion. No operation above invokes SMTP.

## Native IMAP agreement

Native reads reload persisted memberships and check mailbox/account grants on
every operation, including an already selected mailbox. FETCH/SEARCH/COPY use
bounded normal resource reads so current message permissions apply before MIME
assets are hydrated. A revoked PEEK is denied just like a body-modifying read;
STATUS and LIST do not retain a grant from login. Subscription changes require
current update rights, and LSUB filters unsubscribed folders.

Native APPEND and COPY use the shared append/UID/resource lifecycle. COPY keeps
MIME, flags and internal date but creates a distinct canonical reference and no
browser compose/submission identity. APPEND is bounded to 25 MiB before literal
transfer and in the shared append routine. The advertised folder delimiter is
`/`, matching browser folder hierarchy names. Empty TEXT bodies remain valid
FETCH results. Native EXPUNGE uses the same linked-delivery deletion checks as
browser permanent deletion.

The executable regression exercises a real loopback STARTTLS connection,
resource/action HTTP interoperability and same-connection revocation. See
[the test workflow](../scripts/testing/GO_TESTS.md#browser-mail-protocol-acceptance).
It does not certify a desktop mail-client UI or SMTP delivery.

### Outbound SMTP port

The trusted backend setting `outbox.smtp_port` selects the destination port for
normal MX delivery (default `25`; valid range `1`–`65535`). Administrators can
set it through `POST /_config/backend/outbox.smtp_port` with a plain integer
body. It is never accepted from compose/submission input. Invalid settings fail
before an outbox claim, preserving queued work. The loopback SMTP regression
uses an ephemeral unprivileged port with `localhost` recipients and the same
sender, signing and durable uncertainty paths as normal delivery.
