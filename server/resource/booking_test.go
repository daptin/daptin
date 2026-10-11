package resource

import (
	encodingjson "encoding/json"
	"strings"
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

func TestBookingDateOverrides(t *testing.T) {
	base := map[string]interface{}{
		"time_zone": "America/New_York", "weekly_hours": `{"sun":[{"start":"09:00","end":"17:00"}]}`,
		"duration_minutes": int64(30), "increment_minutes": int64(30),
		"buffer_before_minutes": int64(0), "buffer_after_minutes": int64(0),
		"minimum_notice_minutes": int64(0), "horizon_days": int64(366),
		"capacity": int64(1), "daily_limit": int64(0), "weekly_limit": int64(0),
		"date_overrides": `{"2026-03-08":{"hours":[{"start":"02:00","end":"04:00"}]},"2026-03-09":{"closed":true},"2026-11-01":{"hours":[{"start":"01:00","end":"04:00"}]}}`,
	}
	policy, err := parseBookingPolicy(base)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		date string
		want []string
	}{
		{"2026-03-08", []string{"03:00", "03:30"}},
		{"2026-03-09", nil},
		{"2026-11-01", []string{"02:00", "02:30", "03:00", "03:30"}},
	} {
		day, err := bookingDay(test.date, policy.zone)
		if err != nil {
			t.Fatal(err)
		}
		windows := policy.windows(day)
		var got []string
		for _, window := range windows {
			for _, start := range bookingWindowStarts(day, window, policy) {
				got = append(got, start.In(policy.zone).Format("15:04"))
			}
		}
		if len(got) != len(test.want) {
			t.Fatalf("%s starts = %v, want %v", test.date, got, test.want)
		}
		for i := range got {
			if got[i] != test.want[i] {
				t.Fatalf("%s starts = %v, want %v", test.date, got, test.want)
			}
		}
	}
	weeklyDay, err := bookingDay("2026-03-15", policy.zone)
	if err != nil {
		t.Fatal(err)
	}
	weekly := policy.windows(weeklyDay)
	if len(weekly) != 1 || weekly[0].Start != "09:00" || weekly[0].End != "17:00" {
		t.Fatalf("weekly hours were not used without a date override: %v", weekly)
	}
	for _, raw := range []string{
		`{"2026-02-30":{"closed":true}}`,
		`{"2026-03-09":{"closed":false}}`,
		`{"2026-03-09":{"closed":true,"hours":[{"start":"09:00","end":"10:00"}]}}`,
		`{"2026-03-09":{"hours":[]}}`,
		`{"2026-03-09":{"hours":[{"start":"10:00","end":"11:00"},{"start":"10:30","end":"12:00"}]}}`,
		`{"2026-03-09":{"hours":[{"start":"10:00","end":"09:00"}]}}`,
		`{"2026-03-09":{"hours":[{"start":"9:00","end":"10:00"}]}}`,
		`{"2026-03-09":{"hours":[{"start":"09:00","end":"10:00","extra":true}]}}`,
		`{"2026-03-09":{"unknown":true}}`,
		`null`,
		strings.Repeat("x", 32769),
	} {
		base["date_overrides"] = raw
		if _, err := parseBookingPolicy(base); err == nil {
			t.Fatalf("invalid date overrides accepted: %.100s", raw)
		}
	}
	tooMany := make(map[string]interface{})
	first := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 367; i++ {
		tooMany[first.AddDate(0, 0, i).Format("2006-01-02")] = map[string]bool{"closed": true}
	}
	encoded, err := encodingjson.Marshal(tooMany)
	if err != nil {
		t.Fatal(err)
	}
	base["date_overrides"] = string(encoded)
	if _, err := parseBookingPolicy(base); err == nil {
		t.Fatal("more than 366 date overrides accepted")
	}
}
