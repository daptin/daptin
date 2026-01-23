# CalDAV and CardDAV

Calendar and contact synchronization protocols.

## Overview

| Protocol | Purpose | Port |
|----------|---------|------|
| CalDAV | Calendar sync | 8008 |
| CardDAV | Contact sync | 8008 |

## Enable CalDAV

```bash
curl -X POST http://localhost:6336/_config/backend/caldav.enable \
  -H "Authorization: Bearer $TOKEN" \
  -d 'true'
```

## Endpoints

| Endpoint | Description |
|----------|-------------|
| `/caldav/` | CalDAV root |
| `/caldav/{user}/` | User calendars |
| `/caldav/{user}/{calendar}/` | Specific calendar |
| `/carddav/` | CardDAV root |
| `/carddav/{user}/` | User contacts |

## Calendar Table

Daptin uses `calendar` table with iCalendar (RFC 5545) format:

```bash
curl -X POST http://localhost:6336/api/calendar \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/vnd.api+json" \
  -d '{
    "data": {
      "type": "calendar",
      "attributes": {
        "summary": "Team Meeting",
        "description": "Weekly sync",
        "start_time": "2024-01-15T10:00:00Z",
        "end_time": "2024-01-15T11:00:00Z",
        "location": "Conference Room A",
        "ical_data": "BEGIN:VCALENDAR..."
      }
    }
  }'
```

## CalDAV Client Configuration

### macOS Calendar

1. Open Calendar → Preferences → Accounts
2. Add CalDAV account
3. Server: `http://localhost:8008/caldav/`
4. Username: Your Daptin email
5. Password: Your Daptin password

### iOS Calendar

1. Settings → Calendar → Accounts → Add Account
2. Choose "Other" → Add CalDAV Account
3. Server: `http://your-server:8008/caldav/`
4. Username/Password: Daptin credentials

### Thunderbird (Lightning)

1. File → New → Calendar
2. Select "On the Network"
3. Format: CalDAV
4. Location: `http://localhost:8008/caldav/user@example.com/`

## CardDAV Client Configuration

### macOS Contacts

1. Contacts → Preferences → Accounts
2. Add CardDAV account
3. Server: `http://localhost:8008/carddav/`

### iOS Contacts

1. Settings → Contacts → Accounts
2. Add Account → Other → Add CardDAV Account

## iCalendar Format

Events stored in iCal format:

```
BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//Daptin//Calendar//EN
BEGIN:VEVENT
UID:event-123@daptin
DTSTAMP:20240115T100000Z
DTSTART:20240115T100000Z
DTEND:20240115T110000Z
SUMMARY:Team Meeting
DESCRIPTION:Weekly team sync
LOCATION:Conference Room A
END:VEVENT
END:VCALENDAR
```

## vCard Format

Contacts in vCard format:

```
BEGIN:VCARD
VERSION:3.0
FN:John Doe
N:Doe;John;;;
EMAIL:john@example.com
TEL;TYPE=WORK:+1-555-0123
ORG:Acme Inc
END:VCARD
```

## Authentication

CalDAV/CardDAV use HTTP Basic Auth:

```bash
curl http://localhost:8008/caldav/user@example.com/ \
  -u "user@example.com:password"
```

## WebDAV Operations

### PROPFIND (List)

```bash
curl -X PROPFIND http://localhost:8008/caldav/user@example.com/ \
  -u "user@example.com:password" \
  -H "Depth: 1" \
  -H "Content-Type: application/xml" \
  -d '<?xml version="1.0"?>
<propfind xmlns="DAV:">
  <prop>
    <displayname/>
    <resourcetype/>
  </prop>
</propfind>'
```

### PUT (Create/Update)

```bash
curl -X PUT http://localhost:8008/caldav/user@example.com/calendar/event.ics \
  -u "user@example.com:password" \
  -H "Content-Type: text/calendar" \
  -d 'BEGIN:VCALENDAR...'
```

### DELETE

```bash
curl -X DELETE http://localhost:8008/caldav/user@example.com/calendar/event.ics \
  -u "user@example.com:password"
```

## Sync Behavior

- Full sync on initial connect
- Incremental sync via sync-token
- Conflict resolution: Last-write wins
- Supports recurring events

## Limitations

- Single calendar per user (default)
- No calendar sharing (use permissions)
- Basic recurrence support
