package resource

import (
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-ical"
)

func TestCalendarBusyIntervalsRecurrenceAndVisibility(t *testing.T) {
	input := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\n" +
		"BEGIN:VEVENT\r\nUID:series\r\nDTSTAMP:20261001T000000Z\r\nDTSTART:20261006T120000Z\r\nDTEND:20261006T130000Z\r\nRRULE:FREQ=DAILY;COUNT=4\r\nEXDATE:20261007T120000Z\r\nEND:VEVENT\r\n" +
		"BEGIN:VEVENT\r\nUID:series\r\nDTSTAMP:20261001T000000Z\r\nRECURRENCE-ID:20261008T120000Z\r\nDTSTART:20261008T150000Z\r\nDTEND:20261008T160000Z\r\nSTATUS:TENTATIVE\r\nEND:VEVENT\r\n" +
		"BEGIN:VEVENT\r\nUID:transparent\r\nDTSTAMP:20261001T000000Z\r\nDTSTART:20261007T170000Z\r\nDTEND:20261007T180000Z\r\nTRANSP:TRANSPARENT\r\nEND:VEVENT\r\n" +
		"BEGIN:VEVENT\r\nUID:extra-period\r\nDTSTAMP:20261001T000000Z\r\nDTSTART:20261001T100000Z\r\nDTEND:20261001T110000Z\r\nRDATE;VALUE=PERIOD:20261007T160000Z/20261007T170000Z\r\nEND:VEVENT\r\n" +
		"END:VCALENDAR\r\n"
	calendar, err := ical.NewDecoder(strings.NewReader(input)).Decode()
	if err != nil {
		t.Fatal(err)
	}
	start, _ := time.Parse(calendarBusyLayout, "20261006T000000Z")
	end, _ := time.Parse(calendarBusyLayout, "20261010T000000Z")
	count := 0
	intervals, err := calendarBusyIntervals(calendar, start, end, &count)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"20261006T120000Z/20261006T130000Z/BUSY":           false,
		"20261008T150000Z/20261008T160000Z/BUSY-TENTATIVE": false,
		"20261009T120000Z/20261009T130000Z/BUSY":           false,
		"20261007T160000Z/20261007T170000Z/BUSY":           false,
	}
	if len(intervals) != len(want) {
		t.Fatalf("busy intervals = %#v; want %d", intervals, len(want))
	}
	for _, interval := range intervals {
		key := interval.start.UTC().Format(calendarBusyLayout) + "/" + interval.end.UTC().Format(calendarBusyLayout) + "/" + interval.kind
		if _, ok := want[key]; !ok {
			t.Fatalf("unexpected busy interval %s", key)
		}
		want[key] = true
	}
	for key, seen := range want {
		if !seen {
			t.Fatalf("missing busy interval %s", key)
		}
	}
}
