package agent

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestQuiesceYieldsWithoutCancellingCurrentOperation(t *testing.T) {
	var cancelled atomic.Bool
	s := &Service{quiesced: make(chan struct{}), done: make(chan struct{}), cancel: func() { cancelled.Store(true) }}
	s.Quiesce()
	s.Quiesce()
	if !s.stopping.Load() || cancelled.Load() {
		t.Fatal("quiesce must stop new work without cancelling current operation")
	}
	select {
	case <-s.quiesced:
	default:
		t.Fatal("dispatcher not stopped")
	}
	close(s.done)
	if err := s.Drain(context.Background()); err != nil || cancelled.Load() {
		t.Fatal("graceful drain cancelled work", err)
	}
}
func TestDrainDeadlineCancelsCurrentOperation(t *testing.T) {
	var cancelled atomic.Bool
	s := &Service{quiesced: make(chan struct{}), done: make(chan struct{}), cancel: func() { cancelled.Store(true) }}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	if err := s.Drain(ctx); err == nil || !cancelled.Load() {
		t.Fatal("drain deadline did not cancel work")
	}
}
