package resource

import (
	"testing"
	"time"
)

func TestTaskSchedulerRegistersMeteringReservationRecovery(t *testing.T) {
	_, cruds, _ := newCanonicalMeteringDatabase(t)
	scheduler, err := NewTaskScheduler(cruds)
	if err != nil {
		t.Fatal(err)
	}
	entries := scheduler.cronService.Entries()
	if len(entries) != 1 {
		t.Fatalf("internal scheduler jobs = %d, want 1", len(entries))
	}
	now := time.Now()
	delay := entries[0].Schedule.Next(now).Sub(now)
	if delay <= 0 || delay > 10*time.Second {
		t.Fatalf("metering recovery interval = %s, want 0s < interval <= 10s", delay)
	}
}
