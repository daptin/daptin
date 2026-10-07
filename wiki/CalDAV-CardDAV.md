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

The action sets that group's permission on the collection and every event
currently in it in one transaction. `GroupExecute` applies only to the
collection; the other group bits apply to its current events. CalDAV `PUT`
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
individual account requires placing that account in an appropriate persisted
`usergroup`; the action accepts a group reference ID. See [[Permissions]] and
[[Relationships]] for the general resource and relationship workflow.

The `name` field is the collection's URL segment. Do not change it to rename
the displayed calendar: existing object paths contain that segment. DAV
display-name mutation and collection renaming are unsupported.

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

`COPY`, `MOVE`, and collection property mutation are not currently implemented
and return an explicit unsupported response. Scheduling and CalDAV/CardDAV sync
tokens are not implemented. Collection grants do not propagate to events
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
