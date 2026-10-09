package resource

import (
	"strings"
	"testing"

	"github.com/emersion/go-ical"
)

func TestCalDAVAttendeeMayUpdateThunderbirdGeneration(t *testing.T) {
	const previous = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:meeting-1\r\nDTSTAMP:20261009T000000Z\r\nDTSTART:20261009T100000Z\r\nORGANIZER:mailto:organizer@example.test\r\nATTENDEE;PARTSTAT=NEEDS-ACTION:mailto:attendee@example.test\r\nSUMMARY:Meeting\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	updated := strings.Replace(previous, "PARTSTAT=NEEDS-ACTION", "PARTSTAT=ACCEPTED", 1)
	updated = strings.Replace(updated, "END:VEVENT", "X-MOZ-GENERATION:1\r\nEND:VEVENT", 1)
	decodeEvent := func(data string) ical.Event {
		t.Helper()
		calendar, err := ical.NewDecoder(strings.NewReader(data)).Decode()
		if err != nil {
			t.Fatal(err)
		}
		return calendar.Events()[0]
	}
	allowed, err := itipAllowedAttendeeChange(decodeEvent(previous), decodeEvent(updated), "attendee@example.test")
	if err != nil || !allowed {
		t.Fatalf("attendee RSVP with Thunderbird generation: allowed=%v err=%v", allowed, err)
	}
	forged := strings.Replace(updated, "END:VEVENT", "X-OWNER:forged\r\nEND:VEVENT", 1)
	allowed, err = itipAllowedAttendeeChange(decodeEvent(previous), decodeEvent(forged), "attendee@example.test")
	if err != nil || allowed {
		t.Fatalf("organizer-controlled extension accepted: allowed=%v err=%v", allowed, err)
	}
}
