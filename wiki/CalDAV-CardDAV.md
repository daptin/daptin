# CalDAV and CardDAV

Daptin exposes authenticated CalDAV and CardDAV services backed by normal
Daptin resources. CalDAV access follows resource permissions; CardDAV remains
scoped to the authenticated account.

- CalDAV stores collections in `collection` and objects in `calendar`.
- CardDAV stores collections in `address_book` and objects in `contact`.
- A CalDAV URL identifies the calendar collection's owner, while the
  authenticated account supplies the permissions for each request. Each event
  belongs to the account that created it.
- The SQL database is durable authority. No `./storage/caldav` or
  `./storage/carddav` directories are required.
- Writes use the normal resource lifecycle. If an administrator configures a
  content column as a cloud-storage file column, DAV uses that configured
  storage through the same resource path.

## Enable DAV

DAV is disabled by default. Set `caldav.enable` as an administrator, then
restart Daptin:

```bash
TOKEN="your-admin-token"

curl -X POST "http://localhost:6336/_config/backend/caldav.enable" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: text/plain" \
  --data 'true'
```

Both CalDAV and CardDAV endpoints are enabled by this setting. Enabling an
endpoint does not grant CRUD access to its backing resource tables.

CalDAV does not change the built-in permissions of `collection` or `calendar`.
A calendar owner is not automatically a Daptin administrator. An administrator,
or another account with schema-management permission, grants table access and
chooses row defaults through the normal Daptin schema. For example, this setup
lets members of `users` create calendars and gives creators control of their
own rows:

```yaml
Tables:
  - TableName: collection
    DefaultPermission: 16256 # User CRUD and Execute
    AccessGroups:
      - Name: users
        Permission: 2080768 # Group CRUD and Execute
  - TableName: calendar
    DefaultPermission: 12160 # User CRUD
    AccessGroups:
      - Name: users
        Permission: 1556480 # Group CRUD
```

Choose the table group and rights for your installation. Row defaults apply
to newly created rows; an administrator sets permissions on existing rows
through the normal resources. See [[Permissions]] for the schema workflow.

## Client endpoints

Configure clients with the service root and the user's normal Daptin
credentials:

| Protocol | Service URL | Well-known URL |
|---|---|---|
| CalDAV | `https://api.example.com/caldav/` | `https://api.example.com/.well-known/caldav` |
| CardDAV | `https://api.example.com/carddav/` | `https://api.example.com/.well-known/carddav` |

Bearer authentication and HTTP Basic authentication are supported. A client
does not need to know the user's Daptin reference ID in advance. It obtains the
principal and home-set URLs through DAV discovery.

The discovery chain is:

```text
/caldav/
  -> /caldav/{user-reference-id}/
  -> /caldav/{user-reference-id}/calendars/
  -> /caldav/{user-reference-id}/calendars/{calendar}/

/carddav/
  -> /carddav/{user-reference-id}/
  -> /carddav/{user-reference-id}/addressbooks/
  -> /carddav/{user-reference-id}/addressbooks/{address-book}/
```

These are protocol URLs, not authorization input. Daptin derives the caller
from the authenticated `SessionUser`. A CalDAV request to another account's
calendar uses that calendar's canonical owner URL and succeeds only when the
caller has the required resource grants. CardDAV still rejects another
account's path.

## Verify discovery

Ask the CalDAV root for the authenticated principal:

```bash
curl -X PROPFIND "http://localhost:6336/caldav/" \
  -u "user@example.com:password" \
  -H "Depth: 0" \
  -H "Content-Type: application/xml" \
  --data '<?xml version="1.0"?>
<D:propfind xmlns:D="DAV:">
  <D:prop><D:current-user-principal/></D:prop>
</D:propfind>'
```

The response is HTTP 207 and contains an authenticated principal such as:

```xml
<D:current-user-principal>
  <D:href>/caldav/USER_REFERENCE_ID/</D:href>
</D:current-user-principal>
```

Ask that principal for its calendar home:

```bash
curl -X PROPFIND \
  "http://localhost:6336/caldav/USER_REFERENCE_ID/" \
  -u "user@example.com:password" \
  -H "Depth: 0" \
  -H "Content-Type: application/xml" \
  --data '<?xml version="1.0"?>
<D:propfind xmlns:D="DAV:"
            xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop><C:calendar-home-set/></D:prop>
</D:propfind>'
```

For CardDAV, use `/carddav/` and request
`<A:addressbook-home-set/>` in the
`urn:ietf:params:xml:ns:carddav` namespace.

## Calendar example

The examples below use the home-set URL returned by discovery:

```bash
CALENDAR_HOME="http://localhost:6336/caldav/USER_REFERENCE_ID/calendars"

curl -X MKCOL "$CALENDAR_HOME/personal/" \
  -u "user@example.com:password"

curl -X PUT "$CALENDAR_HOME/personal/event.ics" \
  -u "user@example.com:password" \
  -H "Content-Type: text/calendar" \
  --data-binary $'BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Daptin//EN\r\nBEGIN:VEVENT\r\nUID:event-1\r\nDTSTAMP:20260921T120000Z\r\nDTSTART:20260922T120000Z\r\nDTEND:20260922T130000Z\r\nSUMMARY:Team meeting\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n'

curl "$CALENDAR_HOME/personal/event.ics" \
  -u "user@example.com:password"
```

Calendar collections support `calendar-query` and `calendar-multiget` REPORT
requests. A time-range query returns a recurring event when any occurrence
overlaps the range, including one that began before it. The response retains
the complete iCalendar object and its ETag. Calendar data is parsed and
validated before it is stored.

## Address-book example

```bash
ADDRESSBOOK_HOME="http://localhost:6336/carddav/USER_REFERENCE_ID/addressbooks"

curl -X MKCOL "$ADDRESSBOOK_HOME/contacts/" \
  -u "user@example.com:password"

curl -X PUT "$ADDRESSBOOK_HOME/contacts/person.vcf" \
  -u "user@example.com:password" \
  -H "Content-Type: text/vcard" \
  --data-binary $'BEGIN:VCARD\r\nVERSION:3.0\r\nUID:person-1\r\nFN:Test Person\r\nEMAIL:test@example.com\r\nEND:VCARD\r\n'

curl "$ADDRESSBOOK_HOME/contacts/person.vcf" \
  -u "user@example.com:password"
```

Address books support `addressbook-query` and `addressbook-multiget` REPORT
requests. vCard data is parsed and validated before it is stored.

## Conflict-safe updates

DAV objects return an `ETag`. Use it with `If-Match` when updating an existing
object:

```bash
ETAG=$(curl -sSI "$CALENDAR_HOME/personal/event.ics" \
  -u "user@example.com:password" |
  awk 'tolower($1) == "etag:" {gsub(/\r/, "", $2); print $2}')

curl -X PUT "$CALENDAR_HOME/personal/event.ics" \
  -u "user@example.com:password" \
  -H "Content-Type: text/calendar" \
  -H "If-Match: $ETAG" \
  --data-binary @event.ics
```

A stale or incorrect `If-Match`, or a matching `If-None-Match`, returns HTTP
412 without overwriting the stored object.
Use `If-None-Match: *` when creating an object; if another client creates the
same path first, the request returns HTTP 412.

Conditional object DELETE also checks `If-Match` and `If-None-Match` against
the current content ETag. A failed condition returns HTTP 412 and leaves the
object in place.

## Collection metadata

DAV `PROPPATCH` is unsupported: CalDAV returns HTTP 501, while CardDAV reports
HTTP 405 for the property in a 207 response. Clients that need to change a
calendar or address-book description use the normal JSON:API resource:
`collection` for a calendar and `address_book` for an address book. After
obtaining a calendar's public `reference_id` from `/api/collection`:

```bash
curl -X PATCH "http://localhost:6336/api/collection/COLLECTION_REFERENCE_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  --data '{"data":{"type":"collection","id":"COLLECTION_REFERENCE_ID","attributes":{"description":"Work calendar"}}}'
```

The caller needs update permission on both the `collection` table and that
collection row. The schema chosen by the administrator determines whether an
owner receives it.
The updated description is returned by DAV `PROPFIND`. CardDAV uses the same
contract with `address_book` in place of `collection`; its owner does not
receive update permission by default.

## Share a CalDAV calendar

CalDAV evaluates the authenticated account's Daptin permissions on the
collection and each event at the calendar owner's canonical URL. A collection
read grant alone does not grant event content. Free/busy uses peek permission
on the collection and contributing events, without returning event details.

Administrators choose table access, row defaults, and group membership. After
creating a `usergroup` and adding the delegate through the `user_account`
relationship, the calendar owner with `Execute`, an administrator, or a member
with `Execute` on the collection can set that group's access using the
`collection/share` action. The action uses the existing Daptin group permission
bits. For example, `32768` is `GroupRead`, `16384` is `GroupPeek`, and `1556480`
is `GroupCRUD`.
`GroupExecute` (`524288`) on the collection grants the ability to manage its
group shares. The configured table and action permissions also apply.
Granting access also requires `Refer` permission on the target `usergroup`
table and row; removing an existing grant does not.

```bash
curl -X POST "http://localhost:6336/action/collection/share" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  --data '{"attributes":{"calendar_reference_id":"COLLECTION_REFERENCE_ID","usergroup_id":"GROUP_REFERENCE_ID","permission":32768}}'
```

To share with one identified account, use `collection/share_user` with that
account's public reference ID. A calendar manager needs `Refer` access to the
target `user_account` table and row. The action creates a group for this
calendar and account, adds the account through the ordinary membership
relationship, and applies the same collection and event grants. The response
includes the group's reference ID as the share handle.

```bash
curl -X POST "http://localhost:6336/action/collection/share_user" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  --data '{"attributes":{"calendar_reference_id":"COLLECTION_REFERENCE_ID","user_account_id":"ACCOUNT_REFERENCE_ID","permission":32768}}'
```

Call `share_user` again with another permission value to change that account's
rights. Send `"permission":0` to revoke the share. Repeated calls for the same
calendar and account reuse its group. The group membership and row links are
the access authority; administrators can inspect or change them using normal
relationship resources. If membership changes, `share_user` refuses to alter
that group; use its returned group reference ID with `collection/share` to
revoke the links. Changing membership changes who receives the grant.

To decide whether to show sharing controls for a selected calendar, call the
read-only `collection/share_capabilities` action:

```bash
curl -X POST "http://localhost:6336/action/collection/share_capabilities" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  --data '{"attributes":{"calendar_reference_id":"COLLECTION_REFERENCE_ID"}}'
```

Its `can_share_group` and `can_share_user` results reflect the caller's
persisted collection management grant and the two action permissions. HTTP
403 means the caller cannot use this capability query; hide the sharing
controls. A true result does not bypass the target group or account `Refer`
check when a share is submitted. These results do not describe edit rights on
individual events, which retain their own row permissions.

For an event the caller can read, query its current edit rights with
`collection/event_capabilities`:

```bash
curl -X POST "http://localhost:6336/action/collection/event_capabilities" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  --data '{"attributes":{"calendar_reference_id":"COLLECTION_REFERENCE_ID","event_reference_id":"EVENT_REFERENCE_ID"}}'
```

`can_update` and `can_delete` reflect the collection update grant and the
`calendar` table and event row grants for those operations. They can differ
between events in one collection. HTTP 403 means the caller cannot query that
event's edit rights; it does not disclose event details to a free/busy-only
viewer. These flags describe authorization at query time; a later write still
checks the current grants and its ETag condition.

The `share` and `share_user` actions set that group's permission on the
collection and every event currently in it in one transaction. `GroupExecute`
applies only to the collection; the other group bits apply to its current
events. CalDAV `PUT`
also copies the collection's current group links to each new event in the same
transaction, including events created by a delegate. The signed-in account
remains the event row's owner. The action replaces any
existing links for the same group on those rows and leaves other groups' links
alone. Send `"permission":0` to remove those links; subsequent CalDAV and JSON:API
requests then use the remaining row permissions. Other grants may still allow
access.

For revocation to cover direct JSON:API access to delegate-created events,
configure the `calendar` row default without `UserRead` or other owner rights.
For example, `DefaultPermission: 16384` uses the existing `GroupPeek` bit and
adds no owner access. Keep table access for the intended users as shown above.
Create a separate private `usergroup` containing the calendar owner, then grant
that group `GroupCRUD` on the collection through `collection/share` before
creating events. CalDAV copies this owner group grant to new events alongside
delegate grants. Removing only the delegate group's share then leaves owner
access intact and removes the delegate's direct row access. Do not use a group
that also contains delegates as the owner group.

Changing the row default affects new events only. For existing events, first
grant the private owner group through `collection/share`, then have an
administrator update each existing `calendar` row's `permission` through the
normal resource API to remove owner bits. For example, after identifying an
event's reference ID, patch its row with
`{"data":{"type":"calendar","id":"EVENT_REFERENCE_ID","attributes":{"permission":16384}}}`.
Check owner access before revoking delegate groups. Rows with other grants may
still be accessible through those
grants. Table-level denial cannot replace this row policy: it also denies
authorized owners and delegates at the table boundary.

A `GroupPeek` grant exposes free/busy intervals without event details.
`GroupRead` permits details;
`GroupCRUD` permits edits when the corresponding table permissions allow them.
Free/busy reports accept a range of at most one year and examine at most 1,000
event objects in a collection; larger collections receive HTTP 507.

CalDAV-created events inherit collection links; events created or moved through
the JSON:API resource path follow their configured row defaults and relations.
`DefaultGroups` can grant the same group rights to all qualifying new rows,
independent of a particular calendar. The generated `usergroup_id` parent
relationship uses its configured link default and does not accept a per-link
permission.
If an event is later moved to another collection through the resource API, its
event link remains on that row; revoking the original collection's current
events will not find it.
`CLASS:PRIVATE` is iCalendar classification data; it does not change Daptin
permissions. Configure the event's row grants to keep its details private.

The delegate uses the calendar owner's canonical CalDAV URL. The delegate's
own `calendar-home-set` continues to describe calendars owned by that
delegate; it does not list other owners' calendars. A delegate can discover
readable shared collections through the normal `GET /api/collection` resource.
Each returned row includes `name` and `user_account_id`; its CalDAV URL is
`/caldav/{user_account_id}/calendars/{name}/`. A group with only peek
rights can obtain free/busy intervals through `free-busy-query` but cannot
read event content or list it through `/api/calendar`. Sharing with an
individual account uses `collection/share_user` as described above. Group
sharing uses `collection/share` and persisted membership relationships. See
[[Permissions]] and [[Relationships]] for the general resource workflow.

The `name` field is the collection's URL segment. Do not change it to rename
the displayed calendar: existing object paths contain that segment. DAV
display-name mutation and collection renaming are unsupported.

## Send invitations through a mail account

A calendar can use a Daptin `mail_account` for iCalendar email. Create the
mail account and its `mail_server` as usual, configure a signing certificate
for its domain, then set the calendar collection's optional
`scheduling_mail_account_id` relationship. The mail account remains a
standalone mail resource and may be connected to more than one calendar.
Its owner need not be the calendar owner. Editing the relationship uses the
normal collection and mail account relationship permissions.

```bash
curl -X PATCH \
  "http://localhost:6336/api/collection/COLLECTION_REFERENCE_ID/relationships/scheduling_mail_account_id" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  --data '{"data":{"type":"mail_account","id":"MAIL_ACCOUNT_REFERENCE_ID"}}'
```

On CalDAV event `PUT`, Daptin compares the event with its previous version.
When `ORGANIZER` is the connected mail account's address, it queues an iMIP
`REQUEST` for attendees. Removing an attendee or deleting an event queues
`CANCEL`. When the connected address is an attendee of an event organized
elsewhere, changing that attendee's `PARTSTAT` to `ACCEPTED`, `DECLINED`, or
`TENTATIVE` queues `REPLY` to the organizer. Each message retains the event
UID, sequence, and recurrence ID. A full cancellation carries
`STATUS:CANCELLED` and the next sequence number. `SCHEDULE-AGENT=CLIENT` or
`SCHEDULE-AGENT=NONE` on an attendee suppresses server-generated mail for
that attendee, so a client that sends its own invitations does not also get
a Daptin-generated copy. Unconnected calendars continue ordinary CalDAV
operations without sending mail.

Connected scheduling events return a `Schedule-Tag` header on `GET` and `PUT`
and a `schedule-tag` property in DAV object queries. A client may send
`If-Schedule-Tag-Match` on `PUT` or `DELETE`. A matching tag preserves other
attendees' response states when the client's ETag is stale; a stale schedule
tag fails with HTTP 412. An attendee may change their own response and local
event presentation, but cannot change organizer-controlled meeting fields.
Deleting an attendee copy sends a declined reply unless the request includes
`Schedule-Reply: F`. The normal Daptin table, collection, and event grants
still govern the write.

`cal_mail` records each inbound or outbound calendar scheduling message and
links it to the ordinary received `mail` or queued `outbox` record. The event
write, outbound `cal_mail` record, Sent copy, and `outbox` row commit
together. An identical event write does not queue another invitation. Mail
delivery happens later through Daptin's existing outbox worker. An outbound
`cal_mail` state of `submitted` means mail was queued. The outbox row is
the delivery authority. A calendar user can call
`POST /action/collection/scheduling_status` with `collection_id` and
`message_id` reference IDs to read `sent`, `retry_count`, `next_retry_at`,
and `last_error` without access to the sender's mailbox. A failed mail setup
causes the CalDAV write to fail before it commits. The worker retries failures
before SMTP DATA acceptance. An uncertain result during DATA stops automatic
retries; inspect the recipient before manually sending again. Successful DATA
acceptance is not undone by a failed SMTP QUIT. The worker delivers to the
recipient domain's MX host, or to that domain's address host when it has no MX
record. If the receiving server offers STARTTLS, its certificate must be
trusted by the Daptin server; a self-signed certificate needs an installed
trust root. Outbound delivery uses SMTP port 25.

Incoming iCalendar mail is stored in the ordinary `mail` resource. To process
new mail automatically, an administrator creates a durable data exchange with
source `mail` creation, target action `mail.process_itip`, and `as_user_id`
linked to an administrator account:

```json
{
  "name": "process calendar mail",
  "source_type": "self",
  "source_attributes": "{\"name\":\"mail\"}",
  "target_type": "action",
  "target_attributes": "{\"type\":\"mail\",\"action\":\"process_itip\",\"attributes\":{}}",
  "attributes": "{\"name\":\"mail\",\"hook\":\"after\",\"methods\":[\"post\"]}",
  "options": "{}"
}
```

Create these as attributes of `POST /api/data_exchange`, set the
`as_user_id` relationship through JSON:API, then reload the server's exchange
configuration. The normal exchange execution worker processes queued mail;
its `exchange_run` rows report retries and failures. The same administrator
action can process one stored message explicitly:

```bash
curl -X POST "http://localhost:6336/action/mail/process_itip" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  --data '{"attributes":{"mail_id":"MAIL_REFERENCE_ID"}}'
```

The processor uses the existing bounded MIME parser. A `REPLY` must match a
connected account, a previously queued invitation, its UID, recurrence ID,
sequence, and attendee address before it changes the event's `PARTSTAT`.
Reprocessing the same iTIP content, even in another mail row, has no further
effect. Stale replies do not change the event. Incoming `REQUEST` and
`CANCEL` messages create, update, or cancel the attendee's event when the
receiving mail account is connected to one calendar, or when the administrator
supplies that calendar's `collection_id`. Daptin writes the event as the
calendar owner through the normal resource path. When an account is connected
to several calendars and no calendar is selected, the message remains
`pending` in the recipient's scheduling inbox. A mail account shared by
different principals still needs an explicit `collection_id` to identify the
recipient. Ordinary non-calendar mail remains ordinary mail.

For a calendar owner with a connected mail account, a CalDAV `PROPFIND` on
`/caldav/OWNER_REFERENCE_ID/` returns `calendar-user-address-set`,
`schedule-inbox-URL`, and `schedule-outbox-URL`. The address is the connected
mail account's `username`, expressed as a `mailto:` URI. The mailbox may be
owned by another Daptin account; that does not change the calendar principal.
One mail account may serve multiple calendars of the same principal. A shared
sender serving different principals can still send invitations and route
replies by the matching outbound invitation. Its address cannot identify the
principal for a new incoming REQUEST or CANCEL; that mail remains in the
mailbox until an administrator calls `mail.process_itip` with the `mail_id`
and an explicit `collection_id`, or configures that field in a mailbox-specific
data exchange action. The action verifies that the selected collection is
linked to the receiving mail account. The selected owner can then read the
message in their scheduling inbox. Scheduling discovery remains unavailable
for an address shared across principals because it does not identify one
principal. Unconnected principals receive a missing-property response.

When a principal owns exactly one calendar and it has one connected mail
account, DAV `OPTIONS` advertises `calendar-auto-schedule`. This tells CalDAV
clients that Daptin sends invitations from event writes, so the client need
not send a second email. Configure the incoming `mail.process_itip` exchange
above to apply received requests, replies, and cancellations automatically.
With multiple calendars, an incoming request has no unique destination, so
the principal-wide automatic scheduling capability is absent; each connected
calendar can still queue mail through its own account. In Thunderbird, assign
the corresponding email identity to each subscribed calendar so an attendee
reply identifies that attendee.

The inbox URL is `/caldav/OWNER_REFERENCE_ID/schedule-inbox/`. After
`process_itip` succeeds, the owner can list incoming iTIP messages with
`PROPFIND` or `REPORT` and read each `.ics` message with `GET`. The inbox
contains the iCalendar part, not the raw mailbox message. Only the principal
owner can access it. An applied REQUEST appears as an event in the selected
calendar with the attendee awaiting a response. The attendee changes its
PARTSTAT through CalDAV PUT to queue a REPLY. Later REQUEST and CANCEL mail
updates the same event or affected recurring occurrence. A pending message
without a selected calendar can be processed again with `collection_id`;
the owner may also handle it manually. DELETE its inbox `.ics` URL after
handling it; the stored message remains for deduplication.

The outbox URL is `/caldav/OWNER_REFERENCE_ID/schedule-outbox/`. It accepts
`POST` of a `METHOD:REQUEST` `VFREEBUSY` message when its organizer matches
the connected address. For a local recipient, the response contains busy
intervals from calendars for which the requester has peek or read access;
unavailable or denied recipients receive no calendar data. Meeting invitations
continue to be queued by CalDAV event `PUT` and `DELETE`, not outbox `POST`.

The `calendar-auto-schedule` advertisement covers event writes and incoming
mail processing in the connected setup described above. Daptin also exposes
Schedule-Tag and a free/busy scheduling outbox. Other RFC 6638 outbox message
types and scheduling privileges are not implemented; meeting invitations use
CalDAV event writes rather than outbox `POST`.

## Supported behavior

- standards-based current-user-principal and home-set discovery;
- separate CalDAV calendars and CardDAV address books;
- `OPTIONS`, `PROPFIND`, `REPORT`, `MKCOL`, `GET`, `HEAD`, `PUT`, and `DELETE`;
- calendar-query/calendar-multiget and addressbook-query/addressbook-multiget;
- calendar free-busy-query REPORT with a bounded time range;
- stable content ETags and conditional object PUT/DELETE protection;
- per-row Daptin permissions for CalDAV, including direct access at an owner's
  URL when collection and event grants permit it;
- durable SQL-backed storage through Daptin resources.
- optional iCalendar email delivery through a collection's mail account;
- conditional automatic scheduling discovery for a principal with one connected calendar.

`COPY`, `MOVE`, and collection property mutation are not currently implemented
and return an explicit unsupported response. CalDAV/CardDAV sync tokens are not
implemented. Collection grants do not propagate to events
created or moved through JSON:API. The endpoint does not implement the RFC 3744 ACL method or claim
full WebDAV ACL conformance.

## Troubleshooting

### HTTP 401

The credentials are missing or invalid. Use a bearer token or the Daptin
account's email and password with Basic authentication.

### HTTP 403 on a principal path

For CalDAV, the caller lacks a required grant on the collection or event at
that owner URL. For CardDAV, the URL belongs to another account. Start
discovery at `/caldav/` or `/carddav/` for the authenticated account's own
collections.

### HTTP 404 on a collection or object

Follow the discovered home-set URL and create the collection with `MKCOL`
before uploading objects.

### HTTP 412 on PUT

The conditional request does not match current state. Fetch the current ETag,
resolve the conflict, and retry with the new value.

## See also

- [[Authentication|Authentication]]
- [[Permissions|Permissions]]
- [[Asset-Columns|Asset Columns]]
- [[Server-Configuration|Server Configuration]]
