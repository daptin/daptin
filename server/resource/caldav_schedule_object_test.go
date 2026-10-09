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

func TestCalDAVAttendeeMayReorderAttendeesWhenReplying(t *testing.T) {
	const previous = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:meeting-1\r\nDTSTAMP:20261009T000000Z\r\nORGANIZER:mailto:organizer@example.test\r\nATTENDEE;PARTSTAT=NEEDS-ACTION:mailto:local@example.test\r\nATTENDEE;PARTSTAT=NEEDS-ACTION:mailto:attendee@example.test\r\nATTENDEE;PARTSTAT=NEEDS-ACTION:mailto:other@example.test\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	updated := strings.Replace(previous,
		"ATTENDEE;PARTSTAT=NEEDS-ACTION:mailto:attendee@example.test\r\nATTENDEE;PARTSTAT=NEEDS-ACTION:mailto:other@example.test",
		"ATTENDEE;PARTSTAT=NEEDS-ACTION:mailto:other@example.test\r\nATTENDEE;PARTSTAT=ACCEPTED:mailto:attendee@example.test", 1)
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
		t.Fatalf("attendee reply with reordered attendees: allowed=%v err=%v", allowed, err)
	}
	forged := strings.Replace(updated, "PARTSTAT=NEEDS-ACTION:mailto:other@example.test", "PARTSTAT=ACCEPTED:mailto:other@example.test", 1)
	allowed, err = itipAllowedAttendeeChange(decodeEvent(previous), decodeEvent(forged), "attendee@example.test")
	if err != nil || allowed {
		t.Fatalf("another attendee's status changed: allowed=%v err=%v", allowed, err)
	}
}

func TestITIPReplyStatus(t *testing.T) {
	for _, test := range []struct {
		values []string
		want   string
		valid  bool
	}{
		{nil, "2.0", true},
		{[]string{"2.0;Success", "2.4;Success;Extra"}, "2.0,2.4", true},
		{[]string{"2.100;Success"}, "2.100", true},
		{[]string{"bad;Failure"}, "", false},
	} {
		event := ical.NewComponent(ical.CompEvent)
		for _, value := range test.values {
			event.Props.Add(&ical.Prop{Name: "REQUEST-STATUS", Value: value})
		}
		got, err := itipReplyStatus(ical.Event{Component: event})
		if got != test.want || (err == nil) != test.valid {
			t.Fatalf("REQUEST-STATUS %v: got %q, err %v; want %q, valid %v", test.values, got, err, test.want, test.valid)
		}
	}
}

func TestITIPMaterialChangeIgnoresDeliveryState(t *testing.T) {
	const old = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:meeting\r\nDTSTAMP:20261008T000000Z\r\nDTSTART:20261012T120000Z\r\nATTENDEE;PARTSTAT=NEEDS-ACTION;SCHEDULE-STATUS=1.0:mailto:guest@example.test\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	decode := func(data string) ical.Event {
		t.Helper()
		calendar, err := ical.NewDecoder(strings.NewReader(data)).Decode()
		if err != nil {
			t.Fatal(err)
		}
		return calendar.Events()[0]
	}
	updated := strings.Replace(old, "DTSTAMP:20261008T000000Z", "DTSTAMP:20261009T000000Z", 1)
	updated = strings.Replace(updated, "PARTSTAT=NEEDS-ACTION;SCHEDULE-STATUS=1.0", "PARTSTAT=ACCEPTED;SCHEDULE-STATUS=2.0", 1)
	changed, err := itipMaterialChange(decode(old), decode(updated))
	if err != nil || changed {
		t.Fatalf("delivery state retriggered invitation: changed=%v err=%v", changed, err)
	}
	updated = strings.Replace(updated, "DTSTART:20261012T120000Z", "DTSTART:20261012T130000Z", 1)
	changed, err = itipMaterialChange(decode(old), decode(updated))
	if err != nil || !changed {
		t.Fatalf("time change was ignored: changed=%v err=%v", changed, err)
	}
}

func TestReconcileScheduleStatusTargetsOnlyPendingRecipient(t *testing.T) {
	const source = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:meeting\r\nRECURRENCE-ID:20261012T120000Z\r\nDTSTART:20261012T120000Z\r\nATTENDEE;SCHEDULE-STATUS=1.0:mailto:guest@example.test\r\nATTENDEE;SCHEDULE-STATUS=2.0:mailto:responded@example.test\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	calendar, err := ical.NewDecoder(strings.NewReader(source)).Decode()
	if err != nil {
		t.Fatal(err)
	}
	if reconcileScheduleStatus(calendar, "meeting", "", "guest@example.test", "1.1") {
		t.Fatal("unrelated recurrence changed")
	}
	if !reconcileScheduleStatus(calendar, "meeting", "20261012T120000Z", "guest@example.test", "1.1") {
		t.Fatal("pending recipient was not reconciled")
	}
	if reconcileScheduleStatus(calendar, "meeting", "20261012T120000Z", "responded@example.test", "5.1") {
		t.Fatal("reply status was overwritten")
	}
	attendees := calendar.Events()[0].Props.Values("ATTENDEE")
	if attendees[0].Params.Get("SCHEDULE-STATUS") != "1.1" || attendees[1].Params.Get("SCHEDULE-STATUS") != "2.0" {
		t.Fatalf("wrong recipient statuses: %v", attendees)
	}
}

func TestOutboxScheduleStatus(t *testing.T) {
	for _, test := range []struct {
		outbox map[string]interface{}
		status string
		state  string
	}{
		{map[string]interface{}{"sent": false, "retry_count": int64(4)}, "", ""},
		{map[string]interface{}{"sent": true, "retry_count": int64(0)}, "1.1", "sent"},
		{map[string]interface{}{"sent": int64(1), "retry_count": int64(0)}, "1.1", "sent"},
		{map[string]interface{}{"sent": false, "retry_count": int64(5)}, "5.1", "failed"},
	} {
		status, state := outboxScheduleStatus(test.outbox)
		if status != test.status || state != test.state {
			t.Fatalf("outbox %v: status %q, state %q", test.outbox, status, state)
		}
	}
}
