# CalDAV and CardDAV Support

**Tested ✓** - 2026-01-26 with Daptin (commit cf0fb204)

Daptin provides basic CalDAV and CardDAV server functionality using WebDAV protocol for storing calendar events (.ics files) and contacts (.vcf files).

---

## Overview

CalDAV (Calendaring Extensions to WebDAV) and CardDAV (vCard Extensions to WebDAV) enable calendar and contact synchronization with standard clients.

**Important**: Daptin implements **basic WebDAV file storage** for .ics and .vcf files. It does NOT implement the full CalDAV/CardDAV specifications (no REPORT method, calendar-query, etc.). This is suitable for simple calendar/contact storage and sync but may not work with clients that require advanced CalDAV features.

### What Works

✅ **WebDAV Core Methods**:
- PROPFIND - List collections and resources
- GET - Retrieve calendar events/contacts
- PUT - Create/update events/contacts
- DELETE - Remove events/contacts
- MKCOL - Create collections (calendars/address books)
- COPY - Duplicate resources
- MOVE - Rename/move resources
- PROPPATCH - Modify properties

✅ **File Formats**:
- iCalendar (.ics) for calendar events
- vCard (.vcf) for contacts

✅ **Authentication**:
- Bearer token (JWT) via Authorization header
- Basic authentication with email/password

### What Doesn't Work

❌ **Advanced CalDAV/CardDAV Features**:
- REPORT method (calendar queries)
- Calendar-specific WebDAV properties
- Free/busy time queries
- Advanced filtering and search

---

## Configuration

### Enable CalDAV/CardDAV

CalDAV/CardDAV is **disabled by default**. Enable it via configuration:

```bash
# As admin, set the config value
TOKEN="your-admin-token"
curl -X POST "http://localhost:6336/_config/backend/caldav.enable" \
  -H "Authorization: Bearer $TOKEN" \
  -d "true"

# Verify it was set
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:6336/_config/backend/caldav.enable"
# Should return: true

# CRITICAL: Restart server for changes to take effect
pkill daptin && go run main.go
```

### Create Storage Directories

CalDAV/CardDAV stores files in `./storage/caldav/` and `./storage/carddav/`:

```bash
mkdir -p ./storage/caldav ./storage/carddav
```

Without these directories, you'll get "no such file or directory" errors.

---

## Endpoints

| Endpoint | Purpose |
|----------|---------|
| `/caldav/*` | CalDAV resources (calendar events) |
| `/carddav/*` | CardDAV resources (contacts) |

Both endpoints require authentication. Default port: 6336

---

## Authentication

### Option 1: Bearer Token (JWT)

```bash
# Get token via signin
TOKEN=$(curl -s -X POST http://localhost:6336/action/user_account/signin \
  -H "Content-Type: application/json" \
  -d '{"attributes":{"email":"admin@admin.com","password":"adminadmin"}}' | \
  jq -r '.[] | select(.ResponseType == "client.store.set") | .Attributes.value')

# Use token in requests
curl -X PROPFIND "http://localhost:6336/caldav/" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Depth: 0" \
  -H "Content-Type: application/xml" \
  -d '<?xml version="1.0"?><propfind xmlns="DAV:"><prop><displayname/></prop></propfind>'
```

### Option 2: Basic Authentication

```bash
curl -X PROPFIND "http://localhost:6336/caldav/" \
  -u "admin@admin.com:adminadmin" \
  -H "Depth: 0" \
  -H "Content-Type: application/xml" \
  -d '<?xml version="1.0"?><propfind xmlns="DAV:"><prop><displayname/></prop></propfind>'
```

Unauthorized requests return HTTP 401 with `WWW-Authenticate: Basic realm='caldav'`.

---

## CalDAV Usage

### List Available Calendars

```bash
curl -X PROPFIND "http://localhost:6336/caldav/" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Depth: 1" \
  -H "Content-Type: application/xml" \
  -d '<?xml version="1.0" encoding="utf-8" ?>
<D:propfind xmlns:D="DAV:">
  <D:prop>
    <D:displayname/>
    <D:resourcetype/>
  </D:prop>
</D:propfind>'
```

**Response**: HTTP 207 Multi-Status with XML listing calendars

### Create a Calendar

```bash
curl -X MKCOL "http://localhost:6336/caldav/personal/" \
  -H "Authorization: Bearer $TOKEN"
```

**Response**: HTTP 201 Created

**Creates**: `./storage/caldav/personal/` directory

### Add a Calendar Event

```bash
curl -X PUT "http://localhost:6336/caldav/personal/event1.ics" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: text/calendar" \
  -d 'BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Daptin//CalDAV//EN
BEGIN:VEVENT
UID:event1@daptin.local
DTSTART:20260127T100000Z
DTEND:20260127T110000Z
SUMMARY:Team Meeting
DESCRIPTION:Weekly team sync
LOCATION:Conference Room A
END:VEVENT
END:VCALENDAR'
```

**Response**: HTTP 201 Created

**Creates**: `./storage/caldav/personal/event1.ics` file

### Retrieve an Event

```bash
curl -X GET "http://localhost:6336/caldav/personal/event1.ics" \
  -H "Authorization: Bearer $TOKEN"
```

**Response**: HTTP 200 OK with iCalendar content

### Update an Event

Use PUT to the same URL with updated content:

```bash
curl -X PUT "http://localhost:6336/caldav/personal/event1.ics" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: text/calendar" \
  -d 'BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Daptin//CalDAV//EN
BEGIN:VEVENT
UID:event1@daptin.local
DTSTART:20260127T140000Z
DTEND:20260127T150000Z
SUMMARY:Team Meeting - UPDATED
DESCRIPTION:Moved to afternoon
LOCATION:Conference Room B
END:VEVENT
END:VCALENDAR'
```

**Response**: HTTP 201 Created

### List Events in Calendar

```bash
curl -X PROPFIND "http://localhost:6336/caldav/personal/" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Depth: 1" \
  -H "Content-Type: application/xml" \
  -d '<?xml version="1.0" encoding="utf-8" ?>
<D:propfind xmlns:D="DAV:">
  <D:prop>
    <D:displayname/>
    <D:getcontenttype/>
    <D:getetag/>
  </D:prop>
</D:propfind>'
```

**Response**: HTTP 207 Multi-Status with list of events including ETags

### Delete an Event

```bash
curl -X DELETE "http://localhost:6336/caldav/personal/event1.ics" \
  -H "Authorization: Bearer $TOKEN"
```

**Response**: HTTP 204 No Content

**Effect**: Deletes `./storage/caldav/personal/event1.ics`

### Copy an Event

```bash
curl -X COPY "http://localhost:6336/caldav/personal/event1.ics" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Destination: /caldav/personal/event1-copy.ics"
```

**Response**: HTTP 201 Created

### Move/Rename an Event

```bash
curl -X MOVE "http://localhost:6336/caldav/personal/event1.ics" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Destination: /caldav/personal/event1-renamed.ics"
```

**Response**: HTTP 201 Created

---

## CardDAV Usage

CardDAV works identically to CalDAV but with `/carddav/` endpoints and vCard format.

### Create an Address Book

```bash
curl -X MKCOL "http://localhost:6336/carddav/contacts/" \
  -H "Authorization: Bearer $TOKEN"
```

**Response**: HTTP 201 Created

**Creates**: `./storage/carddav/contacts/` directory

### Add a Contact

```bash
curl -X PUT "http://localhost:6336/carddav/contacts/john-doe.vcf" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: text/vcard" \
  -d 'BEGIN:VCARD
VERSION:3.0
FN:John Doe
N:Doe;John;;;
EMAIL;TYPE=INTERNET:john.doe@example.com
TEL;TYPE=CELL:+1-555-1234
ORG:Acme Corp
TITLE:Software Engineer
END:VCARD'
```

**Response**: HTTP 201 Created

**Creates**: `./storage/carddav/contacts/john-doe.vcf` file

### Retrieve a Contact

```bash
curl -X GET "http://localhost:6336/carddav/contacts/john-doe.vcf" \
  -H "Authorization: Bearer $TOKEN"
```

**Response**: HTTP 200 OK with vCard content

### List Contacts

```bash
curl -X PROPFIND "http://localhost:6336/carddav/contacts/" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Depth: 1" \
  -H "Content-Type: application/xml" \
  -d '<?xml version="1.0" encoding="utf-8" ?>
<D:propfind xmlns:D="DAV:">
  <D:prop>
    <D:displayname/>
    <D:getcontenttype/>
  </D:prop>
</D:propfind>'
```

**Response**: HTTP 207 Multi-Status with list of contacts

---

## Client Compatibility

Since Daptin implements basic WebDAV (not full CalDAV/CardDAV), compatibility with clients varies:

### May Work

Clients that primarily use WebDAV methods for file sync:
- Manual sync tools using WebDAV
- Simple calendar apps that store .ics files
- Custom scripts using curl/WebDAV libraries

### May Not Work

Clients requiring full CalDAV/CardDAV protocol:
- Apple Calendar (requires calendar-query)
- Thunderbird Lightning (expects REPORT method)
- Evolution (requires scheduling extensions)
- Most mobile calendar apps (expect CalDAV queries)

**Workaround**: Use Daptin as file storage backend and sync .ics/.vcf files manually or via WebDAV-only clients.

---

## File Storage Structure

```
./storage/
├── caldav/
│   ├── personal/
│   │   ├── event1.ics
│   │   └── event2.ics
│   └── work/
│       └── meeting.ics
└── carddav/
    ├── contacts/
    │   ├── john-doe.vcf
    │   └── jane-smith.vcf
    └── family/
        └── mom.vcf
```

- Collections (calendars/address books) = directories
- Events/contacts = .ics/.vcf files
- File names can be anything with appropriate extension

---

## Troubleshooting

### "404 Not Found: stat storage/caldav: no such file or directory"

**Cause**: Storage directories don't exist

**Solution**:
```bash
mkdir -p ./storage/caldav ./storage/carddav
```

### "401 Unauthorized"

**Cause**: Missing or invalid authentication

**Solution**:
- Verify token is valid (not expired)
- Use Bearer token or Basic auth correctly
- Check user has permission to access resources

### "400 Bad Request: webdav: expected application/xml request"

**Cause**: PROPFIND request missing XML body or Content-Type header

**Solution**:
```bash
curl -X PROPFIND "http://localhost:6336/caldav/" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/xml" \
  -d '<?xml version="1.0"?><propfind xmlns="DAV:"><prop><displayname/></prop></propfind>'
```

### CalDAV Not Responding After Enable

**Cause**: Server restart required after config change

**Solution**:
```bash
pkill daptin
go run main.go
```

### Client Says "Server Does Not Support CalDAV"

**Cause**: Client checking for CalDAV-specific features (REPORT, calendar-query) that Daptin doesn't implement

**Solution**: Use WebDAV-only client or manual file sync instead

---

## Implementation Details

### Code Location

- **Endpoint**: `server/endpoint_caldav.go`
- **Configuration**: `server/server.go` lines 387-436
- **Backend**: Uses `github.com/emersion/go-webdav` library
- **File System**: `webdav.LocalFileSystem("./storage")`

### Supported Methods

All standard WebDAV methods are registered for both `/caldav/*` and `/carddav/*`:

```go
OPTIONS, HEAD, GET, POST, PUT, PATCH, PROPFIND, DELETE, COPY, MOVE, MKCOL, PROPPATCH
```

### Authentication Flow

1. Request arrives at CalDAV/CardDAV endpoint
2. `authMiddleware.AuthCheckMiddlewareWithHttp()` validates token/credentials
3. If unauthorized: HTTP 401 with `WWW-Authenticate: Basic realm='caldav'`
4. If authorized: Request forwarded to WebDAV handler

### Storage Backend

Currently uses local file system only. The code has a commented-out line suggesting database-backed storage was considered:

```go
//caldavStorage, err := resource.NewCaldavStorage(cruds, certificateManager)
```

This functionality is not implemented in the current version.

---

## Limitations

1. **No full CalDAV/CardDAV protocol**: Missing REPORT, calendar-query, etc.
2. **No database storage**: Files only, no integration with Daptin tables
3. **No scheduling extensions**: No free/busy, no meeting invitations
4. **No sync tokens**: Clients can't efficiently detect changes
5. **File system only**: No cloud storage backend support
6. **No multi-user isolation**: All users share the same `./storage/` directory
7. **No calendar metadata**: Can't set calendar colors, descriptions, etc.

---

## Use Cases

### Good For

✅ Simple calendar/contact file storage
✅ Manual sync of .ics/.vcf files
✅ WebDAV-based backup of calendar data
✅ Custom scripts needing calendar file access
✅ Testing CalDAV client implementations

### Not Suitable For

❌ Production calendar/contact server for standard clients
❌ Multi-user calendar sharing with permissions
❌ Calendar scheduling and free/busy queries
❌ Mobile app sync (most expect full CalDAV)
❌ Outlook/Thunderbird/Apple Calendar integration

---

## Future Enhancements

Potential improvements (not currently implemented):

1. Full CalDAV/CardDAV protocol support (REPORT method, queries)
2. Database-backed storage with Daptin tables
3. Per-user calendar/address book isolation
4. Calendar sharing with permissions
5. Scheduling extensions (free/busy, invitations)
6. Sync tokens for efficient change detection
7. Cloud storage backend support
8. Integration with Daptin user/group system

---

## See Also

- [Cloud Storage](Cloud-Storage.md) - File storage backends
- [Authentication](Authentication.md) - User authentication methods
- [Permissions](Permissions.md) - Access control system
