package server

import (
	"context"
	"errors"
	"time"

	"github.com/buraksezer/olric"
	"github.com/daptin/daptin/server/resource"
	log "github.com/sirupsen/logrus"
)

const restartTopic = "runtime-restart"

// Every node observes the same committed SQL marker. PubSub reduces latency;
// polling also observes requests committed after their notification was sent.
func monitorRuntimeRestart(ctx context.Context, state *resource.RuntimeRestart, pubsub *olric.PubSub, requests chan<- struct{}) (context.CancelFunc, <-chan struct{}) {
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		subscription := pubsub.Subscribe(ctx, restartTopic)
		defer subscription.Close()
		events := subscription.Channel()
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			checkCtx, stop := context.WithTimeout(ctx, time.Second)
			err := state.Check(checkCtx)
			stop()
			if errors.Is(err, resource.ErrRestartPending) {
				select {
				case requests <- struct{}{}:
				case <-ctx.Done():
				}
				return
			}
			if err != nil && ctx.Err() == nil {
				log.WithError(err).Error("check runtime restart request")
			}
			select {
			case <-ctx.Done():
				return
			case _, open := <-events:
				if !open {
					events = nil
				}
			case <-ticker.C:
			}
		}
	}()
	return cancel, done
}
