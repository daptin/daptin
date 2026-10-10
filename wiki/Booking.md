# Public appointment booking

Daptin exposes a public booking contract through two resources: `bookable` describes an owner's offer, and `booking` records a confirmed reservation. Every booking belongs to one bookable and one calendar event. The offer uses ordinary Daptin calendar relationships and permissions. Guests use named actions and do not need Daptin accounts.

## Configure an offer

An administrator grants the offer owner the normal `bookable` and `booking` table rights and calendar rights. The owner needs update access to the `bookable` table and row, plus refer access when adding a required calendar; create and refer access to the destination collection; and create and read access to the calendar table. Every required source collection needs peek or read and refer access. A different host may share a collection with the owner using the calendar sharing action. Event rows in that collection must also be peekable or readable by the offer owner. Missing or revoked grants make availability unavailable rather than exposing or ignoring events.

Create a `bookable` through `/api/bookable` with these attributes:

```json
{
  "title": "Consultation",
  "published": true,
  "event_class": "PRIVATE",
  "destination_collection_id": "COLLECTION_REFERENCE_ID",
  "time_zone": "America/New_York",
  "weekly_hours": "{\"mon\":[{\"start\":\"09:00\",\"end\":\"17:00\"}]}",
  "duration_minutes": 30,
  "increment_minutes": 30,
  "buffer_before_minutes": 10,
  "buffer_after_minutes": 10,
  "minimum_notice_minutes": 60,
  "horizon_days": 30,
  "capacity": 1,
  "daily_limit": 0,
  "weekly_limit": 0
}
```

`weekly_hours` is JSON keyed by lowercase three-letter weekday names. Each day contains one or more local start/end pairs. Times that do not exist or occur twice at a daylight-saving transition are not offered. Limits of zero mean no daily or weekly count limit. A bookable can include optional `questions` JSON for an external booking UI; Daptin does not render that UI.

`event_class` accepts `PRIVATE`, `PUBLIC`, or `CONFIDENTIAL` and sets the iCalendar `CLASS` property on new booking events. Calendar table and row permissions still control access. Optional questions use objects such as `{"id":"topic","label":"Topic","required":true}`; reservation sends `answers` as a map from question ID to text.

Add each required source calendar with `POST /action/bookable/set_calendar` as the offer owner:

```json
{"bookable_ref":"BOOKABLE_REFERENCE_ID","calendar_ref":"COLLECTION_REFERENCE_ID","enabled":true}
```

Use `enabled:false` to remove a source calendar. Include the destination collection if its existing events should block slots. The action changes the ordinary `bookable` to `collection` relationship; the owner can only add collections they may peek or read and refer to. An offer without a required calendar cannot serve slots. Set `published` to false to stop new public reservations. Existing guests can still check or cancel their bookings.

## Public actions

An anonymous caller lists slots for one local date:

```http
POST /action/bookable/slots
Content-Type: application/json

{"bookable_ref":"BOOKABLE_REFERENCE_ID","date":"2026-10-12"}
```

The response contains the title, time zone, duration, configured questions, and available starts as RFC 3339 instants. It contains no source calendar reference, event body, busy interval, attendee, or credential. A failed or oversized availability check does not offer a slot.

Reserve a start from that list with a fresh UUIDv4 attempt key:

```http
POST /action/bookable/reserve
Content-Type: application/json

{"bookable_ref":"BOOKABLE_REFERENCE_ID","start":"2026-10-12T13:00:00Z","guest_name":"Guest Name","guest_email":"guest@example.net","attempt_key":"4ac67502-6e5f-487a-8d13-e182910fc05c"}
```

The action locks the offer and all required collections, checks availability again, and writes one booking and its calendar event in the same database transaction. A taken slot returns HTTP 409. Repeating the same attempt key and details returns the same booking reference and guest token, including after a server restart or after the offer is unpublished. Reusing the key with different details returns HTTP 409. Two guests racing for the final place cannot both confirm.

Use the returned `booking_ref` and `token` with `POST /action/bookable/status`, `/cancel`, or `/reschedule`. Reschedule also takes a new `start`. The offer owner may use these actions while signed in without the guest token. The token grants access to that booking only. Status reports the booking state, whether the event's collection currently has a mail account, and counts of the existing calendar-mail delivery states. Keep the token private.

The destination collection's optional `scheduling_mail_account` relationship controls invitations and cancellations through the existing CalDAV scheduling path. Booking works without a connected mail account; then no email invitation is queued. A mail account remains an independent resource and has no booking-specific field.

DAV clients may read booking events according to their normal grants. They cannot directly change, move, copy, overwrite, or delete a booking-linked event; cancellation and rescheduling use the booking actions. Direct JSON:API writes to resources are ordinary resource operations and do not invoke the booking protocol.
