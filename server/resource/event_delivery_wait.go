package resource

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/daptin/daptin/server/database"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/doug-martin/goqu/v9"
)

type eventDeliveryWaiterKey struct{}

// EventDeliveryWaiter belongs to one request. The database queue remains the
// authority for delivery state; this only holds the IDs that request awaits.
type EventDeliveryWaiter struct {
	mu  sync.Mutex
	ids []daptinid.DaptinReferenceId
}

func WithEventDeliveryWaiter(ctx context.Context, waiter *EventDeliveryWaiter) context.Context {
	return context.WithValue(ctx, eventDeliveryWaiterKey{}, waiter)
}

func EventDeliveryWaiterFromContext(ctx context.Context) *EventDeliveryWaiter {
	waiter, _ := ctx.Value(eventDeliveryWaiterKey{}).(*EventDeliveryWaiter)
	return waiter
}

func (waiter *EventDeliveryWaiter) Add(id daptinid.DaptinReferenceId) {
	waiter.mu.Lock()
	waiter.ids = append(waiter.ids, id)
	waiter.mu.Unlock()
}

func (waiter *EventDeliveryWaiter) Wait(ctx context.Context, db database.DatabaseConnection) error {
	waiter.mu.Lock()
	ids := append([]daptinid.DaptinReferenceId(nil), waiter.ids...)
	waiter.mu.Unlock()
	if len(ids) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for len(ids) > 0 {
		remaining := ids[:0]
		for _, id := range ids {
			query, args, err := statementbuilder.Squirrel.Select("state").Prepared(true).
				From(EXCHANGE_RUN_TABLE_NAME).Where(goqu.Ex{"reference_id": id[:]}).ToSQL()
			if err != nil {
				return err
			}
			var state string
			if err := db.Get(&state, query, args...); err != nil {
				return fmt.Errorf("event delivery [%s]: %w", id, err)
			}
			switch state {
			case exchangeExecutionSucceeded:
			case exchangeExecutionTerminalFailed:
				return fmt.Errorf("event delivery [%s] failed", id)
			default:
				remaining = append(remaining, id)
			}
		}
		ids = remaining
		if len(ids) == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("event delivery wait: %w", ctx.Err())
		case <-ticker.C:
		}
	}
	return nil
}
