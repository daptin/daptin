package resource

import (
	"strings"
	"testing"

	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/google/uuid"
)

func TestLegacyScheduleTagIgnoresServerRSVPUpdates(t *testing.T) {
	before := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:legacy-meeting\r\nORGANIZER:mailto:owner@example.test\r\nATTENDEE;PARTSTAT=NEEDS-ACTION:mailto:guest@example.test\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	after := strings.Replace(before, "PARTSTAT=NEEDS-ACTION", "PARTSTAT=ACCEPTED", 1)
	tag := calendarScheduleTag(nil, []byte(before))
	if tag != calendarScheduleTag(nil, []byte(after)) {
		t.Fatal("server RSVP changed the fallback tag of a preexisting event")
	}
	if err := checkScheduleTagCondition(`"`+tag+`"`, true, nil, []byte(after)); err != nil {
		t.Fatalf("unchanged schedule tag rejected a client update: %v", err)
	}
}

func TestCalDAVPathIdentifiesOwnerWithoutGrantingAccess(t *testing.T) {
	referenceID := daptinid.DaptinReferenceId(uuid.MustParse("11111111-1111-1111-1111-111111111111"))
	backend := NewCalDAVBackend(nil, &auth.SessionUser{UserReferenceId: referenceID}, nil)

	owner, name, err := backend.calendarPath("/caldav/11111111-1111-1111-1111-111111111111/calendars/personal/", false)
	if err != nil || owner != referenceID || name != "personal" {
		t.Fatalf("own calendar path = %s, %q, %v", owner, name, err)
	}
	other := daptinid.DaptinReferenceId(uuid.MustParse("22222222-2222-2222-2222-222222222222"))
	owner, name, err = backend.calendarPath("/caldav/22222222-2222-2222-2222-222222222222/calendars/personal/", false)
	if err != nil || owner != other || name != "personal" {
		t.Fatalf("other principal's calendar path = %s, %q, %v", owner, name, err)
	}
	if _, _, err := backend.calendarPath("/caldav/11111111-1111-1111-1111-111111111111/calendars/personal/event.ics", true); err != nil {
		t.Fatalf("calendar object path was rejected: %v", err)
	}
	if _, _, err := backend.calendarPath("/carddav/11111111-1111-1111-1111-111111111111/addressbooks/contacts/", false); err == nil {
		t.Fatal("CardDAV path was accepted as a calendar")
	}
	if _, _, err := backend.calendarPath("/caldav/not-a-reference/calendars/personal/", false); err == nil {
		t.Fatal("invalid calendar principal was accepted")
	}
	if _, err := backend.calendarHomeOwner("/caldav/not-a-reference/calendars/"); err == nil {
		t.Fatal("invalid calendar home principal was accepted")
	}
}
