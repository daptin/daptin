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

func TestCalendarBusyIntervalsHistoricalSeriesAndExpansionLimit(t *testing.T) {
	for _, test := range []struct {
		name, rule    string
		wantIntervals int
		wantError     bool
	}{
		{"ended history", "FREQ=DAILY;COUNT=3", 0, false},
		{"old ongoing series", "FREQ=DAILY", 2, false},
		{"dense ongoing series", "FREQ=MINUTELY", 0, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:old-series\r\nDTSTAMP:20200101T000000Z\r\nDTSTART:20200101T120000Z\r\nDTEND:20200101T130000Z\r\nRRULE:" + test.rule + "\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
			if test.wantError {
				input = strings.ReplaceAll(input, "20200101T", "20261124T")
			}
			calendar, err := ical.NewDecoder(strings.NewReader(input)).Decode()
			if err != nil {
				t.Fatal(err)
			}
			start := time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC)
			count := 0
			intervals, err := calendarBusyIntervals(calendar, start, start.Add(48*time.Hour), &count)
			if test.wantError {
				if err == nil || !strings.Contains(err.Error(), "too many recurring free/busy instances") {
					t.Fatalf("expected expansion limit, got %v", err)
				}
				return
			}
			if err != nil || len(intervals) != test.wantIntervals {
				t.Fatalf("intervals=%v err=%v", intervals, err)
			}
			for _, interval := range intervals {
				if interval.start.Before(start) || interval.end.After(start.Add(48*time.Hour)) {
					t.Fatalf("interval outside requested window: %v", interval)
				}
			}
		})
	}
}
