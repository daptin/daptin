package agent

import (
	"math"
	"testing"
	"time"
)

func TestDefinitionLimitsAllowLongWaitsAndRejectOverflow(t *testing.T) {
	c := Config{Model: "model", Limits: Limits{Seconds: 30 * 86400, ActiveSeconds: 3600, Tokens: 500000, CostMicros: 2000000}}
	if err := c.validate(); err != nil {
		t.Fatal(err)
	}
	cases := []Limits{{Seconds: -1}, {ActiveSeconds: math.MaxInt64}, {Tokens: -1}, {CostMicros: -1}, {ActiveSeconds: -1}}
	if int64(math.MaxInt) > math.MaxInt64/int64(time.Second) {
		cases = append(cases, Limits{Seconds: math.MaxInt})
	}
	for _, limits := range cases {
		invalid := Config{Model: "model", Limits: limits}
		if invalid.validate() == nil {
			t.Fatalf("accepted invalid bounds: %+v", limits)
		}
	}
	legacy := Config{Model: "model"}
	if err := legacy.validate(); err != nil || legacy.Limits.Seconds != 900 || legacy.Limits.ActiveSeconds != 0 || legacy.Limits.Tokens != 0 || legacy.Limits.CostMicros != 0 {
		t.Fatalf("defaults changed: %+v %v", legacy, err)
	}
}

func TestActiveTimeExcludesPersistedWaitsAndRetainsConsumedAllowance(t *testing.T) {
	start := time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC)
	state := runState{Started: start, Config: Config{Limits: Limits{Seconds: 30 * 86400, ActiveSeconds: 3600}}}
	// Five days queued or waiting consume wall lifetime, but no active time.
	now := start.Add(5 * 24 * time.Hour)
	remaining, code, err := state.remaining(now)
	if err != nil || remaining != time.Hour || code != "active_time_exceeded" {
		t.Fatalf("wait charged active time: %v %s %v", remaining, code, err)
	}
	state.ActiveSince = now
	state.finishActive(now.Add(10 * time.Minute))
	if state.ActiveMS != (10 * time.Minute).Milliseconds() {
		t.Fatal(state.ActiveMS)
	}
	// A checkpoint and another five-day wait cannot replenish those ten minutes.
	now = now.Add(5 * 24 * time.Hour)
	remaining, _, err = state.remaining(now)
	if err != nil || remaining != 50*time.Minute {
		t.Fatalf("allowance reset: %v %v", remaining, err)
	}
	state.ActiveSince = now
	_, _, err = state.remaining(now.Add(50 * time.Minute))
	if errorCode(err) != "active_time_exceeded" {
		t.Fatalf("active limit: %v", err)
	}
	state.ActiveSince = time.Time{}
	_, _, err = state.remaining(start.Add(30 * 24 * time.Hour))
	if errorCode(err) != "lifetime_exceeded" {
		t.Fatalf("absolute limit: %v", err)
	}
}
