# CalDAV and CardDAV

Daptin exposes authenticated, user-isolated CalDAV and CardDAV services backed
by normal Daptin resources.

- CalDAV stores collections in `collection` and objects in `calendar`.
- CardDAV stores collections in `address_book` and objects in `contact`.
- The authenticated `user_account` owns every collection and object.
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

Both CalDAV and CardDAV endpoints are enabled by this setting.

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

These are protocol URLs, not authorization input. Daptin derives the permitted
principal from the authenticated `SessionUser`. Requesting another user's URL
returns HTTP 403.

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
requests. Calendar data is parsed and validated before it is stored.

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

## Supported behavior

- standards-based current-user-principal and home-set discovery;
- separate CalDAV calendars and CardDAV address books;
- `OPTIONS`, `PROPFIND`, `REPORT`, `MKCOL`, `GET`, `HEAD`, `PUT`, and `DELETE`;
- calendar-query/calendar-multiget and addressbook-query/addressbook-multiget;
- stable content ETags and conditional PUT protection;
- per-user ownership and HTTP 403 cross-principal denial;
- durable SQL-backed storage through Daptin resources.

`COPY`, `MOVE`, and collection property mutation are not currently implemented
and return an explicit unsupported response. Scheduling, free/busy,
CalDAV/CardDAV sync tokens, and shared-calendar delegation are also not
implemented.

## Troubleshooting

### HTTP 401

The credentials are missing or invalid. Use a bearer token or the Daptin
account's email and password with Basic authentication.

### HTTP 403 on a principal path

The URL belongs to another Daptin account. Start discovery at `/caldav/` or
`/carddav/` while authenticated as the intended user; do not copy another
user's discovered URL.

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
