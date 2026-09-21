package resource

import (
	"testing"

	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/google/uuid"
)

func TestDAVPathsAreScopedToAuthenticatedPrincipal(t *testing.T) {
	referenceID := daptinid.DaptinReferenceId(uuid.MustParse("11111111-1111-1111-1111-111111111111"))
	backend := NewCalDAVBackend(nil, &auth.SessionUser{UserReferenceId: referenceID})

	name, err := backend.collectionName("/caldav/11111111-1111-1111-1111-111111111111/calendars/personal/", false)
	if err != nil || name != "personal" {
		t.Fatalf("own calendar path = %q, %v", name, err)
	}
	if _, err := backend.collectionName("/caldav/22222222-2222-2222-2222-222222222222/calendars/personal/", false); err == nil {
		t.Fatal("cross-principal calendar path was accepted")
	}
	if _, err := backend.collectionName("/caldav/11111111-1111-1111-1111-111111111111/calendars/personal/event.ics", true); err != nil {
		t.Fatalf("own calendar object path was rejected: %v", err)
	}
}
