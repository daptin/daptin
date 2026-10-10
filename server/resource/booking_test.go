package resource

import (
	"testing"
	"time"
)

func TestBookingLocalTimeRejectsDSTGapAndOverlap(t *testing.T) {
	zone, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		day, clock string
		valid      bool
	}{
		{"2026-03-08", "01:30", true},
		{"2026-03-08", "02:30", false},
		{"2026-03-08", "03:30", true},
		{"2026-11-01", "01:30", false},
		{"2026-11-01", "02:30", true},
	} {
		day, err := bookingDay(test.day, zone)
		if err != nil {
			t.Fatal(err)
		}
		_, valid := bookingLocalTime(day, test.clock, zone)
		if valid != test.valid {
			t.Errorf("%s %s: valid=%v, want %v", test.day, test.clock, valid, test.valid)
		}
	}
}

func TestBookingWindowKeepsValidSlotsAfterDSTBoundary(t *testing.T) {
	zone, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	policy := &bookingPolicy{zone: zone, duration: 30, increment: 30}
	for _, test := range []struct {
		day, opening, closing string
		want                  []string
	}{
		{"2026-03-08", "02:00", "04:00", []string{"03:00", "03:30"}},
		{"2026-11-01", "01:00", "04:00", []string{"02:00", "02:30", "03:00", "03:30"}},
	} {
		day, err := bookingDay(test.day, zone)
		if err != nil {
			t.Fatal(err)
		}
		starts := bookingWindowStarts(day, bookingWindow{Start: test.opening, End: test.closing}, policy)
		if len(starts) != len(test.want) {
			t.Fatalf("%s: got %d starts, want %d", test.day, len(starts), len(test.want))
		}
		for index, start := range starts {
			if got := start.In(zone).Format("15:04"); got != test.want[index] {
				t.Errorf("%s slot %d: got %s, want %s", test.day, index, got, test.want[index])
			}
		}
	}
}

func TestBookingPolicyRejectsUnboundedOrInvalidHours(t *testing.T) {
	base := map[string]interface{}{
		"time_zone": "UTC", "weekly_hours": `{"mon":[{"Start":"09:00","End":"17:00"}]}`,
		"duration_minutes": int64(30), "increment_minutes": int64(30),
		"buffer_before_minutes": int64(0), "buffer_after_minutes": int64(0),
		"minimum_notice_minutes": int64(0), "horizon_days": int64(30),
		"capacity": int64(1), "daily_limit": int64(0), "weekly_limit": int64(0),
	}
	if _, err := parseBookingPolicy(base); err != nil {
		t.Fatal(err)
	}
	base["horizon_days"] = int64(1000)
	if _, err := parseBookingPolicy(base); err == nil {
		t.Fatal("unbounded horizon was accepted")
	}
	base["horizon_days"] = int64(30)
	base["weekly_hours"] = `{"mon":[{"Start":"17:00","End":"09:00"}]}`
	if _, err := parseBookingPolicy(base); err == nil {
		t.Fatal("reversed hours were accepted")
	}
}
