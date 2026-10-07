package resource

import (
	"testing"

	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/google/uuid"
)

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
